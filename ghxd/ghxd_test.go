package ghxd

import "testing"

func TestDefaults(t *testing.T) {
	if DefaultEnvironment != "ghxd" {
		t.Fatalf("DefaultEnvironment = %q, want ghxd", DefaultEnvironment)
	}
	want := []string{
		"github.com/dash-xd/github-cdn@6c00e9533d91906c97da7ebfb262104466da27ed",
		"github.com/dash-xd/github-device-auth/cmd/github-device-auth@113339509308abf42efce2e0305f67c65f6910be",
		"github.com/xd-dash/smoke/cmd/github-worktree@dd0a7471383b4266c0368b089433dac0a4e3b3b5",
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
