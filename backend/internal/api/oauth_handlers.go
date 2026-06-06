package api

import (
	"fmt"
	"net/http"

	"github.com/unidisk/unidisk/internal/provider"
	"github.com/unidisk/unidisk/internal/store"
)

// redirectURI builds the provider callback URL from the request's base URL
// (honoring UNIDISK_PUBLIC_URL / proxy headers / the host that was hit). It
// must exactly match a redirect URI registered on the OAuth client, so when
// using OAuth the operator should register the URL they actually access at.
func (s *Server) redirectURI(r *http.Request, providerName string) string {
	return fmt.Sprintf("%s/api/oauth/%s/callback", s.baseURL(r), providerName)
}

// oauthProvider resolves a provider and asserts it supports OAuth.
func (s *Server) oauthProvider(name string) (provider.OAuthProvider, bool) {
	p, err := s.registry.Get(name)
	if err != nil {
		return nil, false
	}
	op, ok := p.(provider.OAuthProvider)
	if !ok || !op.SupportsOAuth() {
		return nil, false
	}
	return op, true
}

// handleOAuthStart returns the provider consent URL for the authenticated
// user to visit. The frontend opens it; the user authorizes; Google redirects
// to handleOAuthCallback. State carries the signed user identity.
func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("provider")
	op, ok := s.oauthProvider(name)
	if !ok {
		writeError(w, http.StatusBadRequest, "provider does not support OAuth on this instance")
		return
	}
	state, err := s.tokens.IssueState(userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start OAuth")
		return
	}
	url := op.AuthCodeURL(s.redirectURI(r, name), state)
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

// handleOAuthCallback is the provider redirect target. It is unauthenticated
// (it's a top-level browser navigation from Google), so the user identity comes
// from the signed state. It exchanges the code, verifies, saves the account,
// then redirects the browser back to the dashboard.
func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("provider")

	// Surface a provider-side denial cleanly.
	if e := r.URL.Query().Get("error"); e != "" {
		s.redirectToDashboard(w, r, "error", e)
		return
	}

	op, ok := s.oauthProvider(name)
	if !ok {
		s.redirectToDashboard(w, r, "error", "unsupported_provider")
		return
	}

	uid, err := s.tokens.ValidateState(r.URL.Query().Get("state"))
	if err != nil {
		s.redirectToDashboard(w, r, "error", "invalid_state")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		s.redirectToDashboard(w, r, "error", "missing_code")
		return
	}

	creds, err := op.ExchangeCode(r.Context(), s.redirectURI(r, name), code)
	if err != nil {
		s.redirectToDashboard(w, r, "error", "exchange_failed")
		return
	}

	result, creds, err := op.Verify(r.Context(), creds)
	if err != nil {
		s.redirectToDashboard(w, r, "error", "verify_failed")
		return
	}

	credsJSON, err := provider.EncodeCreds(creds)
	if err != nil {
		s.redirectToDashboard(w, r, "error", "store_failed")
		return
	}
	credsJSON, err = s.crypto.Encrypt(credsJSON)
	if err != nil {
		s.redirectToDashboard(w, r, "error", "store_failed")
		return
	}
	if _, err := s.store.CreateAccount(r.Context(), &store.Account{
		UserID:          uid,
		Provider:        name,
		DisplayName:     result.DisplayName,
		CredentialsJSON: credsJSON,
		Status:          "active",
		QuotaBytes:      result.Usage.QuotaBytes,
		UsedBytes:       result.Usage.UsedBytes,
		Priority:        100,
	}); err != nil {
		s.redirectToDashboard(w, r, "error", "save_failed")
		return
	}

	s.redirectToDashboard(w, r, "connected", name)
}

// redirectToDashboard sends the browser back to the providers page with a
// status query param the SPA can read to show a toast/message.
func (s *Server) redirectToDashboard(w http.ResponseWriter, r *http.Request, key, value string) {
	http.Redirect(w, r, fmt.Sprintf("/providers?%s=%s", key, value), http.StatusFound)
}
