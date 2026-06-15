package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"kanban/internal/model"
)

const icsMaxBytes = 5 << 20         // 5 MB cap on a fetched feed
const icsSyncInterval = 15 * time.Minute

// fetchICS downloads an iCal feed with SSRF protection (refuses internal hosts),
// a timeout and a size cap. webcal:// is treated as https.
func fetchICS(rawURL string) ([]byte, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("bad url")
	}
	if u.Scheme == "webcal" {
		u.Scheme = "https"
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme")
	}
	if err := icsHostAllowed(u.Hostname()); err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return icsHostAllowed(req.URL.Hostname())
		},
	}
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "kanban-ics/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, icsMaxBytes))
}

// icsHostAllowed rejects hosts that resolve to internal/loopback addresses.
func icsHostAllowed(host string) error {
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("cannot resolve host")
	}
	for _, ip := range ips {
		if isInternalIP(ip) {
			return fmt.Errorf("refusing internal address")
		}
	}
	return nil
}

// syncSubscription fetches + caches one subscription's events (best effort: any
// failure is recorded in last_error, never panics).
func (h *Handler) syncSubscription(sub model.CalendarSubscription) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	data, err := fetchICS(sub.URL)
	if err != nil {
		h.store.SetSubscriptionSynced(sub.ID, now, err.Error())
		return
	}
	parsed := parseICS(data)
	evs := make([]model.CalendarEvent, 0, len(parsed))
	for _, p := range parsed {
		evs = append(evs, model.CalendarEvent{
			Title: p.Title, Description: p.Description, Location: p.Location,
			Start: p.Start, End: p.End, AllDay: p.AllDay, RRule: p.RRule,
		})
	}
	if err := h.store.ReplaceSubscriptionEvents(sub.ID, evs); err != nil {
		h.store.SetSubscriptionSynced(sub.ID, now, err.Error())
		return
	}
	h.store.SetSubscriptionSynced(sub.ID, now, "")
	if h.sse != nil {
		h.sse.sendTo([]int64{sub.UserID}, map[string]string{"type": "calendar"})
	}
}

// runICSSync periodically refreshes every subscription. Runs immediately on start.
func (h *Handler) runICSSync() {
	for {
		if subs, err := h.store.ListAllSubscriptions(); err == nil {
			for _, sub := range subs {
				h.syncSubscription(sub)
			}
		}
		time.Sleep(icsSyncInterval)
	}
}

// handleCalendarSubscriptions: GET list / POST create (per-user).
func (h *Handler) handleCalendarSubscriptions(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		subs, err := h.store.ListSubscriptions(user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, subs)
	case http.MethodPost:
		var req struct {
			Name  string `json:"name"`
			URL   string `json:"url"`
			Color string `json:"color"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		req.Name, req.URL = strings.TrimSpace(req.Name), strings.TrimSpace(req.URL)
		if req.URL == "" || len(req.URL) > 2000 {
			http.Error(w, "url required", http.StatusBadRequest)
			return
		}
		u, err := url.Parse(req.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "webcal") {
			http.Error(w, "url must be http(s)/webcal", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			req.Name = "Calendar"
		}
		if len(req.Name) > 100 {
			http.Error(w, "name too long", http.StatusBadRequest)
			return
		}
		id, err := h.store.CreateSubscription(user.ID, req.Name, req.URL, req.Color)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if sub, e := h.store.GetSubscription(id); e == nil && sub != nil {
			go h.syncSubscription(*sub) // first sync in the background
		}
		jsonResp(w, map[string]int64{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCalendarSubscriptionRoute: DELETE /{id}, POST /{id}/sync (owner only).
func (h *Handler) handleCalendarSubscriptionRoute(w http.ResponseWriter, r *http.Request) {
	user := h.calUser(w, r)
	if user == nil {
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/calendar/subscriptions/")
	parts := strings.SplitN(strings.TrimSuffix(rest, "/"), "/", 2)
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	sub, err := h.store.GetSubscription(id)
	if err != nil || sub == nil || sub.UserID != user.ID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	sub2 := ""
	if len(parts) == 2 {
		sub2 = parts[1]
	}
	switch sub2 {
	case "":
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := h.store.DeleteSubscription(id, user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.sse.sendTo([]int64{user.ID}, map[string]string{"type": "calendar"})
		jsonResp(w, map[string]string{"status": "ok"})
	case "sync":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		go h.syncSubscription(*sub)
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.NotFound(w, r)
	}
}
