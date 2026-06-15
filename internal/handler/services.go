package handler

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Ecosystem service registry.
//
// The kanban board is always present and is not listed here. Each optional
// service ships in its own phase: until its code is wired in, it stays out of
// serviceAvailable and is invisible to both the UI and the toggle API.
// Admin enable/disable state lives in app_settings as svc_<name>_enabled.

// serviceOrder is the DEFAULT display order. The kanban board is a toggleable
// service too (it can be disabled). A custom order is stored in app_settings
// (svc_order) and overrides this.
var serviceOrder = []string{"kanban", "wiki", "chat", "calendar", "mail"}

// serviceAvailable marks services whose implementation is compiled in.
var serviceAvailable = map[string]bool{
	"kanban":   true,
	"wiki":     true,
	"chat":     true,
	"calendar": true,
	"mail":     true,
}

// serviceDefaultOn marks services enabled by default once available.
var serviceDefaultOn = map[string]bool{
	"kanban":   true,
	"wiki":     true,
	"chat":     true,
	"calendar": true,
	"mail":     true, // internal mail works out of the box; external SMTP is optional
}

func (h *Handler) serviceEnabled(name string) bool {
	if !serviceAvailable[name] {
		return false
	}
	switch h.store.GetSetting("svc_" + name + "_enabled") {
	case "true":
		return true
	case "false":
		return false
	}
	return serviceDefaultOn[name]
}

// serviceOrderEffective returns the saved custom order (svc_order), with any
// available services missing from it appended in the default order.
func (h *Handler) serviceOrderEffective() []string {
	out := make([]string, 0, len(serviceOrder))
	seen := map[string]bool{}
	for _, n := range strings.Split(h.store.GetSetting("svc_order"), ",") {
		n = strings.TrimSpace(n)
		if n != "" && serviceAvailable[n] && !seen[n] {
			out = append(out, n)
			seen[n] = true
		}
	}
	for _, n := range serviceOrder {
		if serviceAvailable[n] && !seen[n] {
			out = append(out, n)
			seen[n] = true
		}
	}
	return out
}

// serviceForPath maps a request path to the owning service ("" if none), so a
// disabled service is enforced server-side, not just hidden in the UI.
func serviceForPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/wiki/"):
		return "wiki"
	case strings.HasPrefix(path, "/api/chat/"):
		return "chat"
	case strings.HasPrefix(path, "/api/calendar/"):
		return "calendar"
	case strings.HasPrefix(path, "/api/mail/"):
		return "mail"
	case path == "/api/board" ||
		strings.HasPrefix(path, "/api/columns") ||
		strings.HasPrefix(path, "/api/epics") ||
		strings.HasPrefix(path, "/api/sprints") ||
		strings.HasPrefix(path, "/api/tags") ||
		strings.HasPrefix(path, "/api/tasks") ||
		strings.HasPrefix(path, "/api/comments"):
		return "kanban"
	case strings.HasPrefix(path, "/wap/") && path != "/wap/login":
		return "kanban"
	}
	return ""
}

type serviceInfo struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// handleServices serves GET /api/services (any user) and POST /api/services
// (admin only) to toggle a service on or off.
func (h *Handler) handleServices(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		order := h.serviceOrderEffective()
		list := make([]serviceInfo, 0, len(order))
		for _, name := range order {
			list = append(list, serviceInfo{Name: name, Enabled: h.serviceEnabled(name)})
		}
		jsonResp(w, list)
	case http.MethodPost:
		if user.Role != "admin" {
			http.Error(w, "admin only", http.StatusForbidden)
			return
		}
		var req struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !serviceAvailable[req.Name] {
			http.Error(w, "unknown service", http.StatusBadRequest)
			return
		}
		val := "false"
		if req.Enabled {
			val = "true"
		}
		if err := h.store.SetSetting("svc_"+req.Name+"_enabled", val); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleServicesReorder (admin POST) persists a custom service order.
func (h *Handler) handleServicesReorder(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if user.Role != "admin" {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}
	var req struct {
		Order []string `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	valid := make([]string, 0, len(req.Order))
	seen := map[string]bool{}
	for _, n := range req.Order {
		if serviceAvailable[n] && !seen[n] {
			valid = append(valid, n)
			seen[n] = true
		}
	}
	if err := h.store.SetSetting("svc_order", strings.Join(valid, ",")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
}
