package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/teambition/rrule-go"

	"kanban/internal/model"
)

// All calendar times are stored in this UTC layout so that lexical string
// comparison equals chronological order (used by range queries).
const calTimeLayout = "2006-01-02 15:04:05"

// parseCalTime accepts the canonical layout plus a few ISO variants and
// normalizes to UTC.
func parseCalTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, l := range []string{
		calTimeLayout, time.RFC3339,
		"2006-01-02T15:04:05Z", "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02",
		// Compact iCalendar forms (e.g. RRULE UNTIL=20150101T000000Z, DTSTART dates).
		"20060102T150405Z", "20060102T150405", "20060102",
	} {
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

// normalizeRRuleUntil rewrites an UNTIL value into the compact iCalendar form that
// rrule-go expects. Our native rule builder emits ISO dates (UNTIL=2026-01-29);
// feeds already use the compact form (left as-is). Date-only UNTIL becomes the end
// of that day so the final day's occurrences are included.
func normalizeRRuleUntil(s string) string {
	parts := strings.Split(s, ";")
	for i, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 || !strings.EqualFold(strings.TrimSpace(kv[0]), "UNTIL") {
			continue
		}
		raw := strings.TrimSpace(kv[1])
		if !strings.Contains(raw, "-") {
			continue // already compact iCal form
		}
		t, ok := parseCalTime(raw)
		if !ok {
			continue
		}
		if len(raw) <= 10 { // date-only → inclusive of the whole day
			t = t.Add(24*time.Hour - time.Second)
		}
		parts[i] = "UNTIL=" + t.UTC().Format("20060102T150405Z")
	}
	return strings.Join(parts, ";")
}

// expandOccurrences returns the start times of a (possibly recurring) event that
// fall within [from,to]. A blank rrule yields the single start time (the caller
// still checks overlap). Backed by a full RFC 5545 engine (teambition/rrule-go):
// FREQ/INTERVAL/COUNT/UNTIL plus BYDAY (incl. 2MO/-1FR), BYMONTHDAY, BYMONTH,
// BYSETPOS, WKST, etc. Bounded to [from,to] via Between so it can't run away on
// infinite or huge-COUNT rules.
func expandOccurrences(start time.Time, rruleStr string, from, to time.Time) []time.Time {
	return expandOccurrencesEx(start, rruleStr, nil, from, to)
}

// expandOccurrencesEx is expandOccurrences with EXDATE exclusions (RFC 5545), used
// for ICS subscriptions where Google removes individual occurrences of a series.
func expandOccurrencesEx(start time.Time, rruleStr string, exdates []time.Time, from, to time.Time) []time.Time {
	rruleStr = strings.TrimSpace(rruleStr)
	if rruleStr == "" {
		return []time.Time{start}
	}
	r, err := rrule.StrToRRule(normalizeRRuleUntil(rruleStr))
	if err != nil {
		// Unparseable rule: fall back to the single start if it overlaps the window.
		if !start.Before(from) && !start.After(to) {
			return []time.Time{start}
		}
		return nil
	}
	if len(exdates) == 0 {
		r.DTStart(start.UTC())
		return r.Between(from.UTC(), to.UTC(), true)
	}
	set := &rrule.Set{}
	set.DTStart(start.UTC())
	set.RRule(r)
	for _, ex := range exdates {
		set.ExDate(ex.UTC())
	}
	return set.Between(from.UTC(), to.UTC(), true)
}

// === Calendars ===

