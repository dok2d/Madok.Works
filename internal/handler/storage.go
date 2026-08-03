package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"kanban/internal/db"
	"kanban/internal/model"
)

// === "Files" service: shared folder tree over blobs in the files table ===

// storageUser gates every authenticated storage endpoint: 404 while the service
// is disabled (mirrors mailUser), 401 without a session. Write access is
// already limited to non-readonly users by the global ServeHTTP check.
func (h *Handler) storageUser(w http.ResponseWriter, r *http.Request) *model.User {
	if !h.serviceEnabled("files") {
		http.Error(w, "files service disabled", http.StatusNotFound)
		return nil
	}
	user := h.currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}
	return user
}

// storageErr maps store sentinel errors to HTTP statuses (409 for conflicts).
func storageErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrStorageNameTaken):
		http.Error(w, "name already taken", http.StatusConflict)
	case errors.Is(err, db.ErrStorageCycle):
		http.Error(w, "cannot move a folder into itself", http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// storageChanged notifies open Files views (broadcast; the view refetches).
func (h *Handler) storageChanged() {
	h.sse.sendTo(nil, map[string]string{"type": "storage"})
}

// optFolderID reads an optional folder id (nil = root) from a query/body value.
func optFolderID(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}

// handleStorageTree serves GET /api/storage/tree — the whole folder tree.
func (h *Handler) handleStorageTree(w http.ResponseWriter, r *http.Request) {
	if h.storageUser(w, r) == nil {
		return
	}
	folders, err := h.store.ListStorageFolders()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, folders)
}

// handleStorageList serves GET /api/storage/list?folder=N (0/empty = root).
func (h *Handler) handleStorageList(w http.ResponseWriter, r *http.Request) {
	if h.storageUser(w, r) == nil {
		return
	}
	fid, _ := strconv.ParseInt(r.URL.Query().Get("folder"), 10, 64)
	folder := optFolderID(fid)
	if folder != nil {
		if _, err := h.store.GetStorageFolder(*folder); err != nil {
			http.Error(w, "folder not found", http.StatusNotFound)
			return
		}
	}
	all, err := h.store.ListStorageFolders()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sub := []model.StorageFolder{}
	for _, f := range all {
		if (folder == nil && f.ParentID == nil) || (folder != nil && f.ParentID != nil && *f.ParentID == *folder) {
			sub = append(sub, f)
		}
	}
	items, err := h.store.ListStorageItems(folder)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]any{"folders": sub, "items": items})
}

// cleanStorageName keeps the user-facing name Unicode but strips path
// separators and control characters. Returns "" when nothing displayable is left.
func cleanStorageName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "." || name == ".." || len(name) > 255 {
		return ""
	}
	return name
}

