package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"kanban/internal/model"
)

const projectMaxNameLen = 200

// projectID reads the current project from the ?project= query param (mirrors how
// wiki passes space_id). Defaults to 1 ("Общее").
func (h *Handler) projectID(r *http.Request) int64 {
	if v := r.URL.Query().Get("project"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	return 1
}

// projectRole returns the user's effective role in a project, capping global
// readonly users at viewer. Returns "" when the user has no access.
func (h *Handler) projectRole(user *model.User, projectID int64) (string, *model.Project, error) {
	p, err := h.store.GetProject(projectID, user.ID, user.Role == "admin")
	if err != nil {
		return "", nil, err
	}
	role := p.MyRole
	if user.Role == "readonly" && role != "" {
		role = "viewer"
	}
	p.MyRole = role
	return role, p, nil
}

// projectRoleAtLeast reuses the wiki role ranking ("" < viewer < editor < admin).
func projectRoleAtLeast(role, min string) bool { return wikiRoleAtLeast(role, min) }

// requireProject resolves the current project (?project=) and enforces a minimum
// role. On failure it writes the HTTP error and returns ok=false. Use "viewer"
// for reads, "editor" for writes, "admin" for settings.
func (h *Handler) requireProject(w http.ResponseWriter, r *http.Request, user *model.User, min string) (int64, bool) {
	pid := h.projectID(r)
	role, _, err := h.projectRole(user, pid)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "project not found", http.StatusNotFound)
		return 0, false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return 0, false
	}
	if !projectRoleAtLeast(role, min) {
		http.Error(w, "no access to this project", http.StatusForbidden)
		return 0, false
	}
	return pid, true
}

// requireProjectOf enforces a minimum role in the project that owns an entity.
// Unlike requireProject, the project id comes from the row (not ?project=), which
// is the ownership check for by-id operations (epics/sprints/tasks/tags).
func (h *Handler) requireProjectOf(w http.ResponseWriter, r *http.Request, user *model.User, projectID int64, min string) (string, bool) {
	role, _, err := h.projectRole(user, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "project not found", http.StatusNotFound)
		return "", false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return "", false
	}
	if !projectRoleAtLeast(role, min) {
		http.Error(w, "no access to this project", http.StatusForbidden)
		return "", false
	}
	return role, true
}

// columnInProject reports whether a column belongs to the given project.
func (h *Handler) columnInProject(columnID, projectID int64) bool {
	cols, err := h.store.ListColumns(projectID)
	if err != nil {
		return false
	}
	for _, c := range cols {
		if c.ID == columnID {
			return true
		}
	}
	return false
}

// tagInProject reports whether a tag belongs to the given project.
func (h *Handler) tagInProject(tagID, projectID int64) bool {
	tags, err := h.store.ListTags(projectID)
	if err != nil {
		return false
	}
	for _, t := range tags {
		if t.ID == tagID {
			return true
		}
	}
	return false
}

func (h *Handler) handleProjects(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		projects, err := h.store.ListProjects(user.ID, user.Role == "admin")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, projects)
	case http.MethodPost:
		if user.Role == "readonly" {
			http.Error(w, "read-only access", http.StatusForbidden)
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
		if req.Name == "" || len(req.Name) > projectMaxNameLen {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		id, err := h.store.CreateProject(req.Name, req.Description, req.Visibility, user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.store.LogActivityP(user.ID, "project_create", nil, &id, req.Name)
		jsonResp(w, map[string]int64{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleProjectRoute dispatches /api/projects/{id}[/members].
func (h *Handler) handleProjectRoute(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	parts := strings.SplitN(strings.TrimSuffix(rest, "/"), "/", 2)
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad project id", http.StatusBadRequest)
		return
	}
	role, p, err := h.projectRole(user, id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if role == "" {
		http.Error(w, "no access to this project", http.StatusForbidden)
		return
	}
	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}
	switch sub {
	case "":
		h.handleProject(w, r, user, p, role)
	case "members":
		h.handleProjectMembers(w, r, user, p, role)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleProject(w http.ResponseWriter, r *http.Request, user *model.User, p *model.Project, role string) {
	switch r.Method {
	case http.MethodGet:
		jsonResp(w, p)
	case http.MethodPut:
		if !projectRoleAtLeast(role, "admin") {
			http.Error(w, "project admin only", http.StatusForbidden)
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
		if req.Name == "" || len(req.Name) > projectMaxNameLen {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if err := h.store.UpdateProject(p.ID, req.Name, req.Description, req.Visibility); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if !projectRoleAtLeast(role, "admin") {
			http.Error(w, "project admin only", http.StatusForbidden)
			return
		}
		if err := h.store.DeleteProject(p.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.store.LogActivityP(user.ID, "project_delete", nil, &p.ID, p.Name)
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleProjectMembers(w http.ResponseWriter, r *http.Request, user *model.User, p *model.Project, role string) {
	switch r.Method {
	case http.MethodGet:
		members, err := h.store.ListProjectMembers(p.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, members)
	case http.MethodPost:
		if !projectRoleAtLeast(role, "admin") {
			http.Error(w, "project admin only", http.StatusForbidden)
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
		if err := h.store.SetProjectMember(p.ID, req.UserID, req.Role); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.store.LogActivityP(user.ID, "project_member", nil, &p.ID, h.userName(req.UserID)+" → "+req.Role)
		jsonResp(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if !projectRoleAtLeast(role, "admin") {
			http.Error(w, "project admin only", http.StatusForbidden)
			return
		}
		uid, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
		if uid <= 0 {
			http.Error(w, "user_id required", http.StatusBadRequest)
			return
		}
		if err := h.store.RemoveProjectMember(p.ID, uid); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.store.LogActivityP(user.ID, "project_member_remove", nil, &p.ID, h.userName(uid))
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
