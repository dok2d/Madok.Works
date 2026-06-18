package handler

import (
	"archive/zip"
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
	wikiMaxTitleLen   = 300
	wikiMaxContentLen = 2 * 1024 * 1024 // 2 MB of HTML per page
	wikiMaxKeyLen     = 16
)

var wikiKeyRe = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,15}$`)

// wikiUser authorizes a wiki API call: service must be enabled and the
// caller authenticated. Global readonly users are capped at viewer.
func (h *Handler) wikiUser(w http.ResponseWriter, r *http.Request) *model.User {
	if !h.serviceEnabled("wiki") {
		http.Error(w, "wiki service disabled", http.StatusNotFound)
		return nil
	}
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}
	return user
}

// wikiRole returns the user's effective role in a space, capping global
// readonly users at viewer.
func (h *Handler) wikiRole(user *model.User, spaceID int64) (string, *model.WikiSpace, error) {
	sp, err := h.store.GetWikiSpace(spaceID, user.ID, user.Role == "admin")
	if err != nil {
		return "", nil, err
	}
	role := sp.MyRole
	if user.Role == "readonly" && role != "" {
		role = "viewer"
	}
	sp.MyRole = role
	return role, sp, nil
}

func wikiRoleAtLeast(role, min string) bool {
	rank := map[string]int{"": 0, "viewer": 1, "editor": 2, "admin": 3}
	return rank[role] >= rank[min]
}

// === Spaces ===

func (h *Handler) handleWikiSpaces(w http.ResponseWriter, r *http.Request) {
	user := h.wikiUser(w, r)
	if user == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		pid, ok := h.requireProject(w, r, user, "viewer")
		if !ok {
			return
		}
		spaces, err := h.store.ListWikiSpaces(user.ID, user.Role == "admin", pid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if user.Role == "readonly" {
			for i := range spaces {
				spaces[i].MyRole = "viewer"
			}
		}
		jsonResp(w, spaces)
	case http.MethodPost:
		if user.Role == "readonly" {
			http.Error(w, "read-only access", http.StatusForbidden)
			return
		}
		var req struct {
			Key         string `json:"key"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Visibility  string `json:"visibility"`
			Personal    bool   `json:"personal"` // true = personal space (no project)
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		req.Key = strings.ToUpper(strings.TrimSpace(req.Key))
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" || len(req.Name) > wikiMaxTitleLen {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if !wikiKeyRe.MatchString(req.Key) {
			http.Error(w, "key must be 1-16 chars: A-Z 0-9 _ -", http.StatusBadRequest)
			return
		}
		// Personal spaces need no project access; project spaces require editor.
		var projectID *int64
		if !req.Personal {
			pid, ok := h.requireProject(w, r, user, "editor")
			if !ok {
				return
			}
			projectID = &pid
		}
		id, err := h.store.CreateWikiSpace(req.Key, req.Name, req.Description, req.Visibility, projectID, user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.store.LogActivity(user.ID, "wiki_create_space", nil, req.Key+" "+req.Name)
		jsonResp(w, map[string]int64{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleWikiSpaceRoute dispatches /api/wiki/spaces/{id}[/tree|/members].
func (h *Handler) handleWikiSpaceRoute(w http.ResponseWriter, r *http.Request) {
	user := h.wikiUser(w, r)
	if user == nil {
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/wiki/spaces/")
	parts := strings.SplitN(strings.TrimSuffix(rest, "/"), "/", 2)
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad space id", http.StatusBadRequest)
		return
	}
	role, sp, err := h.wikiRole(user, id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "space not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if role == "" {
		http.Error(w, "no access to this space", http.StatusForbidden)
		return
	}

	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}
	switch sub {
	case "":
		h.handleWikiSpace(w, r, user, sp, role)
	case "tree":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		pages, err := h.store.ListWikiPageTree(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, pages)
	case "members":
		h.handleWikiSpaceMembers(w, r, user, sp, role)
	case "export":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.exportWikiSpace(w, sp)
	default:
		http.NotFound(w, r)
	}
}

// exportWikiSpace streams a zip of every page's markdown in the space.
func (h *Handler) exportWikiSpace(w http.ResponseWriter, sp *model.WikiSpace) {
	pages, err := h.store.ExportWikiSpacePages(sp.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+wikiSafeFilename(sp.Key)+"-wiki.zip\"")
	zw := zip.NewWriter(w)
	defer zw.Close()
	used := map[string]int{}
	for _, p := range pages {
		base := wikiSafeFilename(p.Title)
		name := base + ".md"
		if used[name] > 0 {
			name = fmt.Sprintf("%s-%d.md", base, p.ID)
		}
		used[name]++
		fw, err := zw.Create(name)
		if err != nil {
			return
		}
		fw.Write([]byte("# " + p.Title + "\n\n" + p.ContentMd))
	}
}

func (h *Handler) handleWikiSpace(w http.ResponseWriter, r *http.Request, user *model.User, sp *model.WikiSpace, role string) {
	switch r.Method {
	case http.MethodGet:
		jsonResp(w, sp)
	case http.MethodPut:
		if !wikiRoleAtLeast(role, "admin") {
			http.Error(w, "space admin only", http.StatusForbidden)
			return
		}
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Visibility  string `json:"visibility"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" || len(req.Name) > wikiMaxTitleLen {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if err := h.store.UpdateWikiSpace(sp.ID, req.Name, req.Description, req.Visibility); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if !wikiRoleAtLeast(role, "admin") {
			http.Error(w, "space admin only", http.StatusForbidden)
			return
		}
		if err := h.store.DeleteWikiSpace(sp.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.store.LogActivity(user.ID, "wiki_delete_space", nil, sp.Key)
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleWikiSpaceMembers(w http.ResponseWriter, r *http.Request, user *model.User, sp *model.WikiSpace, role string) {
	switch r.Method {
	case http.MethodGet:
		members, err := h.store.ListWikiSpaceMembers(sp.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, members)
	case http.MethodPost:
		if !wikiRoleAtLeast(role, "admin") {
			http.Error(w, "space admin only", http.StatusForbidden)
			return
		}
		var req struct {
			UserID int64  `json:"user_id"`
			Role   string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := h.store.SetWikiSpaceMember(sp.ID, req.UserID, req.Role); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if !wikiRoleAtLeast(role, "admin") {
			http.Error(w, "space admin only", http.StatusForbidden)
			return
		}
		uid, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
		if uid <= 0 {
			http.Error(w, "user_id required", http.StatusBadRequest)
			return
		}
		if err := h.store.RemoveWikiSpaceMember(sp.ID, uid); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// === Pages ===

func (h *Handler) handleWikiPages(w http.ResponseWriter, r *http.Request) {
	user := h.wikiUser(w, r)
	if user == nil {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SpaceID   int64  `json:"space_id"`
		ParentID  *int64 `json:"parent_id"`
		Title     string `json:"title"`
		Content   string `json:"content"`
		ContentMd string `json:"content_md"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	role, _, err := h.wikiRole(user, req.SpaceID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "space not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !wikiRoleAtLeast(role, "editor") {
		http.Error(w, "no edit access", http.StatusForbidden)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || len(req.Title) > wikiMaxTitleLen {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	if len(req.Content) > wikiMaxContentLen {
		http.Error(w, "content too large", http.StatusBadRequest)
		return
	}
	if req.ParentID != nil {
		parent, err := h.store.GetWikiPage(*req.ParentID)
		if err != nil || parent.SpaceID != req.SpaceID {
			http.Error(w, "parent page not found in space", http.StatusBadRequest)
			return
		}
	}
	content := sanitizeHTML(req.Content)
	id, err := h.store.CreateWikiPage(req.SpaceID, req.ParentID, req.Title, content, req.ContentMd, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.store.LogActivity(user.ID, "wiki_create_page", nil, req.Title)
	h.processWikiPageLinks(req.SpaceID, id, req.ContentMd, req.Title, user)
	jsonResp(w, map[string]int64{"id": id})
}

// processWikiPageLinks rebuilds a page's [[wiki link]] graph and notifies any
// @mentioned users. Called after a page is created or updated.
func (h *Handler) processWikiPageLinks(spaceID, pageID int64, contentMd, title string, author *model.User) {
	_, ids := h.store.ResolveWikiLinks(spaceID, contentMd)
	if err := h.store.SetWikiLinks(pageID, ids); err != nil {
		h.logf("SetWikiLinks(%d): %v", pageID, err)
	}
	h.processWikiMentions(contentMd, author, title)
}

// processWikiMentions notifies users named with @username in a wiki page.
func (h *Handler) processWikiMentions(text string, author *model.User, pageTitle string) {
	if author == nil {
		return
	}
	seen := map[string]bool{}
	for _, m := range mentionRegex.FindAllStringSubmatch(text, -1) {
		username := m[1]
		if seen[username] || username == author.Username {
			continue
		}
		seen[username] = true
		mentioned, err := h.store.GetUserByUsername(username)
		if err != nil {
			continue
		}
		notifText := fmt.Sprintf("@%s упомянул(а) вас на вики-странице: %s", author.Username, pageTitle)
		h.store.CreateNotification(mentioned.ID, "wiki_mention", notifText, nil)
		h.sendTelegramNotification(mentioned.ID, notifText)
	}
}

// handleWikiPageRoute dispatches /api/wiki/pages/{id}[/move|/revisions|/restore].
func (h *Handler) handleWikiPageRoute(w http.ResponseWriter, r *http.Request) {
	user := h.wikiUser(w, r)
	if user == nil {
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/wiki/pages/")
	parts := strings.SplitN(strings.TrimSuffix(rest, "/"), "/", 2)
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad page id", http.StatusBadRequest)
		return
	}
	page, err := h.store.GetWikiPage(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	role, _, err := h.wikiRole(user, page.SpaceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if role == "" {
		http.Error(w, "no access to this space", http.StatusForbidden)
		return
	}

	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}
	switch sub {
	case "":
		h.handleWikiPage(w, r, user, page, role)
	case "move":
		h.handleWikiPageMove(w, r, user, page, role)
	case "revisions":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		revs, err := h.store.ListWikiRevisions(page.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, revs)
	case "restore":
		h.handleWikiPageRestore(w, r, user, page, role)
	case "summary":
		h.handleWikiPageSummary(w, r, page)
	case "comments":
		h.handleWikiComments(w, r, user, page, role)
	case "react":
		h.handleWikiReact(w, r, user, "page", page.ID, role)
	case "export":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+wikiSafeFilename(page.Title)+".md\"")
		w.Write([]byte("# " + page.Title + "\n\n" + page.ContentMd))
	default:
		http.NotFound(w, r)
	}
}

// wikiSafeFilename strips path separators and control characters so a page title
// can be used as a download filename.
func wikiSafeFilename(name string) string {
	name = strings.TrimSpace(name)
	repl := func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}
	name = strings.Map(repl, name)
	if name == "" {
		name = "page"
	}
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}

// handleWikiComments lists (GET) or adds (POST) comments on a page.
func (h *Handler) handleWikiComments(w http.ResponseWriter, r *http.Request, user *model.User, page *model.WikiPage, role string) {
	switch r.Method {
	case http.MethodGet:
		comments, err := h.store.ListWikiComments(page.ID, user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, comments)
	case http.MethodPost:
		if user.Role == "readonly" {
			http.Error(w, "read-only access", http.StatusForbidden)
			return
		}
		var req struct {
			Text     string `json:"text"`
			ParentID *int64 `json:"parent_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		req.Text = strings.TrimSpace(req.Text)
		if req.Text == "" || len(req.Text) > wikiMaxContentLen {
			http.Error(w, "text required", http.StatusBadRequest)
			return
		}
		id, err := h.store.AddWikiComment(page.ID, req.ParentID, req.Text, user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// @mentions in a comment notify, linking the page by title.
		h.processWikiMentions(req.Text, user, page.Title)
		jsonResp(w, map[string]int64{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleWikiReact toggles the user's reaction on a page or comment.
func (h *Handler) handleWikiReact(w http.ResponseWriter, r *http.Request, user *model.User, targetType string, targetID int64, role string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if user.Role == "readonly" {
		http.Error(w, "read-only access", http.StatusForbidden)
		return
	}
	var req struct {
		Emoji string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Emoji) == "" {
		http.Error(w, "emoji required", http.StatusBadRequest)
		return
	}
	if len(req.Emoji) > 16 {
		http.Error(w, "bad emoji", http.StatusBadRequest)
		return
	}
	if err := h.store.ToggleWikiReaction(targetType, targetID, user.ID, req.Emoji); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reacts, _ := h.store.WikiReactions(targetType, targetID, user.ID)
	jsonResp(w, reacts)
}

// handleWikiCommentRoute dispatches /api/wiki/comments/{id}[/react].
func (h *Handler) handleWikiCommentRoute(w http.ResponseWriter, r *http.Request) {
	user := h.wikiUser(w, r)
	if user == nil {
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/wiki/comments/")
	parts := strings.SplitN(strings.TrimSuffix(rest, "/"), "/", 2)
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad comment id", http.StatusBadRequest)
		return
	}
	c, err := h.store.GetWikiComment(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "comment not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Resolve the space role via the comment's page.
	page, err := h.store.GetWikiPage(c.PageID)
	if err != nil {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}
	prole, _, err := h.wikiRole(user, page.SpaceID)
	if err != nil || prole == "" {
		http.Error(w, "no access to this space", http.StatusForbidden)
		return
	}
	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}
	if sub == "react" {
		h.handleWikiReact(w, r, user, "comment", c.ID, prole)
		return
	}
	switch r.Method {
	case http.MethodPut:
		// Author or space admin may edit.
		if !(c.AuthorID != nil && *c.AuthorID == user.ID) && !wikiRoleAtLeast(prole, "admin") {
			http.Error(w, "forbidden", http.StatusForbidden)
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
		if req.Text == "" || len(req.Text) > wikiMaxContentLen {
			http.Error(w, "text required", http.StatusBadRequest)
			return
		}
		if err := h.store.UpdateWikiComment(c.ID, req.Text); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if !(c.AuthorID != nil && *c.AuthorID == user.ID) && !wikiRoleAtLeast(prole, "admin") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := h.store.DeleteWikiComment(c.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleWikiPage(w http.ResponseWriter, r *http.Request, user *model.User, page *model.WikiPage, role string) {
	switch r.Method {
	case http.MethodGet:
		// Resolve [[links]] for rendering and gather backlinks for this page.
		page.Links, _ = h.store.ResolveWikiLinks(page.SpaceID, page.ContentMd)
		if bl, err := h.store.WikiBacklinks(page.ID); err == nil {
			page.Backlinks = bl
		}
		page.Reactions, _ = h.store.WikiReactions("page", page.ID, user.ID)
		jsonResp(w, page)
	case http.MethodPut:
		if !wikiRoleAtLeast(role, "editor") {
			http.Error(w, "no edit access", http.StatusForbidden)
			return
		}
		var req struct {
			Title     string `json:"title"`
			Content   string `json:"content"`
			ContentMd string `json:"content_md"`
			Comment   string `json:"comment"`
			BaseRev   int    `json:"base_rev"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		req.Title = strings.TrimSpace(req.Title)
		if req.Title == "" || len(req.Title) > wikiMaxTitleLen {
			http.Error(w, "title required", http.StatusBadRequest)
			return
		}
		if len(req.Content) > wikiMaxContentLen {
			http.Error(w, "content too large", http.StatusBadRequest)
			return
		}
		content := sanitizeHTML(req.Content)
		rev, err := h.store.UpdateWikiPage(page.ID, req.Title, content, req.ContentMd, req.Comment, user.ID, req.BaseRev)
		if err != nil {
			if strings.HasPrefix(err.Error(), "conflict:") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "current_rev": rev})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.store.LogActivity(user.ID, "wiki_edit_page", nil, page.Title)
		h.processWikiPageLinks(page.SpaceID, page.ID, req.ContentMd, req.Title, user)
		jsonResp(w, map[string]any{"status": "ok", "rev_no": rev})
	case http.MethodDelete:
		// editors may delete pages; space admins always can
		if !wikiRoleAtLeast(role, "editor") {
			http.Error(w, "no edit access", http.StatusForbidden)
			return
		}
		if err := h.store.DeleteWikiPage(page.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.store.LogActivity(user.ID, "wiki_delete_page", nil, page.Title)
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleWikiPageMove(w http.ResponseWriter, r *http.Request, user *model.User, page *model.WikiPage, role string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !wikiRoleAtLeast(role, "editor") {
		http.Error(w, "no edit access", http.StatusForbidden)
		return
	}
	var req struct {
		ParentID *int64 `json:"parent_id"`
		Position int    `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Position < 0 {
		req.Position = 0
	}
	if err := h.store.MoveWikiPage(page.ID, req.ParentID, req.Position); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleWikiPageRestore(w http.ResponseWriter, r *http.Request, user *model.User, page *model.WikiPage, role string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !wikiRoleAtLeast(role, "editor") {
		http.Error(w, "no edit access", http.StatusForbidden)
		return
	}
	var req struct {
		RevisionID int64 `json:"revision_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	rev, err := h.store.GetWikiRevision(req.RevisionID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && rev.PageID != page.ID) {
		http.Error(w, "revision not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	comment := "restored from v" + strconv.Itoa(rev.RevNo)
	newRev, err := h.store.UpdateWikiPage(page.ID, rev.Title, rev.Content, rev.ContentMd, comment, user.ID, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.store.LogActivity(user.ID, "wiki_restore_page", nil, page.Title+" ← v"+strconv.Itoa(rev.RevNo))
	jsonResp(w, map[string]any{"status": "ok", "rev_no": newRev})
}

// === Revisions & search ===

func (h *Handler) handleWikiRevision(w http.ResponseWriter, r *http.Request) {
	user := h.wikiUser(w, r)
	if user == nil {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := extractID(r.URL.Path, "/api/wiki/revisions/")
	rev, err := h.store.GetWikiRevision(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "revision not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	page, err := h.store.GetWikiPage(rev.PageID)
	if err != nil {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}
	role, _, err := h.wikiRole(user, page.SpaceID)
	if err != nil || role == "" {
		http.Error(w, "no access to this space", http.StatusForbidden)
		return
	}
	jsonResp(w, rev)
}

func (h *Handler) handleWikiSearch(w http.ResponseWriter, r *http.Request) {
	user := h.wikiUser(w, r)
	if user == nil {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len(q) > maxSearchQueryLen {
		jsonResp(w, []model.WikiSearchHit{})
		return
	}
	hits, err := h.store.SearchWikiPages(user.ID, user.Role == "admin", q, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, hits)
}
