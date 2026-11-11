package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// wikiEnv enables the wiki service and creates a space, returning the env and
// the space id. The admin is a space admin; the readonly user is capped at viewer.
func wikiEnv(t *testing.T) (*storageTestEnv, int64) {
	t.Helper()
	e := newStorageEnv(t)
	e.store.SetSetting("svc_wiki_enabled", "true")
	w := e.do(t, "POST", "/api/wiki/spaces", e.admin, map[string]any{"key": "DOC", "name": "Docs"})
	expectCode(t, w, http.StatusOK, "create space")
	var sp struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &sp)
	return e, sp.ID
}

func decode(t *testing.T, w interface{ Bytes() []byte }, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Bytes(), v); err != nil {
		t.Fatalf("decode %s: %v", w.Bytes(), err)
	}
}

func TestMindmapPageLifecycle(t *testing.T) {
	e, space := wikiEnv(t)

	// Create a mind-map page with an initial document.
	w := e.do(t, "POST", "/api/wiki/pages", e.admin, map[string]any{
		"space_id": space, "title": "Roadmap", "kind": "mindmap",
		"mindmap": map[string]any{"root": map[string]any{"text": "Roadmap", "children": []any{
			map[string]any{"id": "q1", "text": "Q1", "children": []any{map[string]any{"id": "api", "text": "API", "url": "https://x.y"}}},
			map[string]any{"id": "q2", "text": "Q2", "done": false},
		}}},
	})
	expectCode(t, w, http.StatusOK, "create mindmap page")
	var created struct {
		ID   int64  `json:"id"`
		Kind string `json:"kind"`
	}
	decode(t, w.Body, &created)
	if created.Kind != "mindmap" || created.ID == 0 {
		t.Fatalf("create response: %+v", created)
	}
	pid := created.ID
	base := fmt.Sprintf("/api/wiki/pages/%d", pid)

	// The page carries kind + document, a derived outline and markdown.
	w = e.do(t, "GET", base, e.reader, nil)
	expectCode(t, w, http.StatusOK, "get page as viewer")
	var page struct {
		Kind      string          `json:"kind"`
		Content   string          `json:"content"`
		ContentMd string          `json:"content_md"`
		Mindmap   json.RawMessage `json:"mindmap"`
		RevNo     int             `json:"rev_no"`
	}
	decode(t, w.Body, &page)
	if page.Kind != "mindmap" || !strings.Contains(page.Content, "<li>Q1<ul><li><a href=\"https://x.y\"") || !strings.Contains(page.Content, ">API</a>") || !strings.Contains(page.ContentMd, "- [ ] Q2") || len(page.Mindmap) == 0 {
		t.Fatalf("page view: %+v", page)
	}
	// The tree reports the kind.
	w = e.do(t, "GET", fmt.Sprintf("/api/wiki/spaces/%d/tree", space), e.reader, nil)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"kind":"mindmap"`)) {
		t.Fatalf("tree lacks kind: %s", w.Body.String())
	}

	// Document endpoint and node list.
	w = e.do(t, "GET", base+"/mindmap", e.reader, nil)
	expectCode(t, w, http.StatusOK, "get document")
	var doc struct {
		RevNo     int    `json:"rev_no"`
		NodeCount int    `json:"node_count"`
		Layout    string `json:"layout"`
		Root      struct {
			ID       string `json:"id"`
			Children []struct {
				ID string `json:"id"`
			} `json:"children"`
		} `json:"root"`
	}
	decode(t, w.Body, &doc)
	if doc.NodeCount != 4 || doc.Layout != "balanced" || doc.Root.Children[0].ID != "q1" {
		t.Fatalf("document: %+v", doc)
	}
	w = e.do(t, "GET", base+"/mindmap/nodes", e.reader, nil)
	expectCode(t, w, http.StatusOK, "list nodes")
	if !bytes.Contains(w.Body.Bytes(), []byte(`"parent_id":"q1"`)) {
		t.Fatalf("flat nodes: %s", w.Body.String())
	}

	// Viewers cannot write; readonly users get 403 from the middleware.
	expectCode(t, e.do(t, "POST", base+"/mindmap/nodes", e.reader, map[string]any{"parent_id": "q1", "text": "x"}), http.StatusForbidden, "viewer adds node")

	// Node REST: add, update, move, get, delete — each is a revision.
	w = e.do(t, "POST", base+"/mindmap/nodes", e.admin, map[string]any{"parent_id": "q2", "text": "Mobile", "priority": 2, "tags": []string{"ios"}})
	expectCode(t, w, http.StatusOK, "add node")
	var added struct {
		ID    string `json:"id"`
		RevNo int    `json:"rev_no"`
	}
	decode(t, w.Body, &added)
	if added.ID == "" || added.RevNo != 2 {
		t.Fatalf("add node: %+v", added)
	}
	w = e.do(t, "PUT", base+"/mindmap/nodes/"+added.ID, e.admin, map[string]any{"text": "Mobile app", "note": "ios first", "priority": nil, "base_rev": 2})
	expectCode(t, w, http.StatusOK, "update node")
	if !bytes.Contains(w.Body.Bytes(), []byte(`"text":"Mobile app"`)) || bytes.Contains(w.Body.Bytes(), []byte(`"priority"`)) {
		t.Fatalf("update node: %s", w.Body.String())
	}
	// Stale base_rev → 409 with the current revision.
	w = e.do(t, "PUT", base+"/mindmap/nodes/"+added.ID, e.admin, map[string]any{"text": "stale", "base_rev": 2})
	expectCode(t, w, http.StatusConflict, "stale base_rev")
	if !bytes.Contains(w.Body.Bytes(), []byte(`"current_rev":3`)) {
		t.Fatalf("conflict body: %s", w.Body.String())
	}
	idx := 0
	expectCode(t, e.do(t, "POST", base+"/mindmap/nodes/"+added.ID+"/move", e.admin, map[string]any{"parent_id": "q1", "index": idx}), http.StatusOK, "move node")
	w = e.do(t, "GET", base+"/mindmap/nodes/"+added.ID, e.admin, nil)
	expectCode(t, w, http.StatusOK, "get node")
	if !bytes.Contains(w.Body.Bytes(), []byte(`"parent_id":"q1"`)) {
		t.Fatalf("moved node: %s", w.Body.String())
	}
	expectCode(t, e.do(t, "POST", base+"/mindmap/nodes/"+added.ID+"/move", e.admin, map[string]any{"parent_id": added.ID}), http.StatusBadRequest, "move under itself")
	expectCode(t, e.do(t, "DELETE", base+"/mindmap/nodes/q2", e.admin, nil), http.StatusOK, "delete node")
	expectCode(t, e.do(t, "GET", base+"/mindmap/nodes/q2", e.admin, nil), http.StatusNotFound, "deleted node gone")
	expectCode(t, e.do(t, "DELETE", base+"/mindmap/nodes/"+doc.Root.ID, e.admin, nil), http.StatusBadRequest, "root undeletable")

	// Batch ops in one revision, including a relationship.
	w = e.do(t, "POST", base+"/mindmap/ops", e.admin, map[string]any{"ops": []any{
		map[string]any{"op": "add", "parent_id": "q1", "node": map[string]any{"text": "Docs"}},
		map[string]any{"op": "link_add", "link": map[string]any{"from": "api", "to": "q1", "label": "part of"}},
		map[string]any{"op": "settings", "layout": "right"},
	}, "comment": "batch"})
	expectCode(t, w, http.StatusOK, "ops batch")
	if !bytes.Contains(w.Body.Bytes(), []byte(`"layout":"right"`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"label":"part of"`)) {
		t.Fatalf("ops result: %s", w.Body.String())
	}
	expectCode(t, e.do(t, "POST", base+"/mindmap/ops", e.admin, map[string]any{"ops": []any{map[string]any{"op": "nope"}}}), http.StatusBadRequest, "unknown op")

	// Whole-document replace with autosave squashes consecutive revisions.
	w = e.do(t, "GET", base+"/revisions", e.admin, nil)
	var revs []struct {
		RevNo   int    `json:"rev_no"`
		Comment string `json:"comment"`
	}
	decode(t, w.Body, &revs)
	before := len(revs)
	cur := revs[0].RevNo
	for i := 0; i < 3; i++ {
		w = e.do(t, "PUT", base+"/mindmap", e.admin, map[string]any{
			"root":     map[string]any{"text": fmt.Sprintf("Roadmap v%d", i), "children": []any{map[string]any{"text": "only"}}},
			"base_rev": cur, "autosave": true,
		})
		expectCode(t, w, http.StatusOK, "autosave")
		var r struct {
			RevNo int `json:"rev_no"`
		}
		decode(t, w.Body, &r)
		if r.RevNo != cur+1 {
			t.Fatalf("autosave rev: got %d want %d", r.RevNo, cur+1)
		}
		cur = r.RevNo
	}
	w = e.do(t, "GET", base+"/revisions", e.admin, nil)
	decode(t, w.Body, &revs)
	if len(revs) != before+1 || revs[0].RevNo != cur {
		t.Fatalf("autosave squash: %d revisions (was %d), head %+v", len(revs), before, revs[0])
	}
	// The "batch" revision survived the squash (it had a comment).
	found := false
	for _, r := range revs {
		if r.Comment == "batch" {
			found = true
		}
	}
	if !found {
		t.Fatalf("commented revision lost: %+v", revs)
	}

	// Page PUT with a document + title also works, and the root text follows the title in the outline.
	w = e.do(t, "PUT", base, e.admin, map[string]any{"title": "Roadmap 2026", "mindmap": map[string]any{"root": map[string]any{"text": "Roadmap 2026"}}, "base_rev": cur})
	expectCode(t, w, http.StatusOK, "page put with document")
	w = e.do(t, "GET", base, e.admin, nil)
	decode(t, w.Body, &page)
	if !strings.HasPrefix(page.ContentMd, "# Roadmap 2026") {
		t.Fatalf("outline after page put: %q", page.ContentMd)
	}

	// Restore an older revision brings the document back.
	w = e.do(t, "GET", base+"/revisions", e.admin, nil)
	var all []struct {
		ID    int64 `json:"id"`
		RevNo int   `json:"rev_no"`
	}
	decode(t, w.Body, &all)
	oldest := all[len(all)-1]
	w = e.do(t, "GET", fmt.Sprintf("/api/wiki/revisions/%d", oldest.ID), e.admin, nil)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"kind":"mindmap"`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"id":"q1"`)) {
		t.Fatalf("revision body: %s", w.Body.String())
	}
	expectCode(t, e.do(t, "POST", base+"/restore", e.admin, map[string]any{"revision_id": oldest.ID}), http.StatusOK, "restore")
	w = e.do(t, "GET", base+"/mindmap", e.admin, nil)
	decode(t, w.Body, &doc)
	if doc.NodeCount != 4 || doc.Root.Children[0].ID != "q1" {
		t.Fatalf("restored document: %+v", doc)
	}

	// Regular pages reject the mind-map routes.
	w = e.do(t, "POST", "/api/wiki/pages", e.admin, map[string]any{"space_id": space, "title": "Article", "content_md": "hi"})
	expectCode(t, w, http.StatusOK, "create article")
	var art struct {
		ID int64 `json:"id"`
	}
	decode(t, w.Body, &art)
	expectCode(t, e.do(t, "GET", fmt.Sprintf("/api/wiki/pages/%d/mindmap", art.ID), e.admin, nil), http.StatusBadRequest, "mindmap route on article")
	expectCode(t, e.do(t, "POST", "/api/wiki/pages", e.admin, map[string]any{"space_id": space, "title": "Bad", "kind": "chart"}), http.StatusBadRequest, "unknown kind")
}

