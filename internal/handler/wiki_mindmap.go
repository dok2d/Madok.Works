package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"kanban/internal/mindmap"
	"kanban/internal/model"
)

// Mind-map pages: a wiki page whose kind is "mindmap". The canonical document
// is a JSON tree (internal/mindmap); the page's content/content_md hold a
// derived outline so search, history diffs, LLM summaries, [[links]] and the
// Markdown export keep working unchanged.
//
// Routes (all under the page's space ACL, editor role to write):
//
//	GET    /api/wiki/pages/{id}/mindmap                 document + rev_no
//	PUT    /api/wiki/pages/{id}/mindmap                 replace document {…doc, base_rev, comment, autosave}
//	POST   /api/wiki/pages/{id}/mindmap/ops             batch of node operations {ops:[…], base_rev, comment}
//	GET    /api/wiki/pages/{id}/mindmap/nodes           flat node list
//	POST   /api/wiki/pages/{id}/mindmap/nodes           add node {parent_id, index, …fields}
//	GET    /api/wiki/pages/{id}/mindmap/nodes/{nid}     one node (with subtree)
//	PUT    /api/wiki/pages/{id}/mindmap/nodes/{nid}     update fields (partial)
//	DELETE /api/wiki/pages/{id}/mindmap/nodes/{nid}     delete subtree
//	POST   /api/wiki/pages/{id}/mindmap/nodes/{nid}/move  {parent_id, index, side}
//	GET    /api/wiki/pages/{id}/mindmap/export?format=  json|md|txt|opml|mm|xmind|mermaid (download)
//	POST   /api/wiki/pages/{id}/mindmap/import          {format, data|data_base64, filename, mode: replace|append}
//	POST   /api/wiki/mindmaps/import                    create a page from a file {space_id, parent_id, title, format, data…}
//	GET    /api/wiki/events                             SSE: {"type":"wiki_page","page_id":…,"rev_no":…}

// mindmapTitles resolves page ids to titles for the derived outline.
func (h *Handler) mindmapTitles() mindmap.TitleResolver {
	return func(id int64) string { return h.store.WikiPageTitle(id) }
}

// mindmapDerive renders the stored forms of a document: sanitized HTML outline,
// markdown outline and the canonical JSON.
func (h *Handler) mindmapDerive(doc *mindmap.Doc) (content, contentMd, data string) {
	titles := h.mindmapTitles()
	return sanitizeHTML(doc.HTML(titles)), doc.Markdown(titles), string(doc.Marshal())
}

// mindmapOf parses a page's stored document (an empty page yields a fresh map
// titled like the page).
func mindmapOf(page *model.WikiPage) (*mindmap.Doc, error) {
	if strings.TrimSpace(page.Data) == "" {
		return mindmap.New(page.Title), nil
	}
	return mindmap.Parse([]byte(page.Data))
}

// attachMindmap fills page.Mindmap for API responses of mind-map pages.
func attachMindmap(page *model.WikiPage) {
	if page.Kind != model.WikiKindMindmap {
		return
	}
	if doc, err := mindmapOf(page); err == nil {
		page.Mindmap = doc.Marshal()
	}
}

type sseWikiEvent struct {
	Type   string `json:"type"`
	PageID int64  `json:"page_id"`
	RevNo  int    `json:"rev_no"`
	UserID int64  `json:"user_id"`
	Kind   string `json:"kind"`
}

// notifyWikiPage tells live clients a page changed (they refetch if they show it).
func (h *Handler) notifyWikiPage(page *model.WikiPage, rev int, userID int64) {
	h.sse.sendTo(nil, sseWikiEvent{Type: "wiki_page", PageID: page.ID, RevNo: rev, UserID: userID, Kind: page.Kind})
}

// handleWikiEvents serves GET /api/wiki/events (SSE).
func (h *Handler) handleWikiEvents(w http.ResponseWriter, r *http.Request) {
	user := h.wikiUser(w, r)
	if user == nil {
		return
	}
	h.serveSSE(w, r, user.ID)
}

