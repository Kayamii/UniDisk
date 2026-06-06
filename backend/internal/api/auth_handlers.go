package api

import (
	"errors"
	"net/http"

	"github.com/unidisk/unidisk/internal/auth"
	"github.com/unidisk/unidisk/internal/store"
)

type credentialsBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  *store.User `json:"user"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body credentialsBody
	if !decodeJSON(w, r, &body) {
		return
	}
	user, token, err := s.auth.Login(r.Context(), body.Email, body.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
}

// handleMe returns the current user including role and privileges, so the SPA
// can gate its UI.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}

type changePasswordBody struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// handleChangePassword lets a user change their own password. It is allowed
// even while must_change_password is set (that's the whole point), so it is
// mounted under requireAuth, not requirePrivilege.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var body changePasswordBody
	if !decodeJSON(w, r, &body) {
		return
	}
	err := s.auth.ChangePassword(r.Context(), userID(r), body.CurrentPassword, body.NewPassword)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusBadRequest, "current password is incorrect")
		return
	case errors.Is(err, auth.ErrWeakInput):
		writeError(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not change password")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
