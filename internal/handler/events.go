package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const sseHeartbeatSec = 25

// sseHub fans out server-sent events to subscribed browser connections, keyed by
// user id. It is shared across ecosystem services (chat, calendar, …): each
// service builds its own event payload and calls sendTo/publish.
type sseHub struct {
	mu      sync.Mutex
	clients map[int64]map[chan []byte]bool // userID -> open connections
}

func newSSEHub() *sseHub {
	return &sseHub{clients: make(map[int64]map[chan []byte]bool)}
}

func (hub *sseHub) subscribe(userID int64) chan []byte {
	ch := make(chan []byte, 32)
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if hub.clients[userID] == nil {
		hub.clients[userID] = make(map[chan []byte]bool)
	}
	hub.clients[userID][ch] = true
	return ch
}

func (hub *sseHub) unsubscribe(userID int64, ch chan []byte) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if conns := hub.clients[userID]; conns != nil {
		delete(conns, ch)
		if len(conns) == 0 {
			delete(hub.clients, userID)
		}
	}
	close(ch)
}

// publish pushes raw bytes to all connections of the listed users, or to everyone
// when userIDs is nil. Slow clients drop the event and refetch on demand.
func (hub *sseHub) publish(userIDs []int64, data []byte) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	push := func(conns map[chan []byte]bool) {
		for ch := range conns {
			select {
			case ch <- data:
			default:
			}
		}
	}
	if userIDs == nil {
		for _, conns := range hub.clients {
			push(conns)
		}
		return
	}
	for _, uid := range userIDs {
		if conns := hub.clients[uid]; conns != nil {
			push(conns)
		}
	}
}

// sendTo marshals ev as JSON and publishes it (nil userIDs = broadcast).
func (hub *sseHub) sendTo(userIDs []int64, ev any) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	hub.publish(userIDs, data)
}

// serveSSE streams events to a subscribed user until the request is cancelled.
// Used by every service's /events endpoint after its own auth check.
func (h *Handler) serveSSE(w http.ResponseWriter, r *http.Request, userID int64) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx: do not buffer

	ch := h.sse.subscribe(userID)
	defer h.sse.unsubscribe(userID, ch)

	fmt.Fprintf(w, "event: hello\ndata: {}\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(sseHeartbeatSec * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case data, open := <-ch:
			if !open {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
