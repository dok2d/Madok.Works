package handler

import (
	"encoding/json"
	"net/http"
)

// Ecosystem service registry.
//
// The kanban board is always present and is not listed here. Each optional
// service ships in its own phase: until its code is wired in, it stays out of
// serviceAvailable and is invisible to both the UI and the toggle API.
// Admin enable/disable state lives in app_settings as svc_<name>_enabled.

// serviceOrder defines the display order of optional services in the app rail.
var serviceOrder = []string{"wiki", "chat", "calendar", "mail"}

// serviceAvailable marks services whose implementation is compiled in.
var serviceAvailable = map[string]bool{
	"wiki":     true,
	"chat":     true,
	"calendar": true,
	"mail":     true,
}

// serviceDefaultOn marks services enabled by default once available.
var serviceDefaultOn = map[string]bool{
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
		list := make([]serviceInfo, 0, len(serviceOrder))
		for _, name := range serviceOrder {
			if serviceAvailable[name] {
				list = append(list, serviceInfo{Name: name, Enabled: h.serviceEnabled(name)})
			}
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
