package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
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
		spaces, err := h.store.ListWikiSpaces(user.ID, user.Role == "admin")
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
		id, err := h.store.CreateWikiSpace(req.Key, req.Name, req.Description, req.Visibility, user.ID)
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
	default:
		http.NotFound(w, r)
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
	jsonResp(w, map[string]int64{"id": id})
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
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleWikiPage(w http.ResponseWriter, r *http.Request, user *model.User, page *model.WikiPage, role string) {
	switch r.Method {
	case http.MethodGet:
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
