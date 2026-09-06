package identity

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSetComponentsIsAuthoritativeSortedAndDeduplicated(t *testing.T) {
	SetComponents("example.com/stale")
	RegisterComponent("example.com/init-only")
	SetComponents("example.com/z", "example.com/a", "example.com/a")

	want := []string{"example.com/a", "example.com/z"}
	if got := Components(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Components()=%v want %v", got, want)
	}
	if got := CompositionDigest(); got == "" {
		t.Fatal("empty composition digest")
	}
	if got, again := CompositionDigest(), CompositionDigest(); got != again {
		t.Fatalf("digest changed: %s != %s", got, again)
	}
}

func TestRegisterComponentRemainsCompatibilityAdditive(t *testing.T) {
	SetComponents("example.com/a")
	RegisterComponent("example.com/z")
	RegisterComponent("example.com/z")
	want := []string{"example.com/a", "example.com/z"}
	if got := Components(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Components()=%v want %v", got, want)
	}
}

func TestWorkspaceDigestFromSnapshotPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "abc123", "go.work")
	t.Setenv("SMOKE_ENV_WORKSPACE", path)
	if got := WorkspaceDigest(); got != "abc123" {
		t.Fatalf("WorkspaceDigest()=%q", got)
	}
}
