package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// doBearer performs a request through the full middleware chain with an API token.
func (e *storageTestEnv) doBearer(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

// mintToken creates a token through the API with the given session.
func (e *storageTestEnv) mintToken(t *testing.T, sess string, req map[string]any) (string, int64) {
	t.Helper()
	w := e.do(t, "POST", "/api/user/tokens", sess, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create token %v: %d %s", req, w.Code, w.Body.String())
	}
	var resp struct {
		Token    string `json:"token"`
		APIToken struct {
			ID     int64  `json:"id"`
			Prefix string `json:"prefix"`
		} `json:"api_token"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Token) != 67 || resp.Token[:len(resp.APIToken.Prefix)] != resp.APIToken.Prefix {
		t.Fatalf("unexpected token/prefix: %q / %q", resp.Token, resp.APIToken.Prefix)
	}
	return resp.Token, resp.APIToken.ID
}

func expectCode(t *testing.T, w *httptest.ResponseRecorder, want int, what string) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("%s: got %d (%s), want %d", what, w.Code, w.Body.String(), want)
	}
}

func TestAPITokenRoleClamp(t *testing.T) {
	e := newStorageEnv(t)
	ro, _ := e.mintToken(t, e.admin, map[string]any{"name": "ro", "role": "readonly"})
	rw, _ := e.mintToken(t, e.admin, map[string]any{"name": "rw", "role": "regular"})
	adm, _ := e.mintToken(t, e.admin, map[string]any{"name": "adm", "role": "admin"})

	expectCode(t, e.doBearer(t, "GET", "/api/board", ro, nil), http.StatusOK, "readonly token reads")
	expectCode(t, e.doBearer(t, "POST", "/api/storage/folders", ro, map[string]any{"name": "x"}), http.StatusForbidden, "readonly token writes")
	expectCode(t, e.doBearer(t, "POST", "/api/storage/folders", rw, map[string]any{"name": "x"}), http.StatusOK, "regular token writes")
	expectCode(t, e.doBearer(t, "GET", "/api/admin/audit", rw, nil), http.StatusForbidden, "regular token on admin endpoint")
	expectCode(t, e.doBearer(t, "GET", "/api/admin/audit", adm, nil), http.StatusOK, "admin token on admin endpoint")

	// The owner's current role caps the token: demote alice and her admin token
	// loses admin rights on the very next request.
	if err := e.store.UpdateUserRole(e.adminID, "regular"); err != nil {
		t.Fatal(err)
	}
	expectCode(t, e.doBearer(t, "GET", "/api/admin/audit", adm, nil), http.StatusForbidden, "admin token of a demoted owner")
}

func TestAPITokenCannotExceedOwnerRole(t *testing.T) {
	e := newStorageEnv(t)
	expectCode(t, e.do(t, "POST", "/api/user/tokens", e.reader, map[string]any{"name": "x", "role": "regular"}),
		http.StatusForbidden, "readonly user minting a regular token")
	tok, _ := e.mintToken(t, e.reader, map[string]any{"name": "x", "role": "readonly"})
	expectCode(t, e.doBearer(t, "GET", "/api/board", tok, nil), http.StatusOK, "readonly user's token reads")
	for _, bad := range []map[string]any{
		{"name": "", "role": "readonly"},
		{"name": "x", "role": "root"},
		{"name": "x", "role": "readonly", "services": []string{"nope"}},
		{"name": "x", "role": "readonly", "expires_days": -1},
	} {
		expectCode(t, e.do(t, "POST", "/api/user/tokens", e.admin, bad), http.StatusBadRequest, fmt.Sprint(bad))
	}
}

func TestAPITokenSessionOnlyEndpoints(t *testing.T) {
	e := newStorageEnv(t)
	tok, _ := e.mintToken(t, e.admin, map[string]any{"name": "adm", "role": "admin"})
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/user/tokens"},
		{"POST", "/api/user/tokens"},
		{"DELETE", "/api/user/tokens/1"},
		{"GET", "/api/admin/tokens"},
		{"POST", "/api/user/password"},
		{"POST", "/api/user/telegram/unlink"},
		{"GET", "/api/2fa/status"},
	} {
		expectCode(t, e.doBearer(t, c.method, c.path, tok, map[string]any{"name": "x", "role": "readonly", "password": "zzzzzzzz"}),
			http.StatusForbidden, c.method+" "+c.path)
	}
}

func TestAPITokenServiceScope(t *testing.T) {
	e := newStorageEnv(t)
	tok, _ := e.mintToken(t, e.admin, map[string]any{"name": "files", "role": "admin", "services": []string{"files"}})
	expectCode(t, e.doBearer(t, "GET", "/api/storage/tree", tok, nil), http.StatusOK, "in-scope service")
	expectCode(t, e.doBearer(t, "GET", "/api/auth/me", tok, nil), http.StatusOK, "shared identity endpoint")
	expectCode(t, e.doBearer(t, "GET", "/api/board", tok, nil), http.StatusForbidden, "kanban with a files-only token")
	expectCode(t, e.doBearer(t, "GET", "/api/wiki/spaces", tok, nil), http.StatusForbidden, "wiki with a files-only token")
	expectCode(t, e.doBearer(t, "GET", "/api/admin/audit", tok, nil), http.StatusForbidden, "admin endpoint with a scoped token")

	kb, _ := e.mintToken(t, e.admin, map[string]any{"name": "kb", "role": "readonly", "services": []string{"kanban"}})
	expectCode(t, e.doBearer(t, "GET", "/api/board", kb, nil), http.StatusOK, "kanban board with a kanban token")
	expectCode(t, e.doBearer(t, "GET", "/api/tasks", kb, nil), http.StatusOK, "kanban tasks with a kanban token")
}

func TestAPITokenRevokeAndExpiry(t *testing.T) {
	e := newStorageEnv(t)
	tok, id := e.mintToken(t, e.admin, map[string]any{"name": "t", "role": "readonly"})
	expectCode(t, e.doBearer(t, "GET", "/api/board", tok, nil), http.StatusOK, "fresh token")

	// Someone else can't revoke it; its owner can.
	expectCode(t, e.do(t, "DELETE", fmt.Sprintf("/api/user/tokens/%d", id), e.reader, nil), http.StatusNotFound, "foreign revoke")
	expectCode(t, e.doBearer(t, "GET", "/api/board", tok, nil), http.StatusOK, "token after a refused foreign revoke")
	expectCode(t, e.do(t, "DELETE", fmt.Sprintf("/api/user/tokens/%d", id), e.admin, nil), http.StatusOK, "owner revoke")
	expectCode(t, e.doBearer(t, "GET", "/api/board", tok, nil), http.StatusUnauthorized, "revoked token")

	// Admin revokes another user's token from the admin endpoint.
	rtok, rid := e.mintToken(t, e.reader, map[string]any{"name": "r", "role": "readonly"})
	expectCode(t, e.do(t, "GET", "/api/admin/tokens", e.reader, nil), http.StatusForbidden, "non-admin lists all tokens")
	expectCode(t, e.do(t, "DELETE", fmt.Sprintf("/api/admin/tokens/%d", rid), e.admin, nil), http.StatusOK, "admin revoke")
	expectCode(t, e.doBearer(t, "GET", "/api/board", rtok, nil), http.StatusUnauthorized, "admin-revoked token")

	expired, _, err := e.store.CreateAPIToken(e.adminID, "old", "readonly", nil, time.Now().Add(-time.Minute).Unix())
	if err != nil {
		t.Fatal(err)
	}
	expectCode(t, e.doBearer(t, "GET", "/api/board", expired, nil), http.StatusUnauthorized, "expired token")
	expectCode(t, e.doBearer(t, "GET", "/api/board", "mw_"+fmt.Sprintf("%064d", 0), nil), http.StatusUnauthorized, "unknown token")
	expectCode(t, e.doBearer(t, "GET", "/api/board", "garbage", nil), http.StatusUnauthorized, "malformed token")
}

func TestAPITokenListHidesSecret(t *testing.T) {
	e := newStorageEnv(t)
	tok, _ := e.mintToken(t, e.admin, map[string]any{"name": "t", "role": "readonly", "expires_days": 30})
	w := e.do(t, "GET", "/api/user/tokens", e.admin, nil)
	expectCode(t, w, http.StatusOK, "list")
	if bytes.Contains(w.Body.Bytes(), []byte(tok)) || bytes.Contains(w.Body.Bytes(), []byte(tok[11:])) {
		t.Fatalf("token list leaks the secret: %s", w.Body.String())
	}
	var list []struct {
		Name      string `json:"name"`
		ExpiresAt int64  `json:"expires_at"`
	}
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Name != "t" || list[0].ExpiresAt < time.Now().Add(29*24*time.Hour).Unix() {
		t.Fatalf("unexpected list: %s", w.Body.String())
	}
}

func TestAPITokenScopedAttachments(t *testing.T) {
	e := newStorageEnv(t)
	item, err := e.store.GetStorageItem(e.upload(t, "doc.pdf", []byte("%PDF-fake"), 0))
	if err != nil {
		t.Fatal(err)
	}
	blob := fmt.Sprintf("/api/files/%d", item.FileID)
	files, _ := e.mintToken(t, e.admin, map[string]any{"name": "f", "role": "readonly", "services": []string{"files"}})
	wiki, _ := e.mintToken(t, e.admin, map[string]any{"name": "w", "role": "readonly", "services": []string{"wiki"}})
	all, _ := e.mintToken(t, e.admin, map[string]any{"name": "a", "role": "readonly"})
	expectCode(t, e.doBearer(t, "GET", blob, files, nil), http.StatusOK, "files token reads a Files blob")
	expectCode(t, e.doBearer(t, "GET", blob, all, nil), http.StatusOK, "unscoped token reads any blob")
	expectCode(t, e.doBearer(t, "GET", blob, wiki, nil), http.StatusNotFound, "wiki token reads a Files blob")
}
