package db

import (
	"testing"

	"kanban/internal/model"
)

// TestImportEcosystemSkipsUnknownColumns ensures a backup whose ecosystem rows
// carry a column that no longer exists in the current schema (e.g. a renamed
// column from an older build) still imports — the stray column is skipped
// rather than aborting the whole restore.
func TestImportEcosystemSkipsUnknownColumns(t *testing.T) {
	s := newTestStore(t)
	uid := mustUser(t, s, "alice")

	data := &model.ExportData{
		Ecosystem: map[string][]map[string]any{
			"chat_channels": {
				{"id": float64(5000), "name": "general", "topic": "", "type": "channel", "created_by": float64(uid)},
			},
			"chat_messages": {
				{
					"id": float64(9001), "channel_id": float64(5000), "author_id": float64(uid),
					"text": "hi", "deleted": float64(0), "pinned": float64(0),
					"user_id": float64(uid), // column not present in the current schema
				},
			},
		},
	}
	if err := s.ImportAll(data); err != nil {
		t.Fatalf("ImportAll should tolerate an unknown column, got: %v", err)
	}

	msgs, err := s.ListChatMessages(5000, 0, 0, 0, uid)
	if err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Text != "hi" {
		t.Fatalf("message not imported: %+v", msgs)
	}
}
