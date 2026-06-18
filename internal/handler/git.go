package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"kanban/internal/model"
)

// === Git integration ===
//
// Inbound webhooks (GitHub/GitLab/Gitea) are normalized into git_events linked
// to tasks by KEY-<n> / #<n> references found in branch names, commit messages
// and MR/PR titles. On a merged MR/PR the linked task can be auto-moved to a
// configured column. Repos are configured per project in Settings → Integrations.

const gitMaxWebhookBody = 2 << 20 // 2 MiB

// taskRefRe matches "KEY-123" and bare "#123" task references.
var taskRefRe = regexp.MustCompile(`(?:\b([A-Za-z][A-Za-z0-9]+)-(\d+)\b)|(?:#(\d+)\b)`)

// gitNormalized is the cross-provider shape we extract from a webhook payload.
type gitNormalized struct {
	kind     string // push | branch | mr | pipeline
	title    string
	ref      string
	url      string
	status   string
	author   string
	merged   bool
	scanText string // text to scan for task references
}

// handleGitWebhook ingests /api/integrations/git/webhook/{repoID}.
func (h *Handler) handleGitWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/integrations/git/webhook/")
	repoID, err := strconv.ParseInt(strings.Trim(idStr, "/"), 10, 64)
	if err != nil || repoID <= 0 {
		http.Error(w, "bad repo id", http.StatusBadRequest)
		return
	}
	repo, err := h.store.GetGitRepo(repoID)
	if err != nil {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, gitMaxWebhookBody))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	if !verifyGitSignature(repo, r, body) {
		http.Error(w, "bad signature", http.StatusForbidden)
		return
	}
	norm, ok := normalizeGitPayload(repo.Kind, r, body)
	if !ok {
		// Unhandled event type — acknowledge so the provider doesn't retry.
		jsonResp(w, map[string]string{"status": "ignored"})
		return
	}

	// Resolve project key for KEY-<n> matching.
	projKey := strings.ToUpper(h.store.ProjectKey(repo.ProjectID))
	taskIDs := h.resolveTaskRefs(norm.scanText, repo.ProjectID, projKey)
	linked := 0
	for _, tid := range taskIDs {
		if _, err := h.store.AddGitEvent(model.GitEvent{
			RepoID: repo.ID, TaskID: tid, Kind: norm.kind, Title: norm.title,
			Ref: norm.ref, URL: norm.url, Status: norm.status, Author: norm.author,
		}); err != nil {
			continue
		}
		linked++
		// Auto-move on merge if configured (column belongs to the repo's project).
		if norm.merged && repo.MoveColumnID != nil && *repo.MoveColumnID > 0 {
			h.store.MoveTask(tid, *repo.MoveColumnID, 0)
			h.notifySubscribers(tid, 0, "git: "+norm.title)
		}
	}
	jsonResp(w, map[string]any{"status": "ok", "linked": linked})
}

