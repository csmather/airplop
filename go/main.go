package main

import (
	"context"
	"embed"
	"html/template"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

//go:embed web/index.html
var webFS embed.FS

const port = 8765

func main() {
	ip, iface, err := lanIP()
	if err != nil {
		log.Fatalf("could not detect LAN IP: %v", err)
	}

	tmpl, err := template.ParseFS(webFS, "web/index.html")
	if err != nil {
		log.Fatalf("parse template: %v", err)
	}

	h := newHub()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, struct{ LanIP string }{LanIP: ip})
	})
	mux.HandleFunc("GET /stream", h.streamHandler)
	mux.HandleFunc("POST /update", h.updateHandler)

	srv := &http.Server{
		Handler: mux,
	}

	listener, err := net.Listen("tcp4", ":8765")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	shutdownMDNS, err := registerMDNS(ip, iface, port)
	if err != nil {
		log.Printf("mDNS registration failed (continuing): %v", err)
	} else {
		defer shutdownMDNS()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("airplop listening on http://%s:%d and http://airplop.local:%d", ip, port, port)
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down…")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