// accessibleProjectIDs returns the ids of projects the user may access: open
// projects, ones they created, and ones they belong to (admins get them all).
func (s *Store) accessibleProjectIDs(userID int64, isAdmin bool) ([]int64, error) {
	var rows *sql.Rows
	var err error
	if isAdmin {
		rows, err = s.db.Query(`SELECT id FROM projects`)
	} else {
		rows, err = s.db.Query(`SELECT p.id FROM projects p
			WHERE p.visibility = 'open' OR p.created_by = ?
			   OR EXISTS (SELECT 1 FROM project_members m WHERE m.project_id = p.id AND m.user_id = ?)`, userID, userID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListCalendars returns every calendar the user can access across all projects:
// their own calendars (incl. personal ones, which are owner-only and visible in
// any project), plus shared calendars of any project they can access. Admins see
// all. The caller (calendar UI) groups them by Calendar.ProjectID.
func (s *Store) ListCalendars(userID int64, isAdmin bool) ([]model.Calendar, error) {
	var where string
	var args []any
	if isAdmin {
		where = "1=1"
	} else {
		where = `c.owner_id = ? OR (c.visibility = 'shared' AND c.project_id IN (
			SELECT p.id FROM projects p
			WHERE p.visibility = 'open' OR p.created_by = ?
			   OR EXISTS (SELECT 1 FROM project_members m WHERE m.project_id = p.id AND m.user_id = ?)))`
		args = []any{userID, userID, userID}
	}
	rows, err := s.db.Query(`SELECT c.id, c.name, c.color, c.owner_id, c.project_id, c.visibility, c.created_at, COALESCE(u.username,'')
		FROM calendars c LEFT JOIN users u ON u.id = c.owner_id
		WHERE `+where+`
		ORDER BY c.name COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Calendar
	for rows.Next() {
		var c model.Calendar
		if err := rows.Scan(&c.ID, &c.Name, &c.Color, &c.OwnerID, &c.ProjectID, &c.Visibility, &c.CreatedAt, &c.OwnerName); err != nil {
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
	err := s.db.QueryRow(`SELECT c.id, c.name, c.color, c.owner_id, c.project_id, c.visibility, c.created_at, COALESCE(u.username,'')
		FROM calendars c LEFT JOIN users u ON u.id = c.owner_id WHERE c.id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Color, &c.OwnerID, &c.ProjectID, &c.Visibility, &c.CreatedAt, &c.OwnerName)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateCalendar makes a calendar in a project (projectID != nil) or a personal
// calendar (projectID == nil, visible only to its owner).
func (s *Store) CreateCalendar(name, color string, ownerID int64, visibility string, projectID *int64) (int64, error) {
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
	return s.db.ExecInsert(`INSERT INTO calendars(name, color, owner_id, project_id, visibility) VALUES(?,?,?,?,?)`,
		name, color, ownerID, projectID, visibility)
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
	e, err := scanEvent(s.db.QueryRow(`SELECT `+eventCols+` FROM calendar_events WHERE id = ?`, id))
	if err != nil {
		return nil, err
	}
	e.Reminders, _ = s.GetEventReminders(id)
	return e, nil
}

// GetEventReminders returns an event's reminder definitions (templates).
func (s *Store) GetEventReminders(eventID int64) ([]model.CalendarReminder, error) {
	rows, err := s.db.Query(`SELECT lead_min, channel FROM calendar_reminders WHERE event_id=? AND occurrence_start='' ORDER BY lead_min`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.CalendarReminder
	for rows.Next() {
		var r model.CalendarReminder
		if err := rows.Scan(&r.LeadMin, &r.Channel); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetEventReminders replaces an event's reminder templates (keeps sent markers).
func (s *Store) SetEventReminders(eventID int64, reminders []model.CalendarReminder) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM calendar_reminders WHERE event_id=? AND occurrence_start=''`, eventID); err != nil {
		return err
	}
	for _, r := range reminders {
		if r.LeadMin < 0 {
			continue
		}
		ch := r.Channel
		if ch != "telegram" && ch != "inapp" {
			ch = "both"
		}
		if _, err := tx.Exec(`INSERT INTO calendar_reminders(event_id, lead_min, channel, occurrence_start, sent) VALUES(?,?,?, '', 0)`, eventID, r.LeadMin, ch); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// MarkReminderSent records that a specific occurrence reminder has fired.
// Returns true only on the first call for that (event,lead,occurrence).
func (s *Store) MarkReminderSent(eventID int64, leadMin int, occ string) bool {
	res, err := s.db.Exec(`INSERT INTO calendar_reminders(event_id, lead_min, channel, occurrence_start, sent) VALUES(?,?,'',?,1) ON CONFLICT DO NOTHING`, eventID, leadMin, occ)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// DueReminderInstance is one reminder occurrence whose lead window includes now.
type DueReminderInstance struct {
	EventID         int64
	Title           string
	LeadMin         int
	Channel         string
	OccurrenceStart string
	OwnerID         *int64
	CreatedBy       *int64
}

// DueReminderInstances expands per-event reminder templates into occurrences whose
// lead window currently includes now (recurring events supported). Dedup happens
// later via MarkReminderSent.
func (s *Store) DueReminderInstances(now time.Time) ([]DueReminderInstance, error) {
	rows, err := s.db.Query(`SELECT e.id, e.title, e.start_utc, e.rrule, c.owner_id, e.created_by, r.lead_min, r.channel
		FROM calendar_reminders r
		JOIN calendar_events e ON e.id = r.event_id
		JOIN calendars c ON c.id = e.calendar_id
		WHERE r.occurrence_start = ''`)
	if err != nil {
		return nil, err
	}
	type tmpl struct {
		id               int64
		title, start     string
		rrule            string
		owner, createdBy *int64
		lead             int
		channel          string
	}
	var tmpls []tmpl
	for rows.Next() {
		var x tmpl
		if err := rows.Scan(&x.id, &x.title, &x.start, &x.rrule, &x.owner, &x.createdBy, &x.lead, &x.channel); err != nil {
			rows.Close()
			return nil, err
		}
		tmpls = append(tmpls, x)
	}
	rows.Close()

	var out []DueReminderInstance
	for _, x := range tmpls {
		st, ok := parseCalTime(x.start)
		if !ok {
			continue
		}
		lead := time.Duration(x.lead) * time.Minute
		// occurrences whose start is within [now, now+lead] have a lead window covering now
		for _, occ := range expandOccurrences(st, x.rrule, now, now.Add(lead)) {
			out = append(out, DueReminderInstance{
				EventID: x.id, Title: x.title, LeadMin: x.lead, Channel: x.channel,
				OccurrenceStart: occ.Format(calTimeLayout), OwnerID: x.owner, CreatedBy: x.createdBy,
			})
		}
	}
	return out, nil
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
	id, err := s.db.ExecInsert(`INSERT INTO calendar_events(calendar_id, title, description, location, start_utc, end_utc, all_day, rrule, created_by)
		VALUES(?,?,?,?,?,?,?,?,?)`, e.CalendarID, e.Title, e.Description, e.Location, start, end, allDay, strings.TrimSpace(e.RRule), createdBy)
	if err != nil {
		return nil, err
	}
	s.SetEventReminders(id, e.Reminders)
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
	s.SetEventReminders(id, e.Reminders)
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
	meta := make(map[int64]model.Calendar, len(cals))
	ids := make([]string, 0, len(cals))
	for _, c := range cals {
		meta[c.ID] = c
		ids = append(ids, strconv.FormatInt(c.ID, 10))
	}

	out := []model.CalendarEvent{}
	var base []model.CalendarEvent
	if len(ids) > 0 {
		// start_utc <= to bounds both recurring and single events; for non-recurring
		// also require end_utc >= from so only overlapping events are returned. Rows
		// are fully drained (releasing the single connection) before later queries.
		q := fmt.Sprintf(`SELECT `+eventCols+` FROM calendar_events
			WHERE calendar_id IN (%s) AND start_utc <= ? AND (rrule != '' OR end_utc >= ?)
			ORDER BY start_utc`, strings.Join(ids, ","))
		rows, err := s.db.Query(q, toN, fromN)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			e, err := scanEvent(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			base = append(base, *e)
		}
		rows.Close()
	}
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

	// Overlay read-only kanban events (task deadlines, sprint ranges) for every
	// project the user can access; each is tagged with its project_id so the UI
	// can toggle the overlay per project.
	pids, err := s.accessibleProjectIDs(userID, isAdmin)
	if err != nil {
		return nil, err
	}
	kb, err := s.kanbanEvents(pids, from, to)
	if err != nil {
		return nil, err
	}
	out = append(out, kb...)

	// Overlay read-only external ICS subscription events.
	subs, err := s.subscriptionEvents(userID, from, to)
	if err != nil {
		return nil, err
	}
	out = append(out, subs...)
	return out, nil
}

// kanbanEvents surfaces task deadlines and sprint ranges as read-only virtual
// events so the calendar reflects board activity, for the given projects. They
// belong to no calendar (calendar_id 0) and carry source/ref_id for linking back
// to the board, plus project_id so the UI can toggle the overlay per project.
func (s *Store) kanbanEvents(projectIDs []int64, from, to time.Time) ([]model.CalendarEvent, error) {
	if len(projectIDs) == 0 {
		return nil, nil
	}
	inList := make([]string, len(projectIDs))
	for i, id := range projectIDs {
		inList[i] = strconv.FormatInt(id, 10)
	}
	in := "(" + strings.Join(inList, ",") + ")"
	var out []model.CalendarEvent

	type taskRow struct {
		id, projectID   int64
		title, deadline string
	}
	var tasks []taskRow
	rows, err := s.db.Query(`SELECT id, project_id, title, deadline FROM tasks WHERE deadline != '' AND project_id IN ` + in)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x taskRow
		if err := rows.Scan(&x.id, &x.projectID, &x.title, &x.deadline); err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, x)
	}
	rows.Close()
	for _, x := range tasks {
		dl, ok := parseCalTime(x.deadline)
		if !ok || dl.Before(from) || dl.After(to) {
			continue
		}
		st := dl.Format(calTimeLayout)
		out = append(out, model.CalendarEvent{
			Title: x.title, Start: st, End: st,
			Color: "#ef4444", Source: "task", RefID: x.id, ProjectID: x.projectID, CalName: "Kanban",
		})
	}

	type sprintRow struct {
		id, projectID int64
		name          string
		sd, ed        string
	}
	var sprints []sprintRow
	rows2, err := s.db.Query(`SELECT id, project_id, name, start_date, end_date FROM sprints WHERE start_date != '' AND end_date != '' AND project_id IN ` + in)
	if err != nil {
		return nil, err
	}
	for rows2.Next() {
		var x sprintRow
		if err := rows2.Scan(&x.id, &x.projectID, &x.name, &x.sd, &x.ed); err != nil {
			rows2.Close()
			return nil, err
		}
		sprints = append(sprints, x)
	}
	rows2.Close()
	for _, x := range sprints {
		sd, ok1 := parseCalTime(x.sd)
		ed, ok2 := parseCalTime(x.ed)
		if !ok1 || !ok2 {
			continue
		}
		ed = ed.Add(24*time.Hour - time.Second) // inclusive of the end day
		if ed.Before(from) || sd.After(to) {
			continue
		}
		out = append(out, model.CalendarEvent{
			Title: x.name, Start: sd.Format(calTimeLayout), End: ed.Format(calTimeLayout),
			AllDay: true, Color: "#8b5cf6", Source: "sprint", RefID: x.id, ProjectID: x.projectID, CalName: "Kanban",
		})
	}
	return out, nil
}

// DueReminder is one event whose start time is approaching and that has not yet
// been reminded about.
type DueReminder struct {
	EventID   int64
	Title     string
	StartUTC  string
	OwnerID   *int64
	CreatedBy *int64
}

// DueReminders returns non-recurring, not-yet-reminded events starting within
// [now,until] (UTC strings in calTimeLayout).
func (s *Store) DueReminders(nowStr, untilStr string) ([]DueReminder, error) {
	rows, err := s.db.Query(`SELECT e.id, e.title, e.start_utc, c.owner_id, e.created_by
		FROM calendar_events e JOIN calendars c ON c.id = e.calendar_id
		WHERE e.rrule = '' AND e.reminded = 0 AND e.start_utc >= ? AND e.start_utc <= ?
		AND NOT EXISTS(SELECT 1 FROM calendar_reminders r WHERE r.event_id = e.id AND r.occurrence_start = '')`, nowStr, untilStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DueReminder
	for rows.Next() {
		var d DueReminder
		if err := rows.Scan(&d.EventID, &d.Title, &d.StartUTC, &d.OwnerID, &d.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) MarkReminded(id int64) error {
	_, err := s.db.Exec(`UPDATE calendar_events SET reminded = 1 WHERE id = ?`, id)
	return err
}

// UpcomingEventCount counts real (non-kanban) events visible to the user that
// start within [from,to], for the rail badge.
func (s *Store) UpcomingEventCount(userID int64, isAdmin bool, fromStr, toStr string) int {
	events, err := s.ListCalendarEvents(userID, isAdmin, fromStr, toStr)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range events {
		if e.Source == "" {
			n++
		}
	}
	return n
}

// CalendarSearchHit is one event match for global search.
type CalendarSearchHit struct {
	EventID int64
	Title   string
	Snippet string
	Start   string
}

// SearchCalendarEvents does a LIKE search over events in calendars the user can see.
func (s *Store) SearchCalendarEvents(userID int64, isAdmin bool, query string, limit int) ([]CalendarSearchHit, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	cals, err := s.ListCalendars(userID, isAdmin)
	if err != nil {
		return nil, err
	}
	if len(cals) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(cals))
	for _, c := range cals {
		ids = append(ids, strconv.FormatInt(c.ID, 10))
	}
	where, wargs := termAndOrWhere([]string{"LOWER(title)", "LOWER(description)"}, query)
	args := append([]any{}, wargs...)
	args = append(args, limit)
	rows, err := s.db.Query(`SELECT id, title, description, start_utc FROM calendar_events
		WHERE calendar_id IN (`+strings.Join(ids, ",")+`) AND `+where+`
		ORDER BY start_utc DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CalendarSearchHit
	for rows.Next() {
		var h CalendarSearchHit
		var desc string
		if err := rows.Scan(&h.EventID, &h.Title, &desc, &h.Start); err != nil {
			return nil, err
		}
		h.Snippet = desc
		out = append(out, h)
	}
	return out, rows.Err()
}

// CanEditCalendar reports whether the user may modify the calendar.
func (s *Store) CanEditCalendar(c *model.Calendar, userID int64, isAdmin bool) bool {
	return isAdmin || (c.OwnerID != nil && *c.OwnerID == userID)
}

// CanViewCalendar reports whether the user may see the calendar.
func (s *Store) CanViewCalendar(c *model.Calendar, userID int64, isAdmin bool) bool {
	return isAdmin || c.Visibility == "shared" || (c.OwnerID != nil && *c.OwnerID == userID)
}
