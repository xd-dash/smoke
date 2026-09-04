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
		"-c", "https://example.com/hook",
		"--callback-policy", "fail-fast",
		"--auth-provider", "acl-env",
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
	if !reflect.DeepEqual(got.Callbacks, []string{"https://example.com/hook"}) {
		t.Fatalf("callbacks = %#v", got.Callbacks)
	}
	if got.Policy != callback.FailFast {
		t.Fatalf("policy = %q", got.Policy)
	}
	if got.AuthProvider != "acl-env" {
		t.Fatalf("auth provider = %q", got.AuthProvider)
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

func TestNoStdoutRequiresCallback(t *testing.T) {
	if _, err := parseArgs([]string{"west", "events", "--no-stdout"}); err == nil {
		t.Fatal("expected error")
	}
}
