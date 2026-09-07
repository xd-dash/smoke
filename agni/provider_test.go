package agni

import (
	"context"
	"testing"
)

type testProvider struct{}

func (testProvider) RunTerraform(context.Context, TerraformRequest) error { return nil }

func TestCurrentScope(t *testing.T) {
	t.Setenv("SMOKE_ENV", "")
	if got := CurrentScope(); got.Kind != ScopeOutside || got.Environment != "" {
		t.Fatalf("outside scope = %#v", got)
	}
	t.Setenv("SMOKE_ENV", "world")
	if got := CurrentScope(); got.Kind != ScopeEnvironment || got.Environment != "world" {
		t.Fatalf("environment scope = %#v", got)
	}
}

func TestNamesAreSorted(t *testing.T) {
	Register("zz-test-provider", testProvider{})
	Register("aa-test-provider", testProvider{})
	names := Names()
	var aa, zz = -1, -1
	for i, name := range names {
		if name == "aa-test-provider" {
			aa = i
		}
		if name == "zz-test-provider" {
			zz = i
		}
	}
	if aa < 0 || zz < 0 || aa >= zz {
		t.Fatalf("Names() = %#v", names)
	}
}
