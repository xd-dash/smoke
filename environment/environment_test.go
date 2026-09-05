package environment

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateProducesWorkspaceAndToolsModule(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SMOKE_ENV_DIR", root)

	env, err := Create(context.Background(), "infra")
	if err != nil {
		t.Fatal(err)
	}

	work, err := os.ReadFile(env.WorkFile)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(work); !strings.Contains(got, "use ./tools") {
		t.Fatalf("go.work missing tools module: %q", got)
	}

	mod, err := os.ReadFile(filepath.Join(env.ToolsDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(mod); !strings.Contains(got, "module smoke.local/env/infra/tools") {
		t.Fatalf("unexpected tools go.mod: %q", got)
	}
}

func TestListIsSortedAndOnlyIncludesWorkspaces(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SMOKE_ENV_DIR", root)
	for _, name := range []string{"zeta", "alpha"} {
		if _, err := Create(context.Background(), name); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "not-an-env"), 0o700); err != nil {
		t.Fatal(err)
	}

	envs, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(envs) != 2 || envs[0].Name != "alpha" || envs[1].Name != "zeta" {
		t.Fatalf("unexpected environments: %#v", envs)
	}
}

func TestActivateRestoresProcessEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SMOKE_ENV_DIR", root)
	t.Setenv("GOWORK", "old.work")
	t.Setenv("SMOKE_ENV", "old")
	env, err := Create(context.Background(), "infra")
	if err != nil {
		t.Fatal(err)
	}

	restore, err := Activate(env)
	if err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("GOWORK"); got != env.WorkFile {
		t.Fatalf("GOWORK=%q want %q", got, env.WorkFile)
	}
	if got := os.Getenv("SMOKE_ENV"); got != "infra" {
		t.Fatalf("SMOKE_ENV=%q", got)
	}
	restore()
	if got := os.Getenv("GOWORK"); got != "old.work" {
		t.Fatalf("restored GOWORK=%q", got)
	}
	if got := os.Getenv("SMOKE_ENV"); got != "old" {
		t.Fatalf("restored SMOKE_ENV=%q", got)
	}
}

func TestResolveRejectsUnsafeName(t *testing.T) {
	t.Setenv("SMOKE_ENV_DIR", t.TempDir())
	if _, err := Resolve("../escape"); err == nil {
		t.Fatal("expected invalid environment name")
	}
}
