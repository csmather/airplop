package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// struct tags drive (un)marshalling
type payload struct {
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

type hub struct {
	mu      sync.Mutex               // serializes access to the fields below
	subs    map[chan []byte]struct{} // map[K]V; struct{} value = "set" semantics
	current []byte
}

func newHub() *hub {
	empty, _ := json.Marshal(payload{Text: "", Timestamp: time.Now().Unix()}) // _ discards a return value
	return &hub{ // &T{} composite literal returning *hub
		subs:    make(map[chan []byte]struct{}), // make: allocate maps/slices/channels
		current: empty,
	}
}

// (h *hub) is a method receiver; pointer receiver = can mutate h
func (h *hub) add(ch chan []byte) []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subs[ch] = struct{}{}
	return h.current
}

func (h *hub) remove(ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs, ch) // built-in for removing a map key
	close(ch)          // signals "no more sends"; receivers see ok=false
}

// returns subscriber count under the same lock so callers can log without racing
func (h *hub) broadcast(text string) (int, error) {
	msg, err := json.Marshal(payload{Text: text, Timestamp: time.Now().Unix()})
	if err != nil {
		return 0, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.current = msg
	for ch := range h.subs { // range over a map yields keys
		select { // like switch, but each case is a channel op
		case ch <- msg: // ch <- v sends on the channel
		default: // non-blocking: fires if no other case is ready
			delete(h.subs, ch) // deleting current key during range is allowed
			close(ch)
		}
	}
	return len(h.subs), nil
}

func (h *hub) streamHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher) // type assertion against an interface
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := make(chan []byte, 10) // buffered channel: sends only block when full
	initial := h.add(ch)
	defer h.remove(ch)

	fmt.Fprintf(w, "data: %s\n\n", initial)
	flusher.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-ch: // comma-ok recv: ok=false when channel closed and drained
			if !ok {
				return
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
	var body struct { // anonymous struct: inline type just for this decode
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
