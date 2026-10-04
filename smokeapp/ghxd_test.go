package smokeapp

import "testing"

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
	name, rest, err := parseGHXDEnvironment([]string{"--env", "operator", "refresh", "client", "token"})
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

func TestParseGHXDBootstrapPreset(t *testing.T) {
	name, presets, err := parseGHXDBootstrap([]string{"operator", "--preset", "probot-runtime"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "operator" || len(presets) != 1 || presets[0] != "probot-runtime" {
		t.Fatalf("name=%q presets=%q", name, presets)
	}
}

func TestParseGHXDBootstrapDefaultWithPreset(t *testing.T) {
	name, presets, err := parseGHXDBootstrap([]string{"--preset", "probot-runtime"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "" || len(presets) != 1 || presets[0] != "probot-runtime" {
		t.Fatalf("name=%q presets=%q", name, presets)
	}
}