// saveMindmap stores a new revision of a mind-map page and notifies clients.
func (h *Handler) saveMindmap(w http.ResponseWriter, page *model.WikiPage, doc *mindmap.Doc, title, comment string, user *model.User, baseRev int, squash bool) (int, bool) {
	if title == "" {
		title = page.Title
	}
	content, contentMd, data := h.mindmapDerive(doc)
	rev, err := h.store.UpdateWikiPageFull(page.ID, title, content, contentMd, data, comment, user.ID, baseRev, squash)
	if err != nil {
		if strings.HasPrefix(err.Error(), "conflict:") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "current_rev": rev})
			return rev, false
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return 0, false
	}
	page.RevNo, page.Title, page.Data = rev, title, data
	h.processWikiPageLinks(page.SpaceID, page.ID, contentMd, title, user)
	if !squash {
		h.store.LogActivity(user.ID, "wiki_edit_page", nil, title)
	}
	h.notifyWikiPage(page, rev, user.ID)
	return rev, true
}

// handleWikiMindmap dispatches /api/wiki/pages/{id}/mindmap[/…] (sub is the
// part after "mindmap", without the leading slash).
func (h *Handler) handleWikiMindmap(w http.ResponseWriter, r *http.Request, user *model.User, page *model.WikiPage, role, sub string) {
	if page.Kind != model.WikiKindMindmap {
		http.Error(w, "not a mind map page", http.StatusBadRequest)
		return
	}
	canEdit := wikiRoleAtLeast(role, "editor")
	doc, err := mindmapOf(page)
	if err != nil {
		http.Error(w, "stored mind map is corrupt: "+err.Error(), http.StatusInternalServerError)
		return
	}
	parts := strings.Split(strings.Trim(sub, "/"), "/")
	switch {
	case sub == "":
		switch r.Method {
		case http.MethodGet:
			h.writeMindmap(w, page, doc)
		case http.MethodPut:
			if !canEdit {
				http.Error(w, "no edit access", http.StatusForbidden)
				return
			}
			var req struct {
				mindmap.Doc
				Title    string `json:"title"`
				BaseRev  int    `json:"base_rev"`
				Comment  string `json:"comment"`
				Autosave bool   `json:"autosave"`
			}
			if err := decodeBody(r, &req, mindmap.MaxDocBytes); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			nd := req.Doc
			if err := nd.Normalize(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			title := strings.TrimSpace(req.Title)
			if len(title) > wikiMaxTitleLen {
				http.Error(w, "title too long", http.StatusBadRequest)
				return
			}
			if rev, ok := h.saveMindmap(w, page, &nd, title, strings.TrimSpace(req.Comment), user, req.BaseRev, req.Autosave); ok {
				jsonResp(w, map[string]any{"status": "ok", "rev_no": rev})
			}
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case parts[0] == "ops":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !canEdit {
			http.Error(w, "no edit access", http.StatusForbidden)
			return
		}
		var req struct {
			Ops     []mindmap.Op `json:"ops"`
			BaseRev int          `json:"base_rev"`
			Comment string       `json:"comment"`
		}
		if err := decodeBody(r, &req, mindmap.MaxDocBytes); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		results, err := doc.Apply(req.Ops)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if rev, ok := h.saveMindmap(w, page, doc, "", strings.TrimSpace(req.Comment), user, req.BaseRev, false); ok {
			jsonResp(w, map[string]any{"status": "ok", "rev_no": rev, "results": results, "mindmap": json.RawMessage(doc.Marshal())})
		}
	case parts[0] == "nodes":
		h.handleWikiMindmapNodes(w, r, user, page, doc, canEdit, parts[1:])
	case parts[0] == "export":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
		if format == "" {
			format = mindmap.FormatJSON
		}
		meta, ok := mindmap.ExportFormats[format]
		if !ok {
			http.Error(w, "unknown format", http.StatusBadRequest)
			return
		}
		out, err := doc.Export(format, h.mindmapTitles())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", meta.Mime)
		if r.URL.Query().Get("inline") == "" {
			w.Header().Set("Content-Disposition", "attachment; filename=\""+wikiSafeFilename(page.Title)+meta.Ext+"\"")
		}
		w.Write(out)
	case parts[0] == "import":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !canEdit {
			http.Error(w, "no edit access", http.StatusForbidden)
			return
		}
		req, err := decodeMindmapImport(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		imported, err := mindmap.Import(req.Format, req.Filename, req.bytes, h.mindmapImportOptions(page.SpaceID, page.Title))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch req.Mode {
		case "", "replace":
			doc = imported
		case "append":
			if doc.Count()+imported.Count() > mindmap.MaxNodes {
				http.Error(w, fmt.Sprintf("too many nodes (max %d)", mindmap.MaxNodes), http.StatusBadRequest)
				return
			}
			doc.Root.Children = append(doc.Root.Children, imported.Root)
			imported.Root.Side = ""
			doc.Links = append(doc.Links, imported.Links...)
			if err := doc.Normalize(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		default:
			http.Error(w, "mode must be replace or append", http.StatusBadRequest)
			return
		}
		comment := "import " + strings.TrimSpace(req.Format+" "+req.Filename)
		if rev, ok := h.saveMindmap(w, page, doc, "", comment, user, req.BaseRev, false); ok {
			jsonResp(w, map[string]any{"status": "ok", "rev_no": rev, "nodes": doc.Count(), "mindmap": json.RawMessage(doc.Marshal())})
		}
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) writeMindmap(w http.ResponseWriter, page *model.WikiPage, doc *mindmap.Doc) {
	jsonResp(w, map[string]any{
		"page_id": page.ID, "space_id": page.SpaceID, "title": page.Title, "rev_no": page.RevNo,
		"updated_at": page.UpdatedAt, "node_count": doc.Count(),
		"version": doc.Version, "layout": doc.Layout, "line": doc.Line, "palette": doc.Palette,
		"root": doc.Root, "links": doc.Links,
	})
}

// handleWikiMindmapNodes serves the per-node REST surface; every write is a
// single-op batch so it shares validation and revision logic with /ops.
func (h *Handler) handleWikiMindmapNodes(w http.ResponseWriter, r *http.Request, user *model.User, page *model.WikiPage, doc *mindmap.Doc, canEdit bool, rest []string) {
	write := func(ops []mindmap.Op, baseRev int, comment string, extra func(res []mindmap.OpResult) map[string]any) {
		if !canEdit {
			http.Error(w, "no edit access", http.StatusForbidden)
			return
		}
		results, err := doc.Apply(ops)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rev, ok := h.saveMindmap(w, page, doc, "", comment, user, baseRev, false)
		if !ok {
			return
		}
		out := map[string]any{"status": "ok", "rev_no": rev}
		for k, v := range extra(results) {
			out[k] = v
		}
		jsonResp(w, out)
	}
	if len(rest) == 0 || rest[0] == "" {
		switch r.Method {
		case http.MethodGet:
			jsonResp(w, map[string]any{"rev_no": page.RevNo, "nodes": doc.Flatten()})
		case http.MethodPost:
			var req struct {
				mindmap.Node
				ParentID string `json:"parent_id"`
				Index    *int   `json:"index"`
				BaseRev  int    `json:"base_rev"`
				Comment  string `json:"comment"`
			}
			if err := decodeBody(r, &req, mindmap.MaxDocBytes); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			n := req.Node
			write([]mindmap.Op{{Op: "add", ParentID: req.ParentID, Index: req.Index, Node: &n}}, req.BaseRev, req.Comment,
				func(res []mindmap.OpResult) map[string]any {
					added, _ := doc.Find(res[0].ID)
					return map[string]any{"id": res[0].ID, "node": added}
				})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	nid := rest[0]
	node, parent := doc.Find(nid)
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	if len(rest) >= 2 && rest[1] == "move" {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ParentID string `json:"parent_id"`
			Index    *int   `json:"index"`
			Side     string `json:"side"`
			BaseRev  int    `json:"base_rev"`
			Comment  string `json:"comment"`
		}
		if err := decodeBody(r, &req, 1<<16); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		write([]mindmap.Op{{Op: "move", ID: nid, ParentID: req.ParentID, Index: req.Index, Side: req.Side}}, req.BaseRev, req.Comment,
			func([]mindmap.OpResult) map[string]any { return map[string]any{"id": nid} })
		return
	}
	if len(rest) >= 2 {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		parentID := ""
		if parent != nil {
			parentID = parent.ID
		}
		jsonResp(w, map[string]any{"rev_no": page.RevNo, "parent_id": parentID, "node": node})
	case http.MethodPut, http.MethodPatch:
		var raw map[string]json.RawMessage
		if err := decodeBody(r, &raw, mindmap.MaxDocBytes); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		baseRev, comment := 0, ""
		if v, ok := raw["base_rev"]; ok {
			json.Unmarshal(v, &baseRev)
			delete(raw, "base_rev")
		}
		if v, ok := raw["comment"]; ok {
			json.Unmarshal(v, &comment)
			delete(raw, "comment")
		}
		set, _ := json.Marshal(raw)
		write([]mindmap.Op{{Op: "update", ID: nid, Set: set}}, baseRev, comment,
			func([]mindmap.OpResult) map[string]any {
				upd, _ := doc.Find(nid)
				return map[string]any{"id": nid, "node": upd}
			})
	case http.MethodDelete:
		var req struct {
			BaseRev int    `json:"base_rev"`
			Comment string `json:"comment"`
		}
		decodeBody(r, &req, 1<<16) // body optional
		write([]mindmap.Op{{Op: "delete", ID: nid}}, req.BaseRev, req.Comment,
			func([]mindmap.OpResult) map[string]any { return map[string]any{"id": nid} })
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// mindmapImportReq is the body of the import endpoints: text formats come as
// `data`, binary (xmind) as `data_base64`.
type mindmapImportReq struct {
	Format     string `json:"format"`
	Filename   string `json:"filename"`
	Data       string `json:"data"`
	DataBase64 string `json:"data_base64"`
	Mode       string `json:"mode"`
	BaseRev    int    `json:"base_rev"`
	// create-page variant
	SpaceID  int64  `json:"space_id"`
	ParentID *int64 `json:"parent_id"`
	Title    string `json:"title"`
	bytes    []byte
}

func decodeMindmapImport(r *http.Request) (*mindmapImportReq, error) {
	var req mindmapImportReq
	if err := decodeBody(r, &req, mindmap.MaxDocBytes*6); err != nil {
		return nil, err
	}
	req.Format = strings.ToLower(strings.TrimSpace(req.Format))
	req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))
	switch {
	case req.DataBase64 != "":
		if i := strings.Index(req.DataBase64, ","); strings.HasPrefix(req.DataBase64, "data:") && i > 0 {
			req.DataBase64 = req.DataBase64[i+1:]
		}
		b, err := base64.StdEncoding.DecodeString(req.DataBase64)
		if err != nil {
			return nil, errors.New("bad data_base64")
		}
		req.bytes = b
	case req.Data != "":
		req.bytes = []byte(req.Data)
	default:
		return nil, errors.New("data required")
	}
	if len(req.bytes) > mindmap.MaxDocBytes*4 {
		return nil, errors.New("file too large")
	}
	return &req, nil
}

func (h *Handler) mindmapImportOptions(spaceID int64, title string) mindmap.ImportOptions {
	return mindmap.ImportOptions{
		Title:       title,
		ResolvePage: func(t string) int64 { return h.store.WikiPageIDByTitle(spaceID, t) },
	}
}

// handleWikiMindmapImport serves POST /api/wiki/mindmaps/import: creates a new
// mind-map page in a space from an uploaded file.
func (h *Handler) handleWikiMindmapImport(w http.ResponseWriter, r *http.Request) {
	user := h.wikiUser(w, r)
	if user == nil {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req, err := decodeMindmapImport(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	role, _, err := h.wikiRole(user, req.SpaceID)
	if err != nil || !wikiRoleAtLeast(role, "editor") {
		http.Error(w, "no edit access", http.StatusForbidden)
		return
	}
	if req.ParentID != nil {
		parent, err := h.store.GetWikiPage(*req.ParentID)
		if err != nil || parent.SpaceID != req.SpaceID {
			http.Error(w, "parent page not found in space", http.StatusBadRequest)
			return
		}
	}
	title := strings.TrimSpace(req.Title)
	doc, err := mindmap.Import(req.Format, req.Filename, req.bytes, h.mindmapImportOptions(req.SpaceID, title))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if title == "" {
		title = strings.TrimSpace(strings.SplitN(doc.Root.Text, "\n", 2)[0])
	}
	if title == "" {
		title = "Mind map"
	}
	if len(title) > wikiMaxTitleLen {
		http.Error(w, "title too long", http.StatusBadRequest)
		return
	}
	content, contentMd, data := h.mindmapDerive(doc)
	id, err := h.store.CreateWikiPageFull(req.SpaceID, req.ParentID, title, content, contentMd, model.WikiKindMindmap, data, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.store.LogActivity(user.ID, "wiki_create_page", nil, title)
	h.processWikiPageLinks(req.SpaceID, id, contentMd, title, user)
	jsonResp(w, map[string]any{"id": id, "nodes": doc.Count(), "format": mindmap.DetectFormat(req.Filename, req.bytes)})
}

// decodeBody decodes a JSON body with a size cap.
func decodeBody(r *http.Request, v any, limit int64) error {
	if r.Body == nil {
		return errors.New("bad request")
	}
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, limit))
	if err := dec.Decode(v); err != nil {
		return errors.New("bad request: " + err.Error())
	}
	return nil
}
