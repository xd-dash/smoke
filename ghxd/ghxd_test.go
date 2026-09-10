package ghxd

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

var exactToolRevision = regexp.MustCompile(`@[0-9a-f]{40}$`)

func TestDefaults(t *testing.T) {
	if DefaultEnvironment != "ghxd" {
		t.Fatalf("DefaultEnvironment = %q, want ghxd", DefaultEnvironment)
	}
	want := []string{
		"github.com/dash-xd/github-cdn@6c00e9533d91906c97da7ebfb262104466da27ed",
		"github.com/dash-xd/github-device-auth/cmd/github-device-auth@85a1a81050198e429611cd58d336090868f0adfb",
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

func TestCredentialBundleRoundTrip(t *testing.T) {
	now := time.Date(2026, 9, 9, 16, 0, 0, 0, time.UTC)
	bundle, err := NewCredentialBundle("client", DeviceTokenResponse{
		AccessToken: "access", RefreshToken: "refresh",
		ExpiresIn: 8 * 60 * 60, RefreshTokenExpiresIn: 180 * 24 * 60 * 60,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	body, err := MarshalCredentialBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseCredentialBundle(body)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != bundle {
		t.Fatalf("parsed = %#v, want %#v", parsed, bundle)
	}
	if !parsed.AccessFresh(now.Add(7*time.Hour), 5*time.Minute) {
		t.Fatal("expected access token to be fresh")
	}
	if parsed.AccessFresh(now.Add(7*time.Hour+56*time.Minute), 5*time.Minute) {
		t.Fatal("expected access token inside margin to be stale")
	}
}

func TestCredentialBundleRejectsIncompleteAndUnknownState(t *testing.T) {
	valid := `{"version":1,"client_id":"client","access_token":"access","refresh_token":"refresh","access_token_expires_at":"2026-09-10T00:00:00Z","refresh_token_expires_at":"2027-03-08T00:00:00Z"}`
	if _, err := ParseCredentialBundle([]byte(valid)); err != nil {
		t.Fatalf("valid bundle rejected: %v", err)
	}
	if _, err := ParseCredentialBundle([]byte(`{"version":1,"client_id":"client"}`)); err == nil {
		t.Fatal("expected incomplete bundle rejection")
	}
	if _, err := ParseCredentialBundle([]byte(valid[:len(valid)-1] + `,"provider_state":"bad"}`)); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}
