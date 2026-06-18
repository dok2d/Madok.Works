package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"kanban/internal/db"
)

func TestGitWebhookLinksTaskByRef(t *testing.T) {
	store, err := db.New(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	defer store.Close()
	uid, _ := store.CreateUser("alice", "password123", "admin")

	pid, err := store.CreateProject("Backend", "", "open", uid)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	key := store.ProjectKey(pid)
	cols, _ := store.ListColumns(pid)
	if len(cols) == 0 {
		t.Fatal("project has no seeded columns")
	}
	taskID, err := store.CreateTask(pid, "Fix bug", "", "", "", cols[0].ID, nil, nil, nil, 0, nil, "")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	// First task in the project → number 1, ref KEY-1.
	repoID, err := store.CreateGitRepo(pid, "github", "be", "owner/be", "", nil)
	if err != nil {
		t.Fatalf("CreateGitRepo: %v", err)
	}

	h := &Handler{store: store}
	payload := map[string]any{
		"ref": "refs/heads/feature/" + key + "-1-fix",
		"commits": []any{
			map[string]any{"message": "Fix " + key + "-1 the bug", "url": "http://git/commit/abc"},
		},
		"pusher": map[string]any{"name": "alice"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/integrations/git/webhook/1", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()
	h.handleGitWebhook(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("webhook status = %d, body=%s", rec.Code, rec.Body.String())
	}

	events, err := store.ListGitEventsForTask(taskID, 10)
	if err != nil {
		t.Fatalf("ListGitEventsForTask: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 linked git event, got %d", len(events))
	}
	if events[0].RepoID != repoID || events[0].Kind != "push" {
		t.Errorf("unexpected event: %+v", events[0])
	}
}
