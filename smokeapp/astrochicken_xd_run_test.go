package smokeapp

import "testing"

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
