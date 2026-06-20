package db

import (
	"database/sql"
	"errors"

	"kanban/internal/model"
)

// GoogleAccount holds a user's stored Google OAuth tokens.
type GoogleAccount struct {
	UserID       int64  `json:"user_id"`
	Email        string `json:"email"`
	AccessToken  string `json:"-"`
	RefreshToken string `json:"-"`
	Expiry       string `json:"expiry"` // RFC3339
	Scope        string `json:"scope"`
}

// GoogleCalLink is a 1:1 link between an MW calendar and a Google calendar.
type GoogleCalLink struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"user_id"`
	CalendarID    int64  `json:"calendar_id"`
	GoogleCalID   string `json:"google_cal_id"`
	GoogleCalName string `json:"google_cal_name"`
	SyncToken     string `json:"-"`
	LastSync      string `json:"last_sync"`
	LastError     string `json:"last_error"`
}

// GetGoogleAccount returns the stored Google account for a user, or (nil,nil) if
// the user has not connected one.
func (s *Store) GetGoogleAccount(userID int64) (*GoogleAccount, error) {
	var a GoogleAccount
	err := s.db.QueryRow(`SELECT user_id, email, access_token, refresh_token, expiry, scope
		FROM google_accounts WHERE user_id=?`, userID).
		Scan(&a.UserID, &a.Email, &a.AccessToken, &a.RefreshToken, &a.Expiry, &a.Scope)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// UpsertGoogleAccount stores or replaces a user's Google tokens. A blank
// refreshToken keeps the previously stored one (Google omits it on re-consent).
func (s *Store) UpsertGoogleAccount(a *GoogleAccount) error {
	if a.RefreshToken == "" {
		if prev, _ := s.GetGoogleAccount(a.UserID); prev != nil {
			a.RefreshToken = prev.RefreshToken
		}
	}
	_, err := s.db.Exec(`INSERT INTO google_accounts(user_id, email, access_token, refresh_token, expiry, scope)
		VALUES(?,?,?,?,?,?)
		ON CONFLICT(user_id) DO UPDATE SET email=excluded.email, access_token=excluded.access_token,
			refresh_token=excluded.refresh_token, expiry=excluded.expiry, scope=excluded.scope`,
		a.UserID, a.Email, a.AccessToken, a.RefreshToken, a.Expiry, a.Scope)
	return err
}

// UpdateGoogleTokens refreshes just the access token + expiry after a refresh.
func (s *Store) UpdateGoogleTokens(userID int64, accessToken, expiry string) error {
	_, err := s.db.Exec(`UPDATE google_accounts SET access_token=?, expiry=? WHERE user_id=?`,
		accessToken, expiry, userID)
	return err
}

// DeleteGoogleAccount disconnects a user's Google account and removes all links
// and event mappings owned by that user (cascade via link deletion).
func (s *Store) DeleteGoogleAccount(userID int64) error {
	if _, err := s.db.Exec(`DELETE FROM google_cal_links WHERE user_id=?`, userID); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM google_accounts WHERE user_id=?`, userID)
	return err
}

