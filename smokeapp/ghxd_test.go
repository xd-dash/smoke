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

func TestParseGHXDSyncDefaults(t *testing.T) {
	repo, secret, err := parseGHXDSync([]string{"--repo", "xd-dash/huram-abi-master"})
	if err != nil {
		t.Fatal(err)
	}
	if repo != "xd-dash/huram-abi-master" {
		t.Fatalf("repo = %q", repo)
	}
	if secret != "HURAM_GITHUB_DEVICE_TOKEN" {
		t.Fatalf("secret = %q", secret)
	}
}

func TestParseGHXDSyncExplicitSecret(t *testing.T) {
	repo, secret, err := parseGHXDSync([]string{
		"--repo", "xd-dash/huram-abi-master",
		"--secret", "GH_PRIMARY",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo != "xd-dash/huram-abi-master" || secret != "GH_PRIMARY" {
		t.Fatalf("got repo=%q secret=%q", repo, secret)
	}
}

func TestParseGHXDSyncRejectsRecoverySecretFlag(t *testing.T) {
	if _, _, err := parseGHXDSync([]string{
		"--repo", "xd-dash/huram-abi-master",
		"--recovery-secret", "OLD_RECOVERY",
	}); err == nil {
		t.Fatal("expected obsolete recovery-secret flag to be rejected")
	}
}
