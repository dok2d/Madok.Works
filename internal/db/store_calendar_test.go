package db

import (
	"testing"
	"time"

	"kanban/internal/model"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, ok := parseCalTime(s)
	if !ok {
		t.Fatalf("parseCalTime(%q) failed", s)
	}
	return tm
}

func TestExpandOccurrences(t *testing.T) {
	yearStart := mustTime(t, "2026-01-01 00:00:00")
	yearEnd := mustTime(t, "2026-12-31 23:59:59")

	cases := []struct {
		name      string
		start     string
		rrule     string
		from, to  time.Time
		wantCount int
	}{
		{"none", "2026-03-01 10:00:00", "", yearStart, yearEnd, 1},
		{"daily count 5", "2026-01-01 10:00:00", "FREQ=DAILY;COUNT=5", yearStart, yearEnd, 5},
		{"weekly until", "2026-01-01 09:00:00", "FREQ=WEEKLY;UNTIL=2026-01-29", yearStart, yearEnd, 5},
		{"every 2 days count 4", "2026-01-01 08:00:00", "FREQ=DAILY;INTERVAL=2;COUNT=4", yearStart, yearEnd, 4},
		{"monthly count 12", "2026-01-15 12:00:00", "FREQ=MONTHLY;COUNT=12", yearStart, yearEnd, 12},
		{"daily clipped to window", "2026-01-01 10:00:00", "FREQ=DAILY;COUNT=100",
			mustTime(t, "2026-01-10 00:00:00"), mustTime(t, "2026-01-20 23:59:59"), 11},
	}
	for _, c := range cases {
		got := expandOccurrences(mustTime(t, c.start), c.rrule, c.from, c.to)
		if len(got) != c.wantCount {
			t.Errorf("%s: got %d occurrences, want %d", c.name, len(got), c.wantCount)
		}
	}
}

func TestCalendarEventCRUD(t *testing.T) {
	s := newTestStore(t)
	u := mustUser(t, s, "alice")
	cal, err := s.CreateCalendar("Work", "#ff0000", u, "private")
	if err != nil {
		t.Fatalf("CreateCalendar: %v", err)
	}

	ev, err := s.CreateCalendarEvent(&model.CalendarEvent{
		CalendarID: cal, Title: "Standup", Start: "2026-02-01T09:00:00Z", End: "2026-02-01T09:15:00Z",
	}, u)
	if err != nil {
		t.Fatalf("CreateCalendarEvent: %v", err)
	}
	if ev.Start != "2026-02-01 09:00:00" {
		t.Errorf("time not normalized: %q", ev.Start)
	}

	// invalid: end before start
	if _, err := s.CreateCalendarEvent(&model.CalendarEvent{CalendarID: cal, Title: "x", Start: "2026-02-01 10:00:00", End: "2026-02-01 09:00:00"}, u); err == nil {
		t.Error("expected error for end before start")
	}

	ev.Title = "Daily standup"
	upd, err := s.UpdateCalendarEvent(ev.ID, ev)
	if err != nil || upd.Title != "Daily standup" {
		t.Fatalf("UpdateCalendarEvent: %v (%+v)", err, upd)
	}

	if err := s.DeleteCalendarEvent(ev.ID); err != nil {
		t.Fatalf("DeleteCalendarEvent: %v", err)
	}
}

func TestListCalendarEventsRange(t *testing.T) {
	s := newTestStore(t)
	u := mustUser(t, s, "alice")
	cal, _ := s.CreateCalendar("Work", "#abc", u, "private")

	// one-off inside January, one-off in March (out of range)
	s.CreateCalendarEvent(&model.CalendarEvent{CalendarID: cal, Title: "Jan", Start: "2026-01-05 10:00:00", End: "2026-01-05 11:00:00"}, u)
	s.CreateCalendarEvent(&model.CalendarEvent{CalendarID: cal, Title: "Mar", Start: "2026-03-05 10:00:00", End: "2026-03-05 11:00:00"}, u)
	// weekly recurring from Jan 1, 10 times -> 5 land in January
	s.CreateCalendarEvent(&model.CalendarEvent{CalendarID: cal, Title: "Weekly", Start: "2026-01-01 09:00:00", End: "2026-01-01 09:30:00", RRule: "FREQ=WEEKLY;COUNT=10"}, u)

	events, err := s.ListCalendarEvents(u, false, "2026-01-01 00:00:00", "2026-01-31 23:59:59")
	if err != nil {
		t.Fatalf("ListCalendarEvents: %v", err)
	}
	// 1 one-off (Jan) + 5 weekly occurrences = 6; March excluded
	if len(events) != 6 {
		t.Fatalf("expected 6 events in January, got %d", len(events))
	}
	weekly := 0
	for _, e := range events {
		if e.Title == "Mar" {
			t.Error("out-of-range event leaked into results")
		}
		if e.Occurrence != "" {
			weekly++
			// duration preserved (30 min)
			st, _ := parseCalTime(e.Start)
			en, _ := parseCalTime(e.End)
			if en.Sub(st) != 30*time.Minute {
				t.Errorf("occurrence duration wrong: %v", en.Sub(st))
			}
		}
		if e.Color != "#abc" {
			t.Errorf("calendar color not attached: %q", e.Color)
		}
	}
	if weekly != 5 {
		t.Errorf("expected 5 weekly occurrences, got %d", weekly)
	}
}

