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

// A very old DB whose calendar_events lacks calendar_id must not crash startup:
// an index over a missing column is skipped rather than aborting migration.
func TestMigrateLegacyCalendarEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy_cal.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE calendar_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	raw.Close()

	s, err := New(path)
	if err != nil {
		t.Fatalf("New() on legacy calendar db failed (migration regression): %v", err)
	}
	s.Close()
}

// A DB from before the multi-project schema has kanban tables without
// project_id and tasks without number. New() must upgrade it in place: rows land
// in the default project and tasks get per-project numbers by id.
func TestMigratePreProjectsKanban(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preprojects.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, q := range []string{
		`CREATE TABLE columns (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, position INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE tags (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, color TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			column_id INTEGER NOT NULL REFERENCES columns(id),
			epic_id INTEGER,
			position INTEGER NOT NULL DEFAULT 0,
			priority INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE wiki_spaces (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, key TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO columns(name, position) VALUES('Todo', 0), ('Done', 1)`,
		`INSERT INTO tags(name) VALUES('bug')`,
		`INSERT INTO tasks(title, column_id) VALUES('a', 1), ('b', 2), ('c', 1)`,
		`INSERT INTO wiki_spaces(name) VALUES('Docs')`,
	} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("seed legacy schema: %v\n%s", err, q)
		}
	}
	raw.Close()

	s, err := New(path)
	if err != nil {
		t.Fatalf("New() on pre-projects db failed (migration regression): %v", err)
	}
	defer s.Close()

	cols, err := s.ListColumns(1)
	if err != nil || len(cols) != 2 {
		t.Fatalf("legacy columns not in default project: %v, %d cols", err, len(cols))
	}
	tasks, err := s.ListTasks(1)
	if err != nil || len(tasks) != 3 {
		t.Fatalf("legacy tasks not in default project: %v, %d tasks", err, len(tasks))
	}
	rows, err := s.db.Query(`SELECT id, number FROM tasks ORDER BY id`)
	if err != nil {
		t.Fatalf("query numbers: %v", err)
	}
	defer rows.Close()
	want := int64(1)
	for rows.Next() {
		var id, num int64
		if err := rows.Scan(&id, &num); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if num != want {
			t.Errorf("task %d: number = %d, want %d", id, num, want)
		}
		want++
	}
	var spaceProject sql.NullInt64
	if err := s.db.QueryRow(`SELECT project_id FROM wiki_spaces`).Scan(&spaceProject); err != nil || spaceProject.Int64 != 1 {
		t.Errorf("legacy wiki space project_id = %v (err %v), want 1", spaceProject, err)
	}
	// The project-scoped index skipped on the first pass is built after the upgrade.
	var idx int
	s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_tasks_project'`).Scan(&idx)
	if idx != 1 {
		t.Errorf("idx_tasks_project not created after upgrade")
	}
}
