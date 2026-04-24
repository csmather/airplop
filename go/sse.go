package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type payload struct {
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

type hub struct {
	mu      sync.Mutex
	subs    map[chan []byte]struct{}
	current []byte
}

func newHub() *hub {
	empty, _ := json.Marshal(payload{Text: "", Timestamp: time.Now().Unix()})
	return &hub{
		subs:    make(map[chan []byte]struct{}),
		current: empty,
	}
}

func (h *hub) add(ch chan []byte) []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subs[ch] = struct{}{}
	return h.current
}

func (h *hub) remove(ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs, ch)
	close(ch)
}

func (h *hub) broadcast(text string) {
	msg, err := json.Marshal(payload{Text: text, Timestamp: time.Now().Unix()})
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.current = msg
	for ch := range h.subs {
		select {
		case ch <- msg:
		default:
			// subscriber's buffer is full — drop them.
			delete(h.subs, ch)
			close(ch)
		}
	}
}

func (h *hub) streamHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := make(chan []byte, 10)
	initial := h.add(ch)
	defer h.remove(ch)

	fmt.Fprintf(w, "data: %s\n\n", initial)
	flusher.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return // hub closed our channel (buffer overflow drop)
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *hub) updateHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	h.broadcast(body.Text)
	log.Printf("update: %d bytes, %d subscribers", len(body.Text), len(h.subs))
	w.WriteHeader(http.StatusNoContent)
}
