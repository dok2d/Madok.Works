package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// Simulates a DB created by an early build whose mail_messages lacked thread_id
// (and the SMTP fields). New() must upgrade it without error.
func TestMigrateLegacyMailMessages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE mail_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		from_user_id INTEGER,
		from_name TEXT NOT NULL DEFAULT '',
		subject TEXT NOT NULL DEFAULT '',
		body_text TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO mail_messages(from_name, subject) VALUES('alice','hi')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	raw.Close()

	s, err := New(path)
	if err != nil {
		t.Fatalf("New() on legacy db failed (migration regression): %v", err)
	}
	defer s.Close()

	// thread_id column was added and the dependent index built without error.
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_messages WHERE thread_id = 0`).Scan(&n); err != nil {
		t.Fatalf("thread_id missing after migrate: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 legacy row, got %d", n)
	}
	// SMTP columns also added (used by store_mail queries).
	if _, err := s.db.Exec(`SELECT from_addr, to_addrs, cc_addrs, in_reply_to, message_id FROM mail_messages LIMIT 1`); err != nil {
		t.Errorf("SMTP columns missing after migrate: %v", err)
	}
}
