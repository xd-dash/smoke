package environment

import "context"

// DefaultToolSpecs is Smoke's generic Go-first operator profile. These are
// executable capabilities only; credentials, tenant values, project IDs,
// regions, domains, and other deployment inputs belong to the orchestrator.
var DefaultToolSpecs = []string{
	"github.com/dash-xd/github-cdn@go",
	"github.com/dash-xd/github-device-auth/cmd/github-device-auth@main",
	"github.com/dash-xd/agni@main",
}

// AddDefaultTools composes the default profile using the same native Go tool
// mechanism as explicit `smoke env tool add` calls.
func AddDefaultTools(ctx context.Context, name string) error {
	for _, spec := range DefaultToolSpecs {
		if err := AddTool(ctx, name, spec); err != nil {
			return err
		}
	}
	return nil
}