// resolveTaskRefs finds KEY-<n>/#<n> references and maps them to task ids in the
// repo's project. KEY-<n> only matches when KEY equals the project key.
func (h *Handler) resolveTaskRefs(text string, projectID int64, projKey string) []int64 {
	seen := map[int]bool{}
	var ids []int64
	for _, m := range taskRefRe.FindAllStringSubmatch(text, -1) {
		var num int
		if m[2] != "" { // KEY-<n>
			if projKey != "" && !strings.EqualFold(m[1], projKey) {
				continue
			}
			num, _ = strconv.Atoi(m[2])
		} else if m[3] != "" { // #<n>
			num, _ = strconv.Atoi(m[3])
		}
		if num <= 0 || seen[num] {
			continue
		}
		seen[num] = true
		if id, ok := h.store.GetTaskIDByNumber(projectID, num); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// verifyGitSignature validates the webhook authenticity per provider. An empty
// configured secret skips verification (useful for trusted LAN setups).
func verifyGitSignature(repo *model.GitRepo, r *http.Request, body []byte) bool {
	if repo.Secret == "" {
		return true
	}
	switch repo.Kind {
	case "gitlab":
		return hmac.Equal([]byte(r.Header.Get("X-Gitlab-Token")), []byte(repo.Secret))
	case "gitea":
		if sig := r.Header.Get("X-Gitea-Signature"); sig != "" {
			return hmacHexEqual(sig, body, repo.Secret)
		}
		return hmacHexEqual(strings.TrimPrefix(r.Header.Get("X-Hub-Signature-256"), "sha256="), body, repo.Secret)
	default: // github
		return hmacHexEqual(strings.TrimPrefix(r.Header.Get("X-Hub-Signature-256"), "sha256="), body, repo.Secret)
	}
}

func hmacHexEqual(gotHex string, body []byte, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(strings.ToLower(strings.TrimSpace(gotHex))), []byte(want))
}

// normalizeGitPayload extracts a cross-provider event from the raw payload.
func normalizeGitPayload(kind string, r *http.Request, body []byte) (gitNormalized, bool) {
	var p map[string]any
	if err := json.Unmarshal(body, &p); err != nil {
		return gitNormalized{}, false
	}
	switch kind {
	case "gitlab":
		return normalizeGitlab(r.Header.Get("X-Gitlab-Event"), p)
	case "gitea":
		return normalizeGitHubLike(r.Header.Get("X-Gitea-Event"), p)
	default:
		return normalizeGitHubLike(r.Header.Get("X-GitHub-Event"), p)
	}
}

// normalizeGitHubLike handles GitHub and Gitea (very similar payloads).
func normalizeGitHubLike(event string, p map[string]any) (gitNormalized, bool) {
	switch event {
	case "push":
		var n gitNormalized
		n.kind = "push"
		n.ref = str(p["ref"])
		var msgs []string
		if commits, ok := p["commits"].([]any); ok {
			for _, c := range commits {
				if cm, ok := c.(map[string]any); ok {
					msgs = append(msgs, str(cm["message"]))
					n.url = str(cm["url"])
				}
			}
		}
		n.title = strings.TrimSpace(firstLine(strings.Join(msgs, " | ")))
		if n.title == "" {
			n.title = "push " + n.ref
		}
		n.author = nestedStr(p, "pusher", "name")
		n.scanText = n.ref + " " + strings.Join(msgs, " ")
		return n, true
	case "pull_request":
		pr, _ := p["pull_request"].(map[string]any)
		if pr == nil {
			return gitNormalized{}, false
		}
		var n gitNormalized
		n.kind = "mr"
		n.title = str(pr["title"])
		n.url = str(pr["html_url"])
		n.ref = nestedStr(pr, "head", "ref")
		action := str(p["action"])
		merged, _ := pr["merged"].(bool)
		n.merged = merged && (action == "closed")
		if n.merged {
			n.status = "merged"
		} else {
			n.status = action
		}
		n.author = nestedStr(pr, "user", "login")
		n.scanText = n.title + " " + str(pr["body"]) + " " + n.ref
		return n, true
	}
	return gitNormalized{}, false
}

func normalizeGitlab(event string, p map[string]any) (gitNormalized, bool) {
	switch event {
	case "Push Hook":
		var n gitNormalized
		n.kind = "push"
		n.ref = str(p["ref"])
		var msgs []string
		if commits, ok := p["commits"].([]any); ok {
			for _, c := range commits {
				if cm, ok := c.(map[string]any); ok {
					msgs = append(msgs, str(cm["message"]))
					n.url = str(cm["url"])
				}
			}
		}
		n.title = strings.TrimSpace(firstLine(strings.Join(msgs, " | ")))
		if n.title == "" {
			n.title = "push " + n.ref
		}
		n.author = nestedStr(p, "user_name", "")
		n.scanText = n.ref + " " + strings.Join(msgs, " ")
		return n, true
	case "Merge Request Hook":
		attr, _ := p["object_attributes"].(map[string]any)
		if attr == nil {
			return gitNormalized{}, false
		}
		var n gitNormalized
		n.kind = "mr"
		n.title = str(attr["title"])
		n.url = str(attr["url"])
		n.ref = str(attr["source_branch"])
		state := str(attr["state"])
		action := str(attr["action"])
		n.merged = action == "merge" || state == "merged"
		if n.merged {
			n.status = "merged"
		} else {
			n.status = state
		}
		n.scanText = n.title + " " + str(attr["description"]) + " " + n.ref
		return n, true
	case "Pipeline Hook":
		attr, _ := p["object_attributes"].(map[string]any)
		if attr == nil {
			return gitNormalized{}, false
		}
		var n gitNormalized
		n.kind = "pipeline"
		n.status = str(attr["status"])
		n.ref = str(attr["ref"])
		n.title = "pipeline " + n.status
		n.scanText = n.ref
		return n, true
	}
	return gitNormalized{}, false
}

// === small helpers ===

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func nestedStr(m map[string]any, k1, k2 string) string {
	if k2 == "" {
		return str(m[k1])
	}
	if sub, ok := m[k1].(map[string]any); ok {
		return str(sub[k2])
	}
	return ""
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// === Admin: repository configuration (Settings → Integrations) ===

// handleGitRepos handles GET (list, project-scoped) and POST (create) of repos.
func (h *Handler) handleGitRepos(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	pid := h.projectID(r)
	switch r.Method {
	case http.MethodGet:
		repos, err := h.store.ListGitRepos(pid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Don't leak secrets to the client; report only whether one is set.
		for i := range repos {
			if repos[i].Secret != "" {
				repos[i].Secret = "set"
			}
		}
		jsonResp(w, repos)
	case http.MethodPost:
		if !user.IsAdmin {
			http.Error(w, "admin only", http.StatusForbidden)
			return
		}
		var req struct {
			Kind, Label, Repo, Secret string
			MoveColumnID              *int64 `json:"move_column_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Kind != "github" && req.Kind != "gitlab" && req.Kind != "gitea" {
			req.Kind = "github"
		}
		id, err := h.store.CreateGitRepo(pid, req.Kind, strings.TrimSpace(req.Label), strings.TrimSpace(req.Repo), req.Secret, req.MoveColumnID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]int64{"id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGitRepoItem handles PUT/DELETE /api/integrations/git/repos/{id}.
func (h *Handler) handleGitRepoItem(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil || !user.IsAdmin {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/integrations/git/repos/")
	id, err := strconv.ParseInt(strings.Trim(idStr, "/"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		repo, err := h.store.GetGitRepo(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req struct {
			Label, Repo, Secret string
			MoveColumnID        *int64 `json:"move_column_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// Keep the existing secret when the client sends the "set" sentinel.
		secret := req.Secret
		if secret == "set" {
			secret = repo.Secret
		}
		if err := h.store.UpdateGitRepo(id, strings.TrimSpace(req.Label), strings.TrimSpace(req.Repo), secret, req.MoveColumnID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if err := h.store.DeleteGitRepo(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTaskGitEvents returns the git activity linked to a task.
func (h *Handler) handleTaskGitEvents(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	idStr = strings.TrimSuffix(idStr, "/gitevents")
	id, err := strconv.ParseInt(strings.Trim(idStr, "/"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	events, err := h.store.ListGitEventsForTask(id, 30)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, events)
}
