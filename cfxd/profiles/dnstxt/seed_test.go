package dnstxt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSeedWritesIndependentTerraformRoot(t *testing.T) {
	dst := t.TempDir()
	if err := Seed(dst); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.tf", "variables.tf", "outputs.tf"} {
		if _, err := os.Stat(filepath.Join(dst, name)); err != nil {
			t.Fatalf("seeded %s: %v", name, err)
		}
	}
}
