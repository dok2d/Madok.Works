package db

import (
	"encoding/json"
	"testing"

	"kanban/internal/model"
)

// TestImportResetsSequences verifies that after a bulk import with explicit ids,
// the id generator is re-synced so a fresh insert doesn't collide with imported
// rows. This exercises the SQLite (sqlite_sequence) and Postgres (identity
// setval) branches of resetSequences.
func TestImportResetsSequences(t *testing.T) {
	a := newTestStore(t)
	colID, err := a.CreateColumn(1, "Todo")
	if err != nil {
		t.Fatalf("CreateColumn: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := a.CreateTask(1, "t", "", "", "", colID, nil, nil, nil, 0, nil, ""); err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
	}

	exp, err := a.ExportAll()
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	raw, _ := json.Marshal(exp)
	var data model.ExportData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	b := newTestStore(t)
	if err := b.ImportAll(&data); err != nil {
		t.Fatalf("ImportAll: %v", err)
	}

	tasks, err := b.ListTasks(1)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	var maxID int64
	for _, tk := range tasks {
		if tk.ID > maxID {
			maxID = tk.ID
		}
	}
	if maxID == 0 {
		t.Fatalf("no tasks restored")
	}

	newID, err := b.CreateTask(1, "new", "", "", "", colID, nil, nil, nil, 0, nil, "")
	if err != nil {
		t.Fatalf("CreateTask after import: %v", err)
	}
	if newID <= maxID {
		t.Errorf("sequence not reset: new id %d <= max imported id %d", newID, maxID)
	}
}
