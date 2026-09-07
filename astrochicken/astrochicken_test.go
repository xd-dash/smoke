package astrochicken

import (
	"path/filepath"
	"strings"
	"testing"

	smokeagni "github.com/xd-dash/smoke/agni"
)

func TestWorkspaceDirUsesExplicitOverride(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "probe")
	t.Setenv("SMOKE_ASTROCHICKEN_WORKSPACE", dir)
	got, err := workspaceDir(smokeagni.Scope{Kind: smokeagni.ScopeOutside})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(dir)
	if got != want {
		t.Fatalf("workspaceDir() = %q, want %q", got, want)
	}
}

func TestWorkspaceDirUsesEnvironmentWorkspace(t *testing.T) {
	base := t.TempDir()
	t.Setenv("SMOKE_ASTROCHICKEN_WORKSPACE", "")
	t.Setenv("SMOKE_ENV_WORKSPACE", base)
	got, err := workspaceDir(smokeagni.Scope{Kind: smokeagni.ScopeEnvironment, Environment: "world"})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "agni", "astrochicken")
	if got != want {
		t.Fatalf("workspaceDir() = %q, want %q", got, want)
	}
}

func TestCloneFilesCopiesBytes(t *testing.T) {
	src := map[string][]byte{"main.tf": []byte("a")}
	got := cloneFiles(src)
	got["main.tf"][0] = 'b'
	if string(src["main.tf"]) != "a" {
		t.Fatal("cloneFiles aliased source bytes")
	}
}

func TestRootUsesTwoRepresentativeSlots(t *testing.T) {
	main := string(rootFiles["main.tf"])
	for _, want := range []string{`"0"`, `"1"`, `"gateway"`, `"world"`, `./modules/regional-cell`} {
		if !strings.Contains(main, want) {
			t.Fatalf("main.tf missing %q", want)
		}
	}
}
