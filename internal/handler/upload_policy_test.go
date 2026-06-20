package handler

import (
	"path/filepath"
	"testing"

	"kanban/internal/db"
)

func TestFileTypeAllowedPolicy(t *testing.T) {
	store, err := db.New(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	defer store.Close()
	h := &Handler{store: store}

	check := func(name, mime string, want bool) {
		t.Helper()
		if ok, _ := h.fileTypeAllowed(name, mime); ok != want {
			t.Errorf("fileTypeAllowed(%q,%q)=%v, want %v", name, mime, ok, want)
		}
	}

	// Default policy: MIME allowlist + extension blocklist.
	check("a.pdf", "application/pdf", true)
	check("a.png", "image/png", true)
	check("a.exe", "application/octet-stream", false) // blocked extension
	check("a.docx", "application/octet-stream", false) // MIME not allowlisted

	// Allow any.
	store.SetSetting("upload_allowed", "*")
	check("a.exe", "application/octet-stream", true)
	check("a.docx", "application/octet-stream", true)

	// Custom extension allowlist (MIME ignored).
	store.SetSetting("upload_allowed", "pdf, docx, zip")
	check("a.docx", "application/octet-stream", true)
	check("a.pdf", "application/pdf", true)
	check("a.png", "image/png", false) // not in the list
	check("a.exe", "application/octet-stream", false)
}
