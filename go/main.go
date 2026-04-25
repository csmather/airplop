package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

const port = 8765 // const = compile-time constant

//go:embed web/index.html
var webFS embed.FS // //go:embed bakes the file into the binary; directive must be flush against the var

func main() {
	ip, err := lanIP() // := declares+assigns+infers type; multi-return is idiomatic Go
	if err != nil {
		log.Fatalf("could not detect LAN IP: %v", err) // logs then os.Exit(1)
	}

	tmpl, err := template.ParseFS(webFS, "web/index.html")
	if err != nil {
		log.Fatalf("parse template: %v", err)
	}

	h := newHub()

	mux := http.NewServeMux()
	// Go 1.22+ pattern syntax; "{$}" anchors to exactly "/"
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { // anonymous func
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, struct{ LanIP string }{LanIP: ip}) // anonymous struct as inline type
	})
	mux.HandleFunc("GET /stream", h.streamHandler) // method value: bound to h, callable as a func
	mux.HandleFunc("POST /update", h.updateHandler)

	// force tcp4 — WSL2's IPv6 has edge cases
	listener, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	srv := &http.Server{Handler: mux} // & takes a pointer

	// ctx cancels on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() { // goroutine: concurrent execution; cheap (~few KB stack)
		log.Printf("airplop listening on http://%s:%d", ip, port)
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done() // <-ch: receive; blocks until cancelled
	log.Println("shutting down…")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
