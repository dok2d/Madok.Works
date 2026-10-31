package db

import (
	"encoding/json"
	"testing"

	"kanban/internal/model"
)

// A backup must carry each comment's author: restoring one used to set every
// comment's author_id to NULL because the export never selected it.
func TestExportImportKeepsCommentAuthors(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")
	var col int64
	s.db.QueryRow("SELECT id FROM columns ORDER BY id LIMIT 1").Scan(&col)
	task, err := s.CreateTask(1, "t", "", "", "", col, nil, nil, nil, 0, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	top, _ := s.AddComment(task, "from alice", nil, &alice)
	reply, _ := s.AddComment(task, "from bob", &top, &bob)
	anon, _ := s.AddComment(task, "no author", nil, nil)

	data, err := s.ExportAll()
	if err != nil {
		t.Fatal(err)
	}
	// Go through JSON like a real backup file does.
	raw, _ := json.Marshal(data)
	var back model.ExportData
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if err := s.ImportAll(&back); err != nil {
		t.Fatalf("ImportAll: %v", err)
	}

	want := map[int64]int64{top: alice, reply: bob, anon: 0}
	for id, author := range want {
		var got int64
		if err := s.db.QueryRow("SELECT COALESCE(author_id,0) FROM comments WHERE id=?", id).Scan(&got); err != nil {
			t.Fatalf("comment %d missing after restore: %v", id, err)
		}
		if got != author {
			t.Errorf("comment %d: author_id %d after restore, want %d", id, got, author)
		}
	}
}
