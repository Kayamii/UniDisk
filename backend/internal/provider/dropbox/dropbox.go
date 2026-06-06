// Package dropbox implements the provider.Provider and provider.OAuthProvider
// interfaces for Dropbox, using Dropbox's HTTP API directly (no SDK). Like
// Google Drive, it supports the one-click "Connect" flow: the operator sets an
// instance-wide app key/secret and users authorize accounts in the browser.
package dropbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/unidisk/unidisk/internal/provider"
	"golang.org/x/oauth2"
)

const (
	rpcBase     = "https://api.dropboxapi.com"
	contentBase = "https://content.dropboxapi.com"
)

// dropboxEndpoint is the OAuth2 endpoint for Dropbox.
var dropboxEndpoint = oauth2.Endpoint{
	AuthURL:  "https://www.dropbox.com/oauth2/authorize",
	TokenURL: "https://api.dropboxapi.com/oauth2/token",
}

// Provider implements the storage interfaces for Dropbox.
type Provider struct {
	appKey    string
	appSecret string
}

// New returns a Dropbox provider. Pass the instance app key/secret to enable
// the Connect flow; empty values disable OAuth.
func New(appKey, appSecret string) *Provider {
	return &Provider{appKey: appKey, appSecret: appSecret}
}

func (p *Provider) Name() string  { return "dropbox" }
func (p *Provider) Title() string { return "Dropbox" }

// CredentialSchema declares the manual fallback fields (used when OAuth is not
// configured). The Connect flow stores only the refresh token.
func (p *Provider) CredentialSchema() []provider.CredentialField {
	return []provider.CredentialField{
		{Key: "app_key", Label: "App Key", Type: provider.FieldText, Required: true,
			Help: "From the Dropbox App Console."},
		{Key: "app_secret", Label: "App Secret", Type: provider.FieldPassword, Required: true},
		{Key: "refresh_token", Label: "Refresh Token", Type: provider.FieldPassword, Required: true,
			Help: "Obtain via the OAuth flow with token_access_type=offline."},
	}
}

// oauthConfig builds the OAuth config, preferring per-account key/secret and
// falling back to the instance-wide app credentials (Connect-flow accounts).
func (p *Provider) oauthConfig(creds map[string]string, redirectURI string) *oauth2.Config {
	key := creds["app_key"]
	secret := creds["app_secret"]
	if key == "" {
		key = p.appKey
	}
	if secret == "" {
		secret = p.appSecret
	}
	return &oauth2.Config{
		ClientID:     key,
		ClientSecret: secret,
		Endpoint:     dropboxEndpoint,
		RedirectURL:  redirectURI,
	}
}

// client returns an *http.Client that injects (and refreshes) the access token
// from the stored refresh token.
func (p *Provider) client(ctx context.Context, creds map[string]string) (*http.Client, error) {
	cfg := p.oauthConfig(creds, "")
	if cfg.ClientID == "" || cfg.ClientSecret == "" || creds["refresh_token"] == "" {
		return nil, fmt.Errorf("missing dropbox credentials")
	}
	token := &oauth2.Token{RefreshToken: creds["refresh_token"]}
	return cfg.Client(ctx, token), nil
}

// ---- OAuthProvider ----

func (p *Provider) SupportsOAuth() bool {
	return p.appKey != "" && p.appSecret != ""
}

func (p *Provider) AuthCodeURL(redirectURI, state string) string {
	cfg := p.oauthConfig(nil, redirectURI)
	// token_access_type=offline makes Dropbox return a durable refresh token.
	return cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("token_access_type", "offline"))
}

func (p *Provider) ExchangeCode(ctx context.Context, redirectURI, code string) (map[string]string, error) {
	cfg := p.oauthConfig(nil, redirectURI)
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	if tok.RefreshToken == "" {
		return nil, fmt.Errorf("dropbox did not return a refresh token")
	}
	return map[string]string{"refresh_token": tok.RefreshToken}, nil
}

