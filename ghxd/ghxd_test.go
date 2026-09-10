package ghxd

import (
	"context"
	"testing"
)

func TestDefaults(t *testing.T) {
	if DefaultEnvironment != "ghxd" {
		t.Fatalf("DefaultEnvironment = %q, want ghxd", DefaultEnvironment)
	}
	want := []string{
		"github.com/dash-xd/github-cdn@go",
		"github.com/dash-xd/github-device-auth/cmd/github-device-auth@main",
		"github.com/xd-dash/smoke/cmd/github-worktree@599b3ffb7b0437ed10c80e8677d15a40e954901c",
	}
	if len(ToolSpecs) != len(want) {
		t.Fatalf("ToolSpecs length = %d, want %d", len(ToolSpecs), len(want))
	}
	for i := range want {
		if ToolSpecs[i] != want[i] {
			t.Fatalf("ToolSpecs[%d] = %q, want %q", i, ToolSpecs[i], want[i])
		}
	}
	if _, ok := Presets[ProbotRuntimePreset]; !ok {
		t.Fatalf("missing optional %q preset", ProbotRuntimePreset)
	}
}

func TestProbotRuntimeIsOptional(t *testing.T) {
	t.Setenv("SMOKE_ENV_DIR", t.TempDir())
	if _, err := Bootstrap(context.Background(), "test-ghxd"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := ProbotRuntime("test-ghxd"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("default ghxd bootstrap unexpectedly installed probot-runtime")
	}
}
