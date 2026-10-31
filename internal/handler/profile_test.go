package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"strings"
	"testing"
)

func pngB64(t *testing.T, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestProfileEndpoints(t *testing.T) {
	e := newStorageEnv(t)

	expectCode(t, e.do(t, "PUT", "/api/user/profile", e.admin, map[string]any{"display_name": "Алиса Л."}), http.StatusOK, "set own name")
	expectCode(t, e.do(t, "PUT", "/api/user/profile", e.admin, map[string]any{"display_name": strings.Repeat("я", 65)}), http.StatusBadRequest, "too long")
	expectCode(t, e.do(t, "PUT", "/api/user/profile", e.admin, map[string]any{"display_name": "a\x07b"}), http.StatusBadRequest, "control char")

	w := e.do(t, "POST", "/api/user/avatar", e.admin, map[string]any{"data": pngB64(t, 64, 64)})
	expectCode(t, w, http.StatusOK, "upload avatar")
	var me struct {
		DisplayName string `json:"display_name"`
		AvatarID    int64  `json:"avatar_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &me)
	if me.DisplayName != "Алиса Л." || me.AvatarID == 0 {
		t.Fatalf("profile after upload: %+v", me)
	}
	expectCode(t, e.do(t, "GET", fmt.Sprintf("/api/images/%d", me.AvatarID), e.reader, nil), http.StatusOK, "others see the avatar")
	expectCode(t, e.do(t, "POST", "/api/user/avatar", e.admin, map[string]any{"data": base64.StdEncoding.EncodeToString([]byte("not an image"))}), http.StatusBadRequest, "non-image avatar")
	expectCode(t, e.do(t, "POST", "/api/user/avatar", e.admin, map[string]any{"data": pngB64(t, 2000, 10)}), http.StatusBadRequest, "oversized avatar")

	// The directory and /api/auth/me carry the profile.
	w = e.do(t, "GET", "/api/users", e.reader, nil)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"display_name":"Алиса Л."`)) {
		t.Fatalf("directory lacks the display name: %s", w.Body.String())
	}

	// A read-only user may edit their own profile; a read-only token may not.
	expectCode(t, e.do(t, "PUT", "/api/user/profile", e.reader, map[string]any{"display_name": "Bob"}), http.StatusOK, "readonly user edits own profile")
	ro, _ := e.mintToken(t, e.admin, map[string]any{"name": "ro", "role": "readonly"})
	expectCode(t, e.doBearer(t, "PUT", "/api/user/profile", ro, map[string]any{"display_name": "x"}), http.StatusForbidden, "readonly token edits profile")
	expectCode(t, e.doBearer(t, "DELETE", "/api/user/avatar", ro, nil), http.StatusForbidden, "readonly token drops avatar")

	// /api/auth/me reports the role as narrowed by the token.
	w = e.doBearer(t, "GET", "/api/auth/me", ro, nil)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"role":"readonly"`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"is_admin":false`)) {
		t.Fatalf("auth/me with a readonly token: %s", w.Body.String())
	}

	// Admins rename others and clear their avatar; others can't.
	readerID := mustReaderID(t, e)
	expectCode(t, e.do(t, "PUT", fmt.Sprintf("/api/users/%d", readerID), e.admin, map[string]any{"display_name": "Robert", "clear_avatar": true}), http.StatusOK, "admin renames")
	expectCode(t, e.do(t, "PUT", fmt.Sprintf("/api/users/%d", e.adminID), e.reader, map[string]any{"display_name": "pwned"}), http.StatusForbidden, "non-admin renames another")
	if u, _ := e.store.GetUser(readerID); u.DisplayName != "Robert" {
		t.Fatalf("admin rename not applied: %q", u.DisplayName)
	}
	if u, _ := e.store.GetUser(e.adminID); u.DisplayName != "Алиса Л." {
		t.Fatalf("non-admin rename leaked through: %q", u.DisplayName)
	}

	expectCode(t, e.do(t, "DELETE", "/api/user/avatar", e.admin, nil), http.StatusOK, "remove avatar")
	expectCode(t, e.do(t, "GET", fmt.Sprintf("/api/images/%d", me.AvatarID), e.admin, nil), http.StatusNotFound, "removed avatar image")
}

func mustReaderID(t *testing.T, e *storageTestEnv) int64 {
	t.Helper()
	u, err := e.store.GetUserByUsername("bob")
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}
