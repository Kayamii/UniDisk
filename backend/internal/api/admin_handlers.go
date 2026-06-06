package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/unidisk/unidisk/internal/auth"
	"github.com/unidisk/unidisk/internal/store"
)

// ---- Users (require users.manage) ----

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list users")
		return
	}
	if users == nil {
		users = []*store.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

type createUserBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	RoleID   int64  `json:"role_id"`
}

// handleCreateUser creates a user with a temp password and forces a password
// change at first login.
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body createUserBody
	if !decodeJSON(w, r, &body) {
		return
	}
	email := strings.TrimSpace(strings.ToLower(body.Email))
	if email == "" {
		writeError(w, http.StatusBadRequest, "email required")
		return
	}
	if _, err := s.store.UserByEmail(r.Context(), email); err == nil {
		writeError(w, http.StatusConflict, "email already registered")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "could not check email")
		return
	}
	if _, err := s.store.RoleByID(r.Context(), body.RoleID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid role")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, "temp password must be at least 8 characters")
		return
	}
	user, err := s.store.CreateUser(r.Context(), email, hash, body.RoleID, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

type resetPasswordBody struct {
	Password string `json:"password"`
}

// handleResetUserPassword sets a new temp password and forces a change.
func (s *Server) handleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body resetPasswordBody
	if !decodeJSON(w, r, &body) {
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if err := s.store.ResetPassword(r.Context(), id, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "could not reset password")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type setRoleBody struct {
	RoleID int64 `json:"role_id"`
}

func (s *Server) handleSetUserRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body setRoleBody
	if !decodeJSON(w, r, &body) {
		return
	}
	newRole, err := s.store.RoleByID(r.Context(), body.RoleID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid role")
		return
	}
	// If this change strips users.manage from the user, ensure they aren't the
	// last admin (that would lock out all administration).
	roleIsAdmin := false
	for _, p := range newRole.Privileges {
		if p == store.PrivUsersManage {
			roleIsAdmin = true
			break
		}
	}
	if !roleIsAdmin {
		if ok, err := s.lastAdminGuard(r, id); err != nil {
			writeError(w, http.StatusInternalServerError, "could not verify admins")
			return
		} else if !ok {
			writeError(w, http.StatusBadRequest, "cannot demote the last administrator")
			return
		}
	}
	if err := s.store.SetUserRole(r.Context(), id, body.RoleID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not update role")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// lastAdminGuard returns true if it's safe to remove/demote targetID — i.e.
// either the target isn't an admin, or there is more than one admin left.
func (s *Server) lastAdminGuard(r *http.Request, targetID int64) (bool, error) {
	targetIsAdmin, err := s.store.IsAdmin(r.Context(), targetID)
	if err != nil {
		return false, err
	}
	if !targetIsAdmin {
		return true, nil
	}
	count, err := s.store.CountAdmins(r.Context())
	if err != nil {
		return false, err
	}
	return count > 1, nil
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	// Don't let an admin delete their own account out from under themselves.
	if id == userID(r) {
		writeError(w, http.StatusBadRequest, "you cannot delete your own account")
		return
	}
	// Never remove the last administrator (would lock out all management).
	if ok, err := s.lastAdminGuard(r, id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not verify admins")
		return
	} else if !ok {
		writeError(w, http.StatusBadRequest, "cannot delete the last administrator")
		return
	}
	if err := s.store.DeleteUser(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Roles (require roles.manage) ----

// handleListRolesGated lists roles for anyone who can manage users OR roles
// (the former needs them to fill the assign-role dropdown). Other privilege
// checks are enforced by the route middleware; this one is custom because it
// accepts either privilege.
func (s *Server) handleListRolesGated(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u.MustChangePassword || (!u.Has(store.PrivUsersManage) && !u.Has(store.PrivRolesManage)) {
		writeError(w, http.StatusForbidden, "you do not have permission to perform this action")
		return
	}
	roles, err := s.store.ListRoles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list roles")
		return
	}
	if roles == nil {
		roles = []*store.Role{}
	}
	writeJSON(w, http.StatusOK, roles)
}

// handleListPrivileges returns the catalog of assignable privileges for the
// role editor.
func (s *Server) handleListPrivileges(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, store.AllPrivileges())
}

type roleBody struct {
	Name       string            `json:"name"`
	Privileges []store.Privilege `json:"privileges"`
}

func (s *Server) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var body roleBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "role name required")
		return
	}
	role, err := s.store.CreateRole(r.Context(), strings.TrimSpace(body.Name), body.Privileges)
	if err != nil {
		writeError(w, http.StatusConflict, "could not create role (name may already exist)")
		return
	}
	writeJSON(w, http.StatusCreated, role)
}

func (s *Server) handleUpdateRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body roleBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := s.store.UpdateRolePrivileges(r.Context(), id, body.Privileges); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	role, err := s.store.RoleByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load role")
		return
	}
	writeJSON(w, http.StatusOK, role)
}

func (s *Server) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteRole(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
