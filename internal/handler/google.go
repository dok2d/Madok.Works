package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"kanban/internal/db"
	"kanban/internal/model"
)

// Google Calendar two-way sync.
//
// Auth is per-user OAuth2 (the admin registers one Google Cloud OAuth client and
// fills client_id/secret in Integrations; each user then connects their own
// Google account). Implemented on the stdlib only — no google.golang.org/api or
// x/oauth2 dependency — mirroring the existing manual OIDC flow.
//
// Sync is two-way: pushes are event-driven (MW create/update/delete immediately
// mirror to Google) and pulls are polled (runGoogleSync walks each link's
// incremental syncToken). Echoes are suppressed by storing the etag Google
// returns on our own writes and skipping incoming events whose etag is unchanged.

const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserinfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	googleCalAPI      = "https://www.googleapis.com/calendar/v3"
	googleScope       = "https://www.googleapis.com/auth/calendar https://www.googleapis.com/auth/userinfo.email"
	calTimeLayout     = "2006-01-02 15:04:05" // MW stored UTC time format
)

var googleHTTP = &http.Client{Timeout: 30 * time.Second}

type googleAuthState struct {
	userID int64
	expiry time.Time
}

// googleConfigured reports whether an OAuth client is set up by the admin.
func (h *Handler) googleConfigured() bool {
	return h.store.GetSetting("google_cal_enabled") == "true" &&
		h.store.GetSetting("google_client_id") != "" &&
		h.store.GetSetting("google_client_secret") != ""
}

// googleRedirectURI reconstructs the OAuth redirect URI from the request. The
// admin must register this exact URI in the Google Cloud console.
func (h *Handler) googleRedirectURI(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	}
	host := r.Host
	if f := r.Header.Get("X-Forwarded-Host"); f != "" {
		host = f
	}
	return scheme + "://" + host + "/api/google/callback"
}

// === Admin config ===

