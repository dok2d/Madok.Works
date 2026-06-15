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
func (s *Store) ReplaceSubscriptionEvents(subID int64, evs []model.CalendarEvent) error {
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
		if _, err := tx.Exec(`INSERT INTO calendar_sub_events(sub_id, uid, title, description, location, start_utc, end_utc, all_day, rrule)
			VALUES(?,?,?,?,?,?,?,?,?)`, subID, "", e.Title, e.Description, e.Location, e.Start, e.End, allDay, e.RRule); err != nil {
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

	// 2) cached events for those subscriptions (ids are integers — safe to inline).
	r2, err := s.db.Query(`SELECT sub_id, title, description, location, start_utc, end_utc, all_day, rrule
		FROM calendar_sub_events WHERE sub_id IN (`+strings.Join(ids, ",")+`) AND start_utc <= ? ORDER BY start_utc`, to.Format(calTimeLayout))
	if err != nil {
		return nil, err
	}
	var raw []model.CalendarEvent
	var subOf []int64
	for r2.Next() {
		var e model.CalendarEvent
		var subID int64
		var allDay int
		if err := r2.Scan(&subID, &e.Title, &e.Description, &e.Location, &e.Start, &e.End, &allDay, &e.RRule); err != nil {
			r2.Close()
			return nil, err
		}
		e.AllDay = allDay == 1
		raw = append(raw, e)
		subOf = append(subOf, subID)
	}
	r2.Close()
	if err := r2.Err(); err != nil {
		return nil, err
	}

	out := []model.CalendarEvent{}
	for i := range raw {
		e := raw[i]
		subID := subOf[i]
		m := metas[subID]
		e.CalendarID = -subID
		e.Source = "ics"
		e.Color = m.color
		e.CalName = m.name
		st, ok := parseCalTime(e.Start)
		en, ok2 := parseCalTime(e.End)
		if !ok || !ok2 {
			continue
		}
		if e.RRule == "" {
			// keep only events overlapping the window
			if !en.Before(from) && !st.After(to) {
				out = append(out, e)
			}
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

func itoaI64(n int64) string { return strconv.FormatInt(n, 10) }
