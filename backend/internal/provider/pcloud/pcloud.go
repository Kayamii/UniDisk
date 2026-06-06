// Package pcloud implements the storage provider interfaces for pCloud.
//
// pCloud's OAuth doesn't fit the standard x/oauth2 flow: the callback returns
// both a code and a `hostname` (api.pcloud.com for US, eapi.pcloud.com for EU),
// the token exchange is a GET to oauth2_token, and the returned access token is
// long-lived (no refresh token). So this provider implements OAuth manually and
// persists the access token + region hostname per account.
package pcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/unidisk/unidisk/internal/provider"
)

// defaultHost is used when an account has no stored region (older accounts or
// manual entry); pCloud will redirect/replicate as needed.
const defaultHost = "api.pcloud.com"

type Provider struct {
	clientID     string
	clientSecret string
}

func New(clientID, clientSecret string) *Provider {
	return &Provider{clientID: clientID, clientSecret: clientSecret}
}

func (p *Provider) Name() string  { return "pcloud" }
func (p *Provider) Title() string { return "pCloud" }

func (p *Provider) CredentialSchema() []provider.CredentialField {
	return []provider.CredentialField{
		{Key: "access_token", Label: "Access Token", Type: provider.FieldPassword, Required: true,
			Help: "pCloud OAuth access token."},
		{Key: "hostname", Label: "API Hostname", Type: provider.FieldText, Required: false,
			Help: "api.pcloud.com (US) or eapi.pcloud.com (EU). Defaults to US."},
	}
}

func host(creds map[string]string) string {
	if h := creds["hostname"]; h != "" {
		return h
	}
	return defaultHost
}

// call performs a pCloud API method call. pCloud returns JSON with a non-zero
// "result" field on error. The access token goes in the query string.
func (p *Provider) call(ctx context.Context, creds map[string]string, method string, params url.Values, out any) error {
	if creds["access_token"] == "" {
		return fmt.Errorf("missing pcloud access token")
	}
	if params == nil {
		params = url.Values{}
	}
	params.Set("access_token", creds["access_token"])
	u := fmt.Sprintf("https://%s/%s?%s", host(creds), method, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return doJSON(req, out)
}

// ---- OAuthProvider (manual) ----

func (p *Provider) SupportsOAuth() bool {
	return p.clientID != "" && p.clientSecret != ""
}

func (p *Provider) AuthCodeURL(redirectURI, state string) string {
	v := url.Values{}
	v.Set("client_id", p.clientID)
	v.Set("response_type", "code")
	v.Set("redirect_uri", redirectURI)
	v.Set("state", state)
	return "https://my.pcloud.com/oauth2/authorize?" + v.Encode()
}

// ExchangeCode trades the code for a long-lived access token. pCloud appends a
// `hostname` query param to the callback indicating the user's region; the
// API server reads it via the request, but since our generic callback only
// passes the code, we try the US host then fall back to EU.
func (p *Provider) ExchangeCode(ctx context.Context, _ string, code string) (map[string]string, error) {
	for _, h := range []string{"api.pcloud.com", "eapi.pcloud.com"} {
		v := url.Values{}
		v.Set("client_id", p.clientID)
		v.Set("client_secret", p.clientSecret)
		v.Set("code", code)
		u := fmt.Sprintf("https://%s/oauth2_token?%s", h, v.Encode())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Result      int    `json:"result"`
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
		}
		if err := doJSON(req, &resp); err != nil {
			continue
		}
		if resp.Result == 0 && resp.AccessToken != "" {
			return map[string]string{"access_token": resp.AccessToken, "hostname": h}, nil
		}
	}
	return nil, fmt.Errorf("pcloud token exchange failed")
}

// ---- Provider operations ----

