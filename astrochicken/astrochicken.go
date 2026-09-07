package astrochicken

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

type Request struct {
	Scope Scope
	Args  []string
}

type Provider interface {
	Run(context.Context, Request) error
}

var global = struct {
	sync.RWMutex
	providers map[string]Provider
}{providers: map[string]Provider{}}

func Register(name string, provider Provider) {
	name = normalizeName(name)
	if name == "" {
		panic("astrochicken provider: empty name")
	}
	if provider == nil {
		panic(fmt.Sprintf("astrochicken provider %q: nil provider", name))
	}

	global.Lock()
	defer global.Unlock()
	if _, exists := global.providers[name]; exists {
		panic(fmt.Sprintf("astrochicken provider %q registered twice", name))
	}
	global.providers[name] = provider
}

func Names() []string {
	global.RLock()
	defer global.RUnlock()

	names := make([]string, 0, len(global.providers))
	for name := range global.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func Run(ctx context.Context, name string, args []string) error {
	name = normalizeName(name)
	global.RLock()
	provider := global.providers[name]
	global.RUnlock()
	if provider == nil {
		return fmt.Errorf("astrochicken provider %q is not compiled into this smoke", name)
	}
	return provider.Run(ctx, Request{Scope: CurrentScope(), Args: append([]string(nil), args...)})
}

func CurrentScope() Scope {
	name := strings.TrimSpace(os.Getenv("SMOKE_ENV"))
	if name == "" {
		return Scope{Kind: ScopeOutside}
	}
	return Scope{Kind: ScopeEnvironment, Environment: name}
}

func normalizeName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if strings.ContainsAny(name, " \t\r\n") {
		return ""
	}
	return name
}
