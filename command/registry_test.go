package command

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitCommandRegistrationWins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, executableName("logmash"))
	if err := os.WriteFile(path, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SMOKE_COMMAND_LOGMASH", path)

	registry, err := New(Command{Name: "logmash"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := registry.Resolve(context.Background(), "logmash")
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("Resolve() = %q, want %q", got, path)
	}
}

func TestUnknownCommandRejected(t *testing.T) {
	registry, err := New(Command{Name: "logmash"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Resolve(context.Background(), "other"); err == nil {
		t.Fatal("expected unregistered command error")
	}
}
