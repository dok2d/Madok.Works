package handler

import (
	"encoding/json"
	"net/http"
)

// handleOnboarding drives the one-time, instance-level first-run wizard. The
// admin picks how to preconfigure services; the choice is applied and the
// onboarding_done flag is set so the wizard never shows again (it can be
// re-opened from settings by clearing the flag).
func (h *Handler) handleOnboarding(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		jsonResp(w, map[string]bool{"done": h.store.GetSetting("onboarding_done") == "true"})
	case http.MethodPost:
		if user.Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var req struct {
			Mode     string          `json:"mode"`     // "personal" | "team" | "backup"
			Services map[string]bool `json:"services"` // team mode: explicit per-service flags
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		set := func(name string, on bool) {
			v := "false"
			if on {
				v = "true"
			}
			h.store.SetSetting("svc_"+name+"_enabled", v)
		}
		switch req.Mode {
		case "personal":
			// Solo use: kanban + calendar + wiki on; chat + mail off.
			set("kanban", true)
			set("calendar", true)
			set("wiki", true)
			set("chat", false)
			set("mail", false)
		case "team":
			for _, name := range []string{"kanban", "wiki", "chat", "calendar", "mail"} {
				if v, ok := req.Services[name]; ok {
					set(name, v)
				}
			}
		case "backup":
			// Nothing to preconfigure; the user will restore via the import flow.
		}
		h.store.SetSetting("onboarding_done", "true")
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}