package db

import (
	"fmt"
	"sort"
	"strings"
)

// ecosystemTables lists service tables in parent-before-child order (for insert).
// Clearing on import walks this list in reverse so foreign keys stay satisfied.
var ecosystemTables = []string{
	"wiki_spaces", "wiki_space_members", "wiki_pages", "wiki_revisions", "wiki_links", "wiki_comments", "wiki_reactions", "wiki_diagrams", "wiki_page_pins",
	"chat_channels", "chat_members", "chat_messages", "chat_attachments", "chat_reactions", "chat_pins",
	"calendars", "calendar_events", "calendar_reminders",
	"calendar_subscriptions", "calendar_sub_events",
	"mail_messages", "mail_box", "mail_attachments",
	"mail_folders", "mail_rules", "mail_audit", "mail_aliases",
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
		// Insert in ascending id order so self-referential rows (wiki_pages.parent_id,
		// chat_messages.reply_to) see their parent first — Postgres enforces FKs
		// immediately. Rows without an id column keep their original order.
		sort.SliceStable(rows, func(a, b int) bool { return rowID(rows[a]) < rowID(rows[b]) })
		for _, row := range rows {
			cols := make([]string, 0, len(row))
			ph := make([]string, 0, len(row))
			vals := make([]any, 0, len(row))
			for c, v := range row {
				if !valid[c] {
					continue // column not in the current schema — skip it
				}
				cols = append(cols, c)
				ph = append(ph, "?")
				vals = append(vals, v)
			}
			if len(cols) == 0 {
				continue
			}
			q := "INSERT INTO " + tbl + "(" + strings.Join(cols, ",") + ") VALUES(" + strings.Join(ph, ",") + ")"
			if _, err := tx.Exec(q, vals...); err != nil {
				return fmt.Errorf("import %s: %w", tbl, err)
			}
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
