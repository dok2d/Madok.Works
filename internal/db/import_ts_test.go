package db

import (
	"testing"

	"kanban/internal/model"
)

// TestImportProjectEmptyTimestamp reproduces a converted (pre-projects) backup
// whose synthesized project has no created_at: Postgres rejects "" for a
// timestamptz column, so the import must substitute a valid timestamp.
func TestImportProjectEmptyTimestamp(t *testing.T) {
	s := newTestStore(t)
	data := &model.ExportData{
		Projects: []model.Project{{ID: 5, Name: "Imported", Key: "IMP", Visibility: "open", CreatedAt: ""}},
	}
	if err := s.ImportAll(data); err != nil {
		t.Fatalf("ImportAll with empty project timestamp: %v", err)
	}
	var ca string
	if err := s.db.QueryRow("SELECT CAST(created_at AS TEXT) FROM projects WHERE id=?", 5).Scan(&ca); err != nil {
		t.Fatalf("read imported project: %v", err)
	}
	if ca == "" {
		t.Fatal("imported project created_at is empty")
	}
}
