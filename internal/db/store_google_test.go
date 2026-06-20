package db

import (
	"testing"

	"kanban/internal/model"
)

func TestGoogleAccountAndLinks(t *testing.T) {
	s := newTestStore(t)
	uid := mustUser(t, s, "alice")

	// No account yet.
	if a, err := s.GetGoogleAccount(uid); err != nil || a != nil {
		t.Fatalf("expected no account, got %v / %v", a, err)
	}

	// Upsert, then a second upsert with a blank refresh keeps the first one.
	if err := s.UpsertGoogleAccount(&GoogleAccount{UserID: uid, Email: "a@x.io", AccessToken: "at1", RefreshToken: "rt1", Expiry: "2030-01-01T00:00:00Z"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.UpsertGoogleAccount(&GoogleAccount{UserID: uid, Email: "a@x.io", AccessToken: "at2", RefreshToken: "", Expiry: "2030-01-02T00:00:00Z"}); err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	a, err := s.GetGoogleAccount(uid)
	if err != nil || a == nil {
		t.Fatalf("get account: %v / %v", a, err)
	}
	if a.AccessToken != "at2" || a.RefreshToken != "rt1" {
		t.Fatalf("expected at2/rt1, got %s/%s", a.AccessToken, a.RefreshToken)
	}

	// Token-only refresh.
	if err := s.UpdateGoogleTokens(uid, "at3", "2030-02-01T00:00:00Z"); err != nil {
		t.Fatalf("update tokens: %v", err)
	}
	a, _ = s.GetGoogleAccount(uid)
	if a.AccessToken != "at3" || a.Expiry != "2030-02-01T00:00:00Z" {
		t.Fatalf("token refresh not applied: %+v", a)
	}

	// Link a calendar.
	calID, err := s.CreateCalendar("Work", "#fff", uid, "private", nil)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	linkID, err := s.CreateGoogleLink(uid, calID, "primary", "Primary")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	if l, _ := s.GetGoogleLinkByCalendar(calID); l == nil || l.GoogleCalID != "primary" {
		t.Fatalf("link by calendar mismatch: %+v", l)
	}

	// A real event is required (mapping FKs calendar_events).
	ev, err := s.CreateCalendarEvent(&model.CalendarEvent{CalendarID: calID, Title: "E", Start: "2026-06-20 10:00:00", End: "2026-06-20 11:00:00"}, uid)
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	// Event mapping upsert + lookups both ways.
	if err := s.UpsertGoogleMapping(&GoogleMapping{LinkID: linkID, EventID: ev.ID, GoogleEventID: "gev1", Etag: "e1", UpdatedLocal: "u1"}); err != nil {
		t.Fatalf("upsert mapping: %v", err)
	}
	if err := s.UpsertGoogleMapping(&GoogleMapping{LinkID: linkID, EventID: ev.ID, GoogleEventID: "gev1", Etag: "e2", UpdatedLocal: "u2"}); err != nil {
		t.Fatalf("upsert mapping2: %v", err)
	}
	m, _ := s.GetGoogleMappingByEvent(linkID, ev.ID)
	if m == nil || m.Etag != "e2" {
		t.Fatalf("mapping by event: %+v", m)
	}
	m2, _ := s.GetGoogleMappingByGID(linkID, "gev1")
	if m2 == nil || m2.EventID != ev.ID {
		t.Fatalf("mapping by gid: %+v", m2)
	}

	// Sync token bookkeeping.
	if err := s.SetGoogleLinkSync(linkID, "tok1", "2026-06-20T00:00:00Z", ""); err != nil {
		t.Fatalf("set sync: %v", err)
	}
	if l, _ := s.GetGoogleLink(linkID); l == nil || l.SyncToken != "tok1" {
		t.Fatalf("sync token not stored: %+v", l)
	}

	// Disconnect removes account + links (and mappings via cascade).
	if err := s.DeleteGoogleAccount(uid); err != nil {
		t.Fatalf("delete account: %v", err)
	}
	if a, _ := s.GetGoogleAccount(uid); a != nil {
		t.Fatalf("account still present after disconnect")
	}
	if ls, _ := s.ListGoogleLinks(uid); len(ls) != 0 {
		t.Fatalf("links remain after disconnect: %d", len(ls))
	}
}
