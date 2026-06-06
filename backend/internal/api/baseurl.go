package api

import (
	"net/http"
	"strings"
)

// baseURL returns the absolute base URL (scheme://host) UniDisk is being
// accessed at, used to build OAuth redirect URIs and presigned share links.
//
// Resolution order, so the app "just works" whether reached by IP or DNS and
// whether or not it sits behind a reverse proxy:
//
//  1. The configured UNIDISK_PUBLIC_URL, if set (explicit override for fixed
//     deployments). Trailing slash trimmed.
//  2. The forwarded host/proto from a reverse proxy (X-Forwarded-Proto/Host),
//     so HTTPS-terminating proxies produce correct https:// links.
//  3. The actual request Host and TLS state (direct access by IP or domain).
func (s *Server) baseURL(r *http.Request) string {
	if s.publicURL != "" {
		return strings.TrimRight(s.publicURL, "/")
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// Honor a reverse proxy's forwarded headers if present.
	if proto := firstForwarded(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	}

	host := r.Host
	if fwd := firstForwarded(r.Header.Get("X-Forwarded-Host")); fwd != "" {
		host = fwd
	}

	return scheme + "://" + host
}

// firstForwarded returns the first value of a possibly comma-separated
// forwarded header (proxies may chain them).
func firstForwarded(v string) string {
	if v == "" {
		return ""
	}
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}
