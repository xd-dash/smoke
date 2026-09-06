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

func TestUseAndDropUseDelegateToGoWorkspace(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SMOKE_ENV_DIR", root)
	env, err := Create(context.Background(), "infra")
	if err != nil {
		t.Fatal(err)
	}

	project := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.com/project\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Use(context.Background(), "infra", project); err != nil {
		t.Fatal(err)
	}
	work, err := os.ReadFile(env.WorkFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(work), project) {
		t.Fatalf("go.work does not contain project %q: %s", project, work)
	}

	if err := DropUse(context.Background(), "infra", project); err != nil {
		t.Fatal(err)
	}
	work, err = os.ReadFile(env.WorkFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(work), project) {
		t.Fatalf("go.work still contains dropped project %q: %s", project, work)
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

func TestCommandScopesEnvironmentToChild(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SMOKE_ENV_DIR", root)
	t.Setenv("GOWORK", "parent.work")
	t.Setenv("SMOKE_ENV", "parent")
	env, err := Create(context.Background(), "infra")
	if err != nil {
		t.Fatal(err)
	}

	cmd := Command(context.Background(), env, env.Dir, "echo", "ok")
	values := make(map[string]string)
	for _, item := range cmd.Env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			values[key] = value
		}
	}
	if got := values["GOWORK"]; got != env.WorkFile {
		t.Fatalf("child GOWORK=%q want %q", got, env.WorkFile)
	}
	if got := values["SMOKE_ENV"]; got != "infra" {
		t.Fatalf("child SMOKE_ENV=%q", got)
	}
	if got := os.Getenv("GOWORK"); got != "parent.work" {
		t.Fatalf("parent GOWORK mutated: %q", got)
	}
	if got := os.Getenv("SMOKE_ENV"); got != "parent" {
		t.Fatalf("parent SMOKE_ENV mutated: %q", got)
	}
}

func TestResolveRejectsUnsafeName(t *testing.T) {
	t.Setenv("SMOKE_ENV_DIR", t.TempDir())
	if _, err := Resolve("../escape"); err == nil {
		t.Fatal("expected invalid environment name")
	}
}