// handleStorageUpload serves POST /api/storage/upload. Same base64-JSON
// convention and upload policy as the shared /api/files endpoint, plus a
// folder_id and a Unicode display name kept in storage_items.
func (h *Handler) handleStorageUpload(w http.ResponseWriter, r *http.Request) {
	user := h.storageUser(w, r)
	if user == nil {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	var req struct {
		Data     string `json:"data"`
		Filename string `json:"filename"`
		Mime     string `json:"mime"`
		FolderID int64  `json:"folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request or body too large", http.StatusBadRequest)
		return
	}
	name := cleanStorageName(req.Filename)
	if req.Data == "" || name == "" {
		http.Error(w, "data and filename required", http.StatusBadRequest)
		return
	}
	if req.Mime == "" {
		req.Mime = "application/octet-stream"
	}
	if ok, msg := h.fileTypeAllowed(name, req.Mime); !ok {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	raw, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		http.Error(w, "invalid base64", http.StatusBadRequest)
		return
	}
	if len(raw) > maxUploadSize {
		http.Error(w, fmt.Sprintf("file too large (max %dMB)", maxUploadSize/(1024*1024)), http.StatusRequestEntityTooLarge)
		return
	}
	if strings.HasPrefix(req.Mime, "image/") && !strings.HasPrefix(http.DetectContentType(raw), "image/") {
		http.Error(w, "file content does not match declared MIME type", http.StatusBadRequest)
		return
	}
	folder := optFolderID(req.FolderID)
	if folder != nil {
		if _, err := h.store.GetStorageFolder(*folder); err != nil {
			http.Error(w, "folder not found", http.StatusNotFound)
			return
		}
	}
	// files.filename keeps the ASCII-safe form (used as a download fallback).
	safeName := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, filepath.Base(name))
	fileID, err := h.store.SaveFile(safeName, raw, req.Mime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id, finalName, err := h.store.CreateStorageItem(folder, fileID, name, user.ID)
	if err != nil {
		h.store.DeleteFile(fileID)
		storageErr(w, err)
		return
	}
	log.Printf("[storage] %s uploaded %q (%d bytes)", user.Username, finalName, len(raw))
	h.storageChanged()
	jsonResp(w, map[string]any{"id": id, "name": finalName})
}

// handleStorageUsage serves GET /api/storage/usage — totals + free disk space.
func (h *Handler) handleStorageUsage(w http.ResponseWriter, r *http.Request) {
	if h.storageUser(w, r) == nil {
		return
	}
	count, total := h.store.StorageUsage()
	free, diskTotal := diskUsage(h.dataDir)
	jsonResp(w, map[string]any{"count": count, "size": total, "disk_free": free, "disk_total": diskTotal})
}

// handleStorageStream serves GET /api/storage/events (SSE refresh pings).
func (h *Handler) handleStorageStream(w http.ResponseWriter, r *http.Request) {
	user := h.storageUser(w, r)
	if user == nil {
		return
	}
	h.serveSSE(w, r, user.ID)
}

// handleStorageFolders serves POST /api/storage/folders {name, parent_id}.
func (h *Handler) handleStorageFolders(w http.ResponseWriter, r *http.Request) {
	user := h.storageUser(w, r)
	if user == nil {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name     string `json:"name"`
		ParentID int64  `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := cleanStorageName(req.Name)
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	parent := optFolderID(req.ParentID)
	if parent != nil {
		if _, err := h.store.GetStorageFolder(*parent); err != nil {
			http.Error(w, "parent not found", http.StatusNotFound)
			return
		}
	}
	id, err := h.store.CreateStorageFolder(name, parent, user.ID)
	if err != nil {
		storageErr(w, err)
		return
	}
	h.storageChanged()
	jsonResp(w, map[string]any{"id": id, "name": name})
}

// storageSubroute splits "/api/storage/<kind>/{id}[/action]".
func storageSubroute(path, prefix string) (int64, string) {
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	id, _ := strconv.ParseInt(parts[0], 10, 64)
	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}
	return id, action
}

// handleStorageFolder serves /api/storage/folders/{id}[/rename|/move].
func (h *Handler) handleStorageFolder(w http.ResponseWriter, r *http.Request) {
	if h.storageUser(w, r) == nil {
		return
	}
	id, action := storageSubroute(r.URL.Path, "/api/storage/folders/")
	if id == 0 {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if _, err := h.store.GetStorageFolder(id); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	switch {
	case r.Method == http.MethodDelete && action == "":
		if _, err := h.store.DeleteStorageFolder(id); err != nil {
			storageErr(w, err)
			return
		}
	case r.Method == http.MethodPost && action == "rename":
		var req struct {
			Name string `json:"name"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		name := cleanStorageName(req.Name)
		if name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if err := h.store.RenameStorageFolder(id, name); err != nil {
			storageErr(w, err)
			return
		}
	case r.Method == http.MethodPost && action == "move":
		var req struct {
			FolderID int64 `json:"folder_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		parent := optFolderID(req.FolderID)
		if parent != nil {
			if _, err := h.store.GetStorageFolder(*parent); err != nil {
				http.Error(w, "target folder not found", http.StatusNotFound)
				return
			}
		}
		if err := h.store.MoveStorageFolder(id, parent); err != nil {
			storageErr(w, err)
			return
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.storageChanged()
	jsonResp(w, map[string]string{"status": "ok"})
}

// handleStorageItem serves /api/storage/items/{id}[/download|/preview|/rename|/move|/share].
func (h *Handler) handleStorageItem(w http.ResponseWriter, r *http.Request) {
	if h.storageUser(w, r) == nil {
		return
	}
	id, action := storageSubroute(r.URL.Path, "/api/storage/items/")
	if id == 0 {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	it, err := h.store.GetStorageItem(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	switch {
	case r.Method == http.MethodGet && action == "download":
		h.serveStorageBlob(w, it, false)
		return
	case r.Method == http.MethodGet && action == "preview":
		h.serveStorageBlob(w, it, true)
		return
	case r.Method == http.MethodDelete && action == "":
		if err := h.store.DeleteStorageItem(id); err != nil {
			storageErr(w, err)
			return
		}
	case r.Method == http.MethodPost && action == "rename":
		var req struct {
			Name string `json:"name"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		name := cleanStorageName(req.Name)
		if name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if err := h.store.RenameStorageItem(id, name); err != nil {
			storageErr(w, err)
			return
		}
	case r.Method == http.MethodPost && action == "move":
		var req struct {
			FolderID int64 `json:"folder_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		folder := optFolderID(req.FolderID)
		if folder != nil {
			if _, err := h.store.GetStorageFolder(*folder); err != nil {
				http.Error(w, "target folder not found", http.StatusNotFound)
				return
			}
		}
		if err := h.store.MoveStorageItem(id, folder); err != nil {
			storageErr(w, err)
			return
		}
	case r.Method == http.MethodPost && action == "share":
		buf := make([]byte, 32)
		rand.Read(buf)
		token := base64.RawURLEncoding.EncodeToString(buf)
		if err := h.store.SetStorageItemToken(id, token); err != nil {
			storageErr(w, err)
			return
		}
		jsonResp(w, map[string]string{"token": token, "url": "/api/storage/public/" + token})
		return
	case r.Method == http.MethodDelete && action == "share":
		if err := h.store.SetStorageItemToken(id, ""); err != nil {
			storageErr(w, err)
			return
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.storageChanged()
	jsonResp(w, map[string]string{"status": "ok"})
}

// previewMime returns the safe inline Content-Type for a stored MIME, or false.
// HTML and SVG are never served inline (stored XSS via upload_allowed=*).
func previewMime(m string) (string, bool) {
	base := m
	if i := strings.Index(base, ";"); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}
	switch base {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return base, true
	case "text/plain", "text/csv", "text/markdown", "application/json":
		return "text/plain; charset=utf-8", true
	}
	return "", false
}

// contentDisposition builds an RFC 5987 header so Unicode display names survive
// (the plain filename= keeps an ASCII fallback for ancient clients).
func contentDisposition(kind, name, asciiFallback string) string {
	return fmt.Sprintf("%s; filename=%q; filename*=UTF-8''%s", kind, asciiFallback, url.PathEscape(name))
}

// serveStorageBlob writes an item's blob. Preview serves whitelisted types
// inline; everything else (and all downloads) goes out as an attachment.
func (h *Handler) serveStorageBlob(w http.ResponseWriter, it *model.StorageItem, preview bool) {
	data, mime, fallback, err := h.store.GetFile(it.FileID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	kind := "attachment"
	if preview {
		if safe, ok := previewMime(mime); ok {
			mime = safe
			kind = "inline"
		}
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", contentDisposition(kind, it.Name, fallback))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

// handleStoragePublic serves GET /api/storage/public/{token} WITHOUT auth (the
// path is exempted in ServeHTTP). That exemption also skips the service gate,
// so the disabled-service check lives here: links go dark with the service.
func (h *Handler) handleStoragePublic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.serviceEnabled("files") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	token := strings.TrimPrefix(r.URL.Path, "/api/storage/public/")
	it, err := h.store.GetStorageItemByToken(token)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	h.serveStorageBlob(w, it, false)
}
