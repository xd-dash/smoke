package main

import (
	"reflect"
	"testing"

	"github.com/xd-dash/smoke/callback"
)

func TestParseArgsInterspersedOptions(t *testing.T) {
	got, err := parseArgs([]string{
		"west", "events", "deployments",
		"--pattern", "worker:*",
		"--into", "axiom", "east", "redis-events",
		"--into", "axiom", "eu", "redis-events",
		"--callback-policy", "fail-fast",
		"--auth-provider", "acl-env",
		"--detached",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile != "west" {
		t.Fatalf("profile = %q", got.Profile)
	}
	if !reflect.DeepEqual(got.Channels, []string{"events", "deployments"}) {
		t.Fatalf("channels = %#v", got.Channels)
	}
	if !reflect.DeepEqual(got.Patterns, []string{"worker:*"}) {
		t.Fatalf("patterns = %#v", got.Patterns)
	}
	wantInto := []intoSpec{
		{Provider: "axiom", Profile: "east", Target: "redis-events"},
		{Provider: "axiom", Profile: "eu", Target: "redis-events"},
	}
	if !reflect.DeepEqual(got.Into, wantInto) {
		t.Fatalf("into = %#v", got.Into)
	}
	if !got.Detached {
		t.Fatal("expected detached")
	}
	if got.Policy != callback.FailFast {
		t.Fatalf("policy = %q", got.Policy)
	}
	if got.AuthProvider != "acl-env" {
		t.Fatalf("auth provider = %q", got.AuthProvider)
	}
}

func TestResolveIntoAxiomAliases(t *testing.T) {
	got, err := resolveInto([]intoSpec{
		{Provider: "axiom", Profile: "east", Target: "one"},
		{Provider: "axiom", Profile: "eu", Target: "two"},
		{Provider: "axiom", Profile: "default", Target: "three"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"axiom://one?profile=axiom-us-east-1.logma.sh",
		"axiom://two?profile=axiom-eu-central-1.logma.sh",
		"axiom://three?profile=axiom.logma.sh",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveInto = %#v, want %#v", got, want)
	}
}

func TestForegroundNeedsNoDestination(t *testing.T) {
	got, err := parseArgs([]string{"west", "events"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Detached {
		t.Fatal("foreground invocation unexpectedly detached")
	}
}

func TestDetachedRequiresDestination(t *testing.T) {
	if _, err := parseArgs([]string{"west", "events", "--detached"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeProfile(t *testing.T) {
	if got := normalizeProfile("west"); got != "west.logma.sh" {
		t.Fatalf("normalizeProfile = %q", got)
	}
	if got := normalizeProfile("prod.us-west1.logma.sh"); got != "prod.us-west1.logma.sh" {
		t.Fatalf("normalizeProfile fqdn = %q", got)
	}
}

func TestLegacyNoStdoutAliasesDetached(t *testing.T) {
	got, err := parseArgs([]string{
		"west", "events", "--no-stdout",
		"--callback", "https://example.com/hook",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Detached {
		t.Fatal("expected compatibility alias to detach")
	}
}
