package astrochicken

import (
	"context"
	"testing"
)

type recordingProvider struct {
	request Request
}

func (p *recordingProvider) Run(_ context.Context, request Request) error {
	p.request = request
	return nil
}

func TestCurrentScopeOutside(t *testing.T) {
	t.Setenv("SMOKE_ENV", "")
	got := CurrentScope()
	if got.Kind != ScopeOutside || got.Environment != "" {
		t.Fatalf("CurrentScope() = %#v", got)
	}
}

func TestCurrentScopeEnvironment(t *testing.T) {
	t.Setenv("SMOKE_ENV", "world")
	got := CurrentScope()
	if got.Kind != ScopeEnvironment || got.Environment != "world" {
		t.Fatalf("CurrentScope() = %#v", got)
	}
}

func TestRunPassesScopeAndArgs(t *testing.T) {
	name := "test-run-passes-scope"
	provider := &recordingProvider{}
	Register(name, provider)
	t.Setenv("SMOKE_ENV", "world")

	args := []string{"deploy", "--var", "region=us-west1"}
	if err := Run(context.Background(), name, args); err != nil {
		t.Fatal(err)
	}
	args[0] = "changed"

	if provider.request.Scope.Kind != ScopeEnvironment || provider.request.Scope.Environment != "world" {
		t.Fatalf("scope = %#v", provider.request.Scope)
	}
	if provider.request.Args[0] != "deploy" {
		t.Fatalf("args were not copied: %#v", provider.request.Args)
	}
}

func TestRunRejectsUnknownProvider(t *testing.T) {
	if err := Run(context.Background(), "does-not-exist", nil); err == nil {
		t.Fatal("expected unknown provider error")
	}
}
