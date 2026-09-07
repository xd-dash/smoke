// Package agni defines Smoke's narrow integration contract for an Agni-style
// infrastructure capability. The implementation is supplied by ordinary Go
// composition; Smoke core never imports dash-xd/agni.
package agni

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

type ScopeKind string

const (
	ScopeOutside     ScopeKind = "outside"
	ScopeEnvironment ScopeKind = "environment"
)

type Scope struct {
	Kind        ScopeKind
	Environment string
}

// TerraformRequest preserves Terraform as the deployment language. Files are
// the caller-owned transient root; Modules names select implementation assets
// from the compiled provider; Args are forwarded to Terraform unchanged.
type TerraformRequest struct {
	Scope     Scope
	Workspace string
	Modules   []string
	Files     map[string][]byte
	Args      []string
}

type Provider interface {
	RunTerraform(context.Context, TerraformRequest) error
}

var registry = struct {
	sync.RWMutex
	providers map[string]Provider
}{providers: map[string]Provider{}}

func Register(name string, provider Provider) {
	name = normalize(name)
	if name == "" {
		panic("agni provider: empty name")
	}
	if provider == nil {
		panic(fmt.Sprintf("agni provider %q: nil provider", name))
	}
	registry.Lock()
	defer registry.Unlock()
	if _, exists := registry.providers[name]; exists {
		panic(fmt.Sprintf("agni provider %q registered twice", name))
	}
	registry.providers[name] = provider
}

func Names() []string {
	registry.RLock()
	defer registry.RUnlock()
	names := make([]string, 0, len(registry.providers))
	for name := range registry.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func DefaultProvider() (Provider, error) {
	registry.RLock()
	defer registry.RUnlock()
	if len(registry.providers) == 0 {
		return nil, fmt.Errorf("no Agni provider is compiled into this Smoke")
	}
	if len(registry.providers) != 1 {
		names := make([]string, 0, len(registry.providers))
		for name := range registry.providers {
			names = append(names, name)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("multiple Agni providers are compiled in (%s)", strings.Join(names, ", "))
	}
	for _, provider := range registry.providers {
		return provider, nil
	}
	panic("unreachable")
}

func CurrentScope() Scope {
	name := strings.TrimSpace(os.Getenv("SMOKE_ENV"))
	if name == "" {
		return Scope{Kind: ScopeOutside}
	}
	return Scope{Kind: ScopeEnvironment, Environment: name}
}

func normalize(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if strings.ContainsAny(name, " \t\r\n") {
		return ""
	}
	return name
}
