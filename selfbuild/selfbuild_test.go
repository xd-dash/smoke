package selfbuild

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestWithAddedAndRemoved(t *testing.T) {
	manifest := Manifest{Components: []string{DefaultLogmash}}
	manifest = WithAdded(manifest, "example.com/optional/provider")
	manifest = WithAdded(manifest, "example.com/optional/provider")
	if len(manifest.Components) != 2 {
		t.Fatalf("components = %#v", manifest.Components)
	}
	manifest = WithRemoved(manifest, "example.com/optional/provider")
	if len(manifest.Components) != 1 || manifest.Components[0] != DefaultLogmash {
		t.Fatalf("components after remove = %#v", manifest.Components)
	}
}

func TestRenderMainImportsAndEmbedsSelectedComponents(t *testing.T) {
	source := renderMain(Manifest{Components: []string{
		"example.com/optional/provider",
		DefaultLogmash,
		"example.com/optional/provider",
	}})
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", source, parser.AllErrors); err != nil {
		t.Fatalf("generated main is invalid Go: %v\n%s", err, source)
	}
	for _, want := range []string{
		`"github.com/xd-dash/smoke/identity"`,
		`"github.com/xd-dash/smoke/smokeapp"`,
		`_ "github.com/xd-dash/smoke/cmd/logmash"`,
		`_ "example.com/optional/provider"`,
		`identity.SetComponents(`,
		`"github.com/xd-dash/smoke/cmd/logmash",`,
		`"example.com/optional/provider",`,
		`smokeapp.Main(os.Args[1:])`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("generated main missing %q:\n%s", want, source)
		}
	}
	providerLine := "\t\t\"example.com/optional/provider\","
	logmashLine := "\t\t\"github.com/xd-dash/smoke/cmd/logmash\","
	if got := strings.Count(source, providerLine); got != 1 {
		t.Fatalf("generated identity contains optional provider %d times:\n%s", got, source)
	}
	provider := strings.Index(source, providerLine)
	logmash := strings.Index(source, logmashLine)
	if provider < 0 || logmash < 0 || provider > logmash {
		t.Fatalf("generated identity is not normalized/sorted:\n%s", source)
	}
}

func TestRenderMainEmbedsEmptyComposition(t *testing.T) {
	source := renderMain(Manifest{})
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", source, parser.AllErrors); err != nil {
		t.Fatalf("generated empty main is invalid Go: %v\n%s", err, source)
	}
	if !strings.Contains(source, "identity.SetComponents(\n\t)") {
		t.Fatalf("generated empty composition missing authoritative identity call:\n%s", source)
	}
}

func TestRenderGoModPinsRequestedSmokeVersion(t *testing.T) {
	t.Setenv("SMOKE_MODULE_VERSION", "v1.2.3")
	mod := renderGoMod()
	if !strings.Contains(mod, "require github.com/xd-dash/smoke v1.2.3") {
		t.Fatalf("go.mod = %q", mod)
	}
}

func TestEnsureGoModUsesMountedLocalSmokeSource(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go toolchain unavailable")
	}
	local := t.TempDir()
	if err := os.WriteFile(filepath.Join(local, "go.mod"), []byte("module github.com/xd-dash/smoke\\n\\ngo 1.26\\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SMOKE_LOCAL_SOURCE", local)
	t.Setenv("SMOKE_MODULE_VERSION", "")
	composition := t.TempDir()
	if err := ensureGoMod(context.Background(), composition, goBin); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(composition, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "replace github.com/xd-dash/smoke => "+local) {
		t.Fatalf("composition did not select local Smoke source: %s", data)
	}
}
