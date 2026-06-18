package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"kanban/internal/db"
	"kanban/internal/model"
)

// TestConvertRoundTrip builds a pre-projects backup (no projects / project_id /
// number), runs it through convert(), then imports the result into a fresh
// project-aware database and checks every entity landed in the default project.
func TestConvertRoundTrip(t *testing.T) {
	old := map[string]any{
		"columns": []any{
			map[string]any{"id": 1, "name": "Todo", "position": 0},
			map[string]any{"id": 2, "name": "Done", "position": 1},
		},
		"tags": []any{
			map[string]any{"id": 1, "name": "bug", "color": "#f00"},
		},
		"tasks": []any{
			map[string]any{"id": 20, "title": "second", "column_id": 1, "position": 1,
				"created_at": "2026-01-02T10:00:00Z", "updated_at": "2026-01-02T10:00:00Z"},
			map[string]any{"id": 10, "title": "first", "column_id": 1, "position": 0,
				"created_at": "2026-01-01T10:00:00Z", "updated_at": "2026-01-01T10:00:00Z"},
		},
		"ecosystem": map[string]any{
			"calendars":   []any{map[string]any{"id": 1, "name": "Work", "color": "#abc", "visibility": "private"}},
			"wiki_spaces": []any{map[string]any{"id": 1, "key": "DOC", "name": "Docs", "visibility": "open"}},
		},
	}
	raw, err := json.Marshal(old)
	if err != nil {
		t.Fatalf("marshal old: %v", err)
	}

	converted, err := convert(raw)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	var data model.ExportData
	if err := json.Unmarshal(converted, &data); err != nil {
		t.Fatalf("unmarshal converted: %v", err)
	}
	if len(data.Projects) != 1 || data.Projects[0].ID != 1 || data.Projects[0].Key != "GEN" {
		t.Fatalf("converted projects wrong: %+v", data.Projects)
	}

	store, err := db.New(filepath.Join(t.TempDir(), "conv.db"))
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	if err := store.ImportAll(&data); err != nil {
		t.Fatalf("ImportAll: %v", err)
	}

	cols, _ := store.ListColumns(1)
	if len(cols) != 2 {
		t.Errorf("expected 2 columns in project 1, got %d", len(cols))
	}
	tags, _ := store.ListTags(1)
	if len(tags) != 1 || tags[0].Name != "bug" {
		t.Errorf("tag not migrated to project 1: %+v", tags)
	}
	tasks, _ := store.ListTasks(1)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks in project 1, got %d", len(tasks))
	}
	// Numbers are assigned per project in id order: task 10 -> 1, task 20 -> 2.
	byID := map[int64]model.Task{}
	for _, tk := range tasks {
		if tk.ProjectID != 1 {
			t.Errorf("task %d project_id=%d, want 1", tk.ID, tk.ProjectID)
		}
		byID[tk.ID] = tk
	}
	if byID[10].Number != 1 || byID[20].Number != 2 {
		t.Errorf("task numbers wrong: t10=%d t20=%d", byID[10].Number, byID[20].Number)
	}

	// Ecosystem calendar and wiki space migrated into the default project.
	if cals, _ := store.ListCalendars(1, true, 1); len(cals) != 1 || cals[0].ProjectID == nil || *cals[0].ProjectID != 1 {
		t.Errorf("calendar not migrated to project 1: %+v", cals)
	}
	if spaces, _ := store.ListWikiSpaces(1, true, 1); len(spaces) != 1 || spaces[0].ProjectID == nil || *spaces[0].ProjectID != 1 {
		t.Errorf("wiki space not migrated to project 1: %+v", spaces)
	}
}
