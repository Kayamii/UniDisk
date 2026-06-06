// Package box implements the storage provider interfaces for Box via its REST
// API, using the same OAuth "Connect" flow as the other cloud providers.
package box

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/unidisk/unidisk/internal/provider"
	"golang.org/x/oauth2"
)

const (
	apiBase    = "https://api.box.com/2.0"
	uploadBase = "https://upload.box.com/api/2.0"
)

var boxEndpoint = oauth2.Endpoint{
	AuthURL:  "https://account.box.com/api/oauth2/authorize",
	TokenURL: "https://api.box.com/oauth2/token",
}

type Provider struct {
	clientID     string
	clientSecret string
}

func New(clientID, clientSecret string) *Provider {
	return &Provider{clientID: clientID, clientSecret: clientSecret}
}

func (p *Provider) Name() string  { return "box" }
func (p *Provider) Title() string { return "Box" }

func (p *Provider) CredentialSchema() []provider.CredentialField {
	return []provider.CredentialField{
		{Key: "client_id", Label: "Client ID", Type: provider.FieldText, Required: true,
			Help: "From the Box Developer Console."},
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
		Endpoint:     boxEndpoint,
		RedirectURL:  redirectURI,
	}
}

func (p *Provider) client(ctx context.Context, creds map[string]string) (*http.Client, error) {
	cfg := p.oauthConfig(creds, "")
	if cfg.ClientID == "" || cfg.ClientSecret == "" || creds["refresh_token"] == "" {
		return nil, fmt.Errorf("missing box credentials")
	}
	return cfg.Client(ctx, &oauth2.Token{RefreshToken: creds["refresh_token"]}), nil
}

// ---- OAuthProvider ----

func (p *Provider) SupportsOAuth() bool {
	return p.clientID != "" && p.clientSecret != ""
}

func (p *Provider) AuthCodeURL(redirectURI, state string) string {
	return p.oauthConfig(nil, redirectURI).AuthCodeURL(state)
}

func (p *Provider) ExchangeCode(ctx context.Context, redirectURI, code string) (map[string]string, error) {
	tok, err := p.oauthConfig(nil, redirectURI).Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	if tok.RefreshToken == "" {
		return nil, fmt.Errorf("box did not return a refresh token")
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
		Login       string `json:"login"`
		Name        string `json:"name"`
		SpaceAmount int64  `json:"space_amount"`
		SpaceUsed   int64  `json:"space_used"`
	}
	if err := getJSON(ctx, c, apiBase+"/users/me", &me); err != nil {
		return provider.VerifyResult{}, nil, fmt.Errorf("verify box: %w", err)
	}
	name := me.Login
	if name == "" {
		name = me.Name
	}
	if name == "" {
		name = "Box"
	}
	return provider.VerifyResult{
		DisplayName: name,
		Usage:       provider.Usage{QuotaBytes: me.SpaceAmount, UsedBytes: me.SpaceUsed},
	}, creds, nil
}

func (p *Provider) Usage(ctx context.Context, creds map[string]string) (provider.Usage, error) {
	c, err := p.client(ctx, creds)
	if err != nil {
		return provider.Usage{}, err
	}
	var me struct {
		SpaceAmount int64 `json:"space_amount"`
		SpaceUsed   int64 `json:"space_used"`
	}
	if err := getJSON(ctx, c, apiBase+"/users/me", &me); err != nil {
		return provider.Usage{}, err
	}
	return provider.Usage{QuotaBytes: me.SpaceAmount, UsedBytes: me.SpaceUsed}, nil
}

func (p *Provider) Upload(ctx context.Context, creds map[string]string, name string, _ int64, r io.Reader) (string, error) {
	c, err := p.client(ctx, creds)
	if err != nil {
		return "", err
	}
	// Box upload is multipart: an "attributes" JSON part (name + parent) then
	// the file part. We pipe so the body streams rather than buffering.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		attrs := fmt.Sprintf(`{"name":%q,"parent":{"id":"0"}}`, name)
		_ = mw.WriteField("attributes", attrs)
		part, err := mw.CreateFormFile("file", name)
		if err == nil {
			_, err = io.Copy(part, r)
		}
		if err != nil {
			mw.Close()
			pw.CloseWithError(err)
			return
		}
		pw.CloseWithError(mw.Close())
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadBase+"/files/content", pr)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload to box: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload to box: %s: %s", resp.Status, string(msg))
	}
	var result struct {
		Entries []struct {
			ID string `json:"id"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Entries) == 0 {
		return "", fmt.Errorf("box upload returned no file id")
	}
	return result.Entries[0].ID, nil
}

func (p *Provider) Download(ctx context.Context, creds map[string]string, remoteID string) (io.ReadCloser, error) {
	c, err := p.client(ctx, creds)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/files/%s/content", apiBase, remoteID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download from box: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("download from box: %s: %s", resp.Status, string(msg))
	}
	return resp.Body, nil
}

func (p *Provider) Delete(ctx context.Context, creds map[string]string, remoteID string) error {
	c, err := p.client(ctx, creds)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/files/%s", apiBase, remoteID), nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("delete from box: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete from box: %s: %s", resp.Status, string(msg))
	}
	return nil
}

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
