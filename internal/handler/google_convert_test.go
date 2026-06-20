package handler

import (
	"testing"

	"kanban/internal/model"
)

func TestGoogleEventConversionTimed(t *testing.T) {
	ev := &model.CalendarEvent{
		Title: "Standup", Description: "daily", Location: "room 1",
		Start: "2026-06-20 10:00:00", End: "2026-06-20 10:30:00",
		RRule: "FREQ=WEEKLY;BYDAY=MO",
	}
	ge := mwToGoogleEvent(ev)
	if ge.Start.DateTime != "2026-06-20T10:00:00Z" || ge.End.DateTime != "2026-06-20T10:30:00Z" {
		t.Fatalf("timed start/end: %+v / %+v", ge.Start, ge.End)
	}
	if len(ge.Recurrence) != 1 || ge.Recurrence[0] != "RRULE:FREQ=WEEKLY;BYDAY=MO" {
		t.Fatalf("recurrence: %v", ge.Recurrence)
	}
	back := googleToMWEvent(ge)
	if back.Title != "Standup" || back.Start != "2026-06-20 10:00:00" || back.End != "2026-06-20 10:30:00" {
		t.Fatalf("round-trip timed: %+v", back)
	}
	if back.AllDay {
		t.Fatalf("timed event should not be all-day")
	}
	if back.RRule != "FREQ=WEEKLY;BYDAY=MO" {
		t.Fatalf("round-trip rrule: %q", back.RRule)
	}
}

func TestGoogleEventConversionAllDay(t *testing.T) {
	ev := &model.CalendarEvent{
		Title: "Holiday", AllDay: true,
		Start: "2026-06-20 00:00:00", End: "2026-06-20 00:00:00",
	}
	ge := mwToGoogleEvent(ev)
	// Google all-day uses date fields and an exclusive end.
	if ge.Start.Date != "2026-06-20" || ge.End.Date != "2026-06-21" {
		t.Fatalf("all-day start/end: %+v / %+v", ge.Start, ge.End)
	}
	if ge.Start.DateTime != "" {
		t.Fatalf("all-day should not set dateTime")
	}
	back := googleToMWEvent(ge)
	if !back.AllDay || back.Start != "2026-06-20 00:00:00" || back.End != "2026-06-20 00:00:00" {
		t.Fatalf("round-trip all-day: %+v", back)
	}
}

func TestGoogleToMWEmptyTitle(t *testing.T) {
	ge := &googleEvent{Start: googleEventTime{DateTime: "2026-06-20T09:00:00Z"}, End: googleEventTime{DateTime: "2026-06-20T09:30:00Z"}}
	back := googleToMWEvent(ge)
	if back.Title != "(no title)" {
		t.Fatalf("empty title fallback: %q", back.Title)
	}
}

func TestGoogleToMWTimezoneNormalizes(t *testing.T) {
	// A non-UTC offset must be normalized to MW's UTC storage format.
	ge := &googleEvent{
		Summary: "Call",
		Start:   googleEventTime{DateTime: "2026-06-20T12:00:00+02:00"},
		End:     googleEventTime{DateTime: "2026-06-20T13:00:00+02:00"},
	}
	back := googleToMWEvent(ge)
	if back.Start != "2026-06-20 10:00:00" || back.End != "2026-06-20 11:00:00" {
		t.Fatalf("tz normalization: %+v", back)
	}
}
