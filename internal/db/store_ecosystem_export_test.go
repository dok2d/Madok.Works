package db

import (
	"encoding/json"
	"testing"

	"kanban/internal/model"
)

// TestEcosystemExportRoundTrip ensures wiki/chat/calendar data survives a full
// JSON export -> import cycle (previously these services were silently dropped).
func TestEcosystemExportRoundTrip(t *testing.T) {
	a := newTestStore(t)
	u := mustUser(t, a, "alice")
	sp, err := a.CreateWikiSpace("DOC", "Docs", "", "open", u)
	if err != nil {
		t.Fatalf("CreateWikiSpace: %v", err)
	}
	if _, err := a.CreateWikiPage(sp, nil, "Page", "<p>hello</p>", u); err != nil {
		t.Fatalf("CreateWikiPage: %v", err)
	}
	ch, _ := a.CreateChatChannel("general", "", "channel", u, nil)
	a.CreateChatMessage(ch, u, "hi there", nil)
	cal, _ := a.CreateCalendar("Work", "#abc", u, "private")
	a.CreateCalendarEvent(&model.CalendarEvent{CalendarID: cal, Title: "Launch", Start: "2026-01-01 10:00:00", End: "2026-01-01 11:00:00"}, u)

	exp, err := a.ExportAll()
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	// Round-trip through JSON so numbers become float64 like a real backup.
	raw, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var data model.ExportData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	b := newTestStore(t)
	if err := b.ImportAll(&data); err != nil {
		t.Fatalf("ImportAll: %v", err)
	}

	for _, tbl := range ecosystemTables {
		ra, _ := a.dumpTable(tbl)
		rb, _ := b.dumpTable(tbl)
		if len(ra) != len(rb) {
			t.Errorf("%s: row count A=%d B=%d", tbl, len(ra), len(rb))
		}
	}

	msgs, _ := b.ListChatMessages(ch, 0, 0, 0)
	if len(msgs) != 1 || msgs[0].Text != "hi there" {
		t.Errorf("chat message not restored: %+v", msgs)
	}
	evs, _ := b.ListCalendarEvents(u, true, "2026-01-01 00:00:00", "2026-01-02 00:00:00")
	found := false
	for _, e := range evs {
		if e.Title == "Launch" {
			found = true
		}
	}
	if !found {
		t.Errorf("calendar event not restored, got %d events", len(evs))
	}
	pages, _ := b.dumpTable("wiki_pages")
	if len(pages) != 1 {
		t.Errorf("expected 1 wiki page restored, got %d", len(pages))
	}
}
