package ghxd

import "testing"

func TestDefaults(t *testing.T) {
	if DefaultEnvironment != "ghxd" {
		t.Fatalf("DefaultEnvironment = %q, want ghxd", DefaultEnvironment)
	}
	want := []string{
		"github.com/dash-xd/github-cdn@go",
		"github.com/dash-xd/github-device-auth/cmd/github-device-auth@main",
		"github.com/xd-dash/smoke/cmd/github-worktree@a03ad841b9d8a2c5ef92759312ae4bdc72b17a7f",
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
