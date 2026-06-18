package handler

import (
	"encoding/json"
	"net/http"
	"strings"
)

// === Calls (WebRTC) signaling ===
//
// Media is peer-to-peer (mesh). The server only relays signaling (invite/accept/
// decline/offer/answer/ICE/hangup) between users over the existing SSE hub plus a
// POST relay endpoint — no new socket. STUN/TURN are configured in settings.

// handleCallEvents is the per-user SSE stream for call signaling (separate from
// the chat stream so ringing works on any page).
func (h *Handler) handleCallEvents(w http.ResponseWriter, r *http.Request) {
	if !h.serviceEnabled("chat") {
		http.Error(w, "chat service disabled", http.StatusNotFound)
		return
	}
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	h.serveSSE(w, r, user.ID)
}

// handleCallSignal relays one signaling message to a target user.
func (h *Handler) handleCallSignal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		To        int64           `json:"to"`
		Kind      string          `json:"kind"`
		CallID    string          `json:"call_id"`
		ChannelID int64           `json:"channel_id"`
		Members   []int64         `json:"members"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.To <= 0 || req.Kind == "" {
		http.Error(w, "to and kind required", http.StatusBadRequest)
		return
	}
	// Relay to the target with the sender identity attached.
	h.sse.sendTo([]int64{req.To}, map[string]any{
		"type":       "call",
		"kind":       req.Kind,
		"call_id":    req.CallID,
		"channel_id": req.ChannelID,
		"members":    req.Members,
		"payload":    req.Payload,
		"from":       map[string]any{"id": user.ID, "name": user.Username},
	})
	jsonResp(w, map[string]string{"status": "ok"})
}

// handleCallConfig returns the ICE server list (STUN always; TURN if configured
// in settings). Used by the client to build RTCPeerConnections.
func (h *Handler) handleCallConfig(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	servers := []map[string]any{
		{"urls": "stun:stun.l.google.com:19302"},
	}
	if turn := strings.TrimSpace(h.store.GetSetting("turn_url")); turn != "" {
		entry := map[string]any{"urls": turn}
		if u := strings.TrimSpace(h.store.GetSetting("turn_user")); u != "" {
			entry["username"] = u
		}
		if c := strings.TrimSpace(h.store.GetSetting("turn_cred")); c != "" {
			entry["credential"] = c
		}
		servers = append(servers, entry)
	}
	jsonResp(w, map[string]any{"iceServers": servers})
}

// handleCallTurnSettings reads/writes the TURN server config (admin only).
func (h *Handler) handleCallTurnSettings(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		jsonResp(w, map[string]string{
			"turn_url":  h.store.GetSetting("turn_url"),
			"turn_user": h.store.GetSetting("turn_user"),
			"turn_cred": h.store.GetSetting("turn_cred"),
		})
	case http.MethodPost:
		if !user.IsAdmin {
			http.Error(w, "admin only", http.StatusForbidden)
			return
		}
		var req struct{ URL, User, Cred string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		h.store.SetSetting("turn_url", strings.TrimSpace(req.URL))
		h.store.SetSetting("turn_user", strings.TrimSpace(req.User))
		h.store.SetSetting("turn_cred", strings.TrimSpace(req.Cred))
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
