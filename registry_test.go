package smoke

import (
	"context"
	"net/url"
	"testing"

	"github.com/xd-dash/smoke/callback"
)

type testProvider struct{ schemes []string }

func (p testProvider) Schemes() []string { return p.schemes }
func (testProvider) Run(context.Context, *url.URL, *callback.Dispatcher) error { return nil }

func TestRegistryRejectsNilAndInvalidProviders(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected nil provider error")
	}
	if _, err := New(testProvider{schemes: []string{"bad scheme"}}); err == nil {
		t.Fatal("expected invalid scheme error")
	}
	if _, err := New(testProvider{schemes: []string{"redis"}}, testProvider{schemes: []string{"REDIS"}}); err == nil {
		t.Fatal("expected duplicate scheme error")
	}
}

func TestNilRegistryRunRejected(t *testing.T) {
	var r *Registry
	if err := r.Run(context.Background(), "redis://localhost", callback.New()); err == nil {
		t.Fatal("expected nil registry error")
	}
}
