package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/unidisk/unidisk/internal/store"
)

func (s *Server) handleListPresigned(w http.ResponseWriter, r *http.Request) {
	links, err := s.store.ListPresignedByUser(r.Context(), userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list links")
		return
	}
	if links == nil {
		links = []*store.PresignedURL{}
	}
	writeJSON(w, http.StatusOK, s.withURLs(r, links))
}

type createPresignedBody struct {
	FileID int64 `json:"file_id"`
	// ExpiresInHours of 0 means no expiry.
	ExpiresInHours int `json:"expires_in_hours"`
}

type presignedResponse struct {
	*store.PresignedURL
	URL string `json:"url"`
}

// handleCreatePresigned creates a public download link for a single file.
func (s *Server) handleCreatePresigned(w http.ResponseWriter, r *http.Request) {
	var body createPresignedBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.ExpiresInHours < 0 {
		writeError(w, http.StatusBadRequest, "expires_in_hours must be 0 (never) or positive")
		return
	}
	file, err := s.store.FileByID(r.Context(), body.FileID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load file")
		return
	}
	if file.IsDir {
		writeError(w, http.StatusBadRequest, "presigned links support single files only")
		return
	}

	token, err := randomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate link")
		return
	}
	var expiresAt *time.Time
	if body.ExpiresInHours > 0 {
		t := time.Now().Add(time.Duration(body.ExpiresInHours) * time.Hour)
		expiresAt = &t
	}
	link, err := s.store.CreatePresignedURL(r.Context(), token, file.ID, userID(r), expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save link")
		return
	}
	writeJSON(w, http.StatusCreated, presignedResponse{PresignedURL: link, URL: s.shareURL(r, link.Token)})
}

func (s *Server) handleDeletePresigned(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeletePresigned(r.Context(), userID(r), id); err != nil {
		writeError(w, http.StatusNotFound, "link not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handlePublicDownload streams a file via a presigned token. It is UNAUTHEN-
// TICATED by design — the token is the credential. Expired/invalid tokens 404.
func (s *Server) handlePublicDownload(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	link, file, err := s.store.ResolvePresigned(r.Context(), token)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "link not found or expired")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not resolve link")
		return
	}
	_ = link

	_, rc, err := s.pool.Download(r.Context(), file.ID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not fetch file")
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(file.Name)))
	if file.SizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(file.SizeBytes, 10))
	}
	_, _ = io.Copy(w, rc)
}

// withURLs attaches the absolute public URL to each link for the dashboard.
func (s *Server) withURLs(r *http.Request, links []*store.PresignedURL) []presignedResponse {
	out := make([]presignedResponse, len(links))
	for i, l := range links {
		out[i] = presignedResponse{PresignedURL: l, URL: s.shareURL(r, l.Token)}
	}
	return out
}

// shareURL builds the absolute shareable URL using the request's base URL, so
// share links carry whatever host (IP or domain) the user accessed UniDisk at.
func (s *Server) shareURL(r *http.Request, token string) string {
	return fmt.Sprintf("%s/s/%s", s.baseURL(r), token)
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
