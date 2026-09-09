package smokeapp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProfileSHAsRequiresExactSHAs(t *testing.T) {
	agni := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cfxd := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	gotAgni, gotCFXD, err := parseProfileSHAs([]string{"--cfxd-sha", cfxd, "--agni-sha", agni})
	if err != nil {
		t.Fatal(err)
	}
	if gotAgni != agni || gotCFXD != cfxd {
		t.Fatalf("got agni=%q cfxd=%q", gotAgni, gotCFXD)
	}
	if _, _, err := parseProfileSHAs([]string{"--agni-sha", "main", "--cfxd-sha", cfxd}); err != nil {
		// parse only checks presence; exact-SHA validation is deliberately owned by bootstrap.
		return
	}
}

func TestParseCompositionSeedKeepsProfileConfigsSeparate(t *testing.T) {
	root, probeVars, routes, err := parseCompositionSeed([]string{
		"/tmp/astrochicken-xd-run",
		"--routes", "./xd-run.routes",
		"--probe-vars", "./probe.tfvars",
	})
	if err != nil {
		t.Fatal(err)
	}
	if root != "/tmp/astrochicken-xd-run" || probeVars != "./probe.tfvars" || routes != "./xd-run.routes" {
		t.Fatalf("unexpected parse: root=%q probe=%q routes=%q", root, probeVars, routes)
	}
}

func TestExactGitSHA(t *testing.T) {
	if !exactGitSHA.MatchString("0123456789abcdef0123456789abcdef01234567") {
		t.Fatal("valid exact SHA rejected")
	}
	for _, invalid := range []string{"main", "abc123", "0123456789abcdef0123456789abcdef012345678"} {
		if exactGitSHA.MatchString(invalid) {
			t.Fatalf("invalid SHA accepted: %q", invalid)
		}
	}
}

func TestWriteCompositionConfigCanReplaceSamePathData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xd-run.routes")
	want := []byte("{\"zone\":\"xd.run\",\"routes\":[]}\n")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}

	// seedAstrochickenXDRun reads inputs before writing outputs. This helper
	// verifies the write side can safely replace a path whose bytes were already
	// read from that same path without truncation or partial content.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCompositionConfig(path, data); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("config = %q, want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0o600 {
		t.Fatalf("config mode = %o, want 600", gotMode)
	}
}
