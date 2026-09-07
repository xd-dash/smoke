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

func TestTerraformRootUsesOrdinaryHCLFiles(t *testing.T) {
	files, err := terraformFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.tf", "variables.tf", "outputs.tf"} {
		if len(files[name]) == 0 {
			t.Fatalf("Terraform root missing %s", name)
		}
	}

	main := string(files["main.tf"])
	variables := string(files["variables.tf"])
	outputs := string(files["outputs.tf"])
	for _, want := range []string{
		`"0"`, `"1"`, `"gateway"`, `"world"`, `./modules/regional-cell`,
		`./modules/cloud-function-v1-http`, `./modules/cloud-function-v2-http`,
		`ALLOW_INTERNAL_ONLY`, `private_ip_google_access = true`,
		`execution_service_ip`, `egress_service_ip`, `alias_ip_ranges`,
	} {
		if !strings.Contains(main, want) {
			t.Fatalf("main.tf missing %q", want)
		}
	}
	if !strings.Contains(variables, `== 29`) {
		t.Fatal("variables.tf does not require an Astrochicken /29")
	}
	if !strings.Contains(outputs, `service_addresses`) || !strings.Contains(outputs, `shadow_functions`) {
		t.Fatal("outputs.tf is missing Astrochicken service/shadow outputs")
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
