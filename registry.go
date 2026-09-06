package smoke

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/xd-dash/smoke/callback"
)

// Provider resolves and runs one or more URL schemes. Providers are ordinary Go
// values so callers can compose only the transports they want.
type Provider interface {
	Schemes() []string
	Run(context.Context, *url.URL, *callback.Dispatcher) error
}

// Registry dispatches URLs to providers by scheme.
type Registry struct {
	providers map[string]Provider
}

var validScheme = regexp.MustCompile(`^[a-z][a-z0-9+.-]*$`)

func New(providers ...Provider) (*Registry, error) {
	r := &Registry{providers: make(map[string]Provider)}
	for _, provider := range providers {
		if provider == nil {
			return nil, fmt.Errorf("smoke: nil provider")
		}
		for _, scheme := range provider.Schemes() {
			scheme = strings.ToLower(strings.TrimSpace(scheme))
			if !validScheme.MatchString(scheme) {
				return nil, fmt.Errorf("smoke: provider registered invalid scheme %q", scheme)
			}
			if _, exists := r.providers[scheme]; exists {
				return nil, fmt.Errorf("smoke: duplicate provider for scheme %q", scheme)
			}
			r.providers[scheme] = provider
		}
	}
	return r, nil
}

func (r *Registry) Run(ctx context.Context, rawURL string, dispatcher *callback.Dispatcher) error {
	if r == nil {
		return fmt.Errorf("smoke: nil registry")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("smoke: parse URL: %w", err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("smoke: URL has no scheme: %q", rawURL)
	}

	provider, ok := r.providers[strings.ToLower(u.Scheme)]
	if !ok {
		return fmt.Errorf("smoke: no provider for scheme %q", u.Scheme)
	}
	return provider.Run(ctx, u, dispatcher)
}
