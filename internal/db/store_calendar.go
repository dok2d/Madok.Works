package db

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"kanban/internal/model"
)

// All calendar times are stored in this UTC layout so that lexical string
// comparison equals chronological order (used by range queries).
const calTimeLayout = "2006-01-02 15:04:05"

const calMaxOccurrences = 10000 // safety bound for recurrence expansion

// parseCalTime accepts the canonical layout plus a few ISO variants and
// normalizes to UTC.
func parseCalTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, l := range []string{calTimeLayout, time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// normCalTime reformats an incoming time string to the canonical UTC layout.
func normCalTime(s string) (string, bool) {
	t, ok := parseCalTime(s)
	if !ok {
		return "", false
	}
	return t.Format(calTimeLayout), true
}

func parseRRule(s string) map[string]string {
	m := map[string]string{}
	for _, part := range strings.Split(s, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			m[strings.ToUpper(strings.TrimSpace(kv[0]))] = strings.TrimSpace(kv[1])
		}
	}
	return m
}

func calStep(t time.Time, freq string, interval int) time.Time {
	switch freq {
	case "DAILY":
		return t.AddDate(0, 0, interval)
	case "WEEKLY":
		return t.AddDate(0, 0, 7*interval)
	case "MONTHLY":
		return t.AddDate(0, interval, 0)
	case "YEARLY":
		return t.AddDate(interval, 0, 0)
	}
	return t.AddDate(0, 0, interval)
}

// expandOccurrences returns the start times of a (possibly recurring) event that
// fall within [from,to]. A blank rrule yields the single start time (the caller
// still checks overlap). FREQ, INTERVAL, COUNT and UNTIL are supported.
func expandOccurrences(start time.Time, rrule string, from, to time.Time) []time.Time {
	if rrule == "" {
		return []time.Time{start}
	}
	p := parseRRule(rrule)
	freq := strings.ToUpper(p["FREQ"])
	if freq == "" {
		return []time.Time{start}
	}
	interval := 1
	if v, err := strconv.Atoi(p["INTERVAL"]); err == nil && v > 0 {
		interval = v
	}
	count := 0
	if v, err := strconv.Atoi(p["COUNT"]); err == nil && v > 0 {
		count = v
	}
	until, hasUntil := parseCalTime(p["UNTIL"])
	if hasUntil && len(strings.TrimSpace(p["UNTIL"])) <= 10 {
		// date-only UNTIL is inclusive of the whole day
		until = until.Add(24*time.Hour - time.Second)
	}

	var out []time.Time
	cur := start
	for i := 0; i < calMaxOccurrences; i++ {
		if count > 0 && i >= count {
			break
		}
		if hasUntil && cur.After(until) {
			break
		}
		if cur.After(to) {
			break
		}
		if !cur.Before(from) {
			out = append(out, cur)
		}
		next := calStep(cur, freq, interval)
		if !next.After(cur) { // guard against non-advancing rules
			break
		}
		cur = next
	}
	return out
}

// === Calendars ===

