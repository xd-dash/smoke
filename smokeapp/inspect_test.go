package smokeapp

import (
	"context"
	"testing"

	"github.com/xd-dash/smoke/environment"
	"github.com/xd-dash/smoke/identity"
)

func TestInspectCommandRuns(t *testing.T) {
	identity.RegisterComponent("example.com/inspect")
	if err := Run([]string{"inspect"}); err != nil { t.Fatal(err) }
	if err := Run([]string{"inspect", "--json"}); err != nil { t.Fatal(err) }
}

func TestRuntimeInspectionDocument(t *testing.T) {
	t.Setenv("SMOKE_ENV", "infra")
	t.Setenv("SMOKE_ENV_WORKSPACE", "/tmp/smoke/workspaces/abc123/go.work")
	identity.RegisterComponent("example.com/runtime-json")
	doc := runtimeInspection()
	if doc.Schema != 1 { t.Fatalf("schema = %d", doc.Schema) }
	if doc.Kind != "smoke.runtime" { t.Fatalf("kind = %q", doc.Kind) }
	if doc.Composition.Digest == "" { t.Fatal("missing composition digest") }
	if len(doc.Composition.Components) == 0 { t.Fatal("missing components") }
	if doc.Runtime.Environment != "infra" { t.Fatalf("environment = %q", doc.Runtime.Environment) }
	if doc.Runtime.WorkspaceDigest != "abc123" { t.Fatalf("workspace digest = %q", doc.Runtime.WorkspaceDigest) }
	if doc.Runtime.Workspace != "/tmp/smoke/workspaces/abc123/go.work" { t.Fatalf("workspace = %q", doc.Runtime.Workspace) }
}

func TestEnvInspectCreatesRuntimeSnapshot(t *testing.T) {
	t.Setenv("SMOKE_ENV_DIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if _, err := environment.Create(context.Background(), "infra"); err != nil { t.Fatal(err) }
	if err := Run([]string{"env", "inspect", "infra"}); err != nil { t.Fatal(err) }
	if err := Run([]string{"env", "inspect", "infra", "--json"}); err != nil { t.Fatal(err) }

	doc, err := environmentInspection(context.Background(), "infra")
	if err != nil { t.Fatal(err) }
	if doc.Schema != 1 { t.Fatalf("schema = %d", doc.Schema) }
	if doc.Kind != "smoke.environment" { t.Fatalf("kind = %q", doc.Kind) }
	if doc.Environment.Name != "infra" { t.Fatalf("name = %q", doc.Environment.Name) }
	if doc.RuntimeSnapshot.Digest == "" { t.Fatal("missing workspace digest") }
	if doc.RuntimeSnapshot.Work == "" || doc.RuntimeSnapshot.Tools == "" { t.Fatal("missing workspace paths") }
}

func TestInspectionFlagParsing(t *testing.T) {
	if jsonOutput, err := parseJSONOnlyFlag([]string{"--json"}, "smoke inspect [--json]"); err != nil || !jsonOutput {
		t.Fatalf("json flag: output=%v err=%v", jsonOutput, err)
	}
	name, jsonOutput, err := parseEnvironmentInspectArgs([]string{"infra", "--json"})
	if err != nil || name != "infra" || !jsonOutput { t.Fatalf("env inspect args: name=%q output=%v err=%v", name, jsonOutput, err) }
	name, jsonOutput, err = parseEnvironmentInspectArgs([]string{"--json", "infra"})
	if err != nil || name != "infra" || !jsonOutput { t.Fatalf("env inspect prefix args: name=%q output=%v err=%v", name, jsonOutput, err) }
}
