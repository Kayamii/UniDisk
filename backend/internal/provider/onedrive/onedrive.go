// Package onedrive implements the storage provider interfaces for Microsoft
// OneDrive via the Microsoft Graph API. It uses the same OAuth "Connect" flow
// as Google Drive and Dropbox: the operator sets an instance-wide client
// id/secret, and users authorize accounts in the browser.
package onedrive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/unidisk/unidisk/internal/provider"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

const graphBase = "https://graph.microsoft.com/v1.0"

// scopes: Files.ReadWrite for file ops, offline_access for a refresh token,
// User.Read to read the account's display name/email.
var scopes = []string{"Files.ReadWrite", "offline_access", "User.Read"}

// Provider implements Provider + OAuthProvider for OneDrive.
type Provider struct {
	clientID     string
	clientSecret string
}

func New(clientID, clientSecret string) *Provider {
	return &Provider{clientID: clientID, clientSecret: clientSecret}
}

func (p *Provider) Name() string  { return "onedrive" }
func (p *Provider) Title() string { return "OneDrive" }

func (p *Provider) CredentialSchema() []provider.CredentialField {
	return []provider.CredentialField{
		{Key: "client_id", Label: "Application (client) ID", Type: provider.FieldText, Required: true,
			Help: "From the Azure portal app registration."},
		{Key: "client_secret", Label: "Client Secret", Type: provider.FieldPassword, Required: true},
		{Key: "refresh_token", Label: "Refresh Token", Type: provider.FieldPassword, Required: true},
	}
}

func (p *Provider) oauthConfig(creds map[string]string, redirectURI string) *oauth2.Config {
	id := creds["client_id"]
	secret := creds["client_secret"]
	if id == "" {
		id = p.clientID
	}
	if secret == "" {
		secret = p.clientSecret
	}
	return &oauth2.Config{
		ClientID:     id,
		ClientSecret: secret,
		// "common" tenant lets both personal and work/school accounts sign in.
		Endpoint:    microsoft.AzureADEndpoint("common"),
		RedirectURL: redirectURI,
		Scopes:      scopes,
	}
}

func (p *Provider) client(ctx context.Context, creds map[string]string) (*http.Client, error) {
	cfg := p.oauthConfig(creds, "")
	if cfg.ClientID == "" || cfg.ClientSecret == "" || creds["refresh_token"] == "" {
		return nil, fmt.Errorf("missing onedrive credentials")
	}
	return cfg.Client(ctx, &oauth2.Token{RefreshToken: creds["refresh_token"]}), nil
}

// ---- OAuthProvider ----

func (p *Provider) SupportsOAuth() bool {
	return p.clientID != "" && p.clientSecret != ""
}

func (p *Provider) AuthCodeURL(redirectURI, state string) string {
	cfg := p.oauthConfig(nil, redirectURI)
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"))
}

func (p *Provider) ExchangeCode(ctx context.Context, redirectURI, code string) (map[string]string, error) {
	cfg := p.oauthConfig(nil, redirectURI)
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	if tok.RefreshToken == "" {
		return nil, fmt.Errorf("onedrive did not return a refresh token")
	}
	return map[string]string{"refresh_token": tok.RefreshToken}, nil
}

// ---- Provider operations ----

func (p *Provider) Verify(ctx context.Context, creds map[string]string) (provider.VerifyResult, map[string]string, error) {
	c, err := p.client(ctx, creds)
	if err != nil {
		return provider.VerifyResult{}, nil, err
	}
	var me struct {
		UserPrincipalName string `json:"userPrincipalName"`
		DisplayName       string `json:"displayName"`
	}
	if err := getJSON(ctx, c, graphBase+"/me", &me); err != nil {
		return provider.VerifyResult{}, nil, fmt.Errorf("verify onedrive: %w", err)
	}
	usage, err := p.usage(ctx, c)
	if err != nil {
		return provider.VerifyResult{}, nil, err
	}
	name := me.UserPrincipalName
	if name == "" {
		name = me.DisplayName
	}
	if name == "" {
		name = "OneDrive"
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
	var drive struct {
		Quota struct {
			Total int64 `json:"total"`
			Used  int64 `json:"used"`
		} `json:"quota"`
	}
	if err := getJSON(ctx, c, graphBase+"/me/drive", &drive); err != nil {
		return provider.Usage{}, err
	}
	return provider.Usage{QuotaBytes: drive.Quota.Total, UsedBytes: drive.Quota.Used}, nil
}

func (p *Provider) Upload(ctx context.Context, creds map[string]string, name string, _ int64, r io.Reader) (string, error) {
	c, err := p.client(ctx, creds)
	if err != nil {
		return "", err
	}
	// Simple upload (PUT to the item content). Good for files up to ~250 MB;
	// larger files would need an upload session (future enhancement).
	endpoint := fmt.Sprintf("%s/me/drive/root:/%s:/content", graphBase, url.PathEscape(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, r)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload to onedrive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload to onedrive: %s: %s", resp.Status, string(msg))
	}
	var item struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return "", err
	}
	return item.ID, nil
}

func (p *Provider) Download(ctx context.Context, creds map[string]string, remoteID string) (io.ReadCloser, error) {
	c, err := p.client(ctx, creds)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/me/drive/items/%s/content", graphBase, url.PathEscape(remoteID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download from onedrive: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("download from onedrive: %s: %s", resp.Status, string(msg))
	}
	return resp.Body, nil
}

func (p *Provider) Delete(ctx context.Context, creds map[string]string, remoteID string) error {
	c, err := p.client(ctx, creds)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/me/drive/items/%s", graphBase, url.PathEscape(remoteID))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("delete from onedrive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete from onedrive: %s: %s", resp.Status, string(msg))
	}
	return nil
}

// getJSON performs a GET and decodes a JSON response into out.
func getJSON(ctx context.Context, c *http.Client, urlStr string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, string(msg))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
