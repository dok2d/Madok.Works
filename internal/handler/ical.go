package handler

import (
	"strings"
	"time"
)

// Minimal iCalendar (RFC 5545) parser — enough to import VEVENTs from a Google/
// Outlook "secret iCal address". Supports line unfolding, DTSTART/DTEND with Z
// (UTC), VALUE=DATE (all-day) and TZID=..., SUMMARY/DESCRIPTION/LOCATION/UID/RRULE,
// and TEXT escaping. Unknown lines/components are ignored; it never panics.

const icsTimeLayout = "2006-01-02 15:04:05" // matches calTimeLayout (UTC)

// icsEvent is one parsed VEVENT, with times normalized to the UTC store layout.
type icsEvent struct {
	UID          string
	Title        string
	Description  string
	Location     string
	Start        string // UTC "2006-01-02 15:04:05"
	End          string
	AllDay       bool
	RRule        string
	ExDates      []string // excluded occurrence times (UTC layout)
	RecurrenceID string   // overridden occurrence time (UTC layout); "" = master
	Status       string   // "" or "CANCELLED"
}

// icsUnfold joins folded lines (a CRLF/LF followed by a space or tab is a
// continuation of the previous line, per RFC 5545 §3.1).
func icsUnfold(data string) []string {
	raw := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
	var out []string
	for _, ln := range raw {
		if ln == "" {
			continue
		}
		if (ln[0] == ' ' || ln[0] == '\t') && len(out) > 0 {
			out[len(out)-1] += ln[1:]
		} else {
			out = append(out, ln)
		}
	}
	return out
}

// icsUnescapeText reverses RFC 5545 TEXT escaping.
func icsUnescapeText(s string) string {
	r := strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`)
	return r.Replace(s)
}

// icsSplitLine splits "NAME;PARAM=..:VALUE" into (name, params, value). The colon
// separating value can't appear inside params here (params with quoted colons are
// rare for the properties we read), so a first-colon split is sufficient.
func icsSplitLine(line string) (name string, params map[string]string, value string) {
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return "", nil, ""
	}
	left, value := line[:colon], line[colon+1:]
	parts := strings.Split(left, ";")
	name = strings.ToUpper(parts[0])
	params = map[string]string{}
	for _, p := range parts[1:] {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			params[strings.ToUpper(kv[0])] = kv[1]
		}
	}
	return name, params, value
}

// icsParseTime parses a DTSTART/DTEND value with its params and returns the UTC
// store-layout string and whether it's an all-day (date-only) value.
func icsParseTime(value string, params map[string]string) (string, bool) {
	value = strings.TrimSpace(value)
	// All-day: VALUE=DATE or an 8-digit date.
	if strings.EqualFold(params["VALUE"], "DATE") || (len(value) == 8 && !strings.ContainsAny(value, "T")) {
		if t, err := time.Parse("20060102", value); err == nil {
			return t.UTC().Format(icsTimeLayout), true
		}
	}
	// UTC (...Z)
	if t, err := time.Parse("20060102T150405Z", value); err == nil {
		return t.UTC().Format(icsTimeLayout), false
	}
	// Local with TZID
	if tzid := params["TZID"]; tzid != "" {
		if loc, err := time.LoadLocation(tzid); err == nil {
			if t, err := time.ParseInLocation("20060102T150405", value, loc); err == nil {
				return t.UTC().Format(icsTimeLayout), false
			}
		}
	}
	// Floating/naive local time — treat as UTC (best effort).
	if t, err := time.Parse("20060102T150405", value); err == nil {
		return t.UTC().Format(icsTimeLayout), false
	}
	return "", false
}

// parseICS extracts VEVENTs from an iCalendar document.
func parseICS(data []byte) []icsEvent {
	var out []icsEvent
	var cur *icsEvent
	for _, line := range icsUnfold(string(data)) {
		up := strings.ToUpper(strings.TrimSpace(line))
		if up == "BEGIN:VEVENT" {
			cur = &icsEvent{}
			continue
		}
		if up == "END:VEVENT" {
			if cur != nil {
				if cur.End == "" {
					cur.End = cur.Start
				}
				// Keep events with a start, or override/cancellation entries that key
				// off a RECURRENCE-ID even without their own DTSTART.
				if cur.Start != "" || cur.RecurrenceID != "" {
					out = append(out, *cur)
				}
			}
			cur = nil
			continue
		}
		if cur == nil {
			continue
		}
		name, params, value := icsSplitLine(line)
		switch name {
		case "SUMMARY":
			cur.Title = icsUnescapeText(value)
		case "DESCRIPTION":
			cur.Description = icsUnescapeText(value)
		case "LOCATION":
			cur.Location = icsUnescapeText(value)
		case "UID":
			cur.UID = value
		case "RRULE":
			cur.RRule = strings.TrimSpace(value)
		case "STATUS":
			cur.Status = strings.ToUpper(strings.TrimSpace(value))
		case "RECURRENCE-ID":
			if ts, _ := icsParseTime(value, params); ts != "" {
				cur.RecurrenceID = ts
			}
		case "EXDATE":
			for _, v := range strings.Split(value, ",") {
				if ts, _ := icsParseTime(v, params); ts != "" {
					cur.ExDates = append(cur.ExDates, ts)
				}
			}
		case "DTSTART":
			if ts, allDay := icsParseTime(value, params); ts != "" {
				cur.Start, cur.AllDay = ts, allDay
			}
		case "DTEND":
			if ts, _ := icsParseTime(value, params); ts != "" {
				cur.End = ts
			}
		}
	}
	return out
}
