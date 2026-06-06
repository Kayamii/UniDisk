// Package api wires the HTTP layer: routing, middleware, and handlers that
// delegate to the auth, store, provider, and pool packages.
package api

import (
	"net/http"

	"github.com/unidisk/unidisk/internal/auth"
	"github.com/unidisk/unidisk/internal/crypto"
	"github.com/unidisk/unidisk/internal/pool"
	"github.com/unidisk/unidisk/internal/provider"
	"github.com/unidisk/unidisk/internal/store"
)

// Server holds the dependencies shared by all handlers.
type Server struct {
	store     *store.Store
	auth      *auth.Service
	tokens    *auth.TokenManager
	registry  *provider.Registry
	pool      *pool.Service
	crypto    *crypto.Box
	publicURL string
}

// NewServer builds the API server. publicURL is the externally reachable base
// URL, used to construct OAuth redirect URIs.
func NewServer(s *store.Store, a *auth.Service, tm *auth.TokenManager, reg *provider.Registry, p *pool.Service, box *crypto.Box, publicURL string) *Server {
	return &Server{store: s, auth: a, tokens: tm, registry: reg, pool: p, crypto: box, publicURL: publicURL}
}

// Handler returns the fully-routed HTTP handler, wrapped in CORS. The optional
// spa handler serves the built frontend for any non-/api path; pass nil in
// dev (Vite serves the SPA on its own port).
func (s *Server) Handler(spa http.Handler) http.Handler {
	mux := http.NewServeMux()

	// Health check (unauthenticated).
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Auth. The first admin is seeded from env on boot; there is no
	// self-registration endpoint.
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.handleMe))
	// Changing your own password is allowed even with a pending forced change,
	// so it uses requireAuth (not requirePrivilege).
	mux.HandleFunc("POST /api/auth/change-password", s.requireAuth(s.handleChangePassword))

	// Providers & accounts (providers.manage to mutate; view is part of the
	// dashboard, gated by files.view).
	mux.HandleFunc("GET /api/providers", s.requirePrivilege(store.PrivProvidersManage, s.handleListProviders))
	mux.HandleFunc("GET /api/accounts", s.requirePrivilege(store.PrivFilesView, s.handleListAccounts))
	mux.HandleFunc("POST /api/accounts", s.requirePrivilege(store.PrivProvidersManage, s.handleAddAccount))
	mux.HandleFunc("DELETE /api/accounts/{id}", s.requirePrivilege(store.PrivProvidersManage, s.handleDeleteAccount))

	// OAuth "Connect" flow. /start needs providers.manage; /callback is the
	// provider redirect target and authenticates via the signed state param.
	mux.HandleFunc("GET /api/oauth/{provider}/start", s.requirePrivilege(store.PrivProvidersManage, s.handleOAuthStart))
	mux.HandleFunc("GET /api/oauth/{provider}/callback", s.handleOAuthCallback)

	// Files.
	mux.HandleFunc("GET /api/files", s.requirePrivilege(store.PrivFilesView, s.handleListFiles))
	mux.HandleFunc("GET /api/files/search", s.requirePrivilege(store.PrivFilesView, s.handleSearchFiles))
	mux.HandleFunc("POST /api/files/folder", s.requirePrivilege(store.PrivFilesUpload, s.handleCreateFolder))
	mux.HandleFunc("POST /api/files/upload", s.requirePrivilege(store.PrivFilesUpload, s.handleUpload))
	mux.HandleFunc("GET /api/files/{id}/download", s.requirePrivilege(store.PrivFilesDownload, s.handleDownload))
	mux.HandleFunc("PUT /api/files/{id}", s.requirePrivilege(store.PrivFilesUpload, s.handleRenameFile))
	mux.HandleFunc("DELETE /api/files/{id}", s.requirePrivilege(store.PrivFilesDelete, s.handleDeleteFile))

	// Dashboard.
	mux.HandleFunc("GET /api/stats", s.requirePrivilege(store.PrivFilesView, s.handleStats))

	// Settings (routing threshold).
	mux.HandleFunc("GET /api/settings", s.requirePrivilege(store.PrivFilesView, s.handleGetSettings))
	mux.HandleFunc("PUT /api/settings", s.requirePrivilege(store.PrivSettingsManage, s.handleUpdateSettings))

	// User management (users.manage).
	mux.HandleFunc("GET /api/users", s.requirePrivilege(store.PrivUsersManage, s.handleListUsers))
	mux.HandleFunc("POST /api/users", s.requirePrivilege(store.PrivUsersManage, s.handleCreateUser))
	mux.HandleFunc("PUT /api/users/{id}/role", s.requirePrivilege(store.PrivUsersManage, s.handleSetUserRole))
	mux.HandleFunc("PUT /api/users/{id}/password", s.requirePrivilege(store.PrivUsersManage, s.handleResetUserPassword))
	mux.HandleFunc("DELETE /api/users/{id}", s.requirePrivilege(store.PrivUsersManage, s.handleDeleteUser))

	// Roles. Listing is needed both by user managers (to assign) and role
	// managers (to edit), so it's gated by either; the rest need roles.manage.
	mux.HandleFunc("GET /api/roles", s.requireAuth(s.handleListRolesGated))
	mux.HandleFunc("GET /api/privileges", s.requirePrivilege(store.PrivRolesManage, s.handleListPrivileges))
	mux.HandleFunc("POST /api/roles", s.requirePrivilege(store.PrivRolesManage, s.handleCreateRole))
	mux.HandleFunc("PUT /api/roles/{id}", s.requirePrivilege(store.PrivRolesManage, s.handleUpdateRole))
	mux.HandleFunc("DELETE /api/roles/{id}", s.requirePrivilege(store.PrivRolesManage, s.handleDeleteRole))

	// API keys — session-only (a key can never mint or revoke keys). The
	// create handler caps requested privileges to what the user holds.
	mux.HandleFunc("GET /api/keys", s.requireSession(s.handleListAPIKeys))
	mux.HandleFunc("GET /api/keys/grantable", s.requireSession(s.handleListGrantablePrivileges))
	mux.HandleFunc("POST /api/keys", s.requireSession(s.handleCreateAPIKey))
	mux.HandleFunc("DELETE /api/keys/{id}", s.requireSession(s.handleDeleteAPIKey))

	// Presigned public download links — session-only management; creating one
	// needs files.download.
	mux.HandleFunc("GET /api/presigned", s.requireSessionPrivilege(store.PrivFilesDownload, s.handleListPresigned))
	mux.HandleFunc("POST /api/presigned", s.requireSessionPrivilege(store.PrivFilesDownload, s.handleCreatePresigned))
	mux.HandleFunc("DELETE /api/presigned/{id}", s.requireSessionPrivilege(store.PrivFilesDownload, s.handleDeletePresigned))

	// Public, unauthenticated download via presigned token. The token is the
	// credential; this must NOT be behind auth middleware.
	mux.HandleFunc("GET /s/{token}", s.handlePublicDownload)

	// SPA fallback for the built frontend (production single-container mode).
	if spa != nil {
		mux.Handle("/", spa)
	}

	return withCORS(mux)
}