func TestDueReminders(t *testing.T) {
	s := newTestStore(t)
	u := mustUser(t, s, "alice")
	cal, _ := s.CreateCalendar("Work", "#abc", u, "private")
	soon, _ := s.CreateCalendarEvent(&model.CalendarEvent{CalendarID: cal, Title: "Soon", Start: "2026-05-01 12:10:00", End: "2026-05-01 12:30:00"}, u)
	// far future and recurring events must not be reminded
	s.CreateCalendarEvent(&model.CalendarEvent{CalendarID: cal, Title: "Later", Start: "2026-05-01 18:00:00", End: "2026-05-01 19:00:00"}, u)
	s.CreateCalendarEvent(&model.CalendarEvent{CalendarID: cal, Title: "Recurring", Start: "2026-05-01 12:11:00", End: "2026-05-01 12:30:00", RRule: "FREQ=DAILY;COUNT=5"}, u)

	due, err := s.DueReminders("2026-05-01 12:00:00", "2026-05-01 12:15:00")
	if err != nil {
		t.Fatalf("DueReminders: %v", err)
	}
	if len(due) != 1 || due[0].EventID != soon.ID {
		t.Fatalf("expected only the soon event due, got %+v", due)
	}
	if due[0].OwnerID == nil || *due[0].OwnerID != u {
		t.Errorf("owner not resolved for reminder")
	}

	if err := s.MarkReminded(soon.ID); err != nil {
		t.Fatalf("MarkReminded: %v", err)
	}
	due2, _ := s.DueReminders("2026-05-01 12:00:00", "2026-05-01 12:15:00")
	if len(due2) != 0 {
		t.Errorf("reminded event should not recur, got %d", len(due2))
	}
}

func TestConfigurableReminders(t *testing.T) {
	s := newTestStore(t)
	u := mustUser(t, s, "alice")
	cal, _ := s.CreateCalendar("Work", "#abc", u, "private")
	now := time.Now().UTC().Truncate(time.Minute)
	start := now.Add(10 * time.Minute).Format(calTimeLayout)
	ev, err := s.CreateCalendarEvent(&model.CalendarEvent{
		CalendarID: cal, Title: "Standup", Start: start, End: now.Add(40 * time.Minute).Format(calTimeLayout),
		Reminders: []model.CalendarReminder{{LeadMin: 30, Channel: "both"}},
	}, u)
	if err != nil {
		t.Fatalf("CreateCalendarEvent: %v", err)
	}
	if len(ev.Reminders) != 1 || ev.Reminders[0].LeadMin != 30 {
		t.Fatalf("reminders not persisted: %+v", ev.Reminders)
	}
	// start is 10 min away; a 30-min lead window covers now -> due
	inst, _ := s.DueReminderInstances(now)
	var found *DueReminderInstance
	for i := range inst {
		if inst[i].EventID == ev.ID {
			found = &inst[i]
		}
	}
	if found == nil {
		t.Fatalf("expected a due reminder instance for the event")
	}
	if !s.MarkReminderSent(found.EventID, found.LeadMin, found.OccurrenceStart) {
		t.Error("first MarkReminderSent should return true")
	}
	if s.MarkReminderSent(found.EventID, found.LeadMin, found.OccurrenceStart) {
		t.Error("second MarkReminderSent should return false (dedup)")
	}
	// an event with explicit reminders must not also fire via the default path
	due, _ := s.DueReminders(now.Format(calTimeLayout), now.Add(15*time.Minute).Format(calTimeLayout))
	for _, d := range due {
		if d.EventID == ev.ID {
			t.Error("templated event leaked into default reminder path")
		}
	}
}

