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

func TestSeedRemovesStaleProfileSourceButPreservesRuntimeArtifacts(t *testing.T) {
	dst := t.TempDir()
	stale := filepath.Join(dst, "removed-profile-file.tf")
	generated := filepath.Join(dst, "routes.auto.tfvars.json")
	state := filepath.Join(dst, "terraform.tfstate")
	for path, body := range map[string]string{
		stale:     "resource \"null_resource\" \"stale\" {}\n",
		generated: "{\"records\":{}}\n",
		state:     "{}\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := Seed(dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale profile source still exists: %v", err)
	}
	for _, path := range []string{generated, state} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("runtime artifact %s was not preserved: %v", path, err)
		}
	}
}
