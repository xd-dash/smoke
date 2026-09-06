package environment

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSnapshotIsImmutableAndReleasesCanonicalLock(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SMOKE_ENV_DIR", root)
	env, err := Create(context.Background(), "infra")
	if err != nil {
		t.Fatal(err)
	}

	projectA := makeTestModule(t, "a")
	if err := Use(context.Background(), env.Name, projectA); err != nil {
		t.Fatal(err)
	}

	first, err := Snapshot(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if first.WorkFile == env.WorkFile {
		t.Fatal("snapshot reused mutable canonical go.work")
	}
	if first.ToolsDir == env.ToolsDir {
		t.Fatal("snapshot reused mutable canonical tools module")
	}

	work, err := os.ReadFile(first.WorkFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(work), projectA) {
		t.Fatalf("snapshot missing project %q: %s", projectA, work)
	}
	if !strings.Contains(string(work), `"./tools"`) {
		t.Fatalf("snapshot does not point at its copied tools module: %s", work)
	}
	canonicalTools, err := os.ReadFile(filepath.Join(env.ToolsDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	snapshotTools, err := os.ReadFile(filepath.Join(first.ToolsDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if string(canonicalTools) != string(snapshotTools) {
		t.Fatalf("snapshot tools module differs from canonical state\ncanonical:\n%s\nsnapshot:\n%s", canonicalTools, snapshotTools)
	}

	// Snapshot must release the shared lock before returning. A canonical
	// mutation can therefore begin while a child continues using first.WorkFile.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	lock, err := acquireExclusive(ctx, env)
	if err != nil {
		t.Fatalf("canonical environment remained locked after snapshot: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}

	again, err := Snapshot(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if again.Digest != first.Digest || again.WorkFile != first.WorkFile {
		t.Fatalf("unchanged environment did not reuse content-addressed snapshot: first=%#v again=%#v", first, again)
	}

	projectB := makeTestModule(t, "b")
	if err := Use(context.Background(), env.Name, projectB); err != nil {
		t.Fatal(err)
	}
	second, err := Snapshot(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if second.Digest == first.Digest || second.WorkFile == first.WorkFile {
		t.Fatal("canonical mutation did not produce a new workspace snapshot")
	}
	oldWork, err := os.ReadFile(first.WorkFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(oldWork), projectB) {
		t.Fatalf("old snapshot changed after canonical mutation: %s", oldWork)
	}
	newWork, err := os.ReadFile(second.WorkFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(newWork), projectB) {
		t.Fatalf("new snapshot missing canonical mutation: %s", newWork)
	}
}

func TestWorkspaceCommandUsesSnapshotNotCanonicalWorkFile(t *testing.T) {
	t.Setenv("SMOKE_ENV_DIR", t.TempDir())
	env, err := Create(context.Background(), "infra")
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := Snapshot(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	cmd := workspace.Command(context.Background(), env.Dir, "echo", "ok")
	values := map[string]string{}
	for _, item := range cmd.Env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			values[key] = value
		}
	}
	if got := values["GOWORK"]; got != workspace.WorkFile {
		t.Fatalf("GOWORK=%q want snapshot %q", got, workspace.WorkFile)
	}
	if got := values["SMOKE_ENV_WORKSPACE"]; got != workspace.WorkFile {
		t.Fatalf("SMOKE_ENV_WORKSPACE=%q want %q", got, workspace.WorkFile)
	}
}

func makeTestModule(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := "module example.com/" + name + "\n\ngo 1.26\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}
