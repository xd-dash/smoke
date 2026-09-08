package cli

import (
	"context"
	"reflect"
	"testing"

	"github.com/xd-dash/smoke/environment"
	"github.com/xd-dash/smoke/identity"
)

func TestParseEnvInvocation(t *testing.T) {
	name, dir, rest, err := parseEnvInvocation([]string{"infra", "--dir", "./project", "--", "logmash", "us:west:events"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "infra" || dir != "./project" {
		t.Fatalf("name=%q dir=%q", name, dir)
	}
	want := []string{"logmash", "us:west:events"}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("rest=%q want=%q", rest, want)
	}
}

func TestParseEnvInvocationWithoutSeparator(t *testing.T) {
	name, dir, rest, err := parseEnvInvocation([]string{"dev", "go", "tool", "stringer"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "dev" || dir != "" {
		t.Fatalf("name=%q dir=%q", name, dir)
	}
	want := []string{"go", "tool", "stringer"}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("rest=%q want=%q", rest, want)
	}
}

func TestParseEnvInvocationRequiresCommand(t *testing.T) {
	if _, _, _, err := parseEnvInvocation([]string{"infra", "--"}); err == nil {
		t.Fatal("expected missing command error")
	}
}

func TestParseEnvTerraform(t *testing.T) {
	name, dir, args, err := parseEnvTerraform([]string{"astrochicken", "--dir", "/tmp/root", "--", "plan", "-input=false"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "astrochicken" || dir != "/tmp/root" {
		t.Fatalf("name=%q dir=%q", name, dir)
	}
	want := []string{"plan", "-input=false"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args=%#v want=%#v", args, want)
	}
}

func TestParseEnvTerraformRequiresArguments(t *testing.T) {
	if _, _, _, err := parseEnvTerraform([]string{"astrochicken"}); err == nil {
		t.Fatal("expected Terraform argument error")
	}
}

func TestParseGHXDEnvironment(t *testing.T) {
	name, rest, err := parseGHXDEnvironment([]string{"--env", "operator", "refresh", "client", "token"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "operator" {
		t.Fatalf("name=%q", name)
	}
	want := []string{"refresh", "client", "token"}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("rest=%q want=%q", rest, want)
	}
}

func TestParseGHXDEnvironmentDefaults(t *testing.T) {
	name, rest, err := parseGHXDEnvironment([]string{"device", "client"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "ghxd" {
		t.Fatalf("name=%q want ghxd", name)
	}
	want := []string{"device", "client"}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("rest=%q want=%q", rest, want)
	}
}

func TestRuntimeInspectionUsesWorkspaceDirectory(t *testing.T) {
	t.Setenv("SMOKE_ENV", "infra")
	t.Setenv("SMOKE_ENV_WORKSPACE", "/tmp/smoke/env-workspaces/infra/abc123")
	identity.SetComponents("example.com/runtime-json")

	doc := runtimeInspection()
	if doc.Schema != 1 || doc.Kind != "smoke.runtime" {
		t.Fatalf("unexpected document: %#v", doc)
	}
	if doc.Runtime.Environment != "infra" {
		t.Fatalf("environment=%q", doc.Runtime.Environment)
	}
	if doc.Runtime.WorkspaceDigest != "abc123" {
		t.Fatalf("workspace digest=%q", doc.Runtime.WorkspaceDigest)
	}
	if doc.Runtime.Workspace != "/tmp/smoke/env-workspaces/infra/abc123" {
		t.Fatalf("workspace=%q", doc.Runtime.Workspace)
	}
}

func TestEnvironmentInspectionCreatesRuntimeSnapshot(t *testing.T) {
	t.Setenv("SMOKE_ENV_DIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if _, err := environment.Create(context.Background(), "infra"); err != nil {
		t.Fatal(err)
	}
	doc, err := environmentInspection(context.Background(), "infra")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Schema != 1 || doc.Kind != "smoke.environment" {
		t.Fatalf("unexpected document: %#v", doc)
	}
	if doc.Environment.Name != "infra" {
		t.Fatalf("name=%q", doc.Environment.Name)
	}
	if doc.RuntimeSnapshot.Digest == "" || doc.RuntimeSnapshot.Work == "" || doc.RuntimeSnapshot.Tools == "" {
		t.Fatalf("incomplete runtime snapshot: %#v", doc.RuntimeSnapshot)
	}
}
