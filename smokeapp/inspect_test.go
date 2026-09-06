package smokeapp

import (
	"context"
	"testing"

	"github.com/xd-dash/smoke/environment"
	"github.com/xd-dash/smoke/identity"
)

func TestInspectCommandRuns(t *testing.T) {
	identity.RegisterComponent("example.com/inspect")
	if err := Run([]string{"inspect"}); err != nil { t.Fatal(err) }
}

func TestEnvInspectCreatesRuntimeSnapshot(t *testing.T) {
	t.Setenv("SMOKE_ENV_DIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if _, err := environment.Create(context.Background(), "infra"); err != nil { t.Fatal(err) }
	if err := Run([]string{"env", "inspect", "infra"}); err != nil { t.Fatal(err) }
}
