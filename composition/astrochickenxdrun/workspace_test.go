package astrochickenxdrun

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteConfigIfMissingPreservesExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xd-run.routes")
	if err := writeConfigIfMissing(path, routeConfigTemplate); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeConfigIfMissing(path, routeConfigTemplate); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "custom\n" {
		t.Fatalf("config overwritten: %q", got)
	}
}

func TestCompositionPinsSeparateProfileTools(t *testing.T) {
	if len(ToolSpecs) != 2 {
		t.Fatalf("ToolSpecs has %d entries, want 2", len(ToolSpecs))
	}
	if ToolSpecs[0] == ToolSpecs[1] {
		t.Fatal("probe and dns-txt unexpectedly share one tool spec")
	}
}
