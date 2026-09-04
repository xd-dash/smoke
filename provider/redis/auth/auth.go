package redisauth

import (
	"context"
	"fmt"
	"strings"

	redisprovider "github.com/xd-dash/smoke/provider/redis"
)

// Provider resolves Redis AUTH credentials without owning transport or ACL policy.
// Providers may read local process state, files, agents, or other secret stores.
type Provider interface {
	Name() string
	Resolve(context.Context, string) (redisprovider.Credentials, error)
}

type Registry struct {
	providers map[string]Provider
}

func New(providers ...Provider) (*Registry, error) {
	r := &Registry{providers: make(map[string]Provider)}
	for _, provider := range providers {
		name := strings.ToLower(strings.TrimSpace(provider.Name()))
		if name == "" {
			return nil, fmt.Errorf("redis auth: provider registered empty name")
		}
		if _, exists := r.providers[name]; exists {
			return nil, fmt.Errorf("redis auth: duplicate provider %q", name)
		}
		r.providers[name] = provider
	}
	return r, nil
}

func (r *Registry) Resolve(ctx context.Context, providerName, profile string) (redisprovider.Credentials, error) {
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	provider, ok := r.providers[providerName]
	if !ok {
		return redisprovider.Credentials{}, fmt.Errorf("redis auth: no provider %q", providerName)
	}
	return provider.Resolve(ctx, profile)
}
