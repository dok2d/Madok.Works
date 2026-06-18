package db

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"kanban/internal/auth"
	"kanban/internal/model"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	sqliteBusyTimeout  = 5000                // milliseconds
	sessionDuration    = 90 * 24 * time.Hour // 90 days
	resetCodeDuration  = 10 * time.Minute
	linkHashLen        = 16
	maxRecursionDepth  = 50
)

var fileRefRe = regexp.MustCompile(`/api/(images|files)/(\d+)`)

func collectFileRefs(text string, refImages, refFiles map[int64]bool) {
	for _, m := range fileRefRe.FindAllStringSubmatch(text, -1) {
		id, _ := strconv.ParseInt(m[2], 10, 64)
		if id == 0 {
			continue
		}
		if m[1] == "images" {
			refImages[id] = true
		} else {
			refFiles[id] = true
		}
	}
}

type Store struct {
	db      *dbConn
	verbose bool
}

// parseTime tries multiple date formats that SQLite/go-sqlite3 may return.
func parseTime(s string) time.Time {
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
		// Postgres CAST(timestamptz AS text), e.g. "2026-06-17 09:42:50.659588+00".
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05-07",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (s *Store) SetVerbose(v bool) { s.verbose = v }

func (s *Store) logf(format string, args ...any) {
	if s.verbose {
		log.Printf("[store] "+format, args...)
	}
}

// New opens a Store. The path selects the backend: a postgres://… (or
// postgresql://…) URL uses PostgreSQL; anything else is treated as a SQLite
// file path. SQLite is the default and unchanged from before.
func New(path string) (*Store, error) {
	dia := dialectSQLite
	driver, dsn := "sqlite3", fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=%d&_foreign_keys=on", path, sqliteBusyTimeout)
	if isPostgresURL(path) {
		dia, driver, dsn = dialectPostgres, "pgx", path
	}
	d, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if dia == dialectSQLite {
		// SQLite + WAL is single-writer; one connection avoids "database is
		// locked" and keeps the read-all-rows-before-follow-up pattern valid.
		d.SetMaxOpenConns(1)
	} else {
		d.SetMaxOpenConns(10)
		d.SetMaxIdleConns(5)
		d.SetConnMaxLifetime(time.Hour)
	}
	s := &Store{db: &dbConn{sdb: d, d: dia, dsn: dsn}}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		// users is created first: projects/project_members (and later kanban
		// tables) reference it, and Postgres requires the FK target to pre-exist.
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			is_admin INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		// === Projects (top-level container; owns kanban/wiki/calendar) ===
		// Created before the tables that reference it. Visibility mirrors wiki:
		// 'open' = every user is an implicit member, 'restricted' = members only.
		`CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			key TEXT NOT NULL DEFAULT '' UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			visibility TEXT NOT NULL DEFAULT 'restricted',
			created_by INTEGER REFERENCES users(id),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS project_members (
			project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role TEXT NOT NULL DEFAULT 'editor',
			PRIMARY KEY (project_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS columns (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL DEFAULT 1 REFERENCES projects(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS epics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL DEFAULT 1 REFERENCES projects(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			color TEXT NOT NULL DEFAULT '#6366f1'
		)`,
		`CREATE TABLE IF NOT EXISTS tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL DEFAULT 1 REFERENCES projects(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			color TEXT NOT NULL DEFAULT '#64748b',
			UNIQUE(project_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL DEFAULT 1 REFERENCES projects(id) ON DELETE CASCADE,
			number INTEGER NOT NULL DEFAULT 0,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			column_id INTEGER NOT NULL REFERENCES columns(id),
			epic_id INTEGER REFERENCES epics(id),
			position INTEGER NOT NULL DEFAULT 0,
			priority INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS task_tags (
			task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (task_id, tag_id)
		)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			text TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS images (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			data BLOB NOT NULL,
			mime TEXT NOT NULL DEFAULT 'image/png',
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			filename TEXT NOT NULL,
			data BLOB NOT NULL,
			mime TEXT NOT NULL,
			size INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS task_dependencies (
			task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			depends_on_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			PRIMARY KEY (task_id, depends_on_id)
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at DATETIME NOT NULL
		)`,
		// Notifications
		`CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			type TEXT NOT NULL DEFAULT 'mention',
			text TEXT NOT NULL,
			task_id INTEGER,
			is_read INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		// Task subscriptions
		`CREATE TABLE IF NOT EXISTS task_subscriptions (
			task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			PRIMARY KEY (task_id, user_id)
		)`,
		// Activity log
		`CREATE TABLE IF NOT EXISTS activity_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			action TEXT NOT NULL,
			task_id INTEGER,
			details TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		// TOTP recovery codes (hashed, single-use) for 2FA.
		`CREATE TABLE IF NOT EXISTS totp_recovery (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			code_hash TEXT NOT NULL,
			used INTEGER NOT NULL DEFAULT 0
		)`,
		// App settings (key-value)
		`CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		)`,
		// Sprints
		`CREATE TABLE IF NOT EXISTS sprints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL DEFAULT 1 REFERENCES projects(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			start_date TEXT NOT NULL DEFAULT '',
			end_date TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'planning'
		)`,
		// === Wiki (ecosystem service) ===
		`CREATE TABLE IF NOT EXISTS wiki_spaces (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			visibility TEXT NOT NULL DEFAULT 'open',
			project_id INTEGER REFERENCES projects(id) ON DELETE SET NULL,
			created_by INTEGER REFERENCES users(id),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS wiki_space_members (
			space_id INTEGER NOT NULL REFERENCES wiki_spaces(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role TEXT NOT NULL DEFAULT 'editor',
			PRIMARY KEY (space_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS wiki_pages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			space_id INTEGER NOT NULL REFERENCES wiki_spaces(id) ON DELETE CASCADE,
			parent_id INTEGER REFERENCES wiki_pages(id) ON DELETE SET NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			position INTEGER NOT NULL DEFAULT 0,
			rev_no INTEGER NOT NULL DEFAULT 1,
			created_by INTEGER REFERENCES users(id),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_by INTEGER REFERENCES users(id),
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS wiki_revisions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			page_id INTEGER NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
			rev_no INTEGER NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			author_id INTEGER REFERENCES users(id),
			comment TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// [[wiki links]] graph: which page links to which (rebuilt on each save).
		`CREATE TABLE IF NOT EXISTS wiki_links (
			from_page_id INTEGER NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
			to_page_id INTEGER NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
			PRIMARY KEY (from_page_id, to_page_id)
		)`,
		`CREATE TABLE IF NOT EXISTS wiki_comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			page_id INTEGER NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
			parent_id INTEGER REFERENCES wiki_comments(id) ON DELETE CASCADE,
			author_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
			text TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// draw.io diagrams stored as an entity: editable XML + a rendered PNG.
		// Pages reference one via a [[diagram:ID]] token (source stays out of the article).
		`CREATE TABLE IF NOT EXISTS wiki_diagrams (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			space_id INTEGER NOT NULL REFERENCES wiki_spaces(id) ON DELETE CASCADE,
			title TEXT NOT NULL DEFAULT '',
			xml TEXT NOT NULL DEFAULT '',
			png TEXT NOT NULL DEFAULT '', -- base64-encoded PNG (drawio client export)
			created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// Reactions on a wiki page or comment (target_type: 'page' | 'comment').
		`CREATE TABLE IF NOT EXISTS wiki_reactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			target_type TEXT NOT NULL,
			target_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			emoji TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(target_type, target_id, user_id, emoji)
		)`,
		// === Chat (ecosystem service) ===
		`CREATE TABLE IF NOT EXISTS chat_channels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL DEFAULT '',
			topic TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT 'channel',
			created_by INTEGER REFERENCES users(id),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS chat_members (
			channel_id INTEGER NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			last_read_id INTEGER NOT NULL DEFAULT 0,
			joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (channel_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS chat_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id INTEGER NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
			author_id INTEGER REFERENCES users(id),
			text TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			edited_at DATETIME,
			deleted INTEGER NOT NULL DEFAULT 0
		)`,
		// Per-user pinned channels/DMs (sorted to the top of the chat list).
		`CREATE TABLE IF NOT EXISTS chat_pins (
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			channel_id INTEGER NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
			PRIMARY KEY (user_id, channel_id)
		)`,
		// Wiki page pins: per-user (user_id>0) and shared/space-wide (user_id=0).
		`CREATE TABLE IF NOT EXISTS wiki_page_pins (
			page_id INTEGER NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL,
			PRIMARY KEY (page_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS chat_reactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id INTEGER NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			emoji TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(message_id, user_id, emoji)
		)`,
		// === Calendar (ecosystem service) ===
		`CREATE TABLE IF NOT EXISTS calendars (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL DEFAULT '',
			color TEXT NOT NULL DEFAULT '#6366f1',
			owner_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
			visibility TEXT NOT NULL DEFAULT 'private',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS calendar_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			calendar_id INTEGER NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
			title TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			location TEXT NOT NULL DEFAULT '',
			start_utc TEXT NOT NULL,
			end_utc TEXT NOT NULL,
			all_day INTEGER NOT NULL DEFAULT 0,
			rrule TEXT NOT NULL DEFAULT '',
			created_by INTEGER REFERENCES users(id),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// Per-user read-only external calendar subscriptions (ICS/iCal, e.g. Google).
		`CREATE TABLE IF NOT EXISTS calendar_subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL,
			color TEXT NOT NULL DEFAULT '#0ea5e9',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_synced TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS calendar_sub_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sub_id INTEGER NOT NULL REFERENCES calendar_subscriptions(id) ON DELETE CASCADE,
			uid TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			location TEXT NOT NULL DEFAULT '',
			start_utc TEXT NOT NULL,
			end_utc TEXT NOT NULL,
			all_day INTEGER NOT NULL DEFAULT 0,
			rrule TEXT NOT NULL DEFAULT ''
		)`,
		// === Mail (ecosystem service) ===
		`CREATE TABLE IF NOT EXISTS mail_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			thread_id INTEGER NOT NULL DEFAULT 0,
			from_user_id INTEGER REFERENCES users(id),
			from_name TEXT NOT NULL DEFAULT '',
			from_addr TEXT NOT NULL DEFAULT '',
			to_addrs TEXT NOT NULL DEFAULT '',
			cc_addrs TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL DEFAULT '',
			body_text TEXT NOT NULL DEFAULT '',
			body_html TEXT NOT NULL DEFAULT '',
			in_reply_to TEXT NOT NULL DEFAULT '',
			message_id TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS mail_box (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			message_id INTEGER NOT NULL REFERENCES mail_messages(id) ON DELETE CASCADE,
			folder TEXT NOT NULL DEFAULT 'inbox',
			seen INTEGER NOT NULL DEFAULT 0,
			flagged INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS mail_attachments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id INTEGER NOT NULL REFERENCES mail_messages(id) ON DELETE CASCADE,
			file_id INTEGER NOT NULL REFERENCES files(id),
			filename TEXT NOT NULL DEFAULT ''
		)`,
		// Per-user custom mail folders (system folders inbox/sent/drafts/trash/junk are implicit).
		`CREATE TABLE IF NOT EXISTS mail_folders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, name)
		)`,
		// Per-user inbound mail filters. conditions/actions are small JSON arrays.
		`CREATE TABLE IF NOT EXISTS mail_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			match_type TEXT NOT NULL DEFAULT 'all',
			conditions TEXT NOT NULL DEFAULT '[]',
			actions TEXT NOT NULL DEFAULT '[]',
			position INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// Security / attack-investigation audit (mail). user_id nullable (external events).
		`CREATE TABLE IF NOT EXISTS mail_audit (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			event TEXT NOT NULL,
			source_ip TEXT NOT NULL DEFAULT '',
			mail_from TEXT NOT NULL DEFAULT '',
			rcpt TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL DEFAULT '',
			user_id INTEGER REFERENCES users(id),
			score INTEGER NOT NULL DEFAULT 0,
			detail TEXT NOT NULL DEFAULT ''
		)`,
		// Mailbox aliases: extra local-parts mapping to a user; is_primary picks the From address.
		`CREATE TABLE IF NOT EXISTS mail_aliases (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			localpart TEXT NOT NULL UNIQUE,
			is_primary INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// Chat message attachments (mirrors mail_attachments).
		`CREATE TABLE IF NOT EXISTS chat_attachments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id INTEGER NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
			file_id INTEGER NOT NULL REFERENCES files(id),
			filename TEXT NOT NULL DEFAULT ''
		)`,
		// Per-event calendar reminders. occurrence_start '' = definition template;
		// non-empty rows are per-occurrence "sent" markers (idempotent via unique index).
		`CREATE TABLE IF NOT EXISTS calendar_reminders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id INTEGER NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
			lead_min INTEGER NOT NULL DEFAULT 15,
			channel TEXT NOT NULL DEFAULT 'both',
			occurrence_start TEXT NOT NULL DEFAULT '',
			sent INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// Cached LLM summaries of wiki pages, keyed to the page revision so an edit
		// (new rev_no) recomputes while older revisions keep their cached summary.
		`CREATE TABLE IF NOT EXISTS wiki_summaries (
			page_id INTEGER NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
			rev_no INTEGER NOT NULL,
			summary TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (page_id, rev_no)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_folders_user ON mail_folders(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_rules_user ON mail_rules(user_id, position)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_audit_created ON mail_audit(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_aliases_user ON mail_aliases(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_attachments_msg ON chat_attachments(message_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_reactions_msg ON chat_reactions(message_id)`,
		`CREATE INDEX IF NOT EXISTS idx_calendar_subscriptions_user ON calendar_subscriptions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_calendar_sub_events_sub ON calendar_sub_events(sub_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cal_reminders_event ON calendar_reminders(event_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_cal_reminders_dedup ON calendar_reminders(event_id, lead_min, occurrence_start)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_box_user ON mail_box(user_id, folder, id)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_attachments_msg ON mail_attachments(message_id)`,
		// indexes for FK lookups
		`CREATE INDEX IF NOT EXISTS idx_calendar_events_calendar_id ON calendar_events(calendar_id, start_utc)`,
		`CREATE INDEX IF NOT EXISTS idx_calendars_owner_id ON calendars(owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_channel_id ON chat_messages(channel_id, id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_members_user_id ON chat_members(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_pages_space_id ON wiki_pages(space_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_pages_parent_id ON wiki_pages(parent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_revisions_page_id ON wiki_revisions(page_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_links_to ON wiki_links(to_page_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_comments_page ON wiki_comments(page_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_reactions_target ON wiki_reactions(target_type, target_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_column_id ON tasks(column_id)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_task_id ON comments(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_task_tags_task_id ON task_tags(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_log_user_id ON activity_log(user_id)`,
		// project scoping
		`CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id, column_id, position)`,
		`CREATE INDEX IF NOT EXISTS idx_columns_project ON columns(project_id, position)`,
		`CREATE INDEX IF NOT EXISTS idx_epics_project ON epics(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sprints_project ON sprints(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tags_project ON tags(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_calendars_project ON calendars(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wiki_spaces_project ON wiki_spaces(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(translateDDL(s.db.d, q)); err != nil {
			// A legacy DB may have a table from an older schema that lacks a column
			// a new index references. Skip such an index rather than aborting startup;
			// the column itself is added via the alters pass below where applicable.
			if strings.HasPrefix(q, "CREATE INDEX") {
				log.Printf("migrate: skipping index (legacy schema?): %v", err)
				continue
			}
			label := q
			if len(label) > 60 {
				label = label[:60]
			}
			return fmt.Errorf("exec %q: %w", label, err)
		}
	}

	// add new columns if missing (safe ALTER TABLE for existing DBs)
	alters := []string{
		"ALTER TABLE tasks ADD COLUMN todo TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE tasks ADD COLUMN project_url TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE tasks ADD COLUMN assignee_id INTEGER REFERENCES users(id)",
		"ALTER TABLE epics ADD COLUMN description TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE comments ADD COLUMN parent_id INTEGER REFERENCES comments(id)",
		"ALTER TABLE comments ADD COLUMN updated_at DATETIME",
		"ALTER TABLE comments ADD COLUMN author_id INTEGER REFERENCES users(id)",
		"ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'regular'",
		"ALTER TABLE users ADD COLUMN telegram_chat_id INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE users ADD COLUMN link_hash TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE tasks ADD COLUMN deadline TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN reset_code TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN reset_code_expires DATETIME",
		"ALTER TABLE tasks ADD COLUMN sprint_id INTEGER REFERENCES sprints(id)",
		"ALTER TABLE users ADD COLUMN auth_provider TEXT NOT NULL DEFAULT 'local'",
		"ALTER TABLE users ADD COLUMN external_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE calendar_events ADD COLUMN reminded INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE mail_messages ADD COLUMN spam_score INTEGER NOT NULL DEFAULT 0",
		// Columns added to mail_messages after early builds (idempotent upgrade of
		// databases created before SMTP/threading fields existed).
		"ALTER TABLE mail_messages ADD COLUMN thread_id INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE mail_messages ADD COLUMN from_addr TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE mail_messages ADD COLUMN to_addrs TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE mail_messages ADD COLUMN cc_addrs TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE mail_messages ADD COLUMN in_reply_to TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE mail_messages ADD COLUMN message_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE wiki_pages ADD COLUMN content_md TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE wiki_pages ADD COLUMN is_template INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE activity_log ADD COLUMN project_id INTEGER",
		"ALTER TABLE users ADD COLUMN totp_secret TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN totp_enabled INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE wiki_revisions ADD COLUMN content_md TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE chat_messages ADD COLUMN reply_to INTEGER REFERENCES chat_messages(id)",
		"ALTER TABLE chat_messages ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE chat_messages ADD COLUMN pinned_at DATETIME",
		"ALTER TABLE chat_messages ADD COLUMN pinned_by INTEGER REFERENCES users(id)",
		"ALTER TABLE chat_messages ADD COLUMN kind TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE calendar_sub_events ADD COLUMN exdate TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE calendar_sub_events ADD COLUMN recurrence_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE calendar_sub_events ADD COLUMN status TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE files ADD COLUMN compressed INTEGER NOT NULL DEFAULT 0",
	}
	for _, q := range alters {
		s.db.Exec(translateDDL(s.db.d, q)) // ignore "duplicate column" errors
	}

	// create indexes that depend on ALTER TABLE columns
	s.db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_sprint_id ON tasks(sprint_id)")
	s.db.Exec("CREATE INDEX IF NOT EXISTS idx_mail_messages_thread ON mail_messages(thread_id)")

	// Migrate existing users: set role based on is_admin
	s.db.Exec("UPDATE users SET role='admin' WHERE is_admin=1 AND role='regular'")

	// Seed the default project "Общее" (open to everyone, key GEN). Must exist
	// before any project-scoped seeding. Idempotent via ON CONFLICT.
	if _, err := s.db.Exec(`INSERT INTO projects(id,name,key,description,visibility) VALUES(1,'Общее','GEN','',?) ON CONFLICT(id) DO NOTHING`, "open"); err != nil {
		return fmt.Errorf("seed default project: %w", err)
	}
	// Postgres IDENTITY isn't advanced by the explicit id=1 insert, so the next
	// CreateProject would collide on id=1 — resync the sequence (no-op on SQLite).
	if s.db.d == dialectPostgres {
		s.db.Exec(`SELECT setval(pg_get_serial_sequence('projects','id'), (SELECT MAX(id) FROM projects))`)
	}

	// seed default columns into the default project if it has none
	var cnt int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM columns WHERE project_id=1").Scan(&cnt); err != nil {
		return fmt.Errorf("count columns: %w", err)
	}
	if cnt == 0 {
		if err := s.seedDefaultColumns(1); err != nil {
			return err
		}
	}
	return nil
}

// defaultColumns is the starter board for every new project.
var defaultColumns = []string{"Backlog", "To Do", "In Progress", "Review", "Done"}

// seedDefaultColumns creates the starter columns for a project.
func (s *Store) seedDefaultColumns(projectID int64) error {
	for i, name := range defaultColumns {
		if _, err := s.db.Exec("INSERT INTO columns(project_id,name,position) VALUES(?,?,?)", projectID, name, i); err != nil {
			return fmt.Errorf("seed column %q: %w", name, err)
		}
	}
	return nil
}

// --- Columns ---

func (s *Store) ListColumns(projectID int64) ([]model.Column, error) {
	rows, err := s.db.Query("SELECT id,project_id,name,position FROM columns WHERE project_id=? ORDER BY position", projectID)
	if err != nil {
		s.logf("ListColumns error: %v", err)
		return nil, err
	}
	defer rows.Close()
	cols := make([]model.Column, 0)
	for rows.Next() {
		var c model.Column
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Name, &c.Position); err != nil {
			s.logf("ListColumns scan error: %v", err)
			return nil, fmt.Errorf("scan column: %w", err)
		}
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		s.logf("ListColumns rows error: %v", err)
		return nil, err
	}
	s.logf("ListColumns: returned %d columns", len(cols))
	return cols, nil
}

func (s *Store) CreateColumn(projectID int64, name string) (int64, error) {
	id, err := s.db.ExecInsert(
		"INSERT INTO columns(project_id,name,position) VALUES(?,?, (SELECT COALESCE(MAX(position),0)+1 FROM columns WHERE project_id=?))",
		projectID, name, projectID,
	)
	if err != nil {
		s.logf("CreateColumn(%q) error: %v", name, err)
		return 0, err
	}
	s.logf("CreateColumn(%q) -> id=%d", name, id)
	return id, nil
}

func (s *Store) UpdateColumn(id int64, name string) error {
	s.logf("UpdateColumn(id=%d, name=%q)", id, name)
	_, err := s.db.Exec("UPDATE columns SET name=? WHERE id=?", name, id)
	return err
}

func (s *Store) DeleteColumn(id int64) error {
	s.logf("DeleteColumn(id=%d)", id)
	res, err := s.db.Exec("DELETE FROM columns WHERE id=?", id)
	if err != nil {
		s.logf("DeleteColumn(id=%d) error: %v", id, err)
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("column %d not found", id)
	}
	return nil
}

func (s *Store) ReorderColumns(projectID int64, ids []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var cnt int
	if err := tx.QueryRow("SELECT COUNT(*) FROM columns WHERE project_id=?", projectID).Scan(&cnt); err != nil {
		return err
	}
	if cnt != len(ids) {
		return fmt.Errorf("expected %d column IDs, got %d", cnt, len(ids))
	}

	for i, id := range ids {
		res, err := tx.Exec("UPDATE columns SET position=? WHERE id=? AND project_id=?", i, id, projectID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("column %d not found", id)
		}
	}
	return tx.Commit()
}

// --- Epics ---

func (s *Store) ListEpics(projectID int64) ([]model.Epic, error) {
	rows, err := s.db.Query("SELECT id,project_id,name,color,description FROM epics WHERE project_id=? ORDER BY name", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	epics := make([]model.Epic, 0)
	for rows.Next() {
		var e model.Epic
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Name, &e.Color, &e.Description); err != nil {
			return nil, fmt.Errorf("scan epic: %w", err)
		}
		epics = append(epics, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return epics, nil
}

func (s *Store) GetEpic(id int64) (*model.Epic, error) {
	var e model.Epic
	err := s.db.QueryRow("SELECT id,project_id,name,color,description FROM epics WHERE id=?", id).
		Scan(&e.ID, &e.ProjectID, &e.Name, &e.Color, &e.Description)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *Store) CreateEpic(projectID int64, name, color string) (int64, error) {
	return s.db.ExecInsert("INSERT INTO epics(project_id,name,color) VALUES(?,?,?)", projectID, name, color)
}

func (s *Store) UpdateEpic(id int64, name, color, description string) error {
	_, err := s.db.Exec("UPDATE epics SET name=?,color=?,description=? WHERE id=?", name, color, description, id)
	return err
}

func (s *Store) DeleteEpic(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE tasks SET epic_id=NULL WHERE epic_id=?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM epics WHERE id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// --- Sprints ---

func (s *Store) ListSprints(projectID int64) ([]model.Sprint, error) {
	rows, err := s.db.Query("SELECT id,project_id,name,start_date,end_date,status FROM sprints WHERE project_id=? ORDER BY id", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sprints := make([]model.Sprint, 0)
	for rows.Next() {
		var sp model.Sprint
		if err := rows.Scan(&sp.ID, &sp.ProjectID, &sp.Name, &sp.StartDate, &sp.EndDate, &sp.Status); err != nil {
			return nil, fmt.Errorf("scan sprint: %w", err)
		}
		sprints = append(sprints, sp)
	}
	return sprints, rows.Err()
}

func (s *Store) GetSprint(id int64) (*model.Sprint, error) {
	var sp model.Sprint
	err := s.db.QueryRow("SELECT id,project_id,name,start_date,end_date,status FROM sprints WHERE id=?", id).
		Scan(&sp.ID, &sp.ProjectID, &sp.Name, &sp.StartDate, &sp.EndDate, &sp.Status)
	if err != nil {
		return nil, err
	}
	return &sp, nil
}

func (s *Store) CreateSprint(projectID int64, name, startDate, endDate, status string) (int64, error) {
	if status == "" {
		status = "planning"
	}
	return s.db.ExecInsert("INSERT INTO sprints(project_id,name,start_date,end_date,status) VALUES(?,?,?,?,?)", projectID, name, startDate, endDate, status)
}

func (s *Store) UpdateSprint(id int64, name, startDate, endDate, status string) error {
	_, err := s.db.Exec("UPDATE sprints SET name=?,start_date=?,end_date=?,status=? WHERE id=?", name, startDate, endDate, status, id)
	return err
}

func (s *Store) DeleteSprint(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE tasks SET sprint_id=NULL WHERE sprint_id=?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM sprints WHERE id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CompleteSprint(projectID, id int64, moveToSprintID *int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Mark sprint as completed
	if _, err := tx.Exec("UPDATE sprints SET status='completed' WHERE id=? AND project_id=?", id, projectID); err != nil {
		return err
	}

	// Find last column (Done) within this project
	var lastColID int64
	err = tx.QueryRow("SELECT id FROM columns WHERE project_id=? ORDER BY position DESC LIMIT 1", projectID).Scan(&lastColID)
	if err != nil {
		return err
	}

	// Move incomplete tasks to next sprint (or unassign if nil)
	if moveToSprintID != nil {
		_, err = tx.Exec("UPDATE tasks SET sprint_id=? WHERE sprint_id=? AND column_id!=?", *moveToSprintID, id, lastColID)
	} else {
		_, err = tx.Exec("UPDATE tasks SET sprint_id=NULL WHERE sprint_id=? AND column_id!=?", id, lastColID)
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) SprintTasks(sprintID int64) ([]model.Task, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.project_id, t.number, t.title, t.description, t.todo, t.project_url,
		       t.column_id, t.epic_id, t.sprint_id, t.assignee_id, t.position, t.priority, t.deadline, t.created_at, t.updated_at
		FROM tasks t WHERE t.sprint_id=? ORDER BY t.position, t.id`, sprintID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]model.Task, 0)
	var taskIDs []int64
	for rows.Next() {
		var t model.Task
		var eid, sid, aid sql.NullInt64
		var ca, ua string
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Number, &t.Title, &t.Description, &t.Todo, &t.ProjectURL,
			&t.ColumnID, &eid, &sid, &aid, &t.Position, &t.Priority, &t.Deadline, &ca, &ua); err != nil {
			return nil, fmt.Errorf("scan sprint task: %w", err)
		}
		t.CreatedAt = parseTime(ca)
		t.UpdatedAt = parseTime(ua)
		if eid.Valid {
			t.EpicID = &eid.Int64
		}
		if sid.Valid {
			t.SprintID = &sid.Int64
		}
		if aid.Valid {
			t.AssigneeID = &aid.Int64
		}
		t.Tags = []model.Tag{}
		tasks = append(tasks, t)
		taskIDs = append(taskIDs, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(taskIDs) > 0 {
		tagMap, err := s.batchTaskTags(taskIDs)
		if err == nil {
			for i := range tasks {
				if tags, ok := tagMap[tasks[i].ID]; ok {
					tasks[i].Tags = tags
				}
			}
		}
	}
	return tasks, nil
}

func (s *Store) EpicTasks(epicID int64) ([]model.Task, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.project_id, t.number, t.title, t.description, t.todo, t.project_url,
		       t.column_id, t.epic_id, t.sprint_id, t.assignee_id, t.position, t.priority, t.deadline, t.created_at, t.updated_at
		FROM tasks t WHERE t.epic_id=? ORDER BY t.position, t.id`, epicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]model.Task, 0)
	var taskIDs []int64
	for rows.Next() {
		var t model.Task
		var eid, sid, aid sql.NullInt64
		var ca, ua string
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Number, &t.Title, &t.Description, &t.Todo, &t.ProjectURL,
			&t.ColumnID, &eid, &sid, &aid, &t.Position, &t.Priority, &t.Deadline, &ca, &ua); err != nil {
			return nil, fmt.Errorf("scan epic task: %w", err)
		}
		t.CreatedAt = parseTime(ca)
		t.UpdatedAt = parseTime(ua)
		if sid.Valid {
			t.SprintID = &sid.Int64
		}
		if eid.Valid {
			t.EpicID = &eid.Int64
		}
		if aid.Valid {
			t.AssigneeID = &aid.Int64
		}
		t.Tags = []model.Tag{}
		tasks = append(tasks, t)
		taskIDs = append(taskIDs, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(taskIDs) > 0 {
		tagMap, err := s.batchTaskTags(taskIDs)
		if err == nil {
			for i := range tasks {
				if tags, ok := tagMap[tasks[i].ID]; ok {
					tasks[i].Tags = tags
				}
			}
		}
	}
	return tasks, nil
}

// --- Tags ---

func (s *Store) ListTags(projectID int64) ([]model.Tag, error) {
	rows, err := s.db.Query("SELECT id,project_id,name,color FROM tags WHERE project_id=? ORDER BY name", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := make([]model.Tag, 0)
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Name, &t.Color); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

func (s *Store) CreateTag(projectID int64, name, color string) (int64, error) {
	return s.db.ExecInsert("INSERT INTO tags(project_id,name,color) VALUES(?,?,?)", projectID, name, color)
}

func (s *Store) DeleteTag(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM task_tags WHERE tag_id=?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM tags WHERE id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// --- Tasks ---

func (s *Store) ListTasks(projectID int64) ([]model.Task, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.project_id, t.number, t.title, t.description, t.todo, t.project_url,
		       t.column_id, t.epic_id, t.sprint_id, t.assignee_id,
		       t.position, t.priority, t.deadline, t.created_at, t.updated_at,
		       e.id, e.name, e.color,
		       sp.id, sp.name, sp.start_date, sp.end_date, sp.status,
		       u.id, u.username
		FROM tasks t
		LEFT JOIN epics e ON t.epic_id = e.id
		LEFT JOIN sprints sp ON t.sprint_id = sp.id
		LEFT JOIN users u ON t.assignee_id = u.id
		WHERE t.project_id = ?
		ORDER BY t.position, t.id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]model.Task, 0)
	var taskIDs []int64
	for rows.Next() {
		var t model.Task
		var eid, sid, aid sql.NullInt64
		var epicName, epicColor sql.NullString
		var sprintID sql.NullInt64
		var sprintName, sprintStart, sprintEnd, sprintStatus sql.NullString
		var assigneeID sql.NullInt64
		var assigneeName sql.NullString
		var ca, ua string
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Number, &t.Title, &t.Description, &t.Todo, &t.ProjectURL,
			&t.ColumnID, &eid, &sid, &aid,
			&t.Position, &t.Priority, &t.Deadline, &ca, &ua,
			&eid, &epicName, &epicColor,
			&sprintID, &sprintName, &sprintStart, &sprintEnd, &sprintStatus,
			&assigneeID, &assigneeName); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		t.CreatedAt = parseTime(ca)
		t.UpdatedAt = parseTime(ua)
		if eid.Valid {
			t.EpicID = &eid.Int64
			t.Epic = &model.Epic{ID: eid.Int64, Name: epicName.String, Color: epicColor.String}
		}
		if sprintID.Valid {
			t.SprintID = &sprintID.Int64
			t.Sprint = &model.Sprint{ID: sprintID.Int64, Name: sprintName.String, StartDate: sprintStart.String, EndDate: sprintEnd.String, Status: sprintStatus.String}
		}
		if assigneeID.Valid {
			t.AssigneeID = &assigneeID.Int64
			t.Assignee = &model.User{ID: assigneeID.Int64, Username: assigneeName.String}
		}
		t.Tags = []model.Tag{}
		tasks = append(tasks, t)
		taskIDs = append(taskIDs, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(taskIDs) > 0 {
		tagMap, err := s.batchTaskTags(taskIDs)
		if err != nil {
			s.logf("batchTaskTags error: %v", err)
		} else {
			for i := range tasks {
				if tags, ok := tagMap[tasks[i].ID]; ok {
					tasks[i].Tags = tags
				}
			}
		}
	}

	return tasks, nil
}

func (s *Store) GetTask(id int64) (*model.Task, error) {
	var t model.Task
	var eid, sid, aid sql.NullInt64
	var ca, ua string
	err := s.db.QueryRow(`SELECT id,project_id,number,title,description,todo,project_url,column_id,epic_id,sprint_id,assignee_id,position,priority,deadline,created_at,updated_at
		FROM tasks WHERE id=?`, id).Scan(&t.ID, &t.ProjectID, &t.Number, &t.Title, &t.Description, &t.Todo, &t.ProjectURL, &t.ColumnID, &eid, &sid, &aid,
		&t.Position, &t.Priority, &t.Deadline, &ca, &ua)
	if err != nil {
		return nil, err
	}
	t.CreatedAt = parseTime(ca)
	t.UpdatedAt = parseTime(ua)
	if eid.Valid {
		t.EpicID = &eid.Int64
		var e model.Epic
		if err := s.db.QueryRow("SELECT id,name,color,description FROM epics WHERE id=?", eid.Int64).Scan(&e.ID, &e.Name, &e.Color, &e.Description); err == nil {
			t.Epic = &e
		}
	}
	if sid.Valid {
		t.SprintID = &sid.Int64
		sp, err := s.GetSprint(sid.Int64)
		if err == nil {
			t.Sprint = sp
		}
	}
	if aid.Valid {
		t.AssigneeID = &aid.Int64
		var u model.User
		var admin int
		var role string
		if err := s.db.QueryRow("SELECT id,username,is_admin,role FROM users WHERE id=?", aid.Int64).Scan(&u.ID, &u.Username, &admin, &role); err == nil {
			u.Role = role
			u.IsAdmin = admin == 1
			t.Assignee = &u
		}
	}
	t.Tags = s.taskTags(t.ID)
	t.Comments = s.taskCommentsTree(t.ID)
	t.DependsOn = s.TaskDependencies(t.ID)
	t.Dependents = s.TaskDependents(t.ID)
	return &t, nil
}

func (s *Store) CreateTask(projectID int64, title, desc, todo, projectURL string, colID int64, epicID *int64, sprintID *int64, assigneeID *int64, priority int, tagIDs []int64, deadline string) (int64, error) {
	s.logf("CreateTask(project=%d, %q, col=%d, prio=%d, tags=%v)", projectID, title, colID, priority, tagIDs)
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var maxPos int
	if err := tx.QueryRow("SELECT COALESCE(MAX(position),0) FROM tasks WHERE column_id=?", colID).Scan(&maxPos); err != nil {
		return 0, fmt.Errorf("get max position: %w", err)
	}
	var nextNum int
	if err := tx.QueryRow("SELECT COALESCE(MAX(number),0)+1 FROM tasks WHERE project_id=?", projectID).Scan(&nextNum); err != nil {
		return 0, fmt.Errorf("get next number: %w", err)
	}
	id, err := tx.ExecInsert(`INSERT INTO tasks(project_id,number,title,description,todo,project_url,column_id,epic_id,sprint_id,assignee_id,position,priority,deadline)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, projectID, nextNum, title, desc, todo, projectURL, colID, epicID, sprintID, assigneeID, maxPos+1, priority, deadline)
	if err != nil {
		s.logf("CreateTask error: %v", err)
		return 0, err
	}
	for _, tid := range tagIDs {
		if _, err := tx.Exec("INSERT INTO task_tags(task_id,tag_id) VALUES(?,?) ON CONFLICT DO NOTHING", id, tid); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	s.logf("CreateTask -> id=%d", id)
	return id, nil
}

func (s *Store) UpdateTask(id int64, title, desc, todo, projectURL string, colID int64, epicID *int64, sprintID *int64, assigneeID *int64, priority int, tagIDs []int64, deadline string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE tasks SET title=?,description=?,todo=?,project_url=?,column_id=?,epic_id=?,sprint_id=?,assignee_id=?,priority=?,deadline=?,
		updated_at=datetime('now') WHERE id=?`, title, desc, todo, projectURL, colID, epicID, sprintID, assigneeID, priority, deadline, id)
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM task_tags WHERE task_id=?", id); err != nil {
		return err
	}
	for _, tid := range tagIDs {
		if _, err := tx.Exec("INSERT INTO task_tags(task_id,tag_id) VALUES(?,?) ON CONFLICT DO NOTHING", id, tid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) MoveTask(id, colID int64, position int) error {
	_, err := s.db.Exec(`UPDATE tasks SET column_id=?,position=?,updated_at=datetime('now') WHERE id=?`,
		colID, position, id)
	return err
}

func (s *Store) DeleteTask(id int64) error {
	_, err := s.db.Exec("DELETE FROM tasks WHERE id=?", id)
	return err
}

// --- Comments ---

func (s *Store) AddComment(taskID int64, text string, parentID *int64, authorID *int64) (int64, error) {
	id, err := s.db.ExecInsert("INSERT INTO comments(task_id,text,parent_id,author_id) VALUES(?,?,?,?)", taskID, text, parentID, authorID)
	if err != nil {
		return 0, err
	}
	s.db.Exec("UPDATE tasks SET updated_at=datetime('now') WHERE id=?", taskID)
	return id, nil
}

func (s *Store) UpdateComment(id int64, text string) error {
	_, err := s.db.Exec("UPDATE comments SET text=?,updated_at=datetime('now') WHERE id=?", text, id)
	return err
}

func (s *Store) DeleteComment(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.deleteCommentTreeTx(tx, id, maxRecursionDepth); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) deleteCommentTreeTx(tx *dbTx, id int64, maxDepth int) error {
	if maxDepth <= 0 {
		// Safety: just delete this node, don't recurse further
		_, err := tx.Exec("DELETE FROM comments WHERE id=?", id)
		return err
	}
	rows, err := tx.Query("SELECT id FROM comments WHERE parent_id=?", id)
	if err != nil {
		return err
	}
	var childIDs []int64
	for rows.Next() {
		var cid int64
		if err := rows.Scan(&cid); err != nil {
			rows.Close()
			return err
		}
		childIDs = append(childIDs, cid)
	}
	rows.Close()
	for _, cid := range childIDs {
		if err := s.deleteCommentTreeTx(tx, cid, maxDepth-1); err != nil {
			return err
		}
	}
	_, err = tx.Exec("DELETE FROM comments WHERE id=?", id)
	return err
}

// CountCommentDescendants returns the number of replies (all levels) for a comment
func (s *Store) CountCommentDescendants(id int64) int {
	return s.countDescendants(id, maxRecursionDepth)
}

func (s *Store) countDescendants(id int64, maxDepth int) int {
	if maxDepth <= 0 {
		return 0
	}
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM comments WHERE parent_id=?", id).Scan(&count)
	rows, err := s.db.Query("SELECT id FROM comments WHERE parent_id=?", id)
	if err != nil {
		return count
	}
	defer rows.Close()
	var childIDs []int64
	for rows.Next() {
		var cid int64
		if err := rows.Scan(&cid); err != nil {
			continue
		}
		childIDs = append(childIDs, cid)
	}
	for _, cid := range childIDs {
		count += s.countDescendants(cid, maxDepth-1)
	}
	return count
}

func (s *Store) GetCommentTaskID(commentID int64) (int64, error) {
	var taskID int64
	err := s.db.QueryRow("SELECT task_id FROM comments WHERE id=?", commentID).Scan(&taskID)
	return taskID, err
}

func (s *Store) GetCommentAuthorID(commentID int64) (int64, error) {
	var authorID sql.NullInt64
	err := s.db.QueryRow("SELECT author_id FROM comments WHERE id=?", commentID).Scan(&authorID)
	if err != nil {
		return 0, err
	}
	if !authorID.Valid {
		return 0, nil
	}
	return authorID.Int64, nil
}

func (s *Store) taskTags(taskID int64) []model.Tag {
	rows, err := s.db.Query(`SELECT t.id,t.name,t.color FROM tags t
		JOIN task_tags tt ON tt.tag_id=t.id WHERE tt.task_id=?`, taskID)
	if err != nil {
		s.logf("taskTags(%d) error: %v", taskID, err)
		return []model.Tag{}
	}
	defer rows.Close()
	tags := make([]model.Tag, 0)
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color); err != nil {
			s.logf("taskTags scan error: %v", err)
			continue
		}
		tags = append(tags, t)
	}
	return tags
}

func (s *Store) batchTaskTags(taskIDs []int64) (map[int64][]model.Tag, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]byte, 0, len(taskIDs)*2)
	args := make([]any, len(taskIDs))
	for i, id := range taskIDs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args[i] = id
	}

	query := fmt.Sprintf(`SELECT tt.task_id, t.id, t.name, t.color FROM tags t
		JOIN task_tags tt ON tt.tag_id=t.id WHERE tt.task_id IN (%s)`, string(placeholders))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]model.Tag)
	for rows.Next() {
		var taskID int64
		var t model.Tag
		if err := rows.Scan(&taskID, &t.ID, &t.Name, &t.Color); err != nil {
			return nil, fmt.Errorf("scan batch tag: %w", err)
		}
		result[taskID] = append(result[taskID], t)
	}
	return result, rows.Err()
}

// taskCommentsTree returns comments as a tree (top-level + nested replies).
func (s *Store) taskCommentsTree(taskID int64) []model.Comment {
	rows, err := s.db.Query(`SELECT c.id, c.task_id, COALESCE(c.parent_id,0), c.text, c.created_at, COALESCE(CAST(c.updated_at AS TEXT),''),
		COALESCE(c.author_id,0), COALESCE(u.username,'')
		FROM comments c LEFT JOIN users u ON c.author_id=u.id
		WHERE c.task_id=? ORDER BY c.created_at`, taskID)
	if err != nil {
		s.logf("taskCommentsTree(%d) error: %v", taskID, err)
		return []model.Comment{}
	}
	defer rows.Close()
	all := make([]model.Comment, 0)
	for rows.Next() {
		var c model.Comment
		var pid, authorID int64
		var authorName string
		var ca, ua string
		if err := rows.Scan(&c.ID, &c.TaskID, &pid, &c.Text, &ca, &ua, &authorID, &authorName); err != nil {
			s.logf("taskCommentsTree scan error: %v", err)
			continue
		}
		c.CreatedAt = parseTime(ca)
		if ua != "" {
			c.UpdatedAt = parseTime(ua)
		}
		if pid != 0 {
			c.ParentID = &pid
		}
		if authorID != 0 {
			c.AuthorID = &authorID
			c.Author = &model.User{ID: authorID, Username: authorName}
		}
		c.Replies = []model.Comment{}
		all = append(all, c)
	}
	// Build tree using index-based approach to avoid slice reallocation pointer issues
	byID := make(map[int64]int) // comment ID -> index in all
	for i := range all {
		byID[all[i].ID] = i
	}
	children := make(map[int64][]int64) // parent ID -> child IDs in order
	var rootIDs []int64
	for i := range all {
		if all[i].ParentID != nil {
			if _, ok := byID[*all[i].ParentID]; ok {
				children[*all[i].ParentID] = append(children[*all[i].ParentID], all[i].ID)
				continue
			}
		}
		rootIDs = append(rootIDs, all[i].ID)
	}
	// Recursively build tree from indices
	var buildTree func(id int64, depth int) model.Comment
	buildTree = func(id int64, depth int) model.Comment {
		idx := byID[id]
		c := all[idx]
		c.Replies = []model.Comment{}
		if depth < maxRecursionDepth { // depth limit to prevent stack overflow (#28)
			for _, childID := range children[id] {
				c.Replies = append(c.Replies, buildTree(childID, depth+1))
			}
		}
		return c
	}
	result := make([]model.Comment, 0, len(rootIDs))
	for _, rid := range rootIDs {
		result = append(result, buildTree(rid, 0))
	}
	return result
}

// --- Images ---

func (s *Store) SaveImage(data []byte, mime string) (int64, error) {
	return s.db.ExecInsert("INSERT INTO images(data,mime) VALUES(?,?)", data, mime)
}

func (s *Store) GetImage(id int64) ([]byte, string, error) {
	var data []byte
	var mime string
	err := s.db.QueryRow("SELECT data,mime FROM images WHERE id=?", id).Scan(&data, &mime)
	return data, mime, err
}

// --- Files ---

// gzipBytes / gunzipBytes are used for transparent file-blob compression.
func gzipBytes(b []byte) []byte {
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	w.Write(b)
	w.Close()
	return buf.Bytes()
}
func gunzipBytes(b []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// compressibleMime reports whether a MIME type is worth trying to gzip. Already-
// compressed formats (media, archives, office/zip containers, PDF) are skipped.
func compressibleMime(mime string) bool {
	m := strings.ToLower(mime)
	if strings.HasPrefix(m, "image/") && !strings.Contains(m, "svg") {
		return false
	}
	if strings.HasPrefix(m, "video/") || strings.HasPrefix(m, "audio/") {
		return false
	}
	for _, bad := range []string{"zip", "gzip", "compress", "pdf", "x-7z", "rar", "officedocument", "opendocument", "x-bzip"} {
		if strings.Contains(m, bad) {
			return false
		}
	}
	return true
}

// maybeCompress returns the bytes to store and a flag, gzipping only when the MIME
// is compressible and it actually saves at least ~10%.
func maybeCompress(mime string, data []byte) ([]byte, int) {
	if len(data) < 256 || !compressibleMime(mime) {
		return data, 0
	}
	c := gzipBytes(data)
	if len(c) < len(data)-len(data)/10 {
		return c, 1
	}
	return data, 0
}

func (s *Store) SaveFile(filename string, data []byte, mime string) (int64, error) {
	stored, compressed := maybeCompress(mime, data)
	return s.db.ExecInsert("INSERT INTO files(filename,data,mime,size,compressed) VALUES(?,?,?,?,?)",
		filename, stored, mime, len(data), compressed)
}

func (s *Store) GetFile(id int64) ([]byte, string, string, error) {
	var data []byte
	var mime, filename string
	var compressed int
	err := s.db.QueryRow("SELECT data,mime,filename,COALESCE(compressed,0) FROM files WHERE id=?", id).
		Scan(&data, &mime, &filename, &compressed)
	if err != nil {
		return nil, "", "", err
	}
	if compressed == 1 {
		if raw, derr := gunzipBytes(data); derr == nil {
			data = raw
		}
	}
	return data, mime, filename, nil
}

func (s *Store) DeleteImage(id int64) error {
	_, err := s.db.Exec("DELETE FROM images WHERE id=?", id)
	return err
}

func (s *Store) DeleteFile(id int64) error {
	_, err := s.db.Exec("DELETE FROM files WHERE id=?", id)
	return err
}

// scanFileRefs collects file/image references from the single text column
// returned by query. With SetMaxOpenConns(1) the rows must be fully read and
// closed before the next query runs, so this reads to completion before return.
func (s *Store) scanFileRefs(query string, refImages, refFiles map[int64]bool) error {
	rows, err := s.db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var text string
		rows.Scan(&text)
		collectFileRefs(text, refImages, refFiles)
	}
	return rows.Err()
}

// CleanupOrphanFiles removes images and files no longer referenced anywhere.
func (s *Store) CleanupOrphanFiles() (int, error) {
	refImages := make(map[int64]bool)
	refFiles := make(map[int64]bool)

	// Collect referenced IDs from every place that can embed an uploaded file:
	// kanban comments & task descriptions, wiki pages and their revision history,
	// and chat messages. Omitting any source here would delete live attachments.
	for _, q := range []string{
		"SELECT text FROM comments",
		"SELECT description FROM tasks WHERE description != ''",
		"SELECT content FROM wiki_pages",
		"SELECT content FROM wiki_revisions",
		"SELECT text FROM chat_messages WHERE deleted = 0",
		"SELECT body_html FROM mail_messages",
		"SELECT body_text FROM mail_messages",
	} {
		if err := s.scanFileRefs(q, refImages, refFiles); err != nil {
			return 0, err
		}
	}

	// Mail and chat attachments reference files directly (not via text), so mark them too.
	for _, q := range []string{"SELECT file_id FROM mail_attachments", "SELECT file_id FROM chat_attachments"} {
		if arows, err := s.db.Query(q); err == nil {
			for arows.Next() {
				var id int64
				arows.Scan(&id)
				refFiles[id] = true
			}
			arows.Close()
		}
	}

	count := 0

	// Delete orphan images
	imgRows, err := s.db.Query("SELECT id FROM images")
	if err != nil {
		return 0, err
	}
	var orphanImgs []int64
	for imgRows.Next() {
		var id int64
		imgRows.Scan(&id)
		if !refImages[id] {
			orphanImgs = append(orphanImgs, id)
		}
	}
	imgRows.Close()
	for _, id := range orphanImgs {
		s.db.Exec("DELETE FROM images WHERE id=?", id)
		count++
	}

	// Delete orphan files
	fileRows, err := s.db.Query("SELECT id FROM files")
	if err != nil {
		return count, err
	}
	var orphanFiles []int64
	for fileRows.Next() {
		var id int64
		fileRows.Scan(&id)
		if !refFiles[id] {
			orphanFiles = append(orphanFiles, id)
		}
	}
	fileRows.Close()
	for _, id := range orphanFiles {
		s.db.Exec("DELETE FROM files WHERE id=?", id)
		count++
	}

	return count, nil
}

// SearchTasks searches across tasks (title, description, todo), comments, epics, tags.
// When isRegex is true, it loads all tasks and filters in Go using regexp.
func (s *Store) SearchTasks(projectID int64, query string, isRegex bool) ([]int64, error) {
	if isRegex {
		return s.searchTasksRegex(projectID, query)
	}
	like := "%" + query + "%"
	rows, err := s.db.Query(`
		SELECT DISTINCT t.id FROM tasks t
		LEFT JOIN comments c ON c.task_id = t.id
		LEFT JOIN task_tags tt ON tt.task_id = t.id
		LEFT JOIN tags tg ON tg.id = tt.tag_id
		LEFT JOIN epics e ON e.id = t.epic_id
		WHERE t.project_id = ? AND (t.title LIKE ? OR t.description LIKE ? OR t.todo LIKE ? OR t.project_url LIKE ?
		   OR c.text LIKE ? OR tg.name LIKE ? OR e.name LIKE ? OR e.description LIKE ?)
		ORDER BY t.id`,
		projectID, like, like, like, like, like, like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) searchTasksRegex(projectID int64, pattern string) ([]int64, error) {
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}

	rows, err := s.db.Query(`
		SELECT DISTINCT t.id, t.title, t.description, t.todo, t.project_url,
		       COALESCE(e.name,''), COALESCE(e.description,'')
		FROM tasks t
		LEFT JOIN epics e ON e.id = t.epic_id
		WHERE t.project_id = ?
		ORDER BY t.id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matchedIDs := make(map[int64]bool)
	for rows.Next() {
		var id int64
		var title, desc, todo, url, epicName, epicDesc string
		if err := rows.Scan(&id, &title, &desc, &todo, &url, &epicName, &epicDesc); err != nil {
			return nil, err
		}
		if re.MatchString(title) || re.MatchString(desc) || re.MatchString(todo) ||
			re.MatchString(url) || re.MatchString(epicName) || re.MatchString(epicDesc) {
			matchedIDs[id] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Also search comments and tags
	cRows, err := s.db.Query("SELECT c.task_id, c.text FROM comments c JOIN tasks t ON t.id=c.task_id WHERE t.project_id=?", projectID)
	if err == nil {
		defer cRows.Close()
		for cRows.Next() {
			var taskID int64
			var text string
			if cRows.Scan(&taskID, &text) == nil && re.MatchString(text) {
				matchedIDs[taskID] = true
			}
		}
	}

	tRows, err := s.db.Query(`SELECT tt.task_id, tg.name FROM task_tags tt JOIN tags tg ON tg.id = tt.tag_id
		JOIN tasks t ON t.id = tt.task_id WHERE t.project_id=?`, projectID)
	if err == nil {
		defer tRows.Close()
		for tRows.Next() {
			var taskID int64
			var name string
			if tRows.Scan(&taskID, &name) == nil && re.MatchString(name) {
				matchedIDs[taskID] = true
			}
		}
	}

	ids := make([]int64, 0, len(matchedIDs))
	for id := range matchedIDs {
		ids = append(ids, id)
	}
	// Sort for deterministic results
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	return ids, nil
}

// --- Export / Import ---

func (s *Store) ExportAll() (*model.ExportData, error) {
	projects, members, err := s.exportProjects()
	if err != nil {
		return nil, err
	}
	// Kanban data is per-project; export every project's board.
	var cols []model.Column
	var epics []model.Epic
	var sprints []model.Sprint
	var tags []model.Tag
	var tasks []model.Task
	for _, p := range projects {
		c, err := s.ListColumns(p.ID)
		if err != nil {
			return nil, err
		}
		cols = append(cols, c...)
		e, err := s.ListEpics(p.ID)
		if err != nil {
			return nil, err
		}
		epics = append(epics, e...)
		sp, err := s.ListSprints(p.ID)
		if err != nil {
			return nil, err
		}
		sprints = append(sprints, sp...)
		tg, err := s.ListTags(p.ID)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tg...)
		ts, err := s.ListTasks(p.ID)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, ts...)
	}
	// Load full task data (comments, tags) for each task
	for i := range tasks {
		full, err := s.GetTask(tasks[i].ID)
		if err == nil {
			tasks[i] = *full
		}
	}
	// Flatten all comments
	var allComments []model.Comment
	for _, t := range tasks {
		allComments = append(allComments, s.flatComments(t.ID)...)
	}

	// Export users with password hashes for full restore
	users := s.exportUsers()

	// Export app settings
	settings := s.exportSettings()

	// Export task dependencies
	deps := s.exportDependencies()

	// Export task subscriptions
	subs := s.exportSubscriptions()

	// Export files and images
	files := s.exportFiles()
	images := s.exportImages()

	return &model.ExportData{
		Projects:       projects,
		ProjectMembers: members,
		Columns:        cols,
		Epics:          epics,
		Sprints:        sprints,
		Tags:           tags,
		Tasks:          tasks,
		Comments:       allComments,
		Users:          users,
		Settings:       settings,
		Dependencies:   deps,
		Subscriptions:  subs,
		Files:          files,
		Images:         images,
		Ecosystem:      s.exportEcosystem(),
	}, nil
}

// exportProjects dumps all projects and their members for a full backup.
func (s *Store) exportProjects() ([]model.Project, []model.ProjectMember, error) {
	rows, err := s.db.Query(`SELECT id, name, key, description, visibility, created_by, created_at FROM projects ORDER BY id`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var projects []model.Project
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Key, &p.Description, &p.Visibility, &p.CreatedBy, &p.CreatedAt); err != nil {
			return nil, nil, err
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	mRows, err := s.db.Query(`SELECT project_id, user_id, role FROM project_members ORDER BY project_id, user_id`)
	if err != nil {
		return nil, nil, err
	}
	defer mRows.Close()
	var members []model.ProjectMember
	for mRows.Next() {
		var m model.ProjectMember
		if err := mRows.Scan(&m.ProjectID, &m.UserID, &m.Role); err != nil {
			return nil, nil, err
		}
		members = append(members, m)
	}
	return projects, members, mRows.Err()
}

func (s *Store) exportUsers() []model.ExportUser {
	rows, err := s.db.Query("SELECT id,username,password_hash,role,is_admin,telegram_chat_id FROM users ORDER BY id")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var users []model.ExportUser
	for rows.Next() {
		var u model.ExportUser
		var admin int
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &admin, &u.TelegramID); err != nil {
			continue
		}
		u.IsAdmin = admin == 1
		u.PasswordHash = "" // Strip password hash from export for security
		users = append(users, u)
	}
	return users
}

// exportSettings dumps all app_settings, including secrets (DKIM private key,
// bot token), so a JSON export is a complete, migratable backup. The export is
// therefore sensitive — see docs/security-audit.md (handle/transport securely).
func (s *Store) exportSettings() []model.ExportSetting {
	rows, err := s.db.Query("SELECT key,value FROM app_settings")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var settings []model.ExportSetting
	for rows.Next() {
		var st model.ExportSetting
		if err := rows.Scan(&st.Key, &st.Value); err != nil {
			continue
		}
		settings = append(settings, st)
	}
	return settings
}

func (s *Store) exportDependencies() []model.ExportDependency {
	rows, err := s.db.Query("SELECT task_id,depends_on_id FROM task_dependencies")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var deps []model.ExportDependency
	for rows.Next() {
		var d model.ExportDependency
		if err := rows.Scan(&d.TaskID, &d.DependsOnID); err != nil {
			continue
		}
		deps = append(deps, d)
	}
	return deps
}

func (s *Store) exportSubscriptions() []model.ExportSubscription {
	rows, err := s.db.Query("SELECT task_id,user_id FROM task_subscriptions")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var subs []model.ExportSubscription
	for rows.Next() {
		var sub model.ExportSubscription
		if err := rows.Scan(&sub.TaskID, &sub.UserID); err != nil {
			continue
		}
		subs = append(subs, sub)
	}
	return subs
}

func (s *Store) exportFiles() []model.ExportFile {
	rows, err := s.db.Query("SELECT id,filename,mime,data,COALESCE(compressed,0) FROM files")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var files []model.ExportFile
	for rows.Next() {
		var f model.ExportFile
		var compressed int
		if err := rows.Scan(&f.ID, &f.Filename, &f.Mime, &f.Data, &compressed); err != nil {
			continue
		}
		// Export the original bytes so backups stay portable / version-independent.
		if compressed == 1 {
			if raw, derr := gunzipBytes(f.Data); derr == nil {
				f.Data = raw
			}
		}
		files = append(files, f)
	}
	return files
}

func (s *Store) exportImages() []model.ExportFile {
	rows, err := s.db.Query("SELECT id,mime,data FROM images")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var images []model.ExportFile
	for rows.Next() {
		var f model.ExportFile
		if err := rows.Scan(&f.ID, &f.Mime, &f.Data); err != nil {
			continue
		}
		f.Filename = "" // images table has no filename column
		images = append(images, f)
	}
	return images
}

func (s *Store) flatComments(taskID int64) []model.Comment {
	rows, err := s.db.Query("SELECT id,task_id,COALESCE(parent_id,0),text,created_at,COALESCE(updated_at,created_at) FROM comments WHERE task_id=? ORDER BY created_at", taskID)
	if err != nil {
		return []model.Comment{}
	}
	defer rows.Close()
	comments := make([]model.Comment, 0)
	for rows.Next() {
		var c model.Comment
		var pid int64
		var ca, ua string
		if err := rows.Scan(&c.ID, &c.TaskID, &pid, &c.Text, &ca, &ua); err != nil {
			continue
		}
		c.CreatedAt = parseTime(ca)
		c.UpdatedAt = parseTime(ua)
		if pid != 0 {
			c.ParentID = &pid
		}
		comments = append(comments, c)
	}
	return comments
}

// --- Task Dependencies ---

func (s *Store) SetTaskDependencies(taskID int64, dependsOnIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM task_dependencies WHERE task_id=?", taskID); err != nil {
		return err
	}
	for _, depID := range dependsOnIDs {
		if depID == taskID {
			continue // no self-dependency
		}
		// Check for circular dependency: would depID transitively depend on taskID?
		if s.wouldCreateCycle(tx, taskID, depID) {
			continue // skip — would create circular dependency
		}
		if _, err := tx.Exec("INSERT INTO task_dependencies(task_id,depends_on_id) VALUES(?,?) ON CONFLICT DO NOTHING", taskID, depID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// wouldCreateCycle checks if adding taskID->depID would create a cycle
// by checking if depID already transitively depends on taskID.
func (s *Store) wouldCreateCycle(tx *dbTx, taskID, depID int64) bool {
	visited := map[int64]bool{}
	return s.reachable(tx, depID, taskID, visited, maxRecursionDepth)
}

func (s *Store) reachable(tx *dbTx, from, target int64, visited map[int64]bool, maxDepth int) bool {
	if from == target {
		return true
	}
	if maxDepth <= 0 || visited[from] {
		return false
	}
	visited[from] = true
	rows, err := tx.Query("SELECT depends_on_id FROM task_dependencies WHERE task_id=?", from)
	if err != nil {
		return false
	}
	defer rows.Close()
	var deps []int64
	for rows.Next() {
		var d int64
		if rows.Scan(&d) == nil {
			deps = append(deps, d)
		}
	}
	rows.Close()
	for _, d := range deps {
		if s.reachable(tx, d, target, visited, maxDepth-1) {
			return true
		}
	}
	return false
}

func (s *Store) TaskDependencies(taskID int64) []model.TaskDep {
	rows, err := s.db.Query(`SELECT t.id, t.title, t.column_id FROM tasks t
		JOIN task_dependencies td ON td.depends_on_id = t.id WHERE td.task_id=?`, taskID)
	if err != nil {
		return []model.TaskDep{}
	}
	defer rows.Close()
	deps := make([]model.TaskDep, 0)
	for rows.Next() {
		var d model.TaskDep
		if err := rows.Scan(&d.ID, &d.Title, &d.ColumnID); err != nil {
			continue
		}
		deps = append(deps, d)
	}
	return deps
}

func (s *Store) TaskDependents(taskID int64) []model.TaskDep {
	rows, err := s.db.Query(`SELECT t.id, t.title, t.column_id FROM tasks t
		JOIN task_dependencies td ON td.task_id = t.id WHERE td.depends_on_id=?`, taskID)
	if err != nil {
		return []model.TaskDep{}
	}
	defer rows.Close()
	deps := make([]model.TaskDep, 0)
	for rows.Next() {
		var d model.TaskDep
		if err := rows.Scan(&d.ID, &d.Title, &d.ColumnID); err != nil {
			continue
		}
		deps = append(deps, d)
	}
	return deps
}

// --- Users ---

func (s *Store) UserCount() (int, error) {
	var cnt int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&cnt)
	return cnt, err
}

func (s *Store) CreateUser(username, password string, role string) (int64, error) {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return 0, err
	}
	isAdmin := 0
	if role == "admin" {
		isAdmin = 1
	}
	return s.db.ExecInsert("INSERT INTO users(username,password_hash,is_admin,role) VALUES(?,?,?,?)", username, hash, isAdmin, role)
}

func (s *Store) AuthenticateUser(username, password string) (*model.User, error) {
	var u model.User
	var hash string
	var admin int
	var role string
	err := s.db.QueryRow("SELECT id,username,password_hash,is_admin,role,created_at FROM users WHERE LOWER(username)=LOWER(?)", username).
		Scan(&u.ID, &u.Username, &hash, &admin, &role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if !auth.CheckPassword(password, hash) {
		return nil, fmt.Errorf("invalid credentials")
	}
	u.IsAdmin = admin == 1
	u.Role = role
	return &u, nil
}

func (s *Store) CreateSession(userID int64) (string, error) {
	token, err := auth.GenerateToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(sessionDuration).Format("2006-01-02 15:04:05")
	_, err = s.db.Exec("INSERT INTO sessions(token,user_id,expires_at) VALUES(?,?,?)", token, userID, expires)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) ValidateSession(token string) (*model.User, error) {
	var u model.User
	var admin int
	var role string
	err := s.db.QueryRow(`SELECT u.id,u.username,u.is_admin,u.role,u.created_at FROM users u
		JOIN sessions s ON s.user_id=u.id
		WHERE s.token=? AND s.expires_at > datetime('now')`, token).
		Scan(&u.ID, &u.Username, &admin, &role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.IsAdmin = admin == 1
	u.Role = role
	return &u, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token=?", token)
	return err
}

func (s *Store) CleanExpiredSessions() {
	s.db.Exec("DELETE FROM sessions WHERE expires_at <= datetime('now')")
}

func (s *Store) ListUsers() ([]model.User, error) {
	rows, err := s.db.Query("SELECT id,username,is_admin,role,created_at,telegram_chat_id FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]model.User, 0)
	for rows.Next() {
		var u model.User
		var admin int
		var role string
		if err := rows.Scan(&u.ID, &u.Username, &admin, &role, &u.CreatedAt, &u.TelegramID); err != nil {
			return nil, err
		}
		u.IsAdmin = admin == 1
		u.Role = role
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) GetUser(id int64) (*model.User, error) {
	var u model.User
	var admin int
	var role string
	err := s.db.QueryRow("SELECT id,username,is_admin,role,created_at,telegram_chat_id,link_hash FROM users WHERE id=?", id).
		Scan(&u.ID, &u.Username, &admin, &role, &u.CreatedAt, &u.TelegramID, &u.LinkHash)
	if err != nil {
		return nil, err
	}
	u.IsAdmin = admin == 1
	u.Role = role
	return &u, nil
}

func (s *Store) GetUserByUsername(username string) (*model.User, error) {
	var u model.User
	var admin int
	var role string
	err := s.db.QueryRow("SELECT id,username,is_admin,role,created_at,telegram_chat_id FROM users WHERE LOWER(username)=LOWER(?)", username).
		Scan(&u.ID, &u.Username, &admin, &role, &u.CreatedAt, &u.TelegramID)
	if err != nil {
		return nil, err
	}
	u.IsAdmin = admin == 1
	u.Role = role
	return &u, nil
}

func (s *Store) DeleteUser(id int64) error {
	// Don't delete the last admin
	var adminCount int
	s.db.QueryRow("SELECT COUNT(*) FROM users WHERE is_admin=1").Scan(&adminCount)
	var isAdmin int
	s.db.QueryRow("SELECT is_admin FROM users WHERE id=?", id).Scan(&isAdmin)
	if isAdmin == 1 && adminCount <= 1 {
		return fmt.Errorf("cannot delete the last admin user")
	}
	// Check if it's the only user
	var totalCount int
	s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalCount)
	if totalCount <= 1 {
		return fmt.Errorf("cannot delete the only user")
	}
	_, err := s.db.Exec("DELETE FROM sessions WHERE user_id=?", id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("DELETE FROM users WHERE id=?", id)
	return err
}

func (s *Store) UpdateUserPassword(id int64, newPassword string) error {
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("UPDATE users SET password_hash=? WHERE id=?", hash, id)
	return err
}

func (s *Store) UpdateUserRole(id int64, role string) error {
	isAdmin := 0
	if role == "admin" {
		isAdmin = 1
	}
	_, err := s.db.Exec("UPDATE users SET role=?,is_admin=? WHERE id=?", role, isAdmin, id)
	return err
}

// FindOrCreateSSOUser finds a user by auth provider and external ID,
// or creates a new one if not found. Used for LDAP/OIDC authentication.
func (s *Store) FindOrCreateSSOUser(provider, externalID, username, role string) (*model.User, error) {
	// Try to find by provider + external_id
	var u model.User
	var admin int
	var dbRole string
	err := s.db.QueryRow(
		"SELECT id,username,is_admin,role,created_at FROM users WHERE auth_provider=? AND external_id=?",
		provider, externalID,
	).Scan(&u.ID, &u.Username, &admin, &dbRole, &u.CreatedAt)
	if err == nil {
		u.IsAdmin = admin == 1
		u.Role = dbRole
		// Update username if changed in external provider
		if u.Username != username {
			s.db.Exec("UPDATE users SET username=? WHERE id=?", username, u.ID)
			u.Username = username
		}
		return &u, nil
	}

	// Try to find by username (link existing local account)
	err = s.db.QueryRow(
		"SELECT id,username,is_admin,role,created_at FROM users WHERE LOWER(username)=LOWER(?) AND auth_provider='local'",
		username,
	).Scan(&u.ID, &u.Username, &admin, &dbRole, &u.CreatedAt)
	if err == nil {
		// Link existing account to SSO provider
		s.db.Exec("UPDATE users SET auth_provider=?,external_id=? WHERE id=?", provider, externalID, u.ID)
		u.IsAdmin = admin == 1
		u.Role = dbRole
		return &u, nil
	}

	// Create new user
	isAdmin := 0
	if role == "admin" {
		isAdmin = 1
	}
	// SSO users get a random placeholder password (they authenticate externally)
	placeholder, _ := auth.GenerateToken()
	placeholderHash, _ := auth.HashPassword(placeholder)

	id, err := s.db.ExecInsert(
		"INSERT INTO users(username,password_hash,is_admin,role,auth_provider,external_id) VALUES(?,?,?,?,?,?)",
		username, placeholderHash, isAdmin, role, provider, externalID,
	)
	if err != nil {
		return nil, fmt.Errorf("create SSO user: %w", err)
	}
	return &model.User{
		ID:       id,
		Username: username,
		Role:     role,
		IsAdmin:  isAdmin == 1,
	}, nil
}

func (s *Store) UpdateUserTelegram(id int64, chatID int64) error {
	_, err := s.db.Exec("UPDATE users SET telegram_chat_id=? WHERE id=?", chatID, id)
	return err
}

func (s *Store) GenerateLinkHash(userID int64) (string, error) {
	hash, err := auth.GenerateToken()
	if err != nil {
		return "", err
	}
	// Use first 16 chars for a shorter hash
	hash = hash[:linkHashLen]
	_, err = s.db.Exec("UPDATE users SET link_hash=? WHERE id=?", hash, userID)
	if err != nil {
		return "", err
	}
	return hash, nil
}

func (s *Store) FindUserByLinkHash(hash string) (*model.User, error) {
	if hash == "" {
		return nil, fmt.Errorf("empty hash")
	}
	var u model.User
	var admin int
	var role string
	err := s.db.QueryRow("SELECT id,username,is_admin,role,created_at FROM users WHERE link_hash=?", hash).
		Scan(&u.ID, &u.Username, &admin, &role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.IsAdmin = admin == 1
	u.Role = role
	return &u, nil
}

func (s *Store) ClearLinkHash(userID int64) error {
	_, err := s.db.Exec("UPDATE users SET link_hash='' WHERE id=?", userID)
	return err
}

func (s *Store) FindUserByChatID(chatID int64) *model.User {
	if chatID == 0 {
		return nil
	}
	var u model.User
	var admin int
	var role string
	err := s.db.QueryRow("SELECT id,username,is_admin,role,created_at,telegram_chat_id FROM users WHERE telegram_chat_id=?", chatID).
		Scan(&u.ID, &u.Username, &admin, &role, &u.CreatedAt, &u.TelegramID)
	if err != nil {
		return nil
	}
	u.IsAdmin = admin == 1
	u.Role = role
	return &u
}

func (s *Store) UnlinkTelegram(userID int64) error {
	_, err := s.db.Exec("UPDATE users SET telegram_chat_id=0 WHERE id=?", userID)
	return err
}

// --- Password Reset ---

func (s *Store) SetResetCode(userID int64, code string) error {
	expires := time.Now().UTC().Add(resetCodeDuration).Format("2006-01-02 15:04:05")
	_, err := s.db.Exec("UPDATE users SET reset_code=?, reset_code_expires=? WHERE id=?", code, expires, userID)
	if err == nil {
		s.logf("set reset code for user %d, expires %s", userID, expires)
	}
	return err
}

func (s *Store) ValidateResetCode(username, code string) (*model.User, error) {
	var u model.User
	var admin int
	var role string
	var resetCode string
	var expiresStr sql.NullString
	err := s.db.QueryRow("SELECT id,username,is_admin,role,created_at,reset_code,reset_code_expires FROM users WHERE LOWER(username)=LOWER(?)", username).
		Scan(&u.ID, &u.Username, &admin, &role, &u.CreatedAt, &resetCode, &expiresStr)
	if err != nil {
		s.logf("validate reset code: user %q not found: %v", username, err)
		return nil, fmt.Errorf("user not found")
	}
	if resetCode == "" || resetCode != code {
		s.logf("validate reset code: user %q code mismatch (stored=%q, provided=%q, stored_len=%d, provided_len=%d)", username, resetCode, code, len(resetCode), len(code))
		return nil, fmt.Errorf("invalid code")
	}
	if expiresStr.Valid {
		expires := parseTime(expiresStr.String)
		now := time.Now().UTC()
		if now.After(expires) {
			s.logf("validate reset code: user %q code expired (expires=%s, now=%s)", username, expiresStr.String, now.Format("2006-01-02 15:04:05"))
			return nil, fmt.Errorf("code expired")
		}
	}
	u.IsAdmin = admin == 1
	u.Role = role
	return &u, nil
}

func (s *Store) ClearResetCode(userID int64) error {
	_, err := s.db.Exec("UPDATE users SET reset_code='', reset_code_expires=NULL WHERE id=?", userID)
	return err
}

// --- Notifications ---

func (s *Store) CreateNotification(userID int64, typ, text string, taskID *int64) (int64, error) {
	return s.db.ExecInsert("INSERT INTO notifications(user_id,type,text,task_id) VALUES(?,?,?,?)",
		userID, typ, text, taskID)
}

func (s *Store) ListNotifications(userID int64, limit int) ([]model.Notification, error) {
	rows, err := s.db.Query(`SELECT id,user_id,type,text,task_id,is_read,created_at
		FROM notifications WHERE user_id=? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	notifs := make([]model.Notification, 0)
	for rows.Next() {
		var n model.Notification
		var taskID sql.NullInt64
		var ca string
		var isRead int
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Text, &taskID, &isRead, &ca); err != nil {
			continue
		}
		n.CreatedAt = parseTime(ca)
		n.IsRead = isRead == 1
		if taskID.Valid {
			n.TaskID = &taskID.Int64
		}
		notifs = append(notifs, n)
	}
	return notifs, rows.Err()
}

func (s *Store) UnreadNotificationCount(userID int64) int {
	var cnt int
	s.db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id=? AND is_read=0", userID).Scan(&cnt)
	return cnt
}

func (s *Store) MarkNotificationRead(id, userID int64) error {
	_, err := s.db.Exec("UPDATE notifications SET is_read=1 WHERE id=? AND user_id=?", id, userID)
	return err
}

func (s *Store) MarkAllNotificationsRead(userID int64) error {
	_, err := s.db.Exec("UPDATE notifications SET is_read=1 WHERE user_id=?", userID)
	return err
}

// --- Task Subscriptions ---

func (s *Store) SubscribeToTask(taskID, userID int64) error {
	_, err := s.db.Exec("INSERT INTO task_subscriptions(task_id,user_id) VALUES(?,?) ON CONFLICT DO NOTHING", taskID, userID)
	return err
}

func (s *Store) UnsubscribeFromTask(taskID, userID int64) error {
	_, err := s.db.Exec("DELETE FROM task_subscriptions WHERE task_id=? AND user_id=?", taskID, userID)
	return err
}

func (s *Store) IsSubscribed(taskID, userID int64) bool {
	var cnt int
	s.db.QueryRow("SELECT COUNT(*) FROM task_subscriptions WHERE task_id=? AND user_id=?", taskID, userID).Scan(&cnt)
	return cnt > 0
}

func (s *Store) TaskSubscribers(taskID int64) []model.User {
	rows, err := s.db.Query(`SELECT u.id,u.username,u.is_admin,u.role,u.telegram_chat_id
		FROM users u JOIN task_subscriptions ts ON ts.user_id=u.id WHERE ts.task_id=?`, taskID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var u model.User
		var admin int
		var role string
		var tgID int64
		if err := rows.Scan(&u.ID, &u.Username, &admin, &role, &tgID); err != nil {
			continue
		}
		u.IsAdmin = admin == 1
		u.Role = role
		u.TelegramID = tgID
		users = append(users, u)
	}
	return users
}

// --- Activity Log ---

func (s *Store) LogActivity(userID int64, action string, taskID *int64, details string) {
	s.LogActivityP(userID, action, taskID, nil, details)
}

// LogActivityP records an activity entry tagged with a project (nil when none).
func (s *Store) LogActivityP(userID int64, action string, taskID, projectID *int64, details string) {
	s.db.Exec("INSERT INTO activity_log(user_id,action,task_id,project_id,details) VALUES(?,?,?,?,?)",
		userID, action, taskID, projectID, details)
}

// ListAuditLog returns activity entries (most recent first) for the admin audit
// view, optionally filtered by user, project and action.
func (s *Store) ListAuditLog(userID, projectID *int64, action string, limit int) ([]model.ActivityEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	q := `SELECT a.id, a.user_id, COALESCE(u.username,''), a.action, a.task_id, a.project_id,
		COALESCE(p.name,''), a.details, a.created_at
		FROM activity_log a
		LEFT JOIN users u ON u.id = a.user_id
		LEFT JOIN projects p ON p.id = a.project_id
		WHERE 1=1`
	var args []any
	if userID != nil {
		q += " AND a.user_id = ?"
		args = append(args, *userID)
	}
	if projectID != nil {
		q += " AND a.project_id = ?"
		args = append(args, *projectID)
	}
	if action != "" {
		q += " AND a.action = ?"
		args = append(args, action)
	}
	q += " ORDER BY a.created_at DESC, a.id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ActivityEntry{}
	for rows.Next() {
		var e model.ActivityEntry
		var taskID, projID sql.NullInt64
		var ca string
		if err := rows.Scan(&e.ID, &e.UserID, &e.Username, &e.Action, &taskID, &projID, &e.ProjectName, &e.Details, &ca); err != nil {
			continue
		}
		e.CreatedAt = parseTime(ca)
		if taskID.Valid {
			e.TaskID = &taskID.Int64
		}
		if projID.Valid {
			e.ProjectID = &projID.Int64
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) UserActivity(userID int64, limit int) ([]model.ActivityEntry, error) {
	rows, err := s.db.Query(`SELECT id,user_id,action,task_id,details,created_at
		FROM activity_log WHERE user_id=? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]model.ActivityEntry, 0)
	for rows.Next() {
		var e model.ActivityEntry
		var taskID sql.NullInt64
		var ca string
		if err := rows.Scan(&e.ID, &e.UserID, &e.Action, &taskID, &e.Details, &ca); err != nil {
			continue
		}
		e.CreatedAt = parseTime(ca)
		if taskID.Valid {
			e.TaskID = &taskID.Int64
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// --- App Settings ---

func (s *Store) GetSetting(key string) string {
	var val string
	s.db.QueryRow("SELECT value FROM app_settings WHERE key=?", key).Scan(&val)
	return val
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO app_settings(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// --- Import ---

func (s *Store) ImportAll(data *model.ExportData) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing data (children before parents; project_members is cleared so
	// the later DELETE FROM users has no referencing rows).
	for _, tbl := range []string{"task_subscriptions", "task_dependencies", "comments", "task_tags", "tasks", "tags", "epics", "sprints", "columns", "project_members"} {
		if _, err := tx.Exec("DELETE FROM " + tbl); err != nil {
			return fmt.Errorf("clear %s: %w", tbl, err)
		}
	}
	// Drop non-default projects only when the backup carries its own project list
	// (new-format export); otherwise keep the seeded default project "Общее".
	if len(data.Projects) > 0 {
		if _, err := tx.Exec("DELETE FROM projects WHERE id<>1"); err != nil {
			return fmt.Errorf("clear projects: %w", err)
		}
	}

	// Import users if present (for full restore)
	if len(data.Users) > 0 {
		// Service tables reference users(id); clear them first so DELETE FROM users
		// doesn't hit a foreign-key violation (Postgres enforces FKs immediately,
		// and a failed-but-ignored statement would poison the whole transaction).
		if err := clearEcosystem(tx); err != nil {
			return err
		}
		tx.Exec("DELETE FROM sessions")
		tx.Exec("DELETE FROM notifications")
		tx.Exec("DELETE FROM activity_log")
		tx.Exec("DELETE FROM users")
		for _, u := range data.Users {
			admin := 0
			if u.IsAdmin {
				admin = 1
			}
			if _, err := tx.Exec("INSERT INTO users(id,username,password_hash,role,is_admin,telegram_chat_id) VALUES(?,?,?,?,?,?)",
				u.ID, u.Username, u.PasswordHash, u.Role, admin, u.TelegramID); err != nil {
				return fmt.Errorf("import user: %w", err)
			}
		}
	}

	// Import settings if present (full replace; exports include all keys incl. secrets).
	if len(data.Settings) > 0 {
		tx.Exec("DELETE FROM app_settings")
		for _, st := range data.Settings {
			tx.Exec("INSERT INTO app_settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", st.Key, st.Value)
		}
	}

	// Import projects (after users, before kanban tables that reference them).
	// The default project 1 already exists from the seed, so upsert it.
	for _, p := range data.Projects {
		if _, err := tx.Exec(`INSERT INTO projects(id,name,key,description,visibility,created_by,created_at) VALUES(?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET name=excluded.name, key=excluded.key, description=excluded.description, visibility=excluded.visibility, created_by=excluded.created_by`,
			p.ID, p.Name, p.Key, p.Description, p.Visibility, p.CreatedBy, p.CreatedAt); err != nil {
			return fmt.Errorf("import project: %w", err)
		}
	}
	// Import project members
	for _, m := range data.ProjectMembers {
		if _, err := tx.Exec("INSERT INTO project_members(project_id,user_id,role) VALUES(?,?,?)", m.ProjectID, m.UserID, m.Role); err != nil {
			return fmt.Errorf("import project member: %w", err)
		}
	}
	// Import columns
	for _, c := range data.Columns {
		if _, err := tx.Exec("INSERT INTO columns(id,project_id,name,position) VALUES(?,?,?,?)", c.ID, projectIDOrDefault(c.ProjectID), c.Name, c.Position); err != nil {
			return fmt.Errorf("import column: %w", err)
		}
	}
	// Import epics
	for _, e := range data.Epics {
		if _, err := tx.Exec("INSERT INTO epics(id,project_id,name,color,description) VALUES(?,?,?,?,?)", e.ID, projectIDOrDefault(e.ProjectID), e.Name, e.Color, e.Description); err != nil {
			return fmt.Errorf("import epic: %w", err)
		}
	}
	// Import sprints
	for _, sp := range data.Sprints {
		if _, err := tx.Exec("INSERT INTO sprints(id,project_id,name,start_date,end_date,status) VALUES(?,?,?,?,?,?)", sp.ID, projectIDOrDefault(sp.ProjectID), sp.Name, sp.StartDate, sp.EndDate, sp.Status); err != nil {
			return fmt.Errorf("import sprint: %w", err)
		}
	}
	// Import tags
	for _, t := range data.Tags {
		if _, err := tx.Exec("INSERT INTO tags(id,project_id,name,color) VALUES(?,?,?,?)", t.ID, projectIDOrDefault(t.ProjectID), t.Name, t.Color); err != nil {
			return fmt.Errorf("import tag: %w", err)
		}
	}
	// Import tasks
	for _, t := range data.Tasks {
		if _, err := tx.Exec(`INSERT INTO tasks(id,project_id,number,title,description,todo,project_url,column_id,epic_id,sprint_id,assignee_id,position,priority,deadline,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			t.ID, projectIDOrDefault(t.ProjectID), t.Number, t.Title, t.Description, t.Todo, t.ProjectURL, t.ColumnID, t.EpicID, t.SprintID, t.AssigneeID, t.Position, t.Priority, t.Deadline,
			t.CreatedAt.Format("2006-01-02 15:04:05"), t.UpdatedAt.Format("2006-01-02 15:04:05")); err != nil {
			return fmt.Errorf("import task: %w", err)
		}
		for _, tag := range t.Tags {
			tx.Exec("INSERT INTO task_tags(task_id,tag_id) VALUES(?,?) ON CONFLICT DO NOTHING", t.ID, tag.ID)
		}
	}
	// Import comments
	for _, c := range data.Comments {
		authorID := c.AuthorID
		if _, err := tx.Exec("INSERT INTO comments(id,task_id,parent_id,author_id,text,created_at,updated_at) VALUES(?,?,?,?,?,?,?)",
			c.ID, c.TaskID, c.ParentID, authorID, c.Text,
			c.CreatedAt.Format("2006-01-02 15:04:05"), c.UpdatedAt.Format("2006-01-02 15:04:05")); err != nil {
			return fmt.Errorf("import comment: %w", err)
		}
	}

	// Import dependencies
	for _, d := range data.Dependencies {
		tx.Exec("INSERT INTO task_dependencies(task_id,depends_on_id) VALUES(?,?) ON CONFLICT DO NOTHING", d.TaskID, d.DependsOnID)
	}

	// Import subscriptions
	for _, sub := range data.Subscriptions {
		tx.Exec("INSERT INTO task_subscriptions(task_id,user_id) VALUES(?,?) ON CONFLICT DO NOTHING", sub.TaskID, sub.UserID)
	}

	// Import files (re-compress compressible blobs on the way in).
	if len(data.Files) > 0 {
		tx.Exec("DELETE FROM files")
		for _, f := range data.Files {
			stored, compressed := maybeCompress(f.Mime, f.Data)
			tx.Exec("INSERT INTO files(id,filename,data,mime,size,compressed) VALUES(?,?,?,?,?,?)",
				f.ID, f.Filename, stored, f.Mime, len(f.Data), compressed)
		}
	}

	// Import images
	if len(data.Images) > 0 {
		tx.Exec("DELETE FROM images")
		for _, f := range data.Images {
			tx.Exec("INSERT INTO images(id,data,mime) VALUES(?,?,?)", f.ID, f.Data, f.Mime)
		}
	}

	// Import ecosystem services (wiki/chat/calendar) if present
	if err := importEcosystem(tx, data.Ecosystem); err != nil {
		return err
	}

	// Reset auto-increment/identity counters so future inserts don't collide
	// with the explicit ids we just restored.
	s.resetSequences(tx)

	return tx.Commit()
}

// projectIDOrDefault maps a missing/zero project id to the default project (1),
// so older single-project exports import cleanly into the project-scoped schema.
func projectIDOrDefault(id int64) int64 {
	if id == 0 {
		return 1
	}
	return id
}

// resetSequences re-syncs id generators after an explicit-id bulk import. For
// SQLite it clears sqlite_sequence; for Postgres it sets each IDENTITY sequence
// to MAX(id) (is_called=true when the table has rows, so nextval = MAX(id)+1).
func (s *Store) resetSequences(tx *dbTx) {
	if s.db.d != dialectPostgres {
		tx.Exec("DELETE FROM sqlite_sequence")
		return
	}
	tables := []string{
		"projects",
		"columns", "epics", "tags", "tasks", "comments", "images", "files", "sprints",
		"users", "notifications", "activity_log",
		"wiki_spaces", "wiki_pages", "wiki_revisions",
		"chat_channels", "chat_messages", "chat_reactions", "chat_attachments",
		"calendars", "calendar_events", "calendar_reminders", "calendar_subscriptions", "calendar_sub_events",
		"mail_messages", "mail_box", "mail_attachments", "mail_folders", "mail_rules", "mail_audit", "mail_aliases",
	}
	for _, t := range tables {
		tx.Exec(`SELECT setval(pg_get_serial_sequence('` + t + `','id'),
			(SELECT COALESCE(MAX(id),1) FROM ` + t + `),
			(SELECT COUNT(*) > 0 FROM ` + t + `))`)
	}
}

// DatabaseChecksum returns a quick checksum based on row counts and max IDs to detect changes.
func (s *Store) DatabaseChecksum() string {
	var checksum string
	tables := []string{"tasks", "comments", "columns", "epics", "sprints", "tags", "users", "files", "images"}
	for _, t := range tables {
		var cnt, maxID int64
		s.db.QueryRow("SELECT COUNT(*), COALESCE(MAX(id),0) FROM " + t).Scan(&cnt, &maxID)
		checksum += fmt.Sprintf("%s:%d:%d;", t, cnt, maxID)
	}
	var lastTask, lastComment string
	s.db.QueryRow("SELECT COALESCE(CAST(MAX(updated_at) AS TEXT),'') FROM tasks").Scan(&lastTask)
	s.db.QueryRow("SELECT COALESCE(CAST(MAX(updated_at) AS TEXT),'') FROM comments").Scan(&lastComment)
	checksum += fmt.Sprintf("lt:%s;lc:%s", lastTask, lastComment)
	return checksum
}
