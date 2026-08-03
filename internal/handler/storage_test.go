package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"kanban/internal/db"
)

// storageTestEnv spins up a full Handler (ServeHTTP path, so the auth,
// readonly and service gates all apply) with one admin and one readonly user.
type storageTestEnv struct {
	h       *Handler
	store   *db.Store
	admin   string // session cookies
	reader  string
	adminID int64
}

func newStorageEnv(t *testing.T) *storageTestEnv {
	t.Helper()
	store, err := db.New(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	adminID, err := store.CreateUser("alice", "password123", "admin")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	readerID, _ := store.CreateUser("bob", "password123", "readonly")
	adminSess, _ := store.CreateSession(adminID)
	readerSess, _ := store.CreateSession(readerID)
	return &storageTestEnv{
		h:       New(store, t.TempDir()),
		store:   store,
		admin:   adminSess,
		reader:  readerSess,
		adminID: adminID,
	}
}

// do performs a request through the full middleware chain. sess "" = no cookie.
func (e *storageTestEnv) do(t *testing.T, method, path, sess string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	if sess != "" {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: sess})
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

func (e *storageTestEnv) upload(t *testing.T, name string, data []byte, folderID int64) int64 {
	t.Helper()
	w := e.do(t, "POST", "/api/storage/upload", e.admin, map[string]any{
		"data": base64.StdEncoding.EncodeToString(data), "filename": name,
		"mime": "application/pdf", "folder_id": folderID,
	})
	if w.Code != 200 {
		t.Fatalf("upload %s: %d %s", name, w.Code, w.Body.String())
	}
	var resp struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.ID
}

func TestStorageAuthAndReadonly(t *testing.T) {
	e := newStorageEnv(t)

	// No session → 401 on the API.
	if w := e.do(t, "GET", "/api/storage/list", "", nil); w.Code != http.StatusUnauthorized {
		t.Errorf("unauth list: want 401, got %d", w.Code)
	}

	id := e.upload(t, "doc.pdf", []byte("%PDF-fake"), 0)

	// Readonly user: GET works, writes are rejected by the global gate.
	if w := e.do(t, "GET", "/api/storage/list", e.reader, nil); w.Code != 200 {
		t.Errorf("readonly list: want 200, got %d", w.Code)
	}
	if w := e.do(t, "GET", fmt.Sprintf("/api/storage/items/%d/download", id), e.reader, nil); w.Code != 200 {
		t.Errorf("readonly download: want 200, got %d", w.Code)
	}
	if w := e.do(t, "POST", "/api/storage/folders", e.reader, map[string]any{"name": "X"}); w.Code != http.StatusForbidden {
		t.Errorf("readonly mkdir: want 403, got %d", w.Code)
	}
	if w := e.do(t, "DELETE", fmt.Sprintf("/api/storage/items/%d", id), e.reader, nil); w.Code != http.StatusForbidden {
		t.Errorf("readonly delete: want 403, got %d", w.Code)
	}
}

func TestStorageServiceDisabled(t *testing.T) {
	e := newStorageEnv(t)
	id := e.upload(t, "doc.pdf", []byte("data"), 0)

	// Mint a public link while enabled.
	w := e.do(t, "POST", fmt.Sprintf("/api/storage/items/%d/share", id), e.admin, nil)
	if w.Code != 200 {
		t.Fatalf("share: %d %s", w.Code, w.Body.String())
	}
	var share struct {
		URL string `json:"url"`
	}
	json.Unmarshal(w.Body.Bytes(), &share)

	// Disable the service: API answers 403 (global gate), public link 404.
	e.store.SetSetting("svc_files_enabled", "false")
	if w := e.do(t, "GET", "/api/storage/list", e.admin, nil); w.Code != http.StatusForbidden {
		t.Errorf("disabled list: want 403, got %d", w.Code)
	}
	if w := e.do(t, "GET", share.URL, "", nil); w.Code != http.StatusNotFound {
		t.Errorf("disabled public link: want 404, got %d", w.Code)
	}

	e.store.SetSetting("svc_files_enabled", "true")
	if w := e.do(t, "GET", share.URL, "", nil); w.Code != 200 {
		t.Errorf("re-enabled public link: want 200, got %d", w.Code)
	}
}

func TestStoragePublicToken(t *testing.T) {
	e := newStorageEnv(t)
	id := e.upload(t, "секретный отчёт.pdf", []byte("data-123"), 0)

	w := e.do(t, "POST", fmt.Sprintf("/api/storage/items/%d/share", id), e.admin, nil)
	var share struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	json.Unmarshal(w.Body.Bytes(), &share)
	if len(share.Token) < 40 {
		t.Fatalf("token too short: %q", share.Token)
	}

	// Public link works WITHOUT a cookie and serves as attachment.
	w = e.do(t, "GET", share.URL, "", nil)
	if w.Code != 200 || w.Body.String() != "data-123" {
		t.Fatalf("public download: %d %q", w.Code, w.Body.String())
	}
	if cd := w.Header().Get("Content-Disposition"); cd == "" || !bytes.Contains([]byte(cd), []byte("attachment")) {
		t.Errorf("public download must be an attachment, got %q", cd)
	}

	// Wrong token → 404.
	if w := e.do(t, "GET", "/api/storage/public/nosuchtoken", "", nil); w.Code != http.StatusNotFound {
		t.Errorf("wrong token: want 404, got %d", w.Code)
	}

	// Revoke → 404.
	if w := e.do(t, "DELETE", fmt.Sprintf("/api/storage/items/%d/share", id), e.admin, nil); w.Code != 200 {
		t.Fatalf("revoke: %d", w.Code)
	}
	if w := e.do(t, "GET", share.URL, "", nil); w.Code != http.StatusNotFound {
		t.Errorf("revoked token: want 404, got %d", w.Code)
	}
}

func TestStoragePreviewMimeGate(t *testing.T) {
	e := newStorageEnv(t)
	// Allow any type so an HTML file can be uploaded at all.
	e.store.SetSetting("upload_allowed", "*")

	up := func(name, mime string, data []byte) int64 {
		w := e.do(t, "POST", "/api/storage/upload", e.admin, map[string]any{
			"data": base64.StdEncoding.EncodeToString(data), "filename": name, "mime": mime,
		})
		if w.Code != 200 {
			t.Fatalf("upload %s: %d %s", name, w.Code, w.Body.String())
		}
		var resp struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		return resp.ID
	}

	txt := up("notes.txt", "text/plain", []byte("hello"))
	html := up("evil.bin", "text/html", []byte("<script>alert(1)</script>"))

	w := e.do(t, "GET", fmt.Sprintf("/api/storage/items/%d/preview", txt), e.admin, nil)
	if w.Code != 200 || !bytes.Contains([]byte(w.Header().Get("Content-Disposition")), []byte("inline")) {
		t.Errorf("text preview should be inline: %d %q", w.Code, w.Header().Get("Content-Disposition"))
	}

	// HTML must never render inline — attachment even via /preview.
	w = e.do(t, "GET", fmt.Sprintf("/api/storage/items/%d/preview", html), e.admin, nil)
	if !bytes.Contains([]byte(w.Header().Get("Content-Disposition")), []byte("attachment")) {
		t.Errorf("html preview must fall back to attachment, got %q", w.Header().Get("Content-Disposition"))
	}
}

func TestStorageFolderFlow(t *testing.T) {
	e := newStorageEnv(t)

	w := e.do(t, "POST", "/api/storage/folders", e.admin, map[string]any{"name": "Docs"})
	if w.Code != 200 {
		t.Fatalf("mkdir: %d %s", w.Code, w.Body.String())
	}
	var f struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &f)

	// Duplicate name → 409.
	if w := e.do(t, "POST", "/api/storage/folders", e.admin, map[string]any{"name": "docs"}); w.Code != http.StatusConflict {
		t.Errorf("dup mkdir: want 409, got %d", w.Code)
	}

	id := e.upload(t, "a.pdf", []byte("x"), f.ID)

	// List of the folder shows the item; root doesn't.
	w = e.do(t, "GET", fmt.Sprintf("/api/storage/list?folder=%d", f.ID), e.admin, nil)
	var list struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 1 || list.Items[0].ID != id {
		t.Fatalf("folder list: %s", w.Body.String())
	}

	// Deleting the folder removes the item and its blob.
	if w := e.do(t, "DELETE", fmt.Sprintf("/api/storage/folders/%d", f.ID), e.admin, nil); w.Code != 200 {
		t.Fatalf("rmdir: %d", w.Code)
	}
	if w := e.do(t, "GET", fmt.Sprintf("/api/storage/items/%d/download", id), e.admin, nil); w.Code != http.StatusNotFound {
		t.Errorf("item of deleted folder: want 404, got %d", w.Code)
	}
}
