package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"kanban/internal/model"
)

const (
	calMaxNameLen  = 100
	calMaxTitleLen = 200
	calMaxTextLen  = 4000
)

// calUser gates calendar endpoints on the service being enabled and the request
// being authenticated.
func (h *Handler) calUser(w http.ResponseWriter, r *http.Request) *model.User {
	if !h.serviceEnabled("calendar") {
		http.Error(w, "calendar service disabled", http.StatusNotFound)
		return nil
	}
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}
	return user
}

const calReminderLeadMin = 15 // notify this many minutes before an event starts

// runCalendarReminders periodically notifies users about upcoming events.
// Events with explicit per-event reminders (including recurring ones, via
// per-occurrence dedup) use the configurable path; events without any explicit
// reminder fall back to a single default 15-minute reminder.
func (h *Handler) runCalendarReminders() {
	const layout = "2006-01-02 15:04:05"
	for {
		time.Sleep(time.Minute)
		if !h.serviceEnabled("calendar") {
			continue
		}
		now := time.Now().UTC()

		// Default path: events without explicit reminders (non-recurring, 15 min).
		if due, err := h.store.DueReminders(now.Format(layout), now.Add(calReminderLeadMin*time.Minute).Format(layout)); err == nil {
			for _, d := range due {
				h.notifyReminder("both", d.Title, d.OwnerID, d.CreatedBy)
				h.store.MarkReminded(d.EventID)
			}
		}

		// Configurable path: per-event reminders, recurring-aware (occurrence dedup).
		if inst, err := h.store.DueReminderInstances(now); err == nil {
			for _, it := range inst {
				if !h.store.MarkReminderSent(it.EventID, it.LeadMin, it.OccurrenceStart) {
					continue
				}
				h.notifyReminder(it.Channel, it.Title, it.OwnerID, it.CreatedBy)
			}
		}
	}
}

// notifyReminder delivers a reminder to the event owner/creator via the channel.
func (h *Handler) notifyReminder(channel, title string, owner, createdBy *int64) {
	targets := map[int64]bool{}
	if owner != nil {
		targets[*owner] = true
	}
	if createdBy != nil {
		targets[*createdBy] = true
	}
	text := fmt.Sprintf("Напоминание: «%s» скоро начнётся", title)
	for uid := range targets {
		if channel == "inapp" || channel == "both" || channel == "" {
			h.store.CreateNotification(uid, "calendar_reminder", text, nil)
		}
		if channel == "telegram" || channel == "both" || channel == "" {
			h.sendTelegramNotification(uid, text)
		}
	}
}

// handleCalendarUpcoming returns the count of upcoming events (next 24h) for the
// rail badge.
func (h *Handler) handleCalendarUpcoming(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	const layout = "2006-01-02 15:04:05"
	now := time.Now().UTC()
	n := h.store.UpcomingEventCount(user.ID, user.IsAdmin, now.Format(layout), now.Add(24*time.Hour).Format(layout))
	jsonResp(w, map[string]int{"upcoming": n})
}

// calAudience resolves which users should receive live updates for a calendar:
// nil broadcasts (shared calendar), otherwise just the owner.
func (h *Handler) calAudience(cal *model.Calendar) []int64 {
	if cal.Visibility == "shared" {
		return nil
	}
	if cal.OwnerID != nil {
		return []int64{*cal.OwnerID}
	}
	return []int64{}
}

func (h *Handler) calNotify(cal *model.Calendar) {
	h.sse.sendTo(h.calAudience(cal), map[string]string{"type": "calendar"})
}

// handleCalendarStream is the SSE endpoint for live calendar updates.
func (h *Handler) handleCalendarStream(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	h.serveSSE(w, r, user.ID)
}

// === Calendars ===

func (h *Handler) handleCalendars(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		cals, err := h.store.ListCalendars(user.ID, user.IsAdmin)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, cals)
	case http.MethodPost:
		var req struct {
			Name       string `json:"name"`
			Color      string `json:"color"`
			Visibility string `json:"visibility"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Name == "" || len(req.Name) > calMaxNameLen {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		id, err := h.store.CreateCalendar(req.Name, req.Color, user.ID, req.Visibility)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.calNotify(&model.Calendar{Visibility: req.Visibility, OwnerID: &user.ID})
		jsonResp(w, map[string]int64{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleCalendar(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	id := extractID(r.URL.Path, "/api/calendar/calendars/")
	cal, err := h.store.GetCalendar(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "calendar not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !h.store.CanEditCalendar(cal, user.ID, user.IsAdmin) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var req struct {
			Name       string `json:"name"`
			Color      string `json:"color"`
			Visibility string `json:"visibility"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Name == "" || len(req.Name) > calMaxNameLen {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if err := h.store.UpdateCalendar(id, req.Name, req.Color, req.Visibility); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cal.Visibility = req.Visibility
		h.calNotify(cal)
		jsonResp(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if err := h.store.DeleteCalendar(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.calNotify(cal)
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// === Events ===

func (h *Handler) handleCalendarEvents(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		events, err := h.store.ListCalendarEvents(user.ID, user.IsAdmin, q.Get("from"), q.Get("to"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonResp(w, events)
	case http.MethodPost:
		var ev model.CalendarEvent
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !h.calEventInputOK(w, &ev) {
			return
		}
		cal, err := h.store.GetCalendar(ev.CalendarID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "calendar not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !h.store.CanEditCalendar(cal, user.ID, user.IsAdmin) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		created, err := h.store.CreateCalendarEvent(&ev, user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.calNotify(cal)
		jsonResp(w, created)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleCalendarEvent(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	id := extractID(r.URL.Path, "/api/calendar/events/")
	ev, err := h.store.GetCalendarEvent(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cal, err := h.store.GetCalendar(ev.CalendarID)
	if err != nil {
		http.Error(w, "calendar not found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !h.store.CanViewCalendar(cal, user.ID, user.IsAdmin) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		jsonResp(w, ev)
	case http.MethodPut:
		if !h.store.CanEditCalendar(cal, user.ID, user.IsAdmin) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var in model.CalendarEvent
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		in.CalendarID = ev.CalendarID
		if !h.calEventInputOK(w, &in) {
			return
		}
		updated, err := h.store.UpdateCalendarEvent(id, &in)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.calNotify(cal)
		jsonResp(w, updated)
	case http.MethodDelete:
		if !h.store.CanEditCalendar(cal, user.ID, user.IsAdmin) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := h.store.DeleteCalendarEvent(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.calNotify(cal)
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// calEventInputOK validates user-supplied event fields, writing an error response
// and returning false on failure.
func (h *Handler) calEventInputOK(w http.ResponseWriter, ev *model.CalendarEvent) bool {
	ev.Title = strings.TrimSpace(ev.Title)
	if ev.Title == "" || len(ev.Title) > calMaxTitleLen {
		http.Error(w, "title required (max 200)", http.StatusBadRequest)
		return false
	}
	if len(ev.Description) > calMaxTextLen || len(ev.Location) > calMaxNameLen {
		http.Error(w, "description or location too long", http.StatusBadRequest)
		return false
	}
	if ev.RRule != "" && !strings.Contains(strings.ToUpper(ev.RRule), "FREQ=") {
		http.Error(w, "invalid rrule", http.StatusBadRequest)
		return false
	}
	return true
}
