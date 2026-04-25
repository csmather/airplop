// `package main` declares this file as part of an executable program
// (as opposed to a library). Every .go file belongs to exactly one package,
// and the one named `main` with a `main()` function is what `go build`
// produces a binary from.
package main

// `import` pulls in other packages. Stdlib imports are bare paths
// ("net/http"); third-party imports look like URLs and resolve via go.mod.
// Imports inside parens is just the multi-line form of multiple imports.
import (
	"context"       // cancellation + deadlines plumbed through call stacks
	"embed"         // bundle files into the binary at compile time
	"encoding/json" // marshal/unmarshal JSON
	"errors"
	"fmt"           // formatted I/O (Printf-style)
	"html/template" // HTML templating with auto-escaping
	"log"
	"net"      // TCP/UDP/Interface primitives
	"net/http" // HTTP client + server
	"os/signal"
	"sync" // mutexes and other concurrency primitives
	"syscall"
	"time"

	"github.com/grandcat/zeroconf" // third-party mDNS lib
)

// `const` declares a compile-time constant. Untyped here — the type is
// inferred wherever it's used (e.g. as an int when passed to registerMDNS).
const port = 8765

// `//go:embed` is a compiler directive (NOT a normal comment — must be
// flush against the var below, no blank line). It tells `go build` to
// embed the listed files into the variable, which must be of type
// `string`, `[]byte`, or `embed.FS`.
//
//go:embed web/index.html
var webFS embed.FS // `var` declares a package-level variable.

// `type X struct { ... }` defines a new struct type. The backtick strings
// after each field are *struct tags* — metadata the json package reads
// to map struct fields to JSON keys.
type payload struct {
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

// hub coordinates SSE subscribers. The mutex serializes access to the
// map and `current` so multiple goroutines (one per HTTP handler) can't
// corrupt them.
type hub struct {
	mu sync.Mutex
	// `map[K]V` is Go's built-in hash map.
	// `chan []byte` is a typed channel — a thread-safe FIFO used for
	// goroutine-to-goroutine messaging.
	// `struct{}` is a zero-byte type; using it as the value gives us
	// "set" semantics (only the keys matter).
	subs    map[chan []byte]struct{}
	current []byte // []byte is a slice of bytes — Go's string-ish buffer type.
}

func newHub() *hub {
	// `:=` is short variable declaration: declare + assign + infer type
	// in one shot. Only valid inside functions.
	// `json.Marshal` returns (data, err) — Go's standard multi-return
	// pattern. `_` is the blank identifier: discard this value.
	empty, _ := json.Marshal(payload{Text: "", Timestamp: time.Now().Unix()})
	// `&T{...}` is a composite literal whose address we take, yielding
	// a *T (pointer). Returning *hub avoids copying the struct.
	return &hub{
		// `make` allocates and initializes built-in reference types
		// (maps, slices, channels). A nil map can't be written to.
		subs:    make(map[chan []byte]struct{}),
		current: empty,
	}
}

// `(h *hub)` is a *method receiver*: this attaches `add` as a method on
// *hub. Pointer receiver = method can mutate the underlying value.
func (h *hub) add(ch chan []byte) []byte {
	h.mu.Lock()
	// `defer` schedules a call to run when the surrounding function
	// returns — handy for unlock/close cleanup that should always happen.
	defer h.mu.Unlock()
	h.subs[ch] = struct{}{}
	return h.current
}

func (h *hub) remove(ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs, ch) // `delete` is the built-in for removing a map key.
	close(ch)          // `close` signals "no more sends"; receivers see ok=false.
}

// broadcast publishes `text` to every subscriber and returns the live
// subscriber count under the same lock (so the caller's log line is
// race-free).
func (h *hub) broadcast(text string) (int, error) {
	msg, err := json.Marshal(payload{Text: text, Timestamp: time.Now().Unix()})
	if err != nil {
		return 0, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.current = msg
	// `range` over a map yields keys (we don't need the value here).
	for ch := range h.subs {
		// `select` is like `switch`, but each case is a channel send/recv.
		// A `default` clause makes it non-blocking — fires if no other
		// case is ready right now.
		select {
		case ch <- msg: // `ch <- v` sends v on channel ch.
		default:
			// Subscriber's buffer is full → assume they're dead, drop them.
			// (Deleting the current key during a map range is allowed.)
			delete(h.subs, ch)
			close(ch)
		}
	}
	return len(h.subs), nil
}

// lanIP returns the LAN-facing IPv4 address and the interface that owns it.
//
// Trick: "connect" a UDP socket to a public IP — no packet is actually
// sent, but the kernel populates the socket's local end with the source IP
// it would route through (i.e. the interface bound to the default gateway).
// In WSL2 mirrored mode that's eth0 on the host's LAN IP. We then map the
// IP back to its interface so mDNS can pin its broadcasts to the same one.
func lanIP() (string, net.Interface, error) {
	conn, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		// `fmt.Errorf` with `%w` wraps an error — callers can use
		// `errors.Is`/`errors.As` to inspect the original.
		return "", net.Interface{}, fmt.Errorf("udp route probe: %w", err)
	}
	defer conn.Close()
	// Type assertion `x.(T)`: assert that interface value x holds type T.
	// Panics if wrong — use the comma-ok form below when you're not sure.
	local := conn.LocalAddr().(*net.UDPAddr)

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", net.Interface{}, err
	}
	// `range` over a slice yields (index, value). `_` ignores the index.
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			// Comma-ok type assertion: ok=false instead of panic on miss.
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if ipnet.IP.Equal(local.IP) {
				return local.IP.String(), iface, nil
			}
		}
	}
	return "", net.Interface{}, errors.New("no interface owns " + local.IP.String())
}