func TestCalendarVisibility(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")

	priv, _ := s.CreateCalendar("Alice private", "#111", alice, "private")
	shared, _ := s.CreateCalendar("Team", "#222", alice, "shared")
	s.CreateCalendarEvent(&model.CalendarEvent{CalendarID: priv, Title: "secret", Start: "2026-04-01 10:00:00", End: "2026-04-01 11:00:00"}, alice)
	s.CreateCalendarEvent(&model.CalendarEvent{CalendarID: shared, Title: "meeting", Start: "2026-04-02 10:00:00", End: "2026-04-02 11:00:00"}, alice)

	bobEvents, err := s.ListCalendarEvents(bob, false, "2026-04-01 00:00:00", "2026-04-30 23:59:59")
	if err != nil {
		t.Fatalf("ListCalendarEvents: %v", err)
	}
	if len(bobEvents) != 1 || bobEvents[0].Title != "meeting" {
		t.Errorf("bob should see only the shared event, got %d: %+v", len(bobEvents), bobEvents)
	}

	// admin sees everything
	adminEvents, _ := s.ListCalendarEvents(bob, true, "2026-04-01 00:00:00", "2026-04-30 23:59:59")
	if len(adminEvents) != 2 {
		t.Errorf("admin should see both events, got %d", len(adminEvents))
	}
}

func TestSubscriptionEvents(t *testing.T) {
	s := newTestStore(t)
	u := mustUser(t, s, "alice")
	id, err := s.CreateSubscription(u, "Google", "https://example.com/c.ics", "#0ea5e9")
	if err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}
	evs := []model.CalendarEvent{
		{Title: "Meeting", Start: "2026-01-15 09:00:00", End: "2026-01-15 10:00:00"},
		{Title: "Weekly", Start: "2026-01-01 09:00:00", End: "2026-01-01 09:30:00", RRule: "FREQ=WEEKLY;COUNT=10"},
	}
	if err := s.ReplaceSubscriptionEvents(id, evs); err != nil {
		t.Fatalf("ReplaceSubscriptionEvents: %v", err)
	}
	out, err := s.ListCalendarEvents(u, false, "2026-01-01 00:00:00", "2026-01-31 23:59:59")
	if err != nil {
		t.Fatalf("ListCalendarEvents: %v", err)
	}
	ics, hasMeeting := 0, false
	for _, e := range out {
		if e.Source != "ics" {
			continue
		}
		ics++
		if e.CalendarID != -id {
			t.Errorf("ics event calendar_id = %d, want %d", e.CalendarID, -id)
		}
		if e.Color != "#0ea5e9" || e.CalName != "Google" {
			t.Errorf("ics event meta wrong: %+v", e)
		}
		if e.Title == "Meeting" {
			hasMeeting = true
		}
	}
	if !hasMeeting {
		t.Error("non-recurring ics event missing")
	}
	if ics < 3 {
		t.Errorf("expected expanded weekly + meeting (>=3 in Jan), got %d", ics)
	}
	// Other users don't see this private subscription.
	bob := mustUser(t, s, "bob")
	bout, _ := s.ListCalendarEvents(bob, false, "2026-01-01 00:00:00", "2026-01-31 23:59:59")
	for _, e := range bout {
		if e.Source == "ics" {
			t.Error("subscription leaked to another user")
		}
	}
	// Deleting the subscription removes its events from the overlay.
	if err := s.DeleteSubscription(id, u); err != nil {
		t.Fatalf("DeleteSubscription: %v", err)
	}
	out2, _ := s.ListCalendarEvents(u, false, "2026-01-01 00:00:00", "2026-01-31 23:59:59")
	for _, e := range out2 {
		if e.Source == "ics" {
			t.Error("ics events remain after delete")
		}
	}
}

func TestExpandOccurrencesIcalUntil(t *testing.T) {
	start := mustTime(t, "2015-01-01 09:00:00")
	from := mustTime(t, "2026-06-01 00:00:00")
	to := mustTime(t, "2026-06-30 23:59:59")

	// Weekly series that ended in 2015 (compact iCalendar UNTIL) must NOT recur now.
	occ := expandOccurrences(start, "FREQ=WEEKLY;UNTIL=20150201T000000Z", from, to)
	if len(occ) != 0 {
		t.Errorf("expired UNTIL series should not occur in 2026, got %d", len(occ))
	}
	// Date-only compact UNTIL too.
	occD := expandOccurrences(start, "FREQ=WEEKLY;UNTIL=20150201", from, to)
	if len(occD) != 0 {
		t.Errorf("expired date-only UNTIL should not occur, got %d", len(occD))
	}
	// Sanity: a genuinely infinite weekly series still occurs now.
	if len(expandOccurrences(start, "FREQ=WEEKLY", from, to)) == 0 {
		t.Error("infinite weekly series should still occur in 2026")
	}
}