func (s *Store) ListCalendars(userID int64, isAdmin bool) ([]model.Calendar, error) {
	where := "c.visibility = 'shared' OR c.owner_id = ?"
	if isAdmin {
		where = "1=1"
	}
	rows, err := s.db.Query(`SELECT c.id, c.name, c.color, c.owner_id, c.visibility, c.created_at, COALESCE(u.username,'')
		FROM calendars c LEFT JOIN users u ON u.id = c.owner_id
		WHERE `+where+`
		ORDER BY c.name COLLATE NOCASE`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Calendar
	for rows.Next() {
		var c model.Calendar
		if err := rows.Scan(&c.ID, &c.Name, &c.Color, &c.OwnerID, &c.Visibility, &c.CreatedAt, &c.OwnerName); err != nil {
			return nil, err
		}
		c.CanEdit = isAdmin || (c.OwnerID != nil && *c.OwnerID == userID)
		out = append(out, c)
	}
	if out == nil {
		out = []model.Calendar{}
	}
	return out, rows.Err()
}

func (s *Store) GetCalendar(id int64) (*model.Calendar, error) {
	var c model.Calendar
	err := s.db.QueryRow(`SELECT c.id, c.name, c.color, c.owner_id, c.visibility, c.created_at, COALESCE(u.username,'')
		FROM calendars c LEFT JOIN users u ON u.id = c.owner_id WHERE c.id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Color, &c.OwnerID, &c.Visibility, &c.CreatedAt, &c.OwnerName)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) CreateCalendar(name, color string, ownerID int64, visibility string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("name required")
	}
	if visibility != "shared" {
		visibility = "private"
	}
	if color == "" {
		color = "#6366f1"
	}
	res, err := s.db.Exec(`INSERT INTO calendars(name, color, owner_id, visibility) VALUES(?,?,?,?)`,
		name, color, ownerID, visibility)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateCalendar(id int64, name, color, visibility string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name required")
	}
	if visibility != "shared" {
		visibility = "private"
	}
	_, err := s.db.Exec(`UPDATE calendars SET name=?, color=?, visibility=? WHERE id=?`, name, color, visibility, id)
	return err
}

func (s *Store) DeleteCalendar(id int64) error {
	_, err := s.db.Exec(`DELETE FROM calendars WHERE id=?`, id)
	return err
}

// === Events ===

func scanEvent(sc interface{ Scan(...any) error }) (*model.CalendarEvent, error) {
	var e model.CalendarEvent
	var allDay int
	if err := sc.Scan(&e.ID, &e.CalendarID, &e.Title, &e.Description, &e.Location,
		&e.Start, &e.End, &allDay, &e.RRule, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
	}
	e.AllDay = allDay == 1
	return &e, nil
}

const eventCols = `id, calendar_id, title, description, location, start_utc, end_utc, all_day, rrule, created_by, created_at, updated_at`

func (s *Store) GetCalendarEvent(id int64) (*model.CalendarEvent, error) {
	return scanEvent(s.db.QueryRow(`SELECT `+eventCols+` FROM calendar_events WHERE id = ?`, id))
}

func (s *Store) CreateCalendarEvent(e *model.CalendarEvent, createdBy int64) (*model.CalendarEvent, error) {
	start, ok1 := normCalTime(e.Start)
	end, ok2 := normCalTime(e.End)
	if e.Title = strings.TrimSpace(e.Title); e.Title == "" {
		return nil, fmt.Errorf("title required")
	}
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("invalid start/end time")
	}
	if end < start {
		return nil, fmt.Errorf("end before start")
	}
	allDay := 0
	if e.AllDay {
		allDay = 1
	}
	res, err := s.db.Exec(`INSERT INTO calendar_events(calendar_id, title, description, location, start_utc, end_utc, all_day, rrule, created_by)
		VALUES(?,?,?,?,?,?,?,?,?)`, e.CalendarID, e.Title, e.Description, e.Location, start, end, allDay, strings.TrimSpace(e.RRule), createdBy)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetCalendarEvent(id)
}

func (s *Store) UpdateCalendarEvent(id int64, e *model.CalendarEvent) (*model.CalendarEvent, error) {
	start, ok1 := normCalTime(e.Start)
	end, ok2 := normCalTime(e.End)
	if e.Title = strings.TrimSpace(e.Title); e.Title == "" {
		return nil, fmt.Errorf("title required")
	}
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("invalid start/end time")
	}
	if end < start {
		return nil, fmt.Errorf("end before start")
	}
	allDay := 0
	if e.AllDay {
		allDay = 1
	}
	_, err := s.db.Exec(`UPDATE calendar_events SET title=?, description=?, location=?, start_utc=?, end_utc=?, all_day=?, rrule=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		e.Title, e.Description, e.Location, start, end, allDay, strings.TrimSpace(e.RRule), id)
	if err != nil {
		return nil, err
	}
	return s.GetCalendarEvent(id)
}

func (s *Store) DeleteCalendarEvent(id int64) error {
	_, err := s.db.Exec(`DELETE FROM calendar_events WHERE id=?`, id)
	return err
}

// ListCalendarEvents returns events visible to the user that overlap [from,to],
// expanding recurring events into individual occurrences within the range.
func (s *Store) ListCalendarEvents(userID int64, isAdmin bool, fromStr, toStr string) ([]model.CalendarEvent, error) {
	from, ok1 := parseCalTime(fromStr)
	to, ok2 := parseCalTime(toStr)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("invalid from/to")
	}
	fromN, toN := from.Format(calTimeLayout), to.Format(calTimeLayout)

	cals, err := s.ListCalendars(userID, isAdmin)
	if err != nil {
		return nil, err
	}
	if len(cals) == 0 {
		return []model.CalendarEvent{}, nil
	}
	meta := make(map[int64]model.Calendar, len(cals))
	ids := make([]string, 0, len(cals))
	for _, c := range cals {
		meta[c.ID] = c
		ids = append(ids, strconv.FormatInt(c.ID, 10))
	}

	// start_utc <= to bounds both recurring and single events; for non-recurring
	// also require end_utc >= from so only overlapping events are returned.
	q := fmt.Sprintf(`SELECT `+eventCols+` FROM calendar_events
		WHERE calendar_id IN (%s) AND start_utc <= ? AND (rrule != '' OR end_utc >= ?)
		ORDER BY start_utc`, strings.Join(ids, ","))
	rows, err := s.db.Query(q, toN, fromN)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var base []model.CalendarEvent
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		base = append(base, *e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := []model.CalendarEvent{}
	for i := range base {
		e := base[i]
		c := meta[e.CalendarID]
		e.Color, e.CalName = c.Color, c.Name
		if e.RRule == "" {
			out = append(out, e)
			continue
		}
		st, ok := parseCalTime(e.Start)
		en, ok2 := parseCalTime(e.End)
		if !ok || !ok2 {
			continue
		}
		dur := en.Sub(st)
		for _, occ := range expandOccurrences(st, e.RRule, from, to) {
			inst := e
			inst.Start = occ.Format(calTimeLayout)
			inst.End = occ.Add(dur).Format(calTimeLayout)
			inst.Occurrence = inst.Start
			out = append(out, inst)
		}
	}
	return out, nil
}

// CanEditCalendar reports whether the user may modify the calendar.
func (s *Store) CanEditCalendar(c *model.Calendar, userID int64, isAdmin bool) bool {
	return isAdmin || (c.OwnerID != nil && *c.OwnerID == userID)
}

// CanViewCalendar reports whether the user may see the calendar.
func (s *Store) CanViewCalendar(c *model.Calendar, userID int64, isAdmin bool) bool {
	return isAdmin || c.Visibility == "shared" || (c.OwnerID != nil && *c.OwnerID == userID)
}
