package astrochicken

import (
	"path/filepath"
	"strings"
	"testing"

	smokeagni "github.com/xd-dash/smoke/agni"
)

func TestWorkspaceDirUsesExplicitOverride(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "probe")
	t.Setenv("SMOKE_ASTROCHICKEN_WORKSPACE", dir)
	got, err := workspaceDir(smokeagni.Scope{Kind: smokeagni.ScopeOutside})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(dir)
	if got != want {
		t.Fatalf("workspaceDir() = %q, want %q", got, want)
	}
}

func TestWorkspaceDirUsesEnvironmentWorkspace(t *testing.T) {
	base := t.TempDir()
	t.Setenv("SMOKE_ASTROCHICKEN_WORKSPACE", "")
	t.Setenv("SMOKE_ENV_WORKSPACE", base)
	got, err := workspaceDir(smokeagni.Scope{Kind: smokeagni.ScopeEnvironment, Environment: "world"})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "agni", "astrochicken")
	if got != want {
		t.Fatalf("workspaceDir() = %q, want %q", got, want)
	}
}

func TestCloneFilesCopiesBytes(t *testing.T) {
	src := map[string][]byte{"main.tf": []byte("a")}
	got := cloneFiles(src)
	got["main.tf"][0] = 'b'
	if string(src["main.tf"]) != "a" {
		t.Fatal("cloneFiles aliased source bytes")
	}
}

func TestRootUsesTwoNode29ServiceAliasesAndPrivateServerlessShadows(t *testing.T) {
	main := string(rootFiles["main.tf"])
	variables := string(rootFiles["variables.tf"])
	outputs := string(rootFiles["outputs.tf"])
	for _, want := range []string{
		`"0"`, `"1"`, `"gateway"`, `"world"`, `./modules/regional-cell`,
		`./modules/cloud-function-v1-http`, `./modules/cloud-function-v2-http`,
		`ALLOW_INTERNAL_ONLY`, `private_ip_google_access = true`,
		`internal_addresses`, `execution_service_ip`, `egress_service_ip`,
		`alias_ip_ranges`, `smoke-execution-service-ip`, `smoke-egress-service-ip`,
	} {
		if !strings.Contains(main, want) {
			t.Fatalf("main.tf missing %q", want)
		}
	}
	if !strings.Contains(variables, `== 29`) {
		t.Fatal("variables.tf does not require an Astrochicken /29")
	}
	for _, want := range []string{`service_addresses`, `shadow_functions`} {
		if !strings.Contains(outputs, want) {
			t.Fatalf("outputs.tf missing %q", want)
		}
	}
}

func TestAstrochickenSelectsRequiredAgniModules(t *testing.T) {
	joined := strings.Join(modules, " ")
	for _, want := range []string{"regional-cell", "regional-internal-addresses", "cloud-function-v1-http", "cloud-function-v2-http"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("module selection missing %q", want)
		}
	}
}
