package db

import (
	"strconv"
	"strings"
	"time"

	"kanban/internal/model"
)

// External read-only calendar subscriptions (ICS/iCal). Events are cached in
// calendar_sub_events and overlaid on the calendar (Source="ics", read-only).

func scanSub(sc interface{ Scan(...any) error }) (model.CalendarSubscription, error) {
	var s model.CalendarSubscription
	err := sc.Scan(&s.ID, &s.UserID, &s.Name, &s.URL, &s.Color, &s.CreatedAt, &s.LastSynced, &s.LastError)
	return s, err
}

const subCols = "id, user_id, name, url, color, created_at, last_synced, last_error"

// ListSubscriptions returns one user's subscriptions.
func (s *Store) ListSubscriptions(userID int64) ([]model.CalendarSubscription, error) {
	rows, err := s.db.Query(`SELECT `+subCols+` FROM calendar_subscriptions WHERE user_id=? ORDER BY name COLLATE NOCASE`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.CalendarSubscription{}
	for rows.Next() {
		sub, err := scanSub(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// ListAllSubscriptions returns every subscription (for the background sync).
func (s *Store) ListAllSubscriptions() ([]model.CalendarSubscription, error) {
	rows, err := s.db.Query(`SELECT ` + subCols + ` FROM calendar_subscriptions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.CalendarSubscription
	for rows.Next() {
		sub, err := scanSub(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// GetSubscription loads one subscription (with user_id/url, for sync/ownership).
func (s *Store) GetSubscription(id int64) (*model.CalendarSubscription, error) {
	sub, err := scanSub(s.db.QueryRow(`SELECT `+subCols+` FROM calendar_subscriptions WHERE id=?`, id))
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Store) CreateSubscription(userID int64, name, url, color string) (int64, error) {
	if color == "" {
		color = "#0ea5e9"
	}
	res, err := s.db.Exec(`INSERT INTO calendar_subscriptions(user_id, name, url, color) VALUES(?,?,?,?)`,
		userID, name, url, color)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *Store) DeleteSubscription(id, userID int64) error {
	_, err := s.db.Exec(`DELETE FROM calendar_subscriptions WHERE id=? AND user_id=?`, id, userID)
	return err
}

func (s *Store) SetSubscriptionSynced(id int64, ts, errMsg string) error {
	_, err := s.db.Exec(`UPDATE calendar_subscriptions SET last_synced=?, last_error=? WHERE id=?`, ts, errMsg, id)
	return err
}

// ReplaceSubscriptionEvents swaps the cached events of a subscription in one tx.
func (s *Store) ReplaceSubscriptionEvents(subID int64, evs []model.SubEvent) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM calendar_sub_events WHERE sub_id=?`, subID); err != nil {
		return err
	}
	for _, e := range evs {
		allDay := 0
		if e.AllDay {
			allDay = 1
		}
		if _, err := tx.Exec(`INSERT INTO calendar_sub_events
			(sub_id, uid, title, description, location, start_utc, end_utc, all_day, rrule, exdate, recurrence_id, status)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			subID, e.UID, e.Title, e.Description, e.Location, e.Start, e.End, allDay, e.RRule,
			strings.Join(e.ExDates, ","), e.RecurrenceID, e.Status); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// subscriptionEvents overlays a user's subscribed (read-only) events within
// [from,to], expanding recurrences. CalendarID is -subID so the existing
// visibility toggle works; Source="ics" marks them read-only.
func (s *Store) subscriptionEvents(userID int64, from, to time.Time) ([]model.CalendarEvent, error) {
	// 1) which subscriptions (drain before follow-up query — single conn).
	type subMeta struct{ name, color string }
	metas := map[int64]subMeta{}
	var ids []string
	rows, err := s.db.Query(`SELECT id, name, color FROM calendar_subscriptions WHERE user_id=?`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var name, color string
		if err := rows.Scan(&id, &name, &color); err != nil {
			rows.Close()
			return nil, err
		}
		metas[id] = subMeta{name, color}
		ids = append(ids, itoaI64(id))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	// 2) cached events. Load masters/singles overlapping the window, plus ALL
	//    override rows (recurrence_id != '') so moved/cancelled instances are applied.
	type rawEv struct {
		ev           model.CalendarEvent
		subID        int64
		uid          string
		exdate       string
		recurrenceID string
		status       string
	}
	r2, err := s.db.Query(`SELECT id, sub_id, uid, title, description, location, start_utc, end_utc, all_day, rrule, exdate, recurrence_id, status
		FROM calendar_sub_events WHERE sub_id IN (`+strings.Join(ids, ",")+`)
		AND (start_utc <= ? OR recurrence_id != '') ORDER BY start_utc`, to.Format(calTimeLayout))
	if err != nil {
		return nil, err
	}
	var rows2 []rawEv
	for r2.Next() {
		var x rawEv
		var allDay int
		if err := r2.Scan(&x.ev.ID, &x.subID, &x.uid, &x.ev.Title, &x.ev.Description, &x.ev.Location,
			&x.ev.Start, &x.ev.End, &allDay, &x.ev.RRule, &x.exdate, &x.recurrenceID, &x.status); err != nil {
			r2.Close()
			return nil, err
		}
		x.ev.AllDay = allDay == 1
		rows2 = append(rows2, x)
	}
	r2.Close()
	if err := r2.Err(); err != nil {
		return nil, err
	}

	// Index overrides by sub|uid|recurrence-time.
	ovrKey := func(subID int64, uid, rid string) string { return itoaI64(subID) + "|" + uid + "|" + rid }
	cancelled := map[string]bool{}    // instances removed from a series
	replaced := map[string]bool{}     // instances moved (emitted separately)
	for _, x := range rows2 {
		if x.recurrenceID == "" {
			continue
		}
		key := ovrKey(x.subID, x.uid, x.recurrenceID)
		if x.status == "CANCELLED" {
			cancelled[key] = true
		} else {
			replaced[key] = true
		}
	}

	decorate := func(e *model.CalendarEvent, subID int64) {
		m := metas[subID]
		e.CalendarID = -subID
		e.Source = "ics"
		e.Color = m.color
		e.CalName = m.name
	}

	out := []model.CalendarEvent{}
	for _, x := range rows2 {
		st, ok := parseCalTime(x.ev.Start)
		en, ok2 := parseCalTime(x.ev.End)

		// Override rows are emitted on their own (new) time when not cancelled.
		if x.recurrenceID != "" {
			if x.status == "CANCELLED" || !ok || !ok2 {
				continue
			}
			if !en.Before(from) && !st.After(to) {
				e := x.ev
				decorate(&e, x.subID)
				out = append(out, e)
			}
			continue
		}

		if x.status == "CANCELLED" || !ok || !ok2 {
			continue // whole event cancelled or unparseable
		}
		if x.ev.RRule == "" {
			if !en.Before(from) && !st.After(to) {
				e := x.ev
				decorate(&e, x.subID)
				out = append(out, e)
			}
			continue
		}
		// Recurring master: expand with EXDATE, then drop cancelled/replaced instances.
		var exdates []time.Time
		for _, ex := range strings.Split(x.exdate, ",") {
			if ex == "" {
				continue
			}
			if t, ok := parseCalTime(ex); ok {
				exdates = append(exdates, t)
			}
		}
		dur := en.Sub(st)
		for _, occ := range expandOccurrencesEx(st, x.ev.RRule, exdates, from, to) {
			key := ovrKey(x.subID, x.uid, occ.UTC().Format(calTimeLayout))
			if cancelled[key] || replaced[key] {
				continue // moved/cancelled instance handled via the override row
			}
			inst := x.ev
			decorate(&inst, x.subID)
			inst.Start = occ.Format(calTimeLayout)
			inst.End = occ.Add(dur).Format(calTimeLayout)
			inst.Occurrence = inst.Start
			out = append(out, inst)
		}
	}
	return out, nil
}

func itoaI64(n int64) string { return strconv.FormatInt(n, 10) }
