package handler

import (
	"path/filepath"
	"strings"
	"testing"

	"kanban/internal/db"
)

// TestIngestInbound verifies that a raw MIME message received over SMTP is parsed
// and delivered to the matching local user's inbox, including an attachment.
func TestIngestInbound(t *testing.T) {
	store, err := db.New(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	defer store.Close()
	bobID, err := store.CreateUser("bob", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	h := &Handler{store: store}

	raw := strings.ReplaceAll(`From: Alice <alice@example.com>
To: bob@kanban.local
Subject: Hello over SMTP
Content-Type: multipart/mixed; boundary=BOUND

--BOUND
Content-Type: text/plain; charset=utf-8

This is the body.
--BOUND
Content-Type: text/plain; name="note.txt"
Content-Disposition: attachment; filename="note.txt"

attached file contents
--BOUND--
`, "\n", "\r\n")

	if err := h.ingestInbound("alice@example.com", []string{"bob@kanban.local"}, strings.NewReader(raw), "1.2.3.4:5555"); err != nil {
		t.Fatalf("ingestInbound: %v", err)
	}

	inbox, err := store.ListMailbox(bobID, "inbox", 50, 0)
	if err != nil {
		t.Fatalf("ListMailbox: %v", err)
	}
	if len(inbox) != 1 {
		t.Fatalf("expected 1 inbox message, got %d", len(inbox))
	}
	full, _ := store.GetMailBox(bobID, inbox[0].BoxID)
	if full.Subject != "Hello over SMTP" {
		t.Errorf("subject = %q", full.Subject)
	}
	if !strings.Contains(full.BodyText, "This is the body") {
		t.Errorf("body = %q", full.BodyText)
	}
	if full.FromAddr != "alice@example.com" {
		t.Errorf("from = %q", full.FromAddr)
	}
	if len(full.Attachments) != 1 || full.Attachments[0].Filename != "note.txt" {
		t.Errorf("attachments = %+v", full.Attachments)
	}

	// unknown recipient -> rejected
	if err := h.ingestInbound("x@y.z", []string{"nobody@kanban.local"}, strings.NewReader(raw), ""); err == nil {
		t.Error("expected error for unknown recipient")
	}

	// audit recorded both an accept and a reject
	rows, _ := store.ListMailAudit("", 50, 0)
	var accepts, rejects int
	for _, a := range rows {
		if a.Event == "inbound_accept" {
			accepts++
		}
		if a.Event == "inbound_reject" {
			rejects++
		}
	}
	if accepts != 1 || rejects != 1 {
		t.Errorf("audit: accepts=%d rejects=%d (want 1/1)", accepts, rejects)
	}
}
