package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"kanban/internal/model"
)

const (
	mailMaxSubjectLen = 300
	mailMaxBodyLen    = 100000
	mailMaxRecipients = 50
)

func (h *Handler) mailUser(w http.ResponseWriter, r *http.Request) *model.User {
	if !h.serviceEnabled("mail") {
		http.Error(w, "mail service disabled", http.StatusNotFound)
		return nil
	}
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}
	return user
}

func (h *Handler) handleMailFolders(w http.ResponseWriter, r *http.Request) {
	user := h.mailUser(w, r)
	if user == nil {
		return
	}
	counts, err := h.store.MailFolderCounts(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, counts)
}

func (h *Handler) handleMailUnread(w http.ResponseWriter, r *http.Request) {
	user := h.mailUser(w, r)
	if user == nil {
		return
	}
	jsonResp(w, map[string]int{"unread": h.store.UnreadMailCount(user.ID)})
}

// handleMailMessages lists a folder or searches (GET ?folder= / ?q=).
func (h *Handler) handleMailMessages(w http.ResponseWriter, r *http.Request) {
	user := h.mailUser(w, r)
	if user == nil {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	if query := strings.TrimSpace(q.Get("q")); query != "" {
		hits, err := h.store.SearchMail(user.ID, query, 50)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, hits)
		return
	}
	folder := q.Get("folder")
	if folder == "" {
		folder = "inbox"
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	msgs, err := h.store.ListMailbox(user.ID, folder, 50, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, msgs)
}

// handleMailMessage dispatches /api/mail/messages/{boxID}[/flag|/move].
func (h *Handler) handleMailMessage(w http.ResponseWriter, r *http.Request) {
	user := h.mailUser(w, r)
	if user == nil {
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/mail/messages/")
	parts := strings.SplitN(strings.TrimSuffix(rest, "/"), "/", 2)
	boxID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || boxID <= 0 {
		http.Error(w, "bad message id", http.StatusBadRequest)
		return
	}
	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}
	switch sub {
	case "":
		h.handleMailMessageRoot(w, r, user, boxID)
	case "flag":
		h.handleMailFlag(w, r, user, boxID)
	case "move":
		h.handleMailMove(w, r, user, boxID)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleMailMessageRoot(w http.ResponseWriter, r *http.Request, user *model.User, boxID int64) {
	switch r.Method {
	case http.MethodGet:
		msg, err := h.store.GetMailBox(user.ID, boxID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.store.SetMailSeen(boxID, user.ID, true) // opening marks read
		jsonResp(w, msg)
	case http.MethodDelete:
		if err := h.store.DeleteMailBox(boxID, user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleMailFlag(w http.ResponseWriter, r *http.Request, user *model.User, boxID int64) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Flagged *bool `json:"flagged"`
		Seen    *bool `json:"seen"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Flagged != nil {
		h.store.SetMailFlagged(boxID, user.ID, *req.Flagged)
	}
	if req.Seen != nil {
		h.store.SetMailSeen(boxID, user.ID, *req.Seen)
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleMailMove(w http.ResponseWriter, r *http.Request, user *model.User, boxID int64) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Folder string `json:"folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Folder) == "" {
		http.Error(w, "folder required", http.StatusBadRequest)
		return
	}
	if err := h.store.MoveMail(boxID, user.ID, req.Folder); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleMailThread(w http.ResponseWriter, r *http.Request) {
	user := h.mailUser(w, r)
	if user == nil {
		return
	}
	threadID := extractID(r.URL.Path, "/api/mail/thread/")
	msgs, err := h.store.ThreadForUser(user.ID, threadID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, msgs)
}

// handleMailSend composes and delivers an internal message.
func (h *Handler) handleMailSend(w http.ResponseWriter, r *http.Request) {
	user := h.mailUser(w, r)
	if user == nil {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		To          string                 `json:"to"` // comma-separated usernames
		Cc          string                 `json:"cc"`
		Subject     string                 `json:"subject"`
		BodyText    string                 `json:"body_text"`
		BodyHTML    string                 `json:"body_html"`
		ThreadID    int64                  `json:"thread_id"`
		InReplyTo   string                 `json:"in_reply_to"`
		Attachments []model.MailAttachment `json:"attachments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(req.Subject) > mailMaxSubjectLen || len(req.BodyText) > mailMaxBodyLen || len(req.BodyHTML) > mailMaxBodyLen {
		http.Error(w, "subject or body too long", http.StatusBadRequest)
		return
	}
	// Sanitize any HTML body before storage (shared sanitizer).
	if req.BodyHTML != "" {
		req.BodyHTML = sanitizeHTML(req.BodyHTML)
	}
	toIDs, toNames, external := h.resolveMailRecipients(req.To)
	if len(toIDs) == 0 && len(external) == 0 {
		http.Error(w, "no valid recipients", http.StatusBadRequest)
		return
	}
	if len(toIDs)+len(external) > mailMaxRecipients {
		http.Error(w, "too many recipients", http.StatusBadRequest)
		return
	}
	displayTo := strings.Join(append(append([]string{}, toNames...), external...), ", ")
	id, err := h.store.SendInternalMail(user.ID, user.Username, toIDs, displayTo, req.Cc,
		strings.TrimSpace(req.Subject), req.BodyText, req.BodyHTML, req.InReplyTo, req.ThreadID, req.Attachments)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Relay to external recipients (no-op unless outbound mail is configured).
	if len(external) > 0 && h.outboundEnabled() {
		go h.sendExternal(user.Username, external, strings.TrimSpace(req.Subject), req.BodyText)
	}
	// Notify recipients: live event + in-app + Telegram.
	subj := req.Subject
	if subj == "" {
		subj = "(без темы)"
	}
	for _, uid := range toIDs {
		h.store.CreateNotification(uid, "mail", fmt.Sprintf("Новое письмо от %s: %s", user.Username, subj), nil)
		h.sendTelegramNotification(uid, fmt.Sprintf("Новое письмо от %s: %s", user.Username, subj))
	}
	h.sse.sendTo(append(toIDs, user.ID), map[string]string{"type": "mail"})
	jsonResp(w, map[string]int64{"id": id})
}

// resolveMailRecipients splits comma-separated recipients into local users
// (matched by username) and external email addresses (tokens containing "@"
// that don't match a local user). Unknown bare names are dropped.
func (h *Handler) resolveMailRecipients(list string) (ids []int64, names []string, external []string) {
	seen := map[int64]bool{}
	extSeen := map[string]bool{}
	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if u, err := h.store.GetUserByUsername(name); err == nil && u != nil {
			if !seen[u.ID] {
				seen[u.ID] = true
				ids = append(ids, u.ID)
				names = append(names, u.Username)
			}
			continue
		}
		if strings.Contains(name, "@") && !extSeen[name] {
			extSeen[name] = true
			external = append(external, name)
		}
	}
	return ids, names, external
}

func (h *Handler) handleMailStream(w http.ResponseWriter, r *http.Request) {
	user := h.mailUser(w, r)
	if user == nil {
		return
	}
	h.serveSSE(w, r, user.ID)
}
