// Package provider defines the storage-provider abstraction that lets UniDisk
// treat Google Drive, Dropbox, S3, etc. uniformly. Each provider is added in
// the dashboard: the user picks a type, fills in the credential fields the
// provider declares, and the backend verifies them before the account joins
// the pool.
package provider

import (
	"context"
	"io"
)

// FieldType describes how the dashboard should render a credential input.
type FieldType string

const (
	FieldText     FieldType = "text"     // plain text (e.g. bucket name)
	FieldPassword FieldType = "password" // masked secret (e.g. secret key)
	FieldOAuth    FieldType = "oauth"    // "Connect" button → OAuth redirect
)

// CredentialField declares one input the dashboard form must collect for a
// provider. The frontend renders the form purely from this schema, so adding
// a provider needs no frontend changes.
type CredentialField struct {
	Key      string    `json:"key"`      // machine key, e.g. "access_key_id"
	Label    string    `json:"label"`    // human label, e.g. "Access Key ID"
	Type     FieldType `json:"type"`     // how to render it
	Required bool      `json:"required"` // whether the field is mandatory
	Help     string    `json:"help,omitempty"`
}

// Usage reports a provider account's capacity, in bytes.
type Usage struct {
	QuotaBytes int64 // total capacity (0 = unknown/unlimited)
	UsedBytes  int64 // bytes currently used
}

// VerifyResult is returned after credentials are validated and is used to
// populate the new account row.
type VerifyResult struct {
	DisplayName string // e.g. the connected account's email
	Usage       Usage
}

// Provider is implemented once per storage backend. Implementations are
// stateless: every method takes the account's credentials as a map, so a
// single Provider value serves all users' accounts of that type.
type Provider interface {
	// Name is the stable identifier persisted on accounts (e.g. "googledrive").
	Name() string
	// Title is the human-facing provider name (e.g. "Google Drive").
	Title() string
	// CredentialSchema declares the fields the dashboard must collect.
	CredentialSchema() []CredentialField

	// Verify validates creds with a live API call and returns account info.
	// It may also return mutated creds (e.g. exchanged OAuth tokens) to persist.
	Verify(ctx context.Context, creds map[string]string) (VerifyResult, map[string]string, error)

	// Usage refreshes capacity figures for an account.
	Usage(ctx context.Context, creds map[string]string) (Usage, error)

	// Upload streams r to the provider and returns its remote object id.
	// size may be -1 if unknown. Implementations MUST stream and not buffer
	// the whole body in memory.
	Upload(ctx context.Context, creds map[string]string, name string, size int64, r io.Reader) (remoteID string, err error)

	// Download opens a streaming reader for the remote object. The caller
	// closes it.
	Download(ctx context.Context, creds map[string]string, remoteID string) (io.ReadCloser, error)

	// Delete removes the remote object.
	Delete(ctx context.Context, creds map[string]string, remoteID string) error
}

// OAuthProvider is optionally implemented by providers that support a
// browser-based "Connect" flow instead of (or in addition to) pasted
// credentials. When a provider implements this, the dashboard shows a Connect
// button that round-trips through the provider's consent screen.
type OAuthProvider interface {
	Provider
	// SupportsOAuth reports whether OAuth is configured on this instance
	// (e.g. the operator set the client id/secret). If false, the dashboard
	// falls back to the manual credential form.
	SupportsOAuth() bool
	// AuthCodeURL builds the provider consent URL to redirect the user to.
	// redirectURI must match one registered on the OAuth client; state is an
	// opaque value echoed back to the callback.
	AuthCodeURL(redirectURI, state string) string
	// ExchangeCode trades the authorization code for durable credentials
	// (e.g. a refresh token) that Verify/Upload/etc. can later use.
	ExchangeCode(ctx context.Context, redirectURI, code string) (creds map[string]string, err error)
}
