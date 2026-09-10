package smokeapp

import (
	"testing"
	"time"
)

func TestParseGHXDEnvironmentDefault(t *testing.T) {
	name, rest, err := parseGHXDEnvironment([]string{"device", "client"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "ghxd" {
		t.Fatalf("name = %q, want ghxd", name)
	}
	if len(rest) != 2 || rest[0] != "device" || rest[1] != "client" {
		t.Fatalf("rest = %q", rest)
	}
}

func TestParseGHXDEnvironmentExplicit(t *testing.T) {
	name, rest, err := parseGHXDEnvironment([]string{"--env", "operator", "refresh", "--bundle-env", "TOKEN"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "operator" {
		t.Fatalf("name = %q, want operator", name)
	}
	if len(rest) != 3 || rest[0] != "refresh" {
		t.Fatalf("rest = %q", rest)
	}
}

func TestParseGHXDEnvironmentRequiresName(t *testing.T) {
	if _, _, err := parseGHXDEnvironment([]string{"--env"}); err == nil {
		t.Fatal("expected missing environment error")
	}
}

func TestGHXDBundleInputFromEnvironment(t *testing.T) {
	t.Setenv("TOKEN", `{"version":1,"client_id":"client","access_token":"access","refresh_token":"refresh","access_token_expires_at":"2026-09-10T00:00:00Z","refresh_token_expires_at":"2027-03-08T00:00:00Z"}`)
	bundle, margin, err := ghxdBundleInput([]string{"--bundle-env", "TOKEN", "60"})
	if err != nil {
		t.Fatal(err)
	}
	if bundle.ClientID != "client" || margin != time.Minute {
		t.Fatalf("bundle=%#v margin=%s", bundle, margin)
	}
}

func TestGHXDBundleInputRequiresExplicitEnvironment(t *testing.T) {
	if _, _, err := ghxdBundleInput([]string{"TOKEN"}); err == nil {
		t.Fatal("expected explicit --bundle-env requirement")
	}
}