// ---- RPC helpers ----

// rpc calls a JSON-in/JSON-out RPC endpoint and decodes the response into out.
func (p *Provider) rpc(ctx context.Context, c *http.Client, path string, arg any, out any) error {
	var body io.Reader
	if arg != nil {
		b, err := json.Marshal(arg)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcBase+path, body)
	if err != nil {
		return err
	}
	if arg != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dropbox %s: %s: %s", path, resp.Status, string(msg))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ---- Provider operations ----

func (p *Provider) Verify(ctx context.Context, creds map[string]string) (provider.VerifyResult, map[string]string, error) {
	c, err := p.client(ctx, creds)
	if err != nil {
		return provider.VerifyResult{}, nil, err
	}
	var acct struct {
		Email string `json:"email"`
		Name  struct {
			DisplayName string `json:"display_name"`
		} `json:"name"`
	}
	// get_current_account takes a null body.
	if err := p.rpc(ctx, c, "/2/users/get_current_account", nil, &acct); err != nil {
		return provider.VerifyResult{}, nil, fmt.Errorf("verify dropbox: %w", err)
	}
	usage, err := p.usage(ctx, c)
	if err != nil {
		return provider.VerifyResult{}, nil, err
	}
	name := acct.Email
	if name == "" {
		name = acct.Name.DisplayName
	}
	if name == "" {
		name = "Dropbox"
	}
	return provider.VerifyResult{DisplayName: name, Usage: usage}, creds, nil
}

func (p *Provider) Usage(ctx context.Context, creds map[string]string) (provider.Usage, error) {
	c, err := p.client(ctx, creds)
	if err != nil {
		return provider.Usage{}, err
	}
	return p.usage(ctx, c)
}

func (p *Provider) usage(ctx context.Context, c *http.Client) (provider.Usage, error) {
	var space struct {
		Used      int64 `json:"used"`
		Allocation struct {
			Allocated int64 `json:"allocated"`
		} `json:"allocation"`
	}
	if err := p.rpc(ctx, c, "/2/users/get_space_usage", nil, &space); err != nil {
		return provider.Usage{}, err
	}
	return provider.Usage{QuotaBytes: space.Allocation.Allocated, UsedBytes: space.Used}, nil
}

func (p *Provider) Upload(ctx context.Context, creds map[string]string, name string, _ int64, r io.Reader) (string, error) {
	c, err := p.client(ctx, creds)
	if err != nil {
		return "", err
	}
	// Files are placed at the app/account root. "add" autorename avoids
	// clobbering; "id:" is returned and reused for download/delete.
	arg := map[string]any{
		"path":       "/" + name,
		"mode":       "add",
		"autorename": true,
		"mute":       true,
	}
	argJSON, _ := json.Marshal(arg)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, contentBase+"/2/files/upload", r)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Dropbox-API-Arg", string(argJSON))
	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload to dropbox: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload to dropbox: %s: %s", resp.Status, string(msg))
	}
	var meta struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return "", err
	}
	return meta.ID, nil
}

func (p *Provider) Download(ctx context.Context, creds map[string]string, remoteID string) (io.ReadCloser, error) {
	c, err := p.client(ctx, creds)
	if err != nil {
		return nil, err
	}
	argJSON, _ := json.Marshal(map[string]string{"path": remoteID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, contentBase+"/2/files/download", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Dropbox-API-Arg", string(argJSON))
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download from dropbox: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("download from dropbox: %s: %s", resp.Status, string(msg))
	}
	return resp.Body, nil
}

func (p *Provider) Delete(ctx context.Context, creds map[string]string, remoteID string) error {
	c, err := p.client(ctx, creds)
	if err != nil {
		return err
	}
	return p.rpc(ctx, c, "/2/files/delete_v2", map[string]string{"path": remoteID}, nil)
}