// handleGoogleConfig reads/writes the admin-level Google OAuth client settings.
// GET is open to any authenticated user (returns the redirect URI hint and
// whether a secret is set, never the secret itself); POST is admin-only.
func (h *Handler) handleGoogleConfig(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		jsonResp(w, map[string]any{
			"enabled":      h.store.GetSetting("google_cal_enabled") == "true",
			"client_id":    h.store.GetSetting("google_client_id"),
			"secret_set":   h.store.GetSetting("google_client_secret") != "",
			"redirect_uri": h.googleRedirectURI(r),
			"configured":   h.googleConfigured(),
		})
	case http.MethodPost:
		if !user.IsAdmin {
			http.Error(w, "admin only", http.StatusForbidden)
			return
		}
		var req struct {
			Enabled      *bool   `json:"enabled"`
			ClientID     *string `json:"client_id"`
			ClientSecret *string `json:"client_secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Enabled != nil {
			h.store.SetSetting("google_cal_enabled", boolStr(*req.Enabled))
		}
		if req.ClientID != nil {
			h.store.SetSetting("google_client_id", strings.TrimSpace(*req.ClientID))
		}
		// Only overwrite the secret when a non-empty value is sent, so the admin
		// can re-save other fields without re-entering it.
		if req.ClientSecret != nil && strings.TrimSpace(*req.ClientSecret) != "" {
			h.store.SetSetting("google_client_secret", strings.TrimSpace(*req.ClientSecret))
		}
		jsonResp(w, map[string]any{"status": "ok", "configured": h.googleConfigured()})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// === OAuth flow ===

func (h *Handler) handleGoogleStatus(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	acct, _ := h.store.GetGoogleAccount(user.ID)
	resp := map[string]any{
		"configured": h.googleConfigured(),
		"connected":  acct != nil,
	}
	if acct != nil {
		resp["email"] = acct.Email
	}
	links, _ := h.store.ListGoogleLinks(user.ID)
	resp["links"] = links
	jsonResp(w, resp)
}

func (h *Handler) handleGoogleConnect(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	if !h.googleConfigured() {
		http.Error(w, "Google integration not configured", http.StatusBadRequest)
		return
	}
	stateBytes := make([]byte, 16)
	rand.Read(stateBytes)
	state := fmt.Sprintf("%x", stateBytes)

	h.googleMu.Lock()
	now := time.Now()
	for k, v := range h.googleStates {
		if now.After(v.expiry) {
			delete(h.googleStates, k)
		}
	}
	h.googleStates[state] = googleAuthState{userID: user.ID, expiry: now.Add(10 * time.Minute)}
	h.googleMu.Unlock()

	q := url.Values{}
	q.Set("client_id", h.store.GetSetting("google_client_id"))
	q.Set("redirect_uri", h.googleRedirectURI(r))
	q.Set("response_type", "code")
	q.Set("scope", googleScope)
	q.Set("access_type", "offline")
	q.Set("include_granted_scopes", "true")
	q.Set("prompt", "consent")
	q.Set("state", state)
	http.Redirect(w, r, googleAuthURL+"?"+q.Encode(), http.StatusFound)
}

func (h *Handler) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	state := r.URL.Query().Get("state")
	h.googleMu.Lock()
	st, ok := h.googleStates[state]
	if ok {
		delete(h.googleStates, state)
	}
	h.googleMu.Unlock()
	if !ok || time.Now().After(st.expiry) || st.userID != user.ID {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		log.Printf("[google] oauth error: %s", errParam)
		http.Redirect(w, r, "/?google=denied", http.StatusFound)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	tok, err := h.googleExchangeCode(code, h.googleRedirectURI(r))
	if err != nil {
		log.Printf("[google] token exchange: %v", err)
		http.Redirect(w, r, "/?google=error", http.StatusFound)
		return
	}
	email := h.googleFetchEmail(tok.AccessToken)
	acct := &db.GoogleAccount{
		UserID:       user.ID,
		Email:        email,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).UTC().Format(time.RFC3339),
		Scope:        tok.Scope,
	}
	if err := h.store.UpsertGoogleAccount(acct); err != nil {
		log.Printf("[google] store account: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?google=connected#calendar", http.StatusFound)
}

func (h *Handler) handleGoogleDisconnect(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.store.DeleteGoogleAccount(user.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

// === OAuth token helpers ===

type googleToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

func (h *Handler) googleExchangeCode(code, redirect string) (*googleToken, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", h.store.GetSetting("google_client_id"))
	form.Set("client_secret", h.store.GetSetting("google_client_secret"))
	form.Set("redirect_uri", redirect)
	form.Set("grant_type", "authorization_code")
	return h.googlePostToken(form)
}

func (h *Handler) googlePostToken(form url.Values) (*googleToken, error) {
	resp, err := googleHTTP.PostForm(googleTokenURL, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tok googleToken
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, err
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("empty access token")
	}
	return &tok, nil
}

func (h *Handler) googleFetchEmail(accessToken string) string {
	req, _ := http.NewRequest(http.MethodGet, googleUserinfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := googleHTTP.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var info struct {
		Email string `json:"email"`
	}
	json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info)
	return info.Email
}

// googleAccessToken returns a valid access token for the user, refreshing and
// persisting it when the stored one is expired (or about to expire).
func (h *Handler) googleAccessToken(acct *db.GoogleAccount) (string, error) {
	exp, err := time.Parse(time.RFC3339, acct.Expiry)
	if err == nil && time.Now().Add(60*time.Second).Before(exp) {
		return acct.AccessToken, nil
	}
	if acct.RefreshToken == "" {
		return "", fmt.Errorf("no refresh token; reconnect required")
	}
	form := url.Values{}
	form.Set("client_id", h.store.GetSetting("google_client_id"))
	form.Set("client_secret", h.store.GetSetting("google_client_secret"))
	form.Set("refresh_token", acct.RefreshToken)
	form.Set("grant_type", "refresh_token")
	tok, err := h.googlePostToken(form)
	if err != nil {
		return "", err
	}
	newExp := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	acct.AccessToken = tok.AccessToken
	acct.Expiry = newExp
	h.store.UpdateGoogleTokens(acct.UserID, tok.AccessToken, newExp)
	return tok.AccessToken, nil
}

// googleAPI performs an authenticated Calendar API call and returns the response
// body. A non-2xx status is returned as an error including the body.
func (h *Handler) googleAPI(acct *db.GoogleAccount, method, urlStr string, payload any) ([]byte, error) {
	token, err := h.googleAccessToken(acct)
	if err != nil {
		return nil, err
	}
	var bodyReader io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, urlStr, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := googleHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &googleAPIError{status: resp.StatusCode, body: strings.TrimSpace(string(body))}
	}
	return body, nil
}

type googleAPIError struct {
	status int
	body   string
}

func (e *googleAPIError) Error() string { return fmt.Sprintf("google api %d: %s", e.status, e.body) }

// === Calendar list + links ===

func (h *Handler) handleGoogleCalendars(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	acct, _ := h.store.GetGoogleAccount(user.ID)
	if acct == nil {
		http.Error(w, "not connected", http.StatusBadRequest)
		return
	}
	body, err := h.googleAPI(acct, http.MethodGet, googleCalAPI+"/users/me/calendarList?minAccessRole=writer", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	var list struct {
		Items []struct {
			ID          string `json:"id"`
			Summary     string `json:"summary"`
			Primary     bool   `json:"primary"`
			AccessRole  string `json:"accessRole"`
			Description string `json:"description"`
		} `json:"items"`
	}
	json.Unmarshal(body, &list)
	out := make([]map[string]any, 0, len(list.Items))
	for _, it := range list.Items {
		out = append(out, map[string]any{
			"id": it.ID, "summary": it.Summary, "primary": it.Primary, "access_role": it.AccessRole,
		})
	}
	jsonResp(w, out)
}

func (h *Handler) handleGoogleLinks(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		links, err := h.store.ListGoogleLinks(user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, links)
	case http.MethodPost:
		var req struct {
			CalendarID    int64  `json:"calendar_id"`
			GoogleCalID   string `json:"google_cal_id"`
			GoogleCalName string `json:"google_cal_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.GoogleCalID == "" {
			http.Error(w, "google_cal_id required", http.StatusBadRequest)
			return
		}
		cal, err := h.store.GetCalendar(req.CalendarID)
		if err != nil {
			http.Error(w, "calendar not found", http.StatusNotFound)
			return
		}
		if !h.calAccess(user, cal, true) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if existing, _ := h.store.GetGoogleLinkByCalendar(req.CalendarID); existing != nil {
			http.Error(w, "calendar already linked", http.StatusConflict)
			return
		}
		id, err := h.store.CreateGoogleLink(user.ID, req.CalendarID, req.GoogleCalID, req.GoogleCalName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Kick off an initial sync in the background so existing events flow both ways.
		go func() {
			if link, _ := h.store.GetGoogleLink(id); link != nil {
				h.googleSyncLink(link)
			}
		}()
		jsonResp(w, map[string]int64{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleGoogleLinkRoute(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	id := extractID(r.URL.Path, "/api/google/links/")
	link, err := h.store.GetGoogleLink(id)
	if err != nil || link == nil || link.UserID != user.ID {
		http.Error(w, "link not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.store.DeleteGoogleLink(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleGoogleSyncNow(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	links, _ := h.store.ListGoogleLinks(user.ID)
	for i := range links {
		h.googleSyncLink(&links[i])
	}
	jsonResp(w, map[string]any{"status": "ok", "links": len(links)})
}

// === Sync engine ===

// runGoogleSync polls every linked calendar for incremental changes.
func (h *Handler) runGoogleSync() {
	for {
		time.Sleep(2 * time.Minute)
		if !h.serviceEnabled("calendar") || !h.googleConfigured() {
			continue
		}
		links, err := h.store.ListAllGoogleLinks()
		if err != nil {
			continue
		}
		for i := range links {
			h.googleSyncLink(&links[i])
		}
	}
}

// googleSyncLink pulls remote changes for one link into MW.
func (h *Handler) googleSyncLink(link *db.GoogleCalLink) {
	acct, _ := h.store.GetGoogleAccount(link.UserID)
	if acct == nil {
		return
	}
	cal, err := h.store.GetCalendar(link.CalendarID)
	if err != nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)

	pageToken := ""
	syncToken := link.SyncToken
	newSyncToken := ""
	for {
		q := url.Values{}
		q.Set("showDeleted", "true")
		q.Set("singleEvents", "false")
		q.Set("maxResults", "250")
		if syncToken != "" {
			q.Set("syncToken", syncToken)
		} else {
			// Initial pull: bound to a sensible window to avoid huge backfills.
			q.Set("timeMin", time.Now().AddDate(-1, 0, 0).UTC().Format(time.RFC3339))
		}
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		urlStr := googleCalAPI + "/calendars/" + url.PathEscape(link.GoogleCalID) + "/events?" + q.Encode()
		body, err := h.googleAPI(acct, http.MethodGet, urlStr, nil)
		if err != nil {
			// A 410 Gone means the syncToken expired; restart with a full sync.
			if ge, ok := err.(*googleAPIError); ok && ge.status == http.StatusGone && syncToken != "" {
				h.store.SetGoogleLinkSync(link.ID, "", now, "")
				syncToken = ""
				pageToken = ""
				continue
			}
			h.store.SetGoogleLinkSync(link.ID, link.SyncToken, now, err.Error())
			return
		}
		var page googleEventsPage
		if err := json.Unmarshal(body, &page); err != nil {
			h.store.SetGoogleLinkSync(link.ID, link.SyncToken, now, err.Error())
			return
		}
		for i := range page.Items {
			h.googleApplyRemoteEvent(link, cal, &page.Items[i])
		}
		if page.NextPageToken != "" {
			pageToken = page.NextPageToken
			continue
		}
		newSyncToken = page.NextSyncToken
		break
	}
	h.store.SetGoogleLinkSync(link.ID, newSyncToken, now, "")
	h.calNotify(cal)
}

type googleEventsPage struct {
	Items         []googleEvent `json:"items"`
	NextPageToken string        `json:"nextPageToken"`
	NextSyncToken string        `json:"nextSyncToken"`
}

type googleEventTime struct {
	DateTime string `json:"dateTime,omitempty"`
	Date     string `json:"date,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type googleEvent struct {
	ID          string          `json:"id,omitempty"`
	Status      string          `json:"status,omitempty"`
	Etag        string          `json:"etag,omitempty"`
	Summary     string          `json:"summary"`
	Description string          `json:"description,omitempty"`
	Location    string          `json:"location,omitempty"`
	Start       googleEventTime `json:"start,omitempty"`
	End         googleEventTime `json:"end,omitempty"`
	Recurrence  []string        `json:"recurrence,omitempty"`
	Updated     string          `json:"updated,omitempty"`
}

// googleApplyRemoteEvent reconciles one remote event into the MW calendar.
func (h *Handler) googleApplyRemoteEvent(link *db.GoogleCalLink, cal *model.Calendar, ge *googleEvent) {
	if ge.ID == "" {
		return
	}
	mapping, _ := h.store.GetGoogleMappingByGID(link.ID, ge.ID)

	if ge.Status == "cancelled" {
		if mapping != nil {
			h.store.DeleteGoogleMappingByEvent(link.ID, mapping.EventID)
			h.store.DeleteCalendarEvent(mapping.EventID)
		}
		return
	}
	// Echo suppression: if the remote etag matches what we last wrote, this change
	// originated from us — skip it.
	if mapping != nil && mapping.Etag != "" && mapping.Etag == ge.Etag {
		return
	}

	ev := googleToMWEvent(ge)
	ev.CalendarID = link.CalendarID

	if mapping == nil {
		created, err := h.store.CreateCalendarEvent(ev, link.UserID)
		if err != nil {
			log.Printf("[google] create from remote failed: %v", err)
			return
		}
		h.store.UpsertGoogleMapping(&db.GoogleMapping{
			LinkID: link.ID, EventID: created.ID, GoogleEventID: ge.ID,
			Etag: ge.Etag, UpdatedLocal: created.UpdatedAt,
		})
		return
	}
	updated, err := h.store.UpdateCalendarEvent(mapping.EventID, ev)
	if err != nil {
		log.Printf("[google] update from remote failed: %v", err)
		return
	}
	h.store.UpsertGoogleMapping(&db.GoogleMapping{
		LinkID: link.ID, EventID: mapping.EventID, GoogleEventID: ge.ID,
		Etag: ge.Etag, UpdatedLocal: updated.UpdatedAt,
	})
}

// === Push (MW -> Google), invoked from calendar event handlers ===

// googlePushEvent mirrors a created/updated MW event to Google if its calendar
// is linked. Best-effort: failures are logged, not surfaced to the user.
func (h *Handler) googlePushEvent(ev *model.CalendarEvent) {
	if ev == nil {
		return
	}
	link, _ := h.store.GetGoogleLinkByCalendar(ev.CalendarID)
	if link == nil {
		return
	}
	acct, _ := h.store.GetGoogleAccount(link.UserID)
	if acct == nil {
		return
	}
	payload := mwToGoogleEvent(ev)
	mapping, _ := h.store.GetGoogleMappingByEvent(link.ID, ev.ID)

	var body []byte
	var err error
	if mapping == nil {
		body, err = h.googleAPI(acct, http.MethodPost,
			googleCalAPI+"/calendars/"+url.PathEscape(link.GoogleCalID)+"/events", payload)
	} else {
		body, err = h.googleAPI(acct, http.MethodPut,
			googleCalAPI+"/calendars/"+url.PathEscape(link.GoogleCalID)+"/events/"+url.PathEscape(mapping.GoogleEventID), payload)
	}
	if err != nil {
		log.Printf("[google] push event %d: %v", ev.ID, err)
		return
	}
	var resp googleEvent
	json.Unmarshal(body, &resp)
	gid := resp.ID
	if gid == "" && mapping != nil {
		gid = mapping.GoogleEventID
	}
	h.store.UpsertGoogleMapping(&db.GoogleMapping{
		LinkID: link.ID, EventID: ev.ID, GoogleEventID: gid,
		Etag: resp.Etag, UpdatedLocal: ev.UpdatedAt,
	})
}

// googleDeleteEvent removes an MW event's Google twin if linked.
func (h *Handler) googleDeleteEvent(calendarID, eventID int64) {
	link, _ := h.store.GetGoogleLinkByCalendar(calendarID)
	if link == nil {
		return
	}
	mapping, _ := h.store.GetGoogleMappingByEvent(link.ID, eventID)
	if mapping == nil {
		return
	}
	acct, _ := h.store.GetGoogleAccount(link.UserID)
	if acct != nil {
		_, err := h.googleAPI(acct, http.MethodDelete,
			googleCalAPI+"/calendars/"+url.PathEscape(link.GoogleCalID)+"/events/"+url.PathEscape(mapping.GoogleEventID), nil)
		if err != nil {
			log.Printf("[google] delete event %d: %v", eventID, err)
		}
	}
	h.store.DeleteGoogleMappingByEvent(link.ID, eventID)
}

// === Field conversion ===

func mwToGoogleEvent(ev *model.CalendarEvent) *googleEvent {
	ge := &googleEvent{
		Summary:     ev.Title,
		Description: ev.Description,
		Location:    ev.Location,
	}
	if ev.AllDay {
		ge.Start = googleEventTime{Date: dateOnly(ev.Start)}
		// Google all-day end.date is exclusive — add a day to the inclusive MW end.
		ge.End = googleEventTime{Date: dateOnly(addDay(ev.End))}
	} else {
		ge.Start = googleEventTime{DateTime: mwToRFC3339(ev.Start), TimeZone: "UTC"}
		ge.End = googleEventTime{DateTime: mwToRFC3339(ev.End), TimeZone: "UTC"}
	}
	if ev.RRule != "" {
		ge.Recurrence = []string{"RRULE:" + ev.RRule}
	}
	return ge
}

func googleToMWEvent(ge *googleEvent) *model.CalendarEvent {
	ev := &model.CalendarEvent{
		Title:       strings.TrimSpace(ge.Summary),
		Description: ge.Description,
		Location:    ge.Location,
	}
	if ev.Title == "" {
		ev.Title = "(no title)"
	}
	if ge.Start.Date != "" { // all-day
		ev.AllDay = true
		ev.Start = ge.Start.Date + " 00:00:00"
		end := ge.End.Date
		if end != "" {
			end = subDay(end) // undo Google's exclusive end
		} else {
			end = ge.Start.Date
		}
		ev.End = end + " 00:00:00"
	} else {
		ev.Start = rfc3339ToMW(ge.Start.DateTime)
		ev.End = rfc3339ToMW(ge.End.DateTime)
		if ev.End == "" {
			ev.End = ev.Start
		}
	}
	for _, r := range ge.Recurrence {
		if strings.HasPrefix(r, "RRULE:") {
			ev.RRule = strings.TrimPrefix(r, "RRULE:")
			break
		}
	}
	return ev
}

// --- time helpers ---

func mwToRFC3339(s string) string {
	t, err := time.Parse(calTimeLayout, s)
	if err != nil {
		if t2, err2 := time.Parse(time.RFC3339, s); err2 == nil {
			return t2.UTC().Format(time.RFC3339)
		}
		return s
	}
	return t.UTC().Format(time.RFC3339)
}

func rfc3339ToMW(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return ""
	}
	return t.UTC().Format(calTimeLayout)
}

func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func addDay(s string) string {
	d := dateOnly(s)
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return d
	}
	return t.AddDate(0, 0, 1).Format("2006-01-02")
}

func subDay(s string) string {
	d := dateOnly(s)
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return d
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02")
}
