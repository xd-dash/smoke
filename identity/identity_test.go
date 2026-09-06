package identity

import (
	"path/filepath"
	"testing"
)

func TestComponentIdentityIsSortedDeduplicatedAndStable(t *testing.T) {
	RegisterComponent("example.com/z")
	RegisterComponent("example.com/a")
	RegisterComponent("example.com/a")
	components := Components()
	if len(components) < 2 { t.Fatalf("components=%v", components) }
	var seenA, seenZ bool
	for _, component := range components {
		if component == "example.com/a" { seenA = true }
		if component == "example.com/z" { seenZ = true }
	}
	if !seenA || !seenZ { t.Fatalf("components=%v", components) }
	if got := CompositionDigest(); got == "" { t.Fatal("empty composition digest") }
	if got, again := CompositionDigest(), CompositionDigest(); got != again { t.Fatalf("digest changed: %s != %s", got, again) }
}

func TestWorkspaceDigestFromSnapshotPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "abc123", "go.work")
	t.Setenv("SMOKE_ENV_WORKSPACE", path)
	if got := WorkspaceDigest(); got != "abc123" { t.Fatalf("WorkspaceDigest()=%q", got) }
}