// registerMDNS advertises `airplop._http._tcp.local.` pinned to the given
// IP and interface (so zeroconf doesn't auto-pick the wrong one). Returns
// a shutdown func the caller should defer.
func registerMDNS(ip string, iface net.Interface, port int) (func(), error) {
	server, err := zeroconf.RegisterProxy(
		"airplop",    // instance name
		"_http._tcp", // service type
		"local.",     // domain
		port,
		"airplop",              // host → airplop.local
		[]string{ip},           // `[]T{...}` is a slice composite literal.
		[]string{"path=/"},     // TXT records
		[]net.Interface{iface}, // only send on this adapter
	)
	if err != nil {
		return nil, err
	}
	// Returning a *method value*: a function bound to its receiver,
	// callable later with no args.
	return server.Shutdown, nil
}

func (h *hub) streamHandler(w http.ResponseWriter, r *http.Request) {
	// Type assertion against an *interface*: does w also implement
	// http.Flusher? SSE needs flushing to push bytes immediately.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Buffered channel: capacity 10. Sends only block when the buffer is
	// full; that's how broadcast() detects a stuck subscriber.
	ch := make(chan []byte, 10)
	initial := h.add(ch)
	defer h.remove(ch)

	fmt.Fprintf(w, "data: %s\n\n", initial)
	flusher.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		// Comma-ok recv: ok=false when the channel is closed and drained.
		case msg, ok := <-ch:
			if !ok {
				return // hub dropped us (buffer overflow)
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-r.Context().Done(): // client disconnected
			return
		}
	}
}

func (h *hub) updateHandler(w http.ResponseWriter, r *http.Request) {
	// Anonymous struct: an inline type used just for unmarshalling.
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	subs, err := h.broadcast(body.Text)
	if err != nil {
		http.Error(w, "broadcast failed", http.StatusInternalServerError)
		return
	}
	log.Printf("update: %d bytes, %d subscribers", len(body.Text), subs)
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	ip, iface, err := lanIP()
	if err != nil {
		// log.Fatalf logs the message then calls os.Exit(1).
		log.Fatalf("could not detect LAN IP: %v", err)
	}
	log.Printf("LAN IP %s on interface %q (idx=%d)", ip, iface.Name, iface.Index)

	tmpl, err := template.ParseFS(webFS, "web/index.html")
	if err != nil {
		log.Fatalf("parse template: %v", err)
	}

	h := newHub()

	mux := http.NewServeMux()
	// Go 1.22+ servemux pattern syntax: METHOD plus path. "{$}" anchors
	// the match to *exactly* "/" (without it, "/" would match anything).
	// The handler is an anonymous function — closures capture `tmpl`/`ip`.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, struct{ LanIP string }{LanIP: ip})
	})
	// Method values: `h.streamHandler` is a function bound to h.
	mux.HandleFunc("GET /stream", h.streamHandler)
	mux.HandleFunc("POST /update", h.updateHandler)

	// We bind manually (instead of srv.ListenAndServe) so we can force
	// "tcp4" — IPv4 only. Avoids accidentally binding the IPv6 wildcard
	// in WSL2 networking edge cases.
	listener, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	srv := &http.Server{Handler: mux}

	shutdownMDNS, err := registerMDNS(ip, iface, port)
	if err != nil {
		log.Printf("mDNS registration failed (continuing): %v", err)
	} else {
		defer shutdownMDNS()
	}

	// signal.NotifyContext returns a context that's cancelled when one
	// of the listed signals fires. `defer stop()` releases the handler.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// `go f()` launches a goroutine — concurrent execution, very cheap
	// (a few KB of stack). The anonymous func + immediate call lets us
	// close over `srv` and `listener`.
	go func() {
		log.Printf("airplop listening on http://%s:%d and http://airplop.local:%d", ip, port, port)
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done() // `<-ch` receives from a channel; blocks until ready.
	log.Println("shutting down…")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
