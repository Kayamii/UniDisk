package api

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/unidisk/unidisk/internal/pool"
	"github.com/unidisk/unidisk/internal/store"
)

// commonMimeTypes covers frequent extensions so detection doesn't depend on
// the OS mime database (the slim runtime image lacks entries for some, e.g.
// .txt). mime.TypeByExtension is consulted first; this is the fallback.
var commonMimeTypes = map[string]string{
	".txt": "text/plain", ".md": "text/markdown", ".csv": "text/csv",
	".log": "text/plain", ".html": "text/html", ".css": "text/css",
	".js": "text/javascript", ".json": "application/json", ".xml": "application/xml",
	".pdf": "application/pdf", ".zip": "application/zip", ".gz": "application/gzip",
	".tar": "application/x-tar", ".png": "image/png", ".jpg": "image/jpeg",
	".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
	".svg": "image/svg+xml", ".mp4": "video/mp4", ".webm": "video/webm",
	".mp3": "audio/mpeg", ".wav": "audio/wav", ".doc": "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls": "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
}

// mimeByExtension returns the MIME type for a filename's extension, or "" if
// unknown. The charset suffix (e.g. "; charset=utf-8") is stripped.
func mimeByExtension(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return ""
	}
	if t := mime.TypeByExtension(ext); t != "" {
		if i := strings.IndexByte(t, ';'); i >= 0 {
			t = strings.TrimSpace(t[:i])
		}
		return t
	}
	return commonMimeTypes[ext] // "" if not found
}

// pathID reads the {id} path value (Go 1.22 routing) and parses it as an int.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id in path")
		return 0, false
	}
	return id, true
}

// parseParentID reads an optional ?parent= query param. Absent/empty = root.
func parseParentID(r *http.Request) (*int64, bool) {
	raw := r.URL.Query().Get("parent")
	if raw == "" {
		return nil, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, false
	}
	return &id, true
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	parent, ok := parseParentID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid parent id")
		return
	}
	files, err := s.store.ListChildren(r.Context(), parent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list files")
		return
	}
	if files == nil {
		files = []*store.File{}
	}
	writeJSON(w, http.StatusOK, files)
}

// handleSearchFiles finds files across the pool by name (?q=...).
func (s *Server) handleSearchFiles(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []*store.File{})
		return
	}
	files, err := s.store.SearchFiles(r.Context(), q, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	if files == nil {
		files = []*store.File{}
	}
	writeJSON(w, http.StatusOK, files)
}

type createFolderBody struct {
	Name     string `json:"name"`
	ParentID *int64 `json:"parent_id"`
}

func (s *Server) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	var body createFolderBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "folder name required")
		return
	}
	f, err := s.store.CreateFile(r.Context(), &store.File{
		UserID:   userID(r), // creator
		ParentID: body.ParentID,
		Name:     body.Name,
		IsDir:    true,
		MimeType: "inode/directory",
	})
	if err != nil {
		writeError(w, http.StatusConflict, "could not create folder (name may already exist)")
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

// handleUpload streams the request body straight to a provider via the pool.
// The filename comes from the X-Filename header (URL-encoded) and the optional
// ?parent= query places it in a folder. Content-Length, if present, feeds the
// router's capacity check.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	parent, ok := parseParentID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid parent id")
		return
	}
	rawName := r.Header.Get("X-Filename")
	name, err := url.QueryUnescape(rawName)
	if err != nil || strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, "missing or invalid X-Filename header")
		return
	}

	size := int64(-1)
	if r.ContentLength > 0 {
		size = r.ContentLength
	}

	// Prefer the filename's extension for the MIME type — it's more reliable
	// than the request Content-Type, which clients often omit or set to a
	// generic value (e.g. application/x-www-form-urlencoded). Fall back to the
	// request header, then to a generic binary type.
	mime := mimeByExtension(name)
	if mime == "" {
		mime = r.Header.Get("Content-Type")
	}
	if mime == "" || mime == "application/x-www-form-urlencoded" {
		mime = "application/octet-stream"
	}

	f, err := s.pool.Upload(r.Context(), userID(r), parent, name, size, mime, r.Body)
	if err != nil {
		if errors.Is(err, pool.ErrNoCapacity) {
			writeError(w, http.StatusInsufficientStorage, "no storage account has enough free space")
			return
		}
		writeError(w, http.StatusBadGateway, "upload failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

// handleDownload streams a stored file back to the client.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	f, rc, err := s.pool.Download(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "download failed: "+err.Error())
		return
	}
	defer rc.Close()

	// ?inline=1 serves the file for in-browser preview rather than forcing a
	// save. The bytes are identical; only Content-Disposition differs.
	disposition := "attachment"
	if r.URL.Query().Get("inline") == "1" {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", f.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename*=UTF-8''%s", disposition, url.PathEscape(f.Name)))
	if f.SizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(f.SizeBytes, 10))
	}
	if _, err := io.Copy(w, rc); err != nil {
		// Headers are already sent; just log it (client likely disconnected).
		log.Printf("download stream interrupted for file %d: %v", id, err)
	}
}

type renameBody struct {
	Name string `json:"name"`
}

func (s *Server) handleRenameFile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body renameBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	err := s.store.RenameFile(r.Context(), id, body.Name)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusConflict, "could not rename (name may already exist)")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	err := s.pool.Delete(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete file")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
