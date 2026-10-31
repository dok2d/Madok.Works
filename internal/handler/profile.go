package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"
)

// === Public profile: display name + avatar ===
//
// The login stays the identity (sign-in, @mentions, mail localpart); the
// display name is what other people see. Avatars are small square images the
// client crops and scales before upload; the server only validates them.

const (
	displayNameMax  = 64
	avatarMaxBytes  = 512 * 1024
	avatarMaxPixels = 1024
)

// cleanDisplayName trims the name and rejects control characters and overlong
// names; "" is valid and means "show the login".
func cleanDisplayName(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > displayNameMax {
		return "", false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return s, true
}

// decodeAvatar validates an uploaded avatar: a PNG or JPEG of sane size.
func decodeAvatar(b64 string) ([]byte, string, int, string) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, "", http.StatusBadRequest, "invalid base64"
	}
	if len(raw) > avatarMaxBytes {
		return nil, "", http.StatusRequestEntityTooLarge, "avatar too large (max 512 KB)"
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, "", http.StatusBadRequest, "avatar must be a PNG or JPEG image"
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > avatarMaxPixels || cfg.Height > avatarMaxPixels {
		return nil, "", http.StatusBadRequest, "avatar must be at most 1024×1024"
	}
	return raw, "image/" + format, 0, ""
}

// handleUserProfile: PUT /api/user/profile {display_name} edits your own profile.
func (h *Handler) handleUserProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name, ok := cleanDisplayName(req.DisplayName)
	if !ok {
		http.Error(w, "display name: up to 64 characters, no control characters", http.StatusBadRequest)
		return
	}
	if err := h.store.UpdateUserDisplayName(user.ID, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.writeUser(w, user.ID)
}

// handleUserAvatar: POST /api/user/avatar {data} sets, DELETE removes your avatar.
func (h *Handler) handleUserAvatar(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Data string `json:"data"` // base64 PNG/JPEG
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		raw, mime, status, msg := decodeAvatar(req.Data)
		if raw == nil {
			http.Error(w, msg, status)
			return
		}
		if _, err := h.store.SetUserAvatar(user.ID, raw, mime); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case http.MethodDelete:
		if err := h.store.ClearUserAvatar(user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.writeUser(w, user.ID)
}

// writeUser responds with a user's public record (as in GET /api/users).
func (h *Handler) writeUser(w http.ResponseWriter, id int64) {
	u, err := h.store.GetUser(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	u.LinkHash = ""
	jsonResp(w, u)
}
