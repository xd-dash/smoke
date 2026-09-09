package ghxd

import (
	"regexp"
	"strings"
	"testing"
)

var exactToolRevision = regexp.MustCompile(`@[0-9a-f]{40}$`)

func TestDefaults(t *testing.T) {
	if DefaultEnvironment != "ghxd" {
		t.Fatalf("DefaultEnvironment = %q, want ghxd", DefaultEnvironment)
	}
	want := []string{
		"github.com/dash-xd/github-cdn@6c00e9533d91906c97da7ebfb262104466da27ed",
		"github.com/dash-xd/github-device-auth/cmd/github-device-auth@113339509308abf42efce2e0305f67c65f6910be",
		"github.com/xd-dash/smoke/cmd/github-worktree@599b3ffb7b0437ed10c80e8677d15a40e954901c",
	}
	if len(ToolSpecs) != len(want) {
		t.Fatalf("ToolSpecs length = %d, want %d", len(ToolSpecs), len(want))
	}
	for i := range want {
		if ToolSpecs[i] != want[i] {
			t.Fatalf("ToolSpecs[%d] = %q, want %q", i, ToolSpecs[i], want[i])
		}
		if !exactToolRevision.MatchString(ToolSpecs[i]) {
			t.Fatalf("ToolSpecs[%d] is not pinned to an exact commit: %q", i, ToolSpecs[i])
		}
		if strings.HasSuffix(ToolSpecs[i], "@main") || strings.HasSuffix(ToolSpecs[i], "@go") {
			t.Fatalf("ToolSpecs[%d] uses mutable role/ref authority: %q", i, ToolSpecs[i])
		}
	}
}
