package selfbuild

import (
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

func TestRenderMainImportsSelectedComponents(t *testing.T) {
	source := renderMain(Manifest{Components: []string{
		DefaultLogmash,
		"example.com/optional/provider",
	}})
	for _, want := range []string{
		`"github.com/xd-dash/smoke/smokeapp"`,
		`_ "github.com/xd-dash/smoke/cmd/logmash"`,
		`_ "example.com/optional/provider"`,
		`smokeapp.Main(os.Args[1:])`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("generated main missing %q:\n%s", want, source)
		}
	}
}

func TestRenderGoModPinsRequestedSmokeVersion(t *testing.T) {
	t.Setenv("SMOKE_MODULE_VERSION", "v1.2.3")
	mod := renderGoMod()
	if !strings.Contains(mod, "require github.com/xd-dash/smoke v1.2.3") {
		t.Fatalf("go.mod = %q", mod)
	}
}
