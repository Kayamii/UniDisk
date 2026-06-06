// Package googledrive implements the provider.Provider interface for Google
// Drive. Credentials are entered in the dashboard as an OAuth client id/secret
// plus a refresh token (generated once via Google Cloud Console / OAuth
// playground). This keeps UniDisk fully self-hosted with no shared app.
package googledrive

import (
	"context"
	"fmt"
	"io"

	"github.com/unidisk/unidisk/internal/provider"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Provider implements provider.Provider for Google Drive. It is stateless for
// per-account API calls (auth comes from the credential map), but holds the
// instance-wide OAuth app credentials used by the browser "Connect" flow.
type Provider struct {
	oauthClientID     string
	oauthClientSecret string
}

// New returns a Google Drive provider. Pass the instance OAuth app client
// id/secret to enable the "Connect with Google" flow; empty values disable it
// and the dashboard falls back to the manual credential form.
func New(oauthClientID, oauthClientSecret string) *Provider {
	return &Provider{oauthClientID: oauthClientID, oauthClientSecret: oauthClientSecret}
}

func (p *Provider) Name() string  { return "googledrive" }
func (p *Provider) Title() string { return "Google Drive" }

func (p *Provider) CredentialSchema() []provider.CredentialField {
	return []provider.CredentialField{
		{Key: "client_id", Label: "OAuth Client ID", Type: provider.FieldText, Required: true,
			Help: "From Google Cloud Console → APIs & Services → Credentials."},
		{Key: "client_secret", Label: "OAuth Client Secret", Type: provider.FieldPassword, Required: true},
		{Key: "refresh_token", Label: "Refresh Token", Type: provider.FieldPassword, Required: true,
			Help: "Obtain once via the OAuth Playground with the Drive scope."},
	}
}

// oauthConfig builds the OAuth config for a given redirect URI. It uses the
// account's own client id/secret when present (manual-credential accounts) and
// otherwise the instance-wide OAuth app (Connect-flow accounts).
func (p *Provider) oauthConfig(creds map[string]string, redirectURI string) *oauth2.Config {
	clientID := creds["client_id"]
	clientSecret := creds["client_secret"]
	if clientID == "" {
		clientID = p.oauthClientID
	}
	if clientSecret == "" {
		clientSecret = p.oauthClientSecret
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectURI,
		Scopes:       []string{drive.DriveScope},
	}
}

// driveService builds an authenticated Drive client from credentials.
func (p *Provider) driveService(ctx context.Context, creds map[string]string) (*drive.Service, error) {
	cfg := p.oauthConfig(creds, "")
	if cfg.ClientID == "" || cfg.ClientSecret == "" || creds["refresh_token"] == "" {
		return nil, fmt.Errorf("missing google drive credentials")
	}
	// A refresh token is enough; the library fetches access tokens on demand.
	token := &oauth2.Token{RefreshToken: creds["refresh_token"]}
	client := cfg.Client(ctx, token)
	return drive.NewService(ctx, option.WithHTTPClient(client))
}

// SupportsOAuth reports whether the instance has Google OAuth app credentials.
func (p *Provider) SupportsOAuth() bool {
	return p.oauthClientID != "" && p.oauthClientSecret != ""
}

// AuthCodeURL builds the Google consent URL. access_type=offline + prompt=
// consent ensures Google returns a refresh token every time.
func (p *Provider) AuthCodeURL(redirectURI, state string) string {
	cfg := p.oauthConfig(nil, redirectURI)
	return cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"))
}

// ExchangeCode trades the auth code for a refresh token. Only the refresh
// token is stored per account; the client id/secret stay instance-wide.
func (p *Provider) ExchangeCode(ctx context.Context, redirectURI, code string) (map[string]string, error) {
	cfg := p.oauthConfig(nil, redirectURI)
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	if tok.RefreshToken == "" {
		return nil, fmt.Errorf("google did not return a refresh token; revoke prior access at myaccount.google.com/permissions and retry")
	}
	return map[string]string{"refresh_token": tok.RefreshToken}, nil
}

func (p *Provider) Verify(ctx context.Context, creds map[string]string) (provider.VerifyResult, map[string]string, error) {
	svc, err := p.driveService(ctx, creds)
	if err != nil {
		return provider.VerifyResult{}, nil, err
	}
	about, err := svc.About.Get().Fields("user(emailAddress),storageQuota").Context(ctx).Do()
	if err != nil {
		return provider.VerifyResult{}, nil, fmt.Errorf("verify google drive: %w", err)
	}
	name := "Google Drive"
	if about.User != nil && about.User.EmailAddress != "" {
		name = about.User.EmailAddress
	}
	return provider.VerifyResult{
		DisplayName: name,
		Usage:       quotaUsage(about),
	}, creds, nil
}

func (p *Provider) Usage(ctx context.Context, creds map[string]string) (provider.Usage, error) {
	svc, err := p.driveService(ctx, creds)
	if err != nil {
		return provider.Usage{}, err
	}
	about, err := svc.About.Get().Fields("storageQuota").Context(ctx).Do()
	if err != nil {
		return provider.Usage{}, err
	}
	return quotaUsage(about), nil
}

func quotaUsage(about *drive.About) provider.Usage {
	if about == nil || about.StorageQuota == nil {
		return provider.Usage{}
	}
	// Drive reports limit == 0 for unlimited accounts.
	return provider.Usage{
		QuotaBytes: about.StorageQuota.Limit,
		UsedBytes:  about.StorageQuota.Usage,
	}
}

func (p *Provider) Upload(ctx context.Context, creds map[string]string, name string, size int64, r io.Reader) (string, error) {
	svc, err := p.driveService(ctx, creds)
	if err != nil {
		return "", err
	}
	// Media streams r directly to Drive's resumable upload; bytes are not
	// buffered wholesale in memory.
	f, err := svc.Files.Create(&drive.File{Name: name}).
		Media(r).
		Fields("id").
		Context(ctx).
		Do()
	if err != nil {
		return "", fmt.Errorf("upload to google drive: %w", err)
	}
	return f.Id, nil
}

func (p *Provider) Download(ctx context.Context, creds map[string]string, remoteID string) (io.ReadCloser, error) {
	svc, err := p.driveService(ctx, creds)
	if err != nil {
		return nil, err
	}
	resp, err := svc.Files.Get(remoteID).Context(ctx).Download()
	if err != nil {
		return nil, fmt.Errorf("download from google drive: %w", err)
	}
	return resp.Body, nil
}

func (p *Provider) Delete(ctx context.Context, creds map[string]string, remoteID string) error {
	svc, err := p.driveService(ctx, creds)
	if err != nil {
		return err
	}
	if err := svc.Files.Delete(remoteID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("delete from google drive: %w", err)
	}
	return nil
}
