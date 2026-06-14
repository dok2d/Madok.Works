package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"kanban/internal/model"
)

const (
	chatMaxMessageLen = 8000
	chatMaxNameLen    = 100
	chatMaxTopicLen   = 300
	sseHeartbeatSec   = 25
)

var chatMentionRe = regexp.MustCompile(`@([\p{L}\p{N}_.-]+)`)

// === SSE hub ===

// sseEvent is what gets pushed to connected browsers.
type sseEvent struct {
	Type      string             `json:"type"` // chat_message | chat_edit | chat_delete | chat_channels
	ChannelID int64              `json:"channel_id,omitempty"`
	Message   *model.ChatMessage `json:"message,omitempty"`
}

type sseHub struct {
	mu      sync.Mutex
	clients map[int64]map[chan []byte]bool // userID -> connections
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

// sendTo pushes an event to all connections of the listed users
// (or to everyone when userIDs is nil).
func (hub *sseHub) sendTo(userIDs []int64, ev sseEvent) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	push := func(conns map[chan []byte]bool) {
		for ch := range conns {
			select {
			case ch <- data:
			default: // slow client: drop event, the client refetches on demand
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

// chatAudience resolves which user ids should receive events for a channel;
// nil means broadcast (public channel).
func (h *Handler) chatAudience(ch *model.ChatChannel) []int64 {
	if ch.Type == "channel" {
		return nil
	}
	members, err := h.store.ListChatMembers(ch.ID)
	if err != nil {
		return []int64{}
	}
	ids := make([]int64, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.UserID)
	}
	return ids
}

// === Auth helpers ===

func (h *Handler) chatUser(w http.ResponseWriter, r *http.Request) *model.User {
	if !h.serviceEnabled("chat") {
		http.Error(w, "chat service disabled", http.StatusNotFound)
		return nil
	}
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}
	return user
}

// chatAccess checks the user may see the channel.
func (h *Handler) chatAccess(user *model.User, ch *model.ChatChannel) bool {
	if ch.Type == "channel" {
		return true
	}
	ok, _ := h.store.IsChatMember(ch.ID, user.ID)
	return ok
}

func (h *Handler) chatCanManage(user *model.User, ch *model.ChatChannel) bool {
	return user.Role == "admin" || (ch.CreatedBy != nil && *ch.CreatedBy == user.ID)
}

// === Channels ===

func (h *Handler) handleChatChannels(w http.ResponseWriter, r *http.Request) {
	user := h.chatUser(w, r)
	if user == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		chans, err := h.store.ListChatChannels(user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, chans)
	case http.MethodPost:
		var req struct {
			Name    string  `json:"name"`
			Topic   string  `json:"topic"`
			Type    string  `json:"type"`
			Members []int64 `json:"members"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if len(req.Name) > chatMaxNameLen || len(req.Topic) > chatMaxTopicLen {
			http.Error(w, "name or topic too long", http.StatusBadRequest)
			return
		}
		id, err := h.store.CreateChatChannel(req.Name, req.Topic, req.Type, user.ID, req.Members)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.sse.sendTo(nil, sseEvent{Type: "chat_channels"})
		jsonResp(w, map[string]int64{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleChatChannelRoute dispatches /api/chat/channels/{id}[/members|/messages|/read].
func (h *Handler) handleChatChannelRoute(w http.ResponseWriter, r *http.Request) {
	user := h.chatUser(w, r)
	if user == nil {
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/chat/channels/")
	parts := strings.SplitN(strings.TrimSuffix(rest, "/"), "/", 2)
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad channel id", http.StatusBadRequest)
		return
	}
	ch, err := h.store.GetChatChannel(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "channel not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !h.chatAccess(user, ch) {
		http.Error(w, "no access to this channel", http.StatusForbidden)
		return
	}
	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}
	switch sub {
	case "":
		h.handleChatChannel(w, r, user, ch)
	case "members":
		h.handleChatChannelMembers(w, r, user, ch)
	case "messages":
		h.handleChatMessages(w, r, user, ch)
	case "read":
		h.handleChatRead(w, r, user, ch)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleChatChannel(w http.ResponseWriter, r *http.Request, user *model.User, ch *model.ChatChannel) {
	switch r.Method {
	case http.MethodGet:
		jsonResp(w, ch)
	case http.MethodPut:
		if !h.chatCanManage(user, ch) || ch.Type == "dm" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var req struct {
			Name  string `json:"name"`
			Topic string `json:"topic"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Name == "" || len(req.Name) > chatMaxNameLen || len(req.Topic) > chatMaxTopicLen {
			http.Error(w, "bad name or topic", http.StatusBadRequest)
			return
		}
		if err := h.store.UpdateChatChannel(ch.ID, req.Name, req.Topic); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.sse.sendTo(h.chatAudience(ch), sseEvent{Type: "chat_channels"})
		jsonResp(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if !h.chatCanManage(user, ch) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		audience := h.chatAudience(ch)
		if err := h.store.DeleteChatChannel(ch.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.sse.sendTo(audience, sseEvent{Type: "chat_channels"})
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleChatChannelMembers(w http.ResponseWriter, r *http.Request, user *model.User, ch *model.ChatChannel) {
	switch r.Method {
	case http.MethodGet:
		members, err := h.store.ListChatMembers(ch.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, members)
	case http.MethodPost:
		if ch.Type == "dm" {
			http.Error(w, "cannot modify DM members", http.StatusBadRequest)
			return
		}
		var req struct {
			UserID int64 `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// groups: only manager may add; channels: anyone may join themselves
		if ch.Type == "group" && !h.chatCanManage(user, ch) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if ch.Type == "channel" && req.UserID != user.ID && !h.chatCanManage(user, ch) {
			http.Error(w, "can only join yourself", http.StatusForbidden)
			return
		}
		if err := h.store.AddChatMember(ch.ID, req.UserID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.sse.sendTo([]int64{req.UserID}, sseEvent{Type: "chat_channels"})
		jsonResp(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		uid, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
		if uid <= 0 {
			http.Error(w, "user_id required", http.StatusBadRequest)
			return
		}
		if uid != user.ID && !h.chatCanManage(user, ch) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := h.store.RemoveChatMember(ch.ID, uid); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.sse.sendTo([]int64{uid}, sseEvent{Type: "chat_channels"})
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// === Messages ===

func (h *Handler) handleChatMessages(w http.ResponseWriter, r *http.Request, user *model.User, ch *model.ChatChannel) {
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		before, _ := strconv.ParseInt(q.Get("before"), 10, 64)
		after, _ := strconv.ParseInt(q.Get("after"), 10, 64)
		limit, _ := strconv.Atoi(q.Get("limit"))
		msgs, err := h.store.ListChatMessages(ch.ID, before, after, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, msgs)
	case http.MethodPost:
		var req struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		req.Text = strings.TrimSpace(req.Text)
		if req.Text == "" || len(req.Text) > chatMaxMessageLen {
			http.Error(w, "empty or too long message", http.StatusBadRequest)
			return
		}
		msg, err := h.store.CreateChatMessage(ch.ID, user.ID, req.Text)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.sse.sendTo(h.chatAudience(ch), sseEvent{Type: "chat_message", ChannelID: ch.ID, Message: msg})
		h.notifyChatMentions(user, ch, msg)
		jsonResp(w, msg)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleChatRead(w http.ResponseWriter, r *http.Request, user *model.User, ch *model.ChatChannel) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		LastID int64 `json:"last_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.store.MarkChatRead(ch.ID, user.ID, req.LastID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

// handleChatMessage handles PUT/DELETE /api/chat/messages/{id}.
func (h *Handler) handleChatMessage(w http.ResponseWriter, r *http.Request) {
	user := h.chatUser(w, r)
	if user == nil {
		return
	}
	id := extractID(r.URL.Path, "/api/chat/messages/")
	msg, err := h.store.GetChatMessage(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "message not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ch, err := h.store.GetChatChannel(msg.ChannelID)
	if err != nil {
		http.Error(w, "channel not found", http.StatusNotFound)
		return
	}
	if !h.chatAccess(user, ch) {
		http.Error(w, "no access to this channel", http.StatusForbidden)
		return
	}
	isAuthor := msg.AuthorID != nil && *msg.AuthorID == user.ID
	switch r.Method {
	case http.MethodPut:
		if !isAuthor {
			http.Error(w, "only the author can edit", http.StatusForbidden)
			return
		}
		var req struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		req.Text = strings.TrimSpace(req.Text)
		if req.Text == "" || len(req.Text) > chatMaxMessageLen {
			http.Error(w, "empty or too long message", http.StatusBadRequest)
			return
		}
		updated, err := h.store.UpdateChatMessage(msg.ID, req.Text)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.sse.sendTo(h.chatAudience(ch), sseEvent{Type: "chat_edit", ChannelID: ch.ID, Message: updated})
		jsonResp(w, updated)
	case http.MethodDelete:
		if !isAuthor && !h.chatCanManage(user, ch) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := h.store.DeleteChatMessage(msg.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		deleted, _ := h.store.GetChatMessage(msg.ID)
		h.sse.sendTo(h.chatAudience(ch), sseEvent{Type: "chat_delete", ChannelID: ch.ID, Message: deleted})
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// === DM, search, unread, events ===

func (h *Handler) handleChatDM(w http.ResponseWriter, r *http.Request) {
	user := h.chatUser(w, r)
	if user == nil {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		UserID int64 `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id, err := h.store.GetOrCreateDM(user.ID, req.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.sse.sendTo([]int64{user.ID, req.UserID}, sseEvent{Type: "chat_channels"})
	jsonResp(w, map[string]int64{"id": id})
}

func (h *Handler) handleChatSearch(w http.ResponseWriter, r *http.Request) {
	user := h.chatUser(w, r)
	if user == nil {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len(q) > maxSearchQueryLen {
		jsonResp(w, []model.ChatSearchHit{})
		return
	}
	hits, err := h.store.SearchChatMessages(user.ID, q, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, hits)
}

func (h *Handler) handleChatUnread(w http.ResponseWriter, r *http.Request) {
	user := h.chatUser(w, r)
	if user == nil {
		return
	}
	jsonResp(w, map[string]int{"unread": h.store.TotalChatUnread(user.ID)})
}

// handleChatEvents is the SSE endpoint feeding live updates to the UI.
func (h *Handler) handleChatEvents(w http.ResponseWriter, r *http.Request) {
	user := h.chatUser(w, r)
	if user == nil {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx: do not buffer

	ch := h.sse.subscribe(user.ID)
	defer h.sse.unsubscribe(user.ID, ch)

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

// notifyChatMentions creates in-app + telegram notifications for @mentions.
func (h *Handler) notifyChatMentions(author *model.User, ch *model.ChatChannel, msg *model.ChatMessage) {
	seen := map[string]bool{}
	for _, m := range chatMentionRe.FindAllStringSubmatch(msg.Text, -1) {
		uname := m[1]
		if seen[uname] || uname == author.Username {
			continue
		}
		seen[uname] = true
		target, err := h.store.GetUserByUsername(uname)
		if err != nil || target == nil {
			continue
		}
		// private channels: only notify members
		if ch.Type != "channel" {
			if ok, _ := h.store.IsChatMember(ch.ID, target.ID); !ok {
				continue
			}
		}
		title := ch.Name
		if ch.Type == "dm" {
			title = "DM"
		}
		preview := msg.Text
		if len(preview) > truncateShort {
			preview = preview[:truncateShort] + "…"
		}
		text := fmt.Sprintf("%s упомянул(а) вас в %s: %s", author.Username, title, preview)
		h.store.CreateNotification(target.ID, "chat_mention", text, nil)
		h.sendTelegramNotification(target.ID, text)
	}
}