func TestMindmapImportExport(t *testing.T) {
	e, space := wikiEnv(t)

	// Create a page from a Markdown outline that links to an existing page.
	w := e.do(t, "POST", "/api/wiki/pages", e.admin, map[string]any{"space_id": space, "title": "Target", "content_md": "x"})
	expectCode(t, w, http.StatusOK, "create target page")
	md := "# Plan\n- Phase 1 (P1) #core\n  - [Spec](https://spec) [[Target]]\n    > note here\n- [x] Phase 2\n"
	w = e.do(t, "POST", "/api/wiki/mindmaps/import", e.admin, map[string]any{"space_id": space, "filename": "plan.md", "data": md})
	expectCode(t, w, http.StatusOK, "import md")
	var imp struct {
		ID    int64 `json:"id"`
		Nodes int   `json:"nodes"`
	}
	decode(t, w.Body, &imp)
	if imp.Nodes != 4 {
		t.Fatalf("import: %+v %s", imp, w.Body.String())
	}
	base := fmt.Sprintf("/api/wiki/pages/%d", imp.ID)
	w = e.do(t, "GET", base, e.admin, nil)
	var page struct {
		Title     string           `json:"title"`
		Kind      string           `json:"kind"`
		ContentMd string           `json:"content_md"`
		Links     map[string]int64 `json:"links"`
		Mindmap   json.RawMessage  `json:"mindmap"`
	}
	decode(t, w.Body, &page)
	if page.Title != "Plan" || page.Kind != "mindmap" || !strings.Contains(page.ContentMd, "[[Target]]") || page.Links["Target"] == 0 {
		t.Fatalf("imported page: %+v", page)
	}
	if !bytes.Contains(page.Mindmap, []byte(`"page_id":`)) || !bytes.Contains(page.Mindmap, []byte(`"priority":1`)) {
		t.Fatalf("imported doc lacks page link / priority: %s", page.Mindmap)
	}
	// Backlinks from the mind map show on the target page.
	w = e.do(t, "GET", "/api/wiki/search?q=Spec", e.admin, nil)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"title":"Plan"`)) {
		t.Fatalf("search does not find node text: %s", w.Body.String())
	}

	// Every export format downloads; xmind is a zip; md round-trips.
	for format, want := range map[string]string{
		"json": `"text": "Phase 1"`, "md": "- [x] Phase 2", "txt": "\tPhase 1", "opml": `<outline text="Phase 1"`,
		"mm": `TEXT="Phase 1"`, "mermaid": "root((Plan))", "xmind": "PK",
	} {
		w = e.do(t, "GET", base+"/mindmap/export?format="+format, e.reader, nil)
		expectCode(t, w, http.StatusOK, "export "+format)
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("export %s lacks %q: %.200s", format, want, w.Body.String())
		}
		if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "Plan.") {
			t.Errorf("export %s disposition: %q", format, cd)
		}
	}
	expectCode(t, e.do(t, "GET", base+"/mindmap/export?format=pdf", e.admin, nil), http.StatusBadRequest, "unknown export format")

	// Import into the existing page: append a FreeMind map, then replace with xmind (base64).
	fm := `<map version="1.0.1"><node TEXT="Imported"><node TEXT="A" POSITION="right"/><node TEXT="B"/></node></map>`
	w = e.do(t, "POST", base+"/mindmap/import", e.admin, map[string]any{"format": "mm", "data": fm, "mode": "append"})
	expectCode(t, w, http.StatusOK, "append freemind")
	if !bytes.Contains(w.Body.Bytes(), []byte(`"nodes":7`)) {
		t.Fatalf("append result: %s", w.Body.String())
	}
	w = e.do(t, "GET", base+"/mindmap/export?format=xmind", e.admin, nil)
	xm := base64.StdEncoding.EncodeToString(w.Body.Bytes())
	w = e.do(t, "POST", base+"/mindmap/import", e.admin, map[string]any{"filename": "plan.xmind", "data_base64": xm, "mode": "replace"})
	expectCode(t, w, http.StatusOK, "replace with xmind")
	if !bytes.Contains(w.Body.Bytes(), []byte(`"nodes":7`)) {
		t.Fatalf("xmind replace result: %s", w.Body.String())
	}
	expectCode(t, e.do(t, "POST", base+"/mindmap/import", e.admin, map[string]any{"format": "mm", "data": "<nope>", "mode": "replace"}), http.StatusBadRequest, "bad import data")
	expectCode(t, e.do(t, "POST", base+"/mindmap/import", e.reader, map[string]any{"format": "mm", "data": fm}), http.StatusForbidden, "viewer import")

	// The space export zip carries the outline and the JSON document.
	w = e.do(t, "GET", fmt.Sprintf("/api/wiki/spaces/%d/export", space), e.admin, nil)
	expectCode(t, w, http.StatusOK, "space export")
	if !bytes.Contains(w.Body.Bytes(), []byte("Plan.mindmap.json")) {
		t.Fatalf("space export lacks mindmap json")
	}
}
