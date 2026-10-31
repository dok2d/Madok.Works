package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"kanban/internal/model"
)

// === Personal API tokens ===
//
// Requests to /api/* may authenticate with "Authorization: Bearer mw_…"
// instead of the session cookie. A token acts as its owner, narrowed to the
// token's role (never above the owner's current role) and, optionally, to a
// set of services. Account security (password, 2FA, Telegram link, tokens
// themselves) stays session-only, so a leaked token cannot entrench itself.

const apiTokenContextKey contextKey = "api_token"

const (
	apiTokenMaxPerUser = 50
	apiTokenMaxName    = 100
	apiTokenMaxDays    = 3650
)

// roleRank orders global roles; anything unknown behaves like "regular",
// matching how the rest of the handler treats it (not admin, not read-only).
func roleRank(role string) int {
	switch role {
	case "admin":
		return 3
	case "readonly":
		return 1
	}
	return 2
}

func userRank(u *model.User) int {
	if u.IsAdmin {
		return 3
	}
	return roleRank(u.Role)
}

// clampToTokenRole narrows the owner to the token's role. Both Role and IsAdmin
// are adjusted because handlers check either one.
func clampToTokenRole(u *model.User, tokenRole string) *model.User {
	c := *u
	if roleRank(tokenRole) < userRank(u) {
		c.Role = tokenRole
	}
	if roleRank(tokenRole) < 3 {
		c.IsAdmin = false
	}
	return &c
}

// apiTokenSessionOnly lists account-security endpoints a token may not reach.
func apiTokenSessionOnly(path string) bool {
	return path == "/api/user/password" ||
		path == "/api/user/tokens" || strings.HasPrefix(path, "/api/user/tokens/") ||
		path == "/api/admin/tokens" || strings.HasPrefix(path, "/api/admin/tokens/") ||
		strings.HasPrefix(path, "/api/user/telegram/") ||
		strings.HasPrefix(path, "/api/2fa/")
}

// apiTokenScopeAllows reports whether a service-scoped token may call path.
// Besides the services' own endpoints, a scoped token gets the shared plumbing
// those services need (identity, projects, attachments) and the kanban
// endpoints serviceForPath doesn't list; everything else (settings, users,
// backups, …) needs an unscoped token.
func apiTokenScopeAllows(path string, services []string) bool {
	if len(services) == 0 {
		return true
	}
	switch {
	case path == "/api/auth/me", path == "/api/services",
		path == "/api/projects", strings.HasPrefix(path, "/api/projects/"),
		path == "/api/files", strings.HasPrefix(path, "/api/files/"),
		path == "/api/images", strings.HasPrefix(path, "/api/images/"):
		return true
	}
	svc := serviceForPath(path)
	if svc == "" {
		switch path {
		case "/api/board", "/api/search", "/api/subscribe", "/api/unsubscribe":
			svc = "kanban"
		}
	}
	for _, s := range services {
		if s == svc && svc != "" {
			return true
		}
	}
	return false
}

// bearerToken extracts the token from an "Authorization: Bearer …" header.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if len(h) < 7 || !strings.EqualFold(h[:7], "bearer ") {
		return "", false
	}
	tok := strings.TrimSpace(h[7:])
	return tok, tok != ""
}

// authenticateAPIToken resolves a bearer token for path. On failure it returns
// a nil user with the HTTP status and message to send.
func (h *Handler) authenticateAPIToken(token, path string) (*model.User, *model.APIToken, int, string) {
	owner, t, err := h.store.ValidateAPIToken(token)
	if err != nil {
		return nil, nil, http.StatusUnauthorized, "invalid or expired API token"
	}
	if apiTokenSessionOnly(path) {
		return nil, nil, http.StatusForbidden, "not available with an API token"
	}
	if !apiTokenScopeAllows(path, t.Services) {
		return nil, nil, http.StatusForbidden, "API token is not scoped for this service"
	}
	return clampToTokenRole(owner, t.Role), t, 0, ""
}

// viaAPIToken reports whether the request was authenticated with an API token.
func viaAPIToken(r *http.Request) bool {
	_, ok := r.Context().Value(apiTokenContextKey).(*model.APIToken)
	return ok
}

