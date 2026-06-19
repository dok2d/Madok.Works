package db

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

// ecosystemTables lists service tables in parent-before-child order (for insert).
// Clearing on import walks this list in reverse so foreign keys stay satisfied.
var ecosystemTables = []string{
	"wiki_spaces", "wiki_space_members", "wiki_pages", "wiki_revisions", "wiki_links", "wiki_comments", "wiki_reactions", "wiki_diagrams", "wiki_page_pins",
	"chat_channels", "chat_members", "chat_messages", "chat_attachments", "chat_reactions", "chat_pins", "chat_mutes",
	"git_repos", "git_events",
	"calendars", "calendar_events", "calendar_reminders",
	"calendar_subscriptions", "calendar_sub_events",
	"mail_messages", "mail_box", "mail_attachments",
	"mail_folders", "mail_rules", "mail_audit", "mail_aliases",
}

// selfRefCols lists columns whose FK points back at the same table's id. On
// import these are deferred (inserted NULL, then set in a second pass) so any
// parent/child ordering loads on Postgres, which checks FKs immediately.
var selfRefCols = map[string]string{
	"wiki_pages":    "parent_id",
	"chat_messages": "reply_to",
}

// dumpTable returns every row of a table as column->value maps. Used for lossless
// JSON export of ecosystem services without per-table struct mapping. []byte
// (TEXT) values are normalized to string so they marshal as plain JSON strings.
func (s *Store) dumpTable(table string) ([]map[string]any, error) {
	rows, err := s.db.Query("SELECT * FROM " + table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			v := vals[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			m[c] = v
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// exportEcosystem dumps all service tables keyed by table name.
func (s *Store) exportEcosystem() map[string][]map[string]any {
	eco := make(map[string][]map[string]any)
	for _, tbl := range ecosystemTables {
		if rows, err := s.dumpTable(tbl); err == nil && len(rows) > 0 {
			eco[tbl] = rows
		}
	}
	return eco
}

// clearEcosystem deletes all service-table rows in child-before-parent order so
// foreign keys stay satisfied. It is also used by ImportAll to drop these rows
// before deleting users (the service tables reference users(id)) — Postgres
// enforces FKs immediately, so the order matters there.
func clearEcosystem(tx *dbTx) error {
	for i := len(ecosystemTables) - 1; i >= 0; i-- {
		if _, err := tx.Exec("DELETE FROM " + ecosystemTables[i]); err != nil {
			return fmt.Errorf("clear %s: %w", ecosystemTables[i], err)
		}
	}
	return nil
}

// txTry runs one statement inside a savepoint so that, if it fails, only that
// statement is rolled back and the surrounding transaction stays usable —
// Postgres otherwise aborts the whole transaction on any error. Used to make
// ecosystem restore best-effort across schema versions.
func txTry(tx *dbTx, q string, args ...any) error {
	if _, err := tx.Exec("SAVEPOINT eco_sp"); err != nil {
		return err
	}
	if _, err := tx.Exec(q, args...); err != nil {
		tx.Exec("ROLLBACK TO SAVEPOINT eco_sp")
		tx.Exec("RELEASE SAVEPOINT eco_sp")
		return err
	}
	_, err := tx.Exec("RELEASE SAVEPOINT eco_sp")
	return err
}

// importEcosystem clears and restores the service tables inside an existing
// transaction. SQLite's column affinity coerces JSON float64 ids back to integers.
func importEcosystem(tx *dbTx, eco map[string][]map[string]any) error {
	if len(eco) == 0 {
		return nil
	}
	if err := clearEcosystem(tx); err != nil {
		return err
	}
	for _, tbl := range ecosystemTables {
		rows := eco[tbl]
		if len(rows) == 0 {
			continue
		}
		// Only insert columns that actually exist in the current table, so a backup
		// from a slightly different schema version (renamed/removed columns) imports
		// instead of aborting the whole restore.
		valid, err := tableColumns(tx, tbl)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", tbl, err)
		}
		sort.SliceStable(rows, func(a, b int) bool { return rowID(rows[a]) < rowID(rows[b]) })
		// Self-referential FK columns (a row pointing at another row of the same
		// table) can't be satisfied by any single insert order when a parent has a
		// higher id than its child (e.g. a wiki page re-parented under a newer one).
		// Postgres enforces FKs immediately, so insert these columns as NULL and set
		// them in a second pass once every row of the table exists.
		sr := selfRefCols[tbl]
		deferSR := sr != "" && valid[sr]
		type deferred struct{ id, val any }
		var pend []deferred
		// Each row insert is wrapped in a savepoint: a row that doesn't fit the
		// current schema (e.g. legacy chat_messages from before channels, missing
		// a now-NOT-NULL column) is skipped instead of aborting the whole restore.
		skipped := 0
		for _, row := range rows {
			cols := make([]string, 0, len(row))
			ph := make([]string, 0, len(row))
			vals := make([]any, 0, len(row))
			for c, v := range row {
				if !valid[c] {
					continue // column not in the current schema — skip it
				}
				if deferSR && c == sr {
					continue // inserted as NULL now, fixed up below
				}
				cols = append(cols, c)
				ph = append(ph, "?")
				vals = append(vals, v)
			}
			if len(cols) == 0 {
				continue
			}
			q := "INSERT INTO " + tbl + "(" + strings.Join(cols, ",") + ") VALUES(" + strings.Join(ph, ",") + ")"
			if err := txTry(tx, q, vals...); err != nil {
				skipped++
				continue
			}
			if deferSR {
				if v := row[sr]; v != nil && rowID(map[string]any{"id": v}) != 0 {
					pend = append(pend, deferred{id: row["id"], val: v})
				}
			}
		}
		for _, d := range pend {
			// A deferred parent may have been skipped — tolerate the FK failure.
			_ = txTry(tx, "UPDATE "+tbl+" SET "+sr+"=? WHERE id=?", d.val, d.id)
		}
		if skipped > 0 {
			log.Printf("import: skipped %d incompatible %s row(s)", skipped, tbl)
		}
	}
	return nil
}

// tableColumns returns the set of a table's column names. It uses rows.Columns()
// on a no-row query, so it works on both SQLite and Postgres without any
// dialect-specific introspection SQL.
func tableColumns(tx *dbTx, table string) (map[string]bool, error) {
	rows, err := tx.Query("SELECT * FROM " + table + " LIMIT 0")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(cols))
	for _, c := range cols {
		set[c] = true
	}
	return set, nil
}

// rowID extracts the numeric "id" of an exported row (0 if absent). JSON decoding
// yields float64; direct dumps yield int64.
func rowID(row map[string]any) float64 {
	switch v := row["id"].(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case int:
		return float64(v)
	}
	return 0
}
