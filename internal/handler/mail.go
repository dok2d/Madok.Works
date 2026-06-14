package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"kanban/internal/db"
	"kanban/internal/model"
)

// applyMailRules evaluates a user's filters against a freshly delivered message
// (identified by box id) and applies the resulting actions.
func (h *Handler) applyMailRules(userID, boxID int64, ctx db.RuleContext) {
	if boxID == 0 {
		return
	}
	rules, err := h.store.ListMailRules(userID)
	if err != nil || len(rules) == 0 {
		return
	}
	for _, a := range db.EvaluateRules(rules, ctx) {
		switch a.Type {
		case "move_to_folder":
			if a.Arg != "" {
				h.store.MoveMail(boxID, userID, a.Arg)
			}
		case "flag":
			h.store.SetMailFlagged(boxID, userID, true)
		case "mark_read":
			h.store.SetMailSeen(boxID, userID, true)
		case "delete":
			h.store.DeleteMailBox(boxID, userID)
			return
		}
	}
}

// handleMailRules lists (GET) or creates (POST) the user's rules.
func (h *Handler) handleMailRules(w http.ResponseWriter, r *http.Request) {
	user := h.mailUser(w, r)
	if user == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rules, err := h.store.ListMailRules(user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, rules)
	case http.MethodPost:
		var rule model.MailRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		id, err := h.store.CreateMailRule(user.ID, rule)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonResp(w, map[string]int64{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMailRule updates (PUT) or deletes (DELETE) one rule.
func (h *Handler) handleMailRule(w http.ResponseWriter, r *http.Request) {
	user := h.mailUser(w, r)
	if user == nil {
		return
	}
	id := extractID(r.URL.Path, "/api/mail/rules/")
	switch r.Method {
	case http.MethodPut:
		var rule model.MailRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := h.store.UpdateMailRule(id, user.ID, rule); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if err := h.store.DeleteMailRule(id, user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

const (
	mailMaxSubjectLen = 300
	mailMaxBodyLen    = 100000
	mailMaxRecipients = 50
)

// resolveLocalRecipient maps an address local-part to a local user. Phase 7
// extends this to also match mailbox aliases.
func (h *Handler) resolveLocalRecipient(localpart string) *model.User {
	if u, err := h.store.GetUserByUsername(localpart); err == nil && u != nil {
		return u
	}
	if uid := h.store.ResolveAlias(localpart); uid > 0 {
		if u, err := h.store.GetUser(uid); err == nil {
			return u
		}
	}
	return nil
}

// handleMailSettings reads/writes the mail server config (admin only).
func (h *Handler) handleMailSettings(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil || !user.IsAdmin {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		jsonResp(w, map[string]any{
			"domain":           h.outboundDomain,
			"dkim_selector":    h.dkimSelector,
			"dkim_configured":  h.dkimKey != nil,
			"dkim_txt":         h.dkimPublicTXT(),
			"outbound_enabled": h.outboundEnabled(),
		})
	case http.MethodPost:
		var req struct {
			Domain       string `json:"domain"`
			DkimSelector string `json:"dkim_selector"`
			DkimKey      string `json:"dkim_key"` // "" keep, "__clear__" remove, else set PEM
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		h.store.SetSetting("mail_domain", strings.TrimSpace(req.Domain))
		h.store.SetSetting("mail_dkim_selector", strings.TrimSpace(req.DkimSelector))
		if req.DkimKey == "__clear__" {
			h.store.SetSetting("mail_dkim_key", "")
		} else if strings.TrimSpace(req.DkimKey) != "" {
			if _, err := parseRSAKeyPEM([]byte(req.DkimKey)); err != nil {
				http.Error(w, "invalid DKIM key (expect PEM RSA private key)", http.StatusBadRequest)
				return
			}
			h.store.SetSetting("mail_dkim_key", req.DkimKey)
		}
		h.loadMailOutboundConfig()
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMailAliases lists (GET) or creates (POST) mailbox aliases (admin only).
func (h *Handler) handleMailAliases(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil || !user.IsAdmin {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := h.store.ListAliases()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, rows)
	case http.MethodPost:
		var req struct {
			UserID    int64  `json:"user_id"`
			Localpart string `json:"localpart"`
			IsPrimary bool   `json:"is_primary"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		id, err := h.store.CreateAlias(req.UserID, req.Localpart, req.IsPrimary)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonResp(w, map[string]int64{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMailAlias deletes (DELETE) or sets primary (POST .../primary) an alias.
func (h *Handler) handleMailAlias(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil || !user.IsAdmin {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/mail/aliases/")
	parts := strings.SplitN(strings.TrimSuffix(rest, "/"), "/", 2)
	id, _ := strconv.ParseInt(parts[0], 10, 64)
	if len(parts) == 2 && parts[1] == "primary" {
		if err := h.store.SetPrimaryAlias(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
		return
	}
	if r.Method == http.MethodDelete {
		if err := h.store.DeleteAlias(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// handleMailAudit returns recent mail security events (admin only).
func (h *Handler) handleMailAudit(w http.ResponseWriter, r *http.Request) {
	user := h.mailUser(w, r)
	if user == nil {
		return
	}
	if !user.IsAdmin {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := h.store.ListMailAudit(r.URL.Query().Get("event"), limit, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, rows)
}

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

// handleMailCustomFolders lists (GET) or creates (POST) the user's custom folders.
func (h *Handler) handleMailCustomFolders(w http.ResponseWriter, r *http.Request) {
	user := h.mailUser(w, r)
	if user == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		folders, err := h.store.ListMailFolders(user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, folders)
	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		id, err := h.store.CreateMailFolder(user.ID, req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonResp(w, map[string]int64{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMailCustomFolder renames (PUT) or deletes (DELETE) one custom folder.
func (h *Handler) handleMailCustomFolder(w http.ResponseWriter, r *http.Request) {
	user := h.mailUser(w, r)
	if user == nil {
		return
	}
	id := extractID(r.URL.Path, "/api/mail/folders/custom/")
	switch r.Method {
	case http.MethodPut:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := h.store.RenameMailFolder(id, user.ID, req.Name); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if err := h.store.DeleteMailFolder(id, user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
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
	uid := user.ID
	// Audit unusually large sends and external relay attempts.
	if len(toIDs)+len(external) >= 10 {
		h.store.LogMailAudit(model.MailAudit{Event: "mass_send", MailFrom: user.Username, UserID: &uid,
			Subject: strings.TrimSpace(req.Subject), Detail: fmt.Sprintf("%d recipients", len(toIDs)+len(external))})
	}
	// Relay to external recipients (no-op unless outbound mail is configured).
	if len(external) > 0 {
		ev := "relay_attempt"
		if h.outboundEnabled() {
			ev = "relay"
			go h.sendExternal(user.Username, external, strings.TrimSpace(req.Subject), req.BodyText)
		}
		h.store.LogMailAudit(model.MailAudit{Event: ev, MailFrom: user.Username, UserID: &uid,
			Rcpt: strings.Join(external, ", "), Subject: strings.TrimSpace(req.Subject)})
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
	// Apply each recipient's filter rules to their inbox copy.
	for _, rid := range toIDs {
		h.applyMailRules(rid, h.store.MailBoxForMessage(rid, id),
			db.RuleContext{From: user.Username, To: displayTo, Subject: req.Subject, Body: req.BodyText})
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
