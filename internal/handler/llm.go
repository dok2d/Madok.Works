package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"kanban/internal/db"
	"kanban/internal/model"
)

// LLM integration: an external OpenAI-compatible chat API (llama.cpp's server,
// or any standard endpoint). Configured by an admin under Integrations; when
// successfully enabled it powers the wiki page "Summary" button.

const (
	llmHTTPTimeout     = 120 * time.Second
	llmTestTimeout     = 10 * time.Second
	llmSummaryMaxChars = 24000 // cap the page text sent to the model
)

type llmConfig struct {
	Enabled bool
	BaseURL string // e.g. http://host:8080/v1
	APIKey  string
	Model   string
}

func (h *Handler) llmConfig() llmConfig {
	return llmConfig{
		Enabled: h.store.GetSetting("llm_enabled") == "true",
		BaseURL: strings.TrimRight(strings.TrimSpace(h.store.GetSetting("llm_base_url")), "/"),
		APIKey:  h.store.GetSetting("llm_api_key"),
		Model:   strings.TrimSpace(h.store.GetSetting("llm_model")),
	}
}

// llmEnabled reports whether the wiki summary feature should be offered.
func (h *Handler) llmEnabled() bool {
	c := h.llmConfig()
	return c.Enabled && c.BaseURL != ""
}

// llmTest verifies the endpoint is reachable (GET {base}/models). It is the gate
// for "successfully enabled".
func llmTest(c llmConfig) error {
	if c.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return err
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := (&http.Client{Timeout: llmTestTimeout}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}

// llmSummarize asks the model for a concise summary of the given text.
func llmSummarize(c llmConfig, text string) (string, error) {
	if len(text) > llmSummaryMaxChars {
		text = text[:llmSummaryMaxChars]
	}
	payload := map[string]any{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a concise technical writer. Summarize the wiki page provided by the user as a short paragraph or a few bullet points. Reply in the same language as the page. Output only the summary, no preamble."},
			{"role": "user", "content": text},
		},
		"temperature": 0.3,
		"stream":      false,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := (&http.Client{Timeout: llmHTTPTimeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("empty response from model")
	}
	content := out.Choices[0].Message.Content
	// Some reasoning models prepend a <think>…</think> block — keep only the answer.
	if i := strings.LastIndex(content, "</think>"); i >= 0 {
		content = content[i+len("</think>"):]
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "", fmt.Errorf("empty response from model")
	}
	return content, nil
}

// --- Endpoints ---

// handleLLMSettings reads/writes the LLM integration config (admin only). On a
// POST that requests enabling, the connection is tested first and the feature is
// only marked enabled if that test succeeds.
func (h *Handler) handleLLMSettings(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil || !user.IsAdmin {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		c := h.llmConfig()
		jsonResp(w, map[string]any{
			"enabled":  c.Enabled,
			"base_url": c.BaseURL,
			"model":    c.Model,
			"has_key":  c.APIKey != "",
		})
	case http.MethodPost:
		var req struct {
			BaseURL  string `json:"base_url"`
			APIKey   string `json:"api_key"`
			Model    string `json:"model"`
			Enabled  bool   `json:"enabled"`
			ClearKey bool   `json:"clear_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		h.store.SetSetting("llm_base_url", strings.TrimRight(strings.TrimSpace(req.BaseURL), "/"))
		h.store.SetSetting("llm_model", strings.TrimSpace(req.Model))
		if req.ClearKey {
			h.store.SetSetting("llm_api_key", "")
		} else if req.APIKey != "" {
			h.store.SetSetting("llm_api_key", req.APIKey)
		}
		if req.Enabled {
			cfg := h.llmConfig()
			if err := llmTest(cfg); err != nil {
				h.store.SetSetting("llm_enabled", "false")
				http.Error(w, "connection test failed: "+err.Error(), http.StatusBadGateway)
				return
			}
			h.store.SetSetting("llm_enabled", "true")
		} else {
			h.store.SetSetting("llm_enabled", "false")
		}
		jsonResp(w, map[string]any{"status": "ok", "enabled": h.store.GetSetting("llm_enabled") == "true"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleLLMTest tests a (possibly unsaved) LLM config so the admin can verify
// before enabling.
func (h *Handler) handleLLMTest(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil || !user.IsAdmin {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}
	var req struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		Model   string `json:"model"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	c := h.llmConfig()
	if strings.TrimSpace(req.BaseURL) != "" {
		c.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	}
	if req.APIKey != "" {
		c.APIKey = req.APIKey
	}
	if strings.TrimSpace(req.Model) != "" {
		c.Model = strings.TrimSpace(req.Model)
	}
	if err := llmTest(c); err != nil {
		jsonResp(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	jsonResp(w, map[string]any{"ok": true})
}

// handleLLMStatus tells the frontend whether the wiki summary button should show.
func (h *Handler) handleLLMStatus(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, map[string]any{"enabled": h.llmEnabled()})
}

// handleDBInfo returns sanitized database connection details for the
// integrations page (admin only).
func (h *Handler) handleDBInfo(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil || !user.IsAdmin {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}
	jsonResp(w, h.store.ConnInfo())
}

// handleWikiPageSummary returns an LLM summary of a wiki page, cached per
// revision: the first call computes and stores it; later calls for the same
// revision return the cached copy.
func (h *Handler) handleWikiPageSummary(w http.ResponseWriter, r *http.Request, page *model.WikiPage) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.llmEnabled() {
		http.Error(w, "LLM integration is not enabled", http.StatusForbidden)
		return
	}
	if cached, ok := h.store.GetWikiSummary(page.ID, page.RevNo); ok {
		jsonResp(w, map[string]any{"summary": cached, "cached": true, "rev_no": page.RevNo})
		return
	}
	text := strings.TrimSpace(db.WikiPagePlainText(page))
	if text == "" {
		http.Error(w, "page is empty", http.StatusBadRequest)
		return
	}
	// Summarization can take longer than the server's default WriteTimeout, so
	// extend the write deadline for this response (the LLM client has its own cap).
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(llmHTTPTimeout + 15*time.Second))
	cfg := h.llmConfig()
	summary, err := llmSummarize(cfg, text)
	if err != nil {
		h.logf("llm summarize: %v", err)
		http.Error(w, "summary failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	if err := h.store.SaveWikiSummary(page.ID, page.RevNo, summary, cfg.Model); err != nil {
		h.logf("save summary: %v", err)
	}
	jsonResp(w, map[string]any{"summary": summary, "cached": false, "rev_no": page.RevNo})
}
