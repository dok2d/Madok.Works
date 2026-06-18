package handler

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"strings"

	"kanban/internal/auth"
)

// generateRecoveryCodes returns n short, human-friendly single-use codes.
func generateRecoveryCodes(n int) []string {
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		buf := make([]byte, 6)
		rand.Read(buf)
		s := strings.ToLower(enc.EncodeToString(buf)) // ~10 chars
		out = append(out, s[:5]+"-"+s[5:])
	}
	return out
}

// handle2FA dispatches /api/2fa/{status|setup|enable|disable}.
func (h *Handler) handle2FA(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	action := strings.TrimPrefix(r.URL.Path, "/api/2fa/")
	switch action {
	case "status":
		_, enabled, _ := h.store.GetTOTP(user.ID)
		jsonResp(w, map[string]bool{"enabled": enabled})
	case "setup":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		secret, err := auth.GenerateTOTPSecret()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Store as pending (not yet enabled) until the user confirms a code.
		if err := h.store.SetTOTPSecret(user.ID, secret, false); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		uri := auth.TOTPURI(h.appName(), user.Username, secret)
		jsonResp(w, map[string]string{"secret": secret, "uri": uri})
	case "enable":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Code string `json:"code"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		secret, _, _ := h.store.GetTOTP(user.ID)
		if secret == "" || !auth.VerifyTOTP(secret, req.Code) {
			http.Error(w, "invalid code", http.StatusBadRequest)
			return
		}
		if err := h.store.SetTOTPSecret(user.ID, secret, true); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		codes := generateRecoveryCodes(10)
		if err := h.store.ReplaceRecoveryCodes(user.ID, codes); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.store.LogActivity(user.ID, "2fa_enable", nil, "")
		jsonResp(w, map[string]any{"status": "ok", "recovery_codes": codes})
	case "disable":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Code string `json:"code"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		secret, enabled, _ := h.store.GetTOTP(user.ID)
		if enabled && !auth.VerifyTOTP(secret, req.Code) && !h.store.UseRecoveryCode(user.ID, req.Code) {
			http.Error(w, "invalid code", http.StatusBadRequest)
			return
		}
		if err := h.store.DisableTOTP(user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.store.LogActivity(user.ID, "2fa_disable", nil, "")
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.NotFound(w, r)
	}
}
