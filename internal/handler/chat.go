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

	"kanban/internal/model"
)

const (
	chatMaxMessageLen = 8000
	chatMaxNameLen    = 100
	chatMaxTopicLen   = 300
)

var chatMentionRe = regexp.MustCompile(`@([\p{L}\p{N}_.-]+)`)

// === Chat SSE events ===

// sseEvent is the chat-specific payload pushed to connected browsers via the
// shared sseHub (see events.go).
type sseEvent struct {
	Type      string             `json:"type"` // chat_message | chat_edit | chat_delete | chat_channels | chat_pin | chat_react
	ChannelID int64              `json:"channel_id,omitempty"`
	MessageID int64              `json:"message_id,omitempty"`
	Message   *model.ChatMessage `json:"message,omitempty"`
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
	case "pins":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		pins, err := h.store.ListPinnedMessages(ch.ID, user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, pins)
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
		msgs, err := h.store.ListChatMessages(ch.ID, before, after, limit, user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, msgs)
	case http.MethodPost:
		var req struct {
			Text        string                 `json:"text"`
			Attachments []model.ChatAttachment `json:"attachments"`
			ReplyTo     int64                  `json:"reply_to"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		req.Text = strings.TrimSpace(req.Text)
		if len(req.Text) > chatMaxMessageLen {
			http.Error(w, "message too long", http.StatusBadRequest)
			return
		}
		if req.Text == "" && len(req.Attachments) == 0 {
			http.Error(w, "empty message", http.StatusBadRequest)
			return
		}
		if len(req.Attachments) > 10 {
			http.Error(w, "too many attachments", http.StatusBadRequest)
			return
		}
		// Only allow replying to a message in the same channel.
		if req.ReplyTo > 0 {
			if parent, err := h.store.GetChatMessage(req.ReplyTo); err != nil || parent.ChannelID != ch.ID {
				req.ReplyTo = 0
			}
		}
		msg, err := h.store.CreateChatMessage(ch.ID, user.ID, req.Text, req.Attachments, req.ReplyTo)
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

// handleChatMessage handles /api/chat/messages/{id}[/pin|/react].
func (h *Handler) handleChatMessage(w http.ResponseWriter, r *http.Request) {
	user := h.chatUser(w, r)
	if user == nil {
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/chat/messages/")
	parts := strings.SplitN(strings.TrimSuffix(rest, "/"), "/", 2)
	id, perr := strconv.ParseInt(parts[0], 10, 64)
	if perr != nil || id <= 0 {
		http.Error(w, "bad message id", http.StatusBadRequest)
		return
	}
	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}
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
	switch sub {
	case "pin":
		h.handleChatPin(w, r, user, ch, msg)
		return
	case "react":
		h.handleChatReact(w, r, user, ch, msg)
		return
	case "":
		// handled below
	default:
		http.NotFound(w, r)
		return
	}
	isAuthor := msg.AuthorID != nil && *msg.AuthorID == user.ID
	switch r.Method {
	case http.MethodGet:
		full, err := h.store.GetChatMessageFor(msg.ID, user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, full)
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

// handleChatPin pins/unpins a message. In public channels only managers may pin;
// in groups and DMs any member may.
func (h *Handler) handleChatPin(w http.ResponseWriter, r *http.Request, user *model.User, ch *model.ChatChannel, msg *model.ChatMessage) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if ch.Type == "channel" && !h.chatCanManage(user, ch) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Pinned bool `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.store.SetChatPinned(msg.ID, user.ID, req.Pinned); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updated, _ := h.store.GetChatMessage(msg.ID)
	h.sse.sendTo(h.chatAudience(ch), sseEvent{Type: "chat_pin", ChannelID: ch.ID, MessageID: msg.ID, Message: updated})
	jsonResp(w, updated)
}

// handleChatReact toggles an emoji reaction for the current user.
func (h *Handler) handleChatReact(w http.ResponseWriter, r *http.Request, user *model.User, ch *model.ChatChannel, msg *model.ChatMessage) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if msg.Deleted {
		http.Error(w, "message deleted", http.StatusBadRequest)
		return
	}
	var req struct {
		Emoji string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	req.Emoji = strings.TrimSpace(req.Emoji)
	if req.Emoji == "" || len(req.Emoji) > 16 {
		http.Error(w, "bad emoji", http.StatusBadRequest)
		return
	}
	if err := h.store.ToggleChatReaction(msg.ID, user.ID, req.Emoji); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Broadcast a lightweight event; each client refetches to compute its own "mine".
	h.sse.sendTo(h.chatAudience(ch), sseEvent{Type: "chat_react", ChannelID: ch.ID, MessageID: msg.ID})
	updated, _ := h.store.GetChatMessageFor(msg.ID, user.ID)
	jsonResp(w, updated)
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
	h.serveSSE(w, r, user.ID)
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
