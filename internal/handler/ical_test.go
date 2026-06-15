package handler

import "testing"

func TestParseICS(t *testing.T) {
	data := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:abc-1\r\n" +
		"SUMMARY:Standup\\, daily\r\n" +
		"DESCRIPTION:line one\\nline two\r\n" +
		"LOCATION:Room 5\r\n" +
		"DTSTART:20260115T090000Z\r\n" +
		"DTEND:20260115T093000Z\r\n" +
		"RRULE:FREQ=WEEKLY;COUNT=5\r\n" +
		"END:VEVENT\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:abc-2\r\n" +
		"SUMMARY:Holiday\r\n" +
		"DTSTART;VALUE=DATE:20260101\r\n" +
		"END:VEVENT\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:folded\r\n" +
		"SUMMARY:Long tit\r\n le\r\n" +
		"DTSTART:20260201T120000Z\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	evs := parseICS([]byte(data))
	if len(evs) != 3 {
		t.Fatalf("expected 3 events, got %d", len(evs))
	}

	e := evs[0]
	if e.Title != "Standup, daily" {
		t.Errorf("escape failed: %q", e.Title)
	}
	if e.Description != "line one\nline two" {
		t.Errorf("desc escape failed: %q", e.Description)
	}
	if e.Start != "2026-01-15 09:00:00" || e.End != "2026-01-15 09:30:00" {
		t.Errorf("time parse failed: %q .. %q", e.Start, e.End)
	}
	if e.RRule != "FREQ=WEEKLY;COUNT=5" {
		t.Errorf("rrule failed: %q", e.RRule)
	}
	if e.AllDay {
		t.Error("timed event marked all-day")
	}

	if !evs[1].AllDay || evs[1].Start != "2026-01-01 00:00:00" {
		t.Errorf("all-day parse failed: %+v", evs[1])
	}
	// DTEND defaults to DTSTART when missing.
	if evs[1].End != evs[1].Start {
		t.Errorf("missing DTEND should default to DTSTART: %+v", evs[1])
	}

	if evs[2].Title != "Long title" {
		t.Errorf("line unfolding failed: %q", evs[2].Title)
	}
}

func TestParseICSGarbage(t *testing.T) {
	// Must not panic and yields no events from non-ICS input.
	if evs := parseICS([]byte("not a calendar\nrandom junk")); len(evs) != 0 {
		t.Errorf("expected 0 events, got %d", len(evs))
	}
}

func TestFetchICSRejectsInternal(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1/c.ics", "http://localhost/c.ics", "ftp://example.com/c.ics"} {
		if _, err := fetchICS(u); err == nil {
			t.Errorf("expected error for %q", u)
		}
	}
}
