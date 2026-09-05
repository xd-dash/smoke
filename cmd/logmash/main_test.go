package main

import (
	"reflect"
	"testing"

	"github.com/xd-dash/smoke/callback"
)

func TestParseArgsSourceQualified(t *testing.T) {
	got, err := parseArgs([]string{
		"us:west:events",
		"us:west:ratelimiters",
		"us:east:events",
		"us:west:ratelimiters",
		"--pattern", "us:east:worker:*",
		"--into", "axiom", "east", "redis-events",
		"--into", "axiom", "eu", "redis-events",
		"--callback-policy", "fail-fast",
		"--auth-provider", "acl-env",
		"--detached",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantSources := []sourceSelector{
		{Country: "us", Region: "west", Value: "events"},
		{Country: "us", Region: "west", Value: "ratelimiters"},
		{Country: "us", Region: "east", Value: "events"},
		{Country: "us", Region: "west", Value: "ratelimiters"},
		{Country: "us", Region: "east", Value: "worker:*", Pattern: true},
	}
	if !reflect.DeepEqual(got.Sources, wantSources) {
		t.Fatalf("sources = %#v", got.Sources)
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

func TestParseSourceSelector(t *testing.T) {
	got, err := parseSourceSelector("US:West:events", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Country != "us" || got.Region != "west" || got.Value != "events" || got.Pattern {
		t.Fatalf("selector = %#v", got)
	}

	pattern, err := parseSourceSelector("us:east:worker:*", true)
	if err != nil {
		t.Fatal(err)
	}
	if pattern.Country != "us" || pattern.Region != "east" || pattern.Value != "worker:*" || !pattern.Pattern {
		t.Fatalf("pattern = %#v", pattern)
	}
}

func TestSourceSelectorRequiresCountryRegionRelationship(t *testing.T) {
	for _, args := range [][]string{{"west:events"}, {"usa:west:events"}, {"us", "west", "events"}} {
		if _, err := parseArgs(args); err == nil {
			t.Fatalf("expected invalid grammar for %#v", args)
		}
	}
}

func TestSourceProfileHierarchy(t *testing.T) {
	if got := sourceProfile("us", "west"); got != "west.us.logma.sh" {
		t.Fatalf("sourceProfile = %q", got)
	}
	if got := logicalSource("US", "West"); got != "us:west" {
		t.Fatalf("logicalSource = %q", got)
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
	got, err := parseArgs([]string{"us:west:events"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Detached {
		t.Fatal("foreground invocation unexpectedly detached")
	}
}

func TestDetachedRequiresDestination(t *testing.T) {
	if _, err := parseArgs([]string{"us:west:events", "--detached"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestLegacyNoStdoutAliasesDetached(t *testing.T) {
	got, err := parseArgs([]string{
		"us:west:events", "--no-stdout",
		"--callback", "https://example.com/hook",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Detached {
		t.Fatal("expected compatibility alias to detach")
	}
}
