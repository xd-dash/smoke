package session

import (
	"path/filepath"
	"testing"

	"github.com/xd-dash/smoke/identity"
)

func TestBeginCapturesRuntimeIdentity(t *testing.T) {
	t.Setenv("SMOKE_SESSION_DIR", t.TempDir())
	workspace := filepath.Join(t.TempDir(), "workspace-digest", "go.work")
	t.Setenv("SMOKE_ENV", "infra")
	t.Setenv("SMOKE_ENV_WORKSPACE", workspace)
	identity.RegisterComponent("example.com/session-component")

	handle, record, err := Begin(Record{Callbacks: []string{"stdout"}})
	if err != nil { t.Fatal(err) }
	defer handle.Close()
	if record.CompositionDigest == "" { t.Fatal("missing composition digest") }
	if record.Environment != "infra" { t.Fatalf("environment=%q", record.Environment) }
	if record.WorkspaceDigest != "workspace-digest" { t.Fatalf("workspace digest=%q", record.WorkspaceDigest) }
	if record.Workspace != workspace { t.Fatalf("workspace=%q", record.Workspace) }
	if record.Lease == "" { t.Fatal("missing lease path") }
}
