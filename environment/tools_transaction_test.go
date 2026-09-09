package environment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAddToolsRollsBackManifestsOnFailure(t *testing.T) {
	t.Setenv("SMOKE_ENV_DIR", t.TempDir())
	env, err := Create(context.Background(), "infra")
	if err != nil {
		t.Fatal(err)
	}
	modPath := filepath.Join(env.ToolsDir, "go.mod")
	sumPath := filepath.Join(env.ToolsDir, "go.sum")
	modBefore, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sumPath, []byte("original sum\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sumBefore, err := os.ReadFile(sumPath)
	if err != nil {
		t.Fatal(err)
	}

	calls := 0
	runner := func(_ context.Context, dir, gowork string, args ...string) error {
		calls++
		if dir != env.ToolsDir || gowork != "off" {
			t.Fatalf("runner scope dir=%q gowork=%q", dir, gowork)
		}
		if err := os.WriteFile(modPath, []byte("mutated mod\n"), 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(sumPath, []byte("mutated sum\n"), 0o600); err != nil {
			return err
		}
		if calls == 2 {
			return errors.New("synthetic second-tool failure")
		}
		return nil
	}

	if err := addToolsWithRunner(context.Background(), env.Name, []string{"example.com/one@v1.0.0", "example.com/two@v1.0.0"}, runner); err == nil {
		t.Fatal("expected batch failure")
	}
	modAfter, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatal(err)
	}
	sumAfter, err := os.ReadFile(sumPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(modAfter) != string(modBefore) {
		t.Fatalf("go.mod not restored:\n%s", modAfter)
	}
	if string(sumAfter) != string(sumBefore) {
		t.Fatalf("go.sum not restored:\n%s", sumAfter)
	}
}

func TestAddToolsRemovesNewGoSumOnRollback(t *testing.T) {
	t.Setenv("SMOKE_ENV_DIR", t.TempDir())
	env, err := Create(context.Background(), "infra")
	if err != nil {
		t.Fatal(err)
	}
	sumPath := filepath.Join(env.ToolsDir, "go.sum")

	runner := func(_ context.Context, _, _ string, _ ...string) error {
		if err := os.WriteFile(sumPath, []byte("new sum\n"), 0o600); err != nil {
			return err
		}
		return errors.New("synthetic failure")
	}
	if err := addToolsWithRunner(context.Background(), env.Name, []string{"example.com/tool@v1.0.0"}, runner); err == nil {
		t.Fatal("expected batch failure")
	}
	if _, err := os.Stat(sumPath); !os.IsNotExist(err) {
		t.Fatalf("new go.sum survived rollback: %v", err)
	}
}