// ListGoogleLinks returns the calendar links for a user.
func (s *Store) ListGoogleLinks(userID int64) ([]GoogleCalLink, error) {
	rows, err := s.db.Query(`SELECT id, user_id, calendar_id, google_cal_id, google_cal_name, sync_token, last_sync, last_error
		FROM google_cal_links WHERE user_id=? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GoogleCalLink{}
	for rows.Next() {
		var l GoogleCalLink
		if err := rows.Scan(&l.ID, &l.UserID, &l.CalendarID, &l.GoogleCalID, &l.GoogleCalName,
			&l.SyncToken, &l.LastSync, &l.LastError); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ListAllGoogleLinks returns every link across users (for the background sync loop).
func (s *Store) ListAllGoogleLinks() ([]GoogleCalLink, error) {
	rows, err := s.db.Query(`SELECT id, user_id, calendar_id, google_cal_id, google_cal_name, sync_token, last_sync, last_error
		FROM google_cal_links ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GoogleCalLink{}
	for rows.Next() {
		var l GoogleCalLink
		if err := rows.Scan(&l.ID, &l.UserID, &l.CalendarID, &l.GoogleCalID, &l.GoogleCalName,
			&l.SyncToken, &l.LastSync, &l.LastError); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetGoogleLink returns a single link by id.
func (s *Store) GetGoogleLink(id int64) (*GoogleCalLink, error) {
	var l GoogleCalLink
	err := s.db.QueryRow(`SELECT id, user_id, calendar_id, google_cal_id, google_cal_name, sync_token, last_sync, last_error
		FROM google_cal_links WHERE id=?`, id).
		Scan(&l.ID, &l.UserID, &l.CalendarID, &l.GoogleCalID, &l.GoogleCalName, &l.SyncToken, &l.LastSync, &l.LastError)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// GetGoogleLinkByCalendar returns the link mirroring the given MW calendar, if any.
func (s *Store) GetGoogleLinkByCalendar(calendarID int64) (*GoogleCalLink, error) {
	var l GoogleCalLink
	err := s.db.QueryRow(`SELECT id, user_id, calendar_id, google_cal_id, google_cal_name, sync_token, last_sync, last_error
		FROM google_cal_links WHERE calendar_id=?`, calendarID).
		Scan(&l.ID, &l.UserID, &l.CalendarID, &l.GoogleCalID, &l.GoogleCalName, &l.SyncToken, &l.LastSync, &l.LastError)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// CreateGoogleLink links an MW calendar to a Google calendar (1:1).
func (s *Store) CreateGoogleLink(userID, calendarID int64, googleCalID, googleCalName string) (int64, error) {
	return s.db.ExecInsert(`INSERT INTO google_cal_links(user_id, calendar_id, google_cal_id, google_cal_name)
		VALUES(?,?,?,?)`, userID, calendarID, googleCalID, googleCalName)
}

// DeleteGoogleLink removes a link (and its event mappings via cascade).
func (s *Store) DeleteGoogleLink(id int64) error {
	_, err := s.db.Exec(`DELETE FROM google_cal_links WHERE id=?`, id)
	return err
}

// SetGoogleLinkSync records the sync token and outcome after a sync pass.
func (s *Store) SetGoogleLinkSync(id int64, syncToken, lastSync, lastErr string) error {
	_, err := s.db.Exec(`UPDATE google_cal_links SET sync_token=?, last_sync=?, last_error=? WHERE id=?`,
		syncToken, lastSync, lastErr, id)
	return err
}

// GoogleMapping is one MW-event <-> Google-event mapping row.
type GoogleMapping struct {
	LinkID        int64
	EventID       int64
	GoogleEventID string
	Etag          string
	UpdatedLocal  string
}

// GetGoogleMappingByEvent returns the mapping for an MW event under a link.
func (s *Store) GetGoogleMappingByEvent(linkID, eventID int64) (*GoogleMapping, error) {
	var m GoogleMapping
	err := s.db.QueryRow(`SELECT link_id, event_id, google_event_id, etag, updated_local
		FROM google_event_map WHERE link_id=? AND event_id=?`, linkID, eventID).
		Scan(&m.LinkID, &m.EventID, &m.GoogleEventID, &m.Etag, &m.UpdatedLocal)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetGoogleMappingByGID returns the mapping for a Google event id under a link.
func (s *Store) GetGoogleMappingByGID(linkID int64, googleEventID string) (*GoogleMapping, error) {
	var m GoogleMapping
	err := s.db.QueryRow(`SELECT link_id, event_id, google_event_id, etag, updated_local
		FROM google_event_map WHERE link_id=? AND google_event_id=?`, linkID, googleEventID).
		Scan(&m.LinkID, &m.EventID, &m.GoogleEventID, &m.Etag, &m.UpdatedLocal)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// UpsertGoogleMapping stores or updates a mapping row.
func (s *Store) UpsertGoogleMapping(m *GoogleMapping) error {
	_, err := s.db.Exec(`INSERT INTO google_event_map(link_id, event_id, google_event_id, etag, updated_local)
		VALUES(?,?,?,?,?)
		ON CONFLICT(link_id, event_id) DO UPDATE SET google_event_id=excluded.google_event_id,
			etag=excluded.etag, updated_local=excluded.updated_local`,
		m.LinkID, m.EventID, m.GoogleEventID, m.Etag, m.UpdatedLocal)
	return err
}

// DeleteGoogleMappingByEvent removes the mapping for an MW event under a link.
func (s *Store) DeleteGoogleMappingByEvent(linkID, eventID int64) error {
	_, err := s.db.Exec(`DELETE FROM google_event_map WHERE link_id=? AND event_id=?`, linkID, eventID)
	return err
}

// GetCalendarEventRaw fetches a single stored (non-expanded) event by id. Unlike
// GetCalendarEvent it is safe to call for sync bookkeeping (no RRULE expansion).
func (s *Store) GetCalendarEventRaw(id int64) (*model.CalendarEvent, error) {
	return s.GetCalendarEvent(id)
}
