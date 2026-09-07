package environment

import "testing"

func TestDefaultToolSpecs(t *testing.T) {
	want := []string{
		"github.com/dash-xd/github-cdn@go",
		"github.com/dash-xd/github-device-auth/cmd/github-device-auth@main",
		"github.com/dash-xd/agni@main",
	}
	if len(DefaultToolSpecs) != len(want) {
		t.Fatalf("DefaultToolSpecs length = %d, want %d", len(DefaultToolSpecs), len(want))
	}
	for i := range want {
		if DefaultToolSpecs[i] != want[i] {
			t.Fatalf("DefaultToolSpecs[%d] = %q, want %q", i, DefaultToolSpecs[i], want[i])
		}
	}
}
