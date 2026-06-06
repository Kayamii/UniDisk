package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/unidisk/unidisk/internal/auth"
	"github.com/unidisk/unidisk/internal/store"
)

type ctxKey int

const (
	userKey ctxKey = iota
	// keyScopeKey holds the privileges of the API key used (nil for session
	// auth). When set, effective privileges are key ∩ user.
	keyScopeKey
)

// requireAuth accepts either a session JWT or an API key. It injects the
// resolved user (privileges loaded fresh) into the context. For an API key it
// also records the key's privilege scope, so a key can never exceed its owner's
// current privileges.
//
// Credentials: "Authorization: Bearer <jwt>" for sessions,
// "Authorization: Bearer udk_..." or "X-API-Key: udk_..." for API keys.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing credentials")
			return
		}

		// API key path.
		if strings.HasPrefix(token, "udk_") {
			key, user, err := s.store.AuthByKeyHash(r.Context(), auth.HashAPIKey(token))
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired API key")
				return
			}
			ctx := context.WithValue(r.Context(), userKey, user)
			ctx = context.WithValue(ctx, keyScopeKey, key.Privileges)
			next(w, r.WithContext(ctx))
			return
		}

		// Session JWT path.
		uid, err := s.tokens.Validate(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		user, err := s.store.UserByID(r.Context(), uid)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
	}
}

// requireSession is like requireAuth but rejects API-key callers. Used for
// sensitive self-management (creating/revoking keys, managing share links) so a
// key can never mint or manage other keys.
func (s *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if isAPIKeyRequest(r) {
			writeError(w, http.StatusForbidden, "this action requires a logged-in session, not an API key")
			return
		}
		if currentUser(r).MustChangePassword {
			writeError(w, http.StatusForbidden, "password change required before continuing")
			return
		}
		next(w, r)
	})
}

// requireSessionPrivilege combines requireSession with a privilege check.
func (s *Server) requireSessionPrivilege(priv store.Privilege, next http.HandlerFunc) http.HandlerFunc {
	return s.requireSession(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).Has(priv) {
			writeError(w, http.StatusForbidden, "you do not have permission to perform this action")
			return
		}
		next(w, r)
	})
}

// requirePrivilege enforces that the caller's EFFECTIVE privileges include priv.
// For sessions that's the user's role privileges; for API keys it's the
// intersection of the key's scope and the user's current privileges.
func (s *Server) requirePrivilege(priv store.Privilege, next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r)
		if user.MustChangePassword {
			writeError(w, http.StatusForbidden, "password change required before continuing")
			return
		}
		if !effectiveHas(r, priv) {
			writeError(w, http.StatusForbidden, "you do not have permission to perform this action")
			return
		}
		next(w, r)
	})
}

// bearerToken extracts the credential from the Authorization bearer header or
// the X-API-Key header.
func bearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return r.Header.Get("X-API-Key")
}

// currentUser returns the authenticated user placed by requireAuth.
func currentUser(r *http.Request) *store.User {
	u, _ := r.Context().Value(userKey).(*store.User)
	return u
}

// keyScope returns the API key's privilege scope, or nil for session auth.
func keyScope(r *http.Request) []store.Privilege {
	s, _ := r.Context().Value(keyScopeKey).([]store.Privilege)
	return s
}

// effectiveHas reports whether the request is authorized for priv, honoring API
// key scoping (key ∩ user). The user must hold priv; if an API key is in use,
// the key must also grant it.
func effectiveHas(r *http.Request, priv store.Privilege) bool {
	user := currentUser(r)
	if user == nil || !user.Has(priv) {
		return false
	}
	scope := keyScope(r)
	if scope == nil {
		return true // session auth: full user privileges
	}
	for _, p := range scope {
		if p == priv {
			return true
		}
	}
	return false
}

// userID returns the authenticated user's id.
func userID(r *http.Request) int64 {
	if u := currentUser(r); u != nil {
		return u.ID
	}
	return 0
}

// isAPIKeyRequest reports whether the caller authenticated with an API key.
func isAPIKeyRequest(r *http.Request) bool {
	return keyScope(r) != nil
}

// withCORS allows the Vite dev server (different origin) to call the API in
// development. In production the SPA is served same-origin so this is a no-op.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Filename, X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
