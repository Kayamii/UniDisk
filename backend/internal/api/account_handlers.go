package api

import (
	"errors"
	"net/http"

	"github.com/unidisk/unidisk/internal/provider"
	"github.com/unidisk/unidisk/internal/store"
)

// handleListProviders returns the credential-free provider descriptors used to
// build the "add provider" picker and dynamic form in the dashboard.
func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.registry.Descriptors())
}

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.store.ListAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list accounts")
		return
	}
	if accounts == nil {
		accounts = []*store.Account{}
	}
	writeJSON(w, http.StatusOK, accounts)
}

type addAccountBody struct {
	Provider    string            `json:"provider"`
	DisplayName string            `json:"display_name"`
	Credentials map[string]string `json:"credentials"`
}

// handleAddAccount verifies the submitted credentials with a live provider
// call and, only on success, persists the account into the pool. This is the
// "choose provider → enter credentials → confirm & verify → added" flow.
func (s *Server) handleAddAccount(w http.ResponseWriter, r *http.Request) {
	var body addAccountBody
	if !decodeJSON(w, r, &body) {
		return
	}
	p, err := s.registry.Get(body.Provider)
	if err != nil {
		writeError(w, http.StatusBadRequest, "unknown provider")
		return
	}

	// Verify against the live provider before storing anything.
	result, creds, err := p.Verify(r.Context(), body.Credentials)
	if err != nil {
		writeError(w, http.StatusBadRequest, "verification failed: "+err.Error())
		return
	}

	credsJSON, err := provider.EncodeCreds(creds)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not store credentials")
		return
	}
	credsJSON, err = s.crypto.Encrypt(credsJSON)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not encrypt credentials")
		return
	}

	name := body.DisplayName
	if name == "" {
		name = result.DisplayName
	}
	account, err := s.store.CreateAccount(r.Context(), &store.Account{
		UserID:          userID(r),
		Provider:        body.Provider,
		DisplayName:     name,
		CredentialsJSON: credsJSON,
		Status:          "active",
		QuotaBytes:      result.Usage.QuotaBytes,
		UsedBytes:       result.Usage.UsedBytes,
		Priority:        100,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save account")
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	err := s.store.DeleteAccount(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete account")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
