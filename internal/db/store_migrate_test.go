package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"kanban/internal/model"
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

// Reproduces a DB from the earliest ecosystem builds (as found on a real
// instance): chat_messages, calendar_events and mail_messages with unrelated
// legacy layouts (empty), and chat_channels/chat_members missing later columns
// but holding a DM. Startup must rebuild the empty tables, keep the DM, and
// leave chat, calendar and mail able to store data.
func TestMigrateEarlyEcosystemLayouts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "early.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, q := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL, is_admin INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')))`,
		`INSERT INTO users(id, username, password_hash) VALUES(1,'alice','x'),(3,'bob','x')`,
		`CREATE TABLE chat_channels (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'group', created_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')))`,
		`INSERT INTO chat_channels(id, name, type, created_by) VALUES(1,'','dm',1)`,
		`CREATE TABLE chat_members (channel_id INTEGER NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, last_read_id INTEGER NOT NULL DEFAULT 0,
			joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (channel_id, user_id))`,
		`INSERT INTO chat_members(channel_id, user_id) VALUES(1,1),(1,3)`,
		`CREATE TABLE chat_messages (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL,
			text TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT (datetime('now')), reply_to INTEGER,
			pinned INTEGER NOT NULL DEFAULT 0, pinned_at DATETIME, pinned_by INTEGER, kind TEXT)`,
		`CREATE TABLE calendar_events (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL,
			title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', start_date TEXT NOT NULL, end_date TEXT,
			start_time TEXT, end_time TEXT, color TEXT, created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			is_shared INTEGER NOT NULL DEFAULT 0, recurrence TEXT, reminder_min INTEGER, reminded INTEGER)`,
		`CREATE TABLE mail_messages (id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			to_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, subject TEXT NOT NULL DEFAULT '',
			body TEXT NOT NULL DEFAULT '', is_read INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')))`,
	} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q[:40], err)
		}
	}
	raw.Close()

	s, err := New(path)
	if err != nil {
		t.Fatalf("New() on early ecosystem db: %v", err)
	}
	defer s.Close()

	// No index was left behind for a missing column.
	for _, idx := range []string{"idx_chat_messages_channel_id", "idx_calendar_events_calendar_id"} {
		var n int
		s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&n)
		if n != 1 {
			t.Errorf("index %s missing after migrate", idx)
		}
	}

	// The existing DM survives and takes messages.
	if dm, err := s.GetOrCreateDM(1, 3); err != nil || dm != 1 {
		t.Fatalf("GetOrCreateDM = %d, %v; want the existing DM 1", dm, err)
	}
	if _, err := s.CreateChatMessage(1, 1, "hello", nil, 0); err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}
	if _, err := s.CreateChatChannel("general", "topic", "channel", 1, nil); err != nil {
		t.Fatalf("CreateChatChannel: %v", err)
	}

	cal, err := s.CreateCalendar("Work", "#36c", 1, "private", nil)
	if err != nil {
		t.Fatalf("CreateCalendar: %v", err)
	}
	if _, err := s.CreateCalendarEvent(&model.CalendarEvent{CalendarID: cal, Title: "standup",
		Start: "2026-11-02T09:00:00Z", End: "2026-11-02T09:15:00Z"}, 1); err != nil {
		t.Fatalf("CreateCalendarEvent: %v", err)
	}

	if _, err := s.SendInternalMail(1, "alice", []int64{3}, "bob", "", "hi", "body", "", "", 0, nil); err != nil {
		t.Fatalf("SendInternalMail: %v", err)
	}
}

// A legacy-layout table that still holds rows is left untouched, not dropped.
func TestMigrateKeepsNonEmptyLegacyTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy_rows.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE chat_messages (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, text TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO chat_messages(user_id, text) VALUES(1, 'old')`); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	s, err := New(path)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	defer s.Close()
	var text string
	if err := s.db.QueryRow(`SELECT text FROM chat_messages WHERE user_id = 1`).Scan(&text); err != nil || text != "old" {
		t.Fatalf("legacy row lost: %q, %v", text, err)
	}
}