func (p *Provider) Verify(ctx context.Context, creds map[string]string) (provider.VerifyResult, map[string]string, error) {
	var info struct {
		Result    int    `json:"result"`
		Email     string `json:"email"`
		Quota     int64  `json:"quota"`
		UsedQuota int64  `json:"usedquota"`
		Error     string `json:"error"`
	}
	if err := p.call(ctx, creds, "userinfo", nil, &info); err != nil {
		return provider.VerifyResult{}, nil, fmt.Errorf("verify pcloud: %w", err)
	}
	if info.Result != 0 {
		return provider.VerifyResult{}, nil, fmt.Errorf("verify pcloud: %s", info.Error)
	}
	name := info.Email
	if name == "" {
		name = "pCloud"
	}
	return provider.VerifyResult{
		DisplayName: name,
		Usage:       provider.Usage{QuotaBytes: info.Quota, UsedBytes: info.UsedQuota},
	}, creds, nil
}

func (p *Provider) Usage(ctx context.Context, creds map[string]string) (provider.Usage, error) {
	var info struct {
		Quota     int64 `json:"quota"`
		UsedQuota int64 `json:"usedquota"`
	}
	if err := p.call(ctx, creds, "userinfo", nil, &info); err != nil {
		return provider.Usage{}, err
	}
	return provider.Usage{QuotaBytes: info.Quota, UsedBytes: info.UsedQuota}, nil
}

func (p *Provider) Upload(ctx context.Context, creds map[string]string, name string, _ int64, r io.Reader) (string, error) {
	if creds["access_token"] == "" {
		return "", fmt.Errorf("missing pcloud access token")
	}
	// Multipart upload to folderid 0 (root). Stream via a pipe.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
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

	v := url.Values{}
	v.Set("access_token", creds["access_token"])
	v.Set("folderid", "0")
	u := fmt.Sprintf("https://%s/uploadfile?%s", host(creds), v.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, pr)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	var resp struct {
		Result   int `json:"result"`
		Metadata []struct {
			FileID int64 `json:"fileid"`
		} `json:"metadata"`
		Error string `json:"error"`
	}
	if err := doJSON(req, &resp); err != nil {
		return "", fmt.Errorf("upload to pcloud: %w", err)
	}
	if resp.Result != 0 || len(resp.Metadata) == 0 {
		return "", fmt.Errorf("upload to pcloud failed: %s", resp.Error)
	}
	return fmt.Sprintf("%d", resp.Metadata[0].FileID), nil
}

func (p *Provider) Download(ctx context.Context, creds map[string]string, remoteID string) (io.ReadCloser, error) {
	// getfilelink returns hosts + a path; we then GET the file from a host.
	var link struct {
		Result int      `json:"result"`
		Hosts  []string `json:"hosts"`
		Path   string   `json:"path"`
		Error  string   `json:"error"`
	}
	params := url.Values{}
	params.Set("fileid", remoteID)
	if err := p.call(ctx, creds, "getfilelink", params, &link); err != nil {
		return nil, fmt.Errorf("pcloud getfilelink: %w", err)
	}
	if link.Result != 0 || len(link.Hosts) == 0 {
		return nil, fmt.Errorf("pcloud getfilelink failed: %s", link.Error)
	}
	fileURL := fmt.Sprintf("https://%s%s", link.Hosts[0], link.Path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download from pcloud: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		return nil, fmt.Errorf("download from pcloud: %s", resp.Status)
	}
	return resp.Body, nil
}

func (p *Provider) Delete(ctx context.Context, creds map[string]string, remoteID string) error {
	var resp struct {
		Result int    `json:"result"`
		Error  string `json:"error"`
	}
	params := url.Values{}
	params.Set("fileid", remoteID)
	if err := p.call(ctx, creds, "deletefile", params, &resp); err != nil {
		return fmt.Errorf("delete from pcloud: %w", err)
	}
	if resp.Result != 0 {
		return fmt.Errorf("delete from pcloud failed: %s", resp.Error)
	}
	return nil
}

// doJSON runs req and decodes the JSON body into out.
func doJSON(req *http.Request, out any) error {
	resp, err := http.DefaultClient.Do(req)
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
