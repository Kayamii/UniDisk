package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/unidisk/unidisk/internal/auth"
	"github.com/unidisk/unidisk/internal/store"
)

// fileKeyPrivileges is the set an API key may carry — file operations only.
// Keys never grant providers/users/roles/settings management.
var fileKeyPrivileges = []store.Privilege{
	store.PrivFilesView, store.PrivFilesUpload, store.PrivFilesDownload, store.PrivFilesDelete,
}

func isFileKeyPrivilege(p store.Privilege) bool {
	for _, fp := range fileKeyPrivileges {
		if fp == p {
			return true
		}
	}
	return false
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.store.ListAPIKeys(r.Context(), userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list API keys")
		return
	}
	if keys == nil {
		keys = []*store.APIKey{}
	}
	writeJSON(w, http.StatusOK, keys)
}

type createAPIKeyBody struct {
	Name       string            `json:"name"`
	Privileges []store.Privilege `json:"privileges"`
	// ExpiresInDays of 0 means no expiry.
	ExpiresInDays int `json:"expires_in_days"`
}

type createAPIKeyResponse struct {
	Key    string        `json:"key"` // plaintext, shown once
	APIKey *store.APIKey `json:"api_key"`
}

// handleCreateAPIKey mints a key. Requested privileges are filtered to file
// privileges the CURRENT USER actually holds — so a Viewer can only create
// view/download keys, and no key can ever escalate.
func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var body createAPIKeyBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "key name required")
		return
	}

	user := currentUser(r)
	// Cap to file privileges the user holds; silently drop anything else.
	var granted []store.Privilege
	for _, p := range body.Privileges {
		if isFileKeyPrivilege(p) && user.Has(p) {
			granted = append(granted, p)
		}
	}
	if len(granted) == 0 {
		writeError(w, http.StatusBadRequest, "select at least one file permission you hold")
		return
	}

	var expiresAt *time.Time
	if body.ExpiresInDays > 0 {
		t := time.Now().AddDate(0, 0, body.ExpiresInDays)
		expiresAt = &t
	}

	plaintext, hash, prefix, err := auth.GenerateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate key")
		return
	}
	key, err := s.store.CreateAPIKey(r.Context(), user.ID, body.Name, hash, prefix, granted, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save key")
		return
	}
	// Plaintext is returned ONLY here and never stored.
	writeJSON(w, http.StatusCreated, createAPIKeyResponse{Key: plaintext, APIKey: key})
}

func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteAPIKey(r.Context(), userID(r), id); err != nil {
		writeError(w, http.StatusNotFound, "key not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListGrantablePrivileges tells the UI which key privileges the current
// user is allowed to grant (the file privileges they hold).
func (s *Server) handleListGrantablePrivileges(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	out := []store.Privilege{}
	for _, p := range fileKeyPrivileges {
		if user.Has(p) {
			out = append(out, p)
		}
	}
	writeJSON(w, http.StatusOK, out)
}
