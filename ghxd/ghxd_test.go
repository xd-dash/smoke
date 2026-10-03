package ghxd

import "testing"

func TestDefaults(t *testing.T) {
	if DefaultEnvironment != "ghxd" {
		t.Fatalf("DefaultEnvironment = %q, want ghxd", DefaultEnvironment)
	}
	want := []string{
		"github.com/dash-xd/github-cdn@go",
		"github.com/dash-xd/github-device-auth/cmd/github-device-auth@main",
		"github.com/xd-dash/smoke/cmd/github-worktree@5a1987d1d17dbf9b7d509e8e960b469baf51a52a",
	}
	if len(ToolSpecs) != len(want) {
		t.Fatalf("ToolSpecs length = %d, want %d", len(ToolSpecs), len(want))
	}
	for i := range want {
		if ToolSpecs[i] != want[i] {
			t.Fatalf("ToolSpecs[%d] = %q, want %q", i, ToolSpecs[i], want[i])
		}
	}
}
