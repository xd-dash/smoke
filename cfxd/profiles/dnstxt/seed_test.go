package dnstxt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSeedWritesCompleteTerraformRoot(t *testing.T) {
	dst := t.TempDir()
	if err := Seed(dst); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"main.tf", "variables.tf", "outputs.tf"} {
		if _, err := os.Stat(filepath.Join(dst, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
}

func TestSeedRejectsEmptyDestination(t *testing.T) {
	if err := Seed(" "); err == nil {
		t.Fatal("expected empty destination error")
	}
}
