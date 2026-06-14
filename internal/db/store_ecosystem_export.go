package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// ecosystemTables lists service tables in parent-before-child order (for insert).
// Clearing on import walks this list in reverse so foreign keys stay satisfied.
var ecosystemTables = []string{
	"wiki_spaces", "wiki_space_members", "wiki_pages", "wiki_revisions",
	"chat_channels", "chat_members", "chat_messages",
	"calendars", "calendar_events",
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

// importEcosystem clears and restores the service tables inside an existing
// transaction. SQLite's column affinity coerces JSON float64 ids back to integers.
func importEcosystem(tx *sql.Tx, eco map[string][]map[string]any) error {
	if len(eco) == 0 {
		return nil
	}
	for i := len(ecosystemTables) - 1; i >= 0; i-- {
		if _, err := tx.Exec("DELETE FROM " + ecosystemTables[i]); err != nil {
			return fmt.Errorf("clear %s: %w", ecosystemTables[i], err)
		}
	}
	for _, tbl := range ecosystemTables {
		for _, row := range eco[tbl] {
			cols := make([]string, 0, len(row))
			ph := make([]string, 0, len(row))
			vals := make([]any, 0, len(row))
			for c, v := range row {
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