// sessionUser returns the current user for session-only endpoints, writing the
// error response itself when there is none or the request used a token.
func (h *Handler) sessionUser(w http.ResponseWriter, r *http.Request) *model.User {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}
	if viaAPIToken(r) {
		http.Error(w, "not available with an API token", http.StatusForbidden)
		return nil
	}
	return user
}

// handleUserTokens: GET lists the caller's tokens, POST creates one.
func (h *Handler) handleUserTokens(w http.ResponseWriter, r *http.Request) {
	user := h.sessionUser(w, r)
	if user == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := h.store.ListAPITokens(user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, list)
	case http.MethodPost:
		var req struct {
			Name        string   `json:"name"`
			Role        string   `json:"role"`
			Services    []string `json:"services"`
			ExpiresDays int      `json:"expires_days"` // 0 = never
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" || utf8.RuneCountInString(req.Name) > apiTokenMaxName {
			http.Error(w, "name is required (max 100 chars)", http.StatusBadRequest)
			return
		}
		if req.Role != "admin" && req.Role != "regular" && req.Role != "readonly" {
			http.Error(w, "role must be admin, regular or readonly", http.StatusBadRequest)
			return
		}
		if roleRank(req.Role) > userRank(user) {
			http.Error(w, "token role cannot exceed your own", http.StatusForbidden)
			return
		}
		services := []string{}
		seen := map[string]bool{}
		for _, s := range req.Services {
			s = strings.TrimSpace(s)
			if !serviceAvailable[s] {
				http.Error(w, "unknown service: "+s, http.StatusBadRequest)
				return
			}
			if !seen[s] {
				seen[s] = true
				services = append(services, s)
			}
		}
		if req.ExpiresDays < 0 || req.ExpiresDays > apiTokenMaxDays {
			http.Error(w, "expires_days must be 0..3650", http.StatusBadRequest)
			return
		}
		existing, err := h.store.ListAPITokens(user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(existing) >= apiTokenMaxPerUser {
			http.Error(w, "too many API tokens (max 50)", http.StatusConflict)
			return
		}
		var expires int64
		if req.ExpiresDays > 0 {
			expires = time.Now().Add(time.Duration(req.ExpiresDays) * 24 * time.Hour).Unix()
		}
		secret, t, err := h.store.CreateAPIToken(user.ID, req.Name, req.Role, services, expires)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.store.LogActivity(user.ID, "api_token_create", nil, t.Prefix+" "+t.Name+" ("+t.Role+")")
		jsonResp(w, map[string]any{"token": secret, "api_token": t})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleUserToken: DELETE /api/user/tokens/{id} revokes one of the caller's tokens.
func (h *Handler) handleUserToken(w http.ResponseWriter, r *http.Request) {
	user := h.sessionUser(w, r)
	if user == nil {
		return
	}
	h.revokeAPIToken(w, r, user, extractID(r.URL.Path, "/api/user/tokens/"), user.ID)
}

// handleAdminTokens: GET /api/admin/tokens lists every user's tokens.
func (h *Handler) handleAdminTokens(w http.ResponseWriter, r *http.Request) {
	user := h.sessionUser(w, r)
	if user == nil {
		return
	}
	if !user.IsAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	list, err := h.store.ListAPITokens(0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, list)
}

// handleAdminToken: DELETE /api/admin/tokens/{id} revokes any user's token.
func (h *Handler) handleAdminToken(w http.ResponseWriter, r *http.Request) {
	user := h.sessionUser(w, r)
	if user == nil {
		return
	}
	if !user.IsAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	h.revokeAPIToken(w, r, user, extractID(r.URL.Path, "/api/admin/tokens/"), 0)
}

func (h *Handler) revokeAPIToken(w http.ResponseWriter, r *http.Request, actor *model.User, id, ownerID int64) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	t, err := h.store.GetAPIToken(id)
	if err != nil || (ownerID != 0 && t.UserID != ownerID) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ok, err := h.store.DeleteAPIToken(id, ownerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	h.store.LogActivity(actor.ID, "api_token_revoke", nil, t.Prefix+" "+t.Name+" (@"+t.Username+")")
	jsonResp(w, map[string]string{"status": "ok"})
}
