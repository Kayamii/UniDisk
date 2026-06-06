package provider

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Registry holds the available providers keyed by Name().
type Registry struct {
	providers map[string]Provider
}

// NewRegistry builds a registry from the given providers.
func NewRegistry(ps ...Provider) *Registry {
	r := &Registry{providers: make(map[string]Provider, len(ps))}
	for _, p := range ps {
		r.providers[p.Name()] = p
	}
	return r
}

// Get returns the provider for name, or an error if unknown.
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return p, nil
}

// Descriptor is the public, credential-free description of a provider used to
// populate the "add provider" picker and form in the dashboard.
type Descriptor struct {
	Name   string            `json:"name"`
	Title  string            `json:"title"`
	Fields []CredentialField `json:"fields"`
	// OAuth is true when the dashboard should offer a one-click "Connect"
	// button (the provider implements OAuthProvider and it's configured).
	OAuth bool `json:"oauth"`
}

// Descriptors lists all registered providers, sorted by title, for the UI.
func (r *Registry) Descriptors() []Descriptor {
	out := make([]Descriptor, 0, len(r.providers))
	for _, p := range r.providers {
		oauth := false
		if op, ok := p.(OAuthProvider); ok {
			oauth = op.SupportsOAuth()
		}
		out = append(out, Descriptor{
			Name: p.Name(), Title: p.Title(), Fields: p.CredentialSchema(), OAuth: oauth,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

// EncodeCreds serializes a credential map for storage.
func EncodeCreds(creds map[string]string) (string, error) {
	b, err := json.Marshal(creds)
	return string(b), err
}

// DecodeCreds parses stored credentials back into a map.
func DecodeCreds(s string) (map[string]string, error) {
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}
