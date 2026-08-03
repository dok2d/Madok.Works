package handler

import (
	"net/http"
	"strconv"
	"strings"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// globalHit is one cross-service search result for the unified search palette.
type globalHit struct {
	Service string `json:"service"` // kanban | wiki | chat | mail
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
}

const globalSearchPerService = 6

// handleGlobalSearch aggregates results across the board and enabled services.
func (h *Handler) handleGlobalSearch(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len(q) > maxSearchQueryLen {
		jsonResp(w, []globalHit{})
		return
	}
	// scope "" or "all" = everything; otherwise restrict to one service.
	scope := r.URL.Query().Get("scope")
	want := func(s string) bool { return scope == "" || scope == "all" || scope == s }
	out := []globalHit{}

	// Kanban tasks (scoped to the current project)
	if want("kanban") {
		pid := h.projectID(r)
		if ids, err := h.store.SearchTasks(pid, q, false); err == nil {
			for i, id := range ids {
				if i >= globalSearchPerService {
					break
				}
				if t, err := h.store.GetTask(id); err == nil {
					out = append(out, globalHit{Service: "kanban", Title: t.Title, URL: "/task/" + itoa(id)})
				}
			}
		}
	}

	// Wiki pages
	if want("wiki") && h.serviceEnabled("wiki") {
		if hits, err := h.store.SearchWikiPages(user.ID, user.IsAdmin, q, globalSearchPerService); err == nil {
			for _, hit := range hits {
				out = append(out, globalHit{Service: "wiki", Title: hit.SpaceKey + " / " + hit.Title, Snippet: hit.Snippet, URL: "/wiki/page/" + itoa(hit.PageID)})
			}
		}
	}

	// Chat messages
	if want("chat") && h.serviceEnabled("chat") {
		if hits, err := h.store.SearchChatMessages(user.ID, q, globalSearchPerService); err == nil {
			for _, hit := range hits {
				out = append(out, globalHit{Service: "chat", Title: hit.ChannelName, Snippet: hit.Text, URL: "/chat/channel/" + itoa(hit.ChannelID)})
			}
		}
	}

	// Calendar events
	if want("calendar") && h.serviceEnabled("calendar") {
		if hits, err := h.store.SearchCalendarEvents(user.ID, user.IsAdmin, q, globalSearchPerService); err == nil {
			for _, hit := range hits {
				out = append(out, globalHit{Service: "calendar", Title: hit.Title, Snippet: hit.Snippet, URL: "/calendar"})
			}
		}
	}

	// Mail
	if want("mail") && h.serviceEnabled("mail") {
		if hits, err := h.store.SearchMail(user.ID, q, globalSearchPerService); err == nil {
			for _, hit := range hits {
				title := hit.Subject
				if title == "" {
					title = hit.FromName
				}
				out = append(out, globalHit{Service: "mail", Title: title, Snippet: hit.Snippet, URL: "/mail"})
			}
		}
	}

	// Files (shared storage): folder and file names
	if want("files") && h.serviceEnabled("files") {
		if folders, items, err := h.store.SearchStorage(q, globalSearchPerService); err == nil {
			for _, f := range folders {
				out = append(out, globalHit{Service: "files", Title: f.Name, URL: "/files/folder/" + itoa(f.ID)})
			}
			for _, it := range items {
				u := "/files"
				if it.FolderID != nil {
					u = "/files/folder/" + itoa(*it.FolderID)
				}
				out = append(out, globalHit{Service: "files", Title: it.Name, Snippet: it.Mime, URL: u})
			}
		}
	}

	jsonResp(w, out)
}
