// Package ghxd defines Smoke's GitHub tool environment.
//
// ghxd is intentionally GitHub-specific. Other source-control providers belong
// beside ghxd as their own Smoke provider/tool environments rather than behind
// a forge-neutral abstraction inside this package.
//
// Each capability remains an ordinary Go tool. Smoke owns only the named
// environment, immutable snapshot, and execution lifecycle. Credentials and
// organization-specific values are never persisted here.
package ghxd

import (
	"context"
	"fmt"
	"strings"

	"github.com/xd-dash/smoke/environment"
)

const DefaultEnvironment = "ghxd"

// ToolSpecs is the default GitHub capability set. GitHub-specific capability
// families may grow beneath ghxd (for example auth/device, auth/oauth,
// auth/wif, worktree, webhook, and workflow) when reusable Go behavior exists.
var ToolSpecs = []string{
	"github.com/dash-xd/github-cdn@go",
	"github.com/dash-xd/github-device-auth/cmd/github-device-auth@main",
}

// Apply composes ghxd into an existing Smoke environment using Go's native
// tool dependency mechanism.
func Apply(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("environment name is required")
	}
	for _, spec := range ToolSpecs {
		if err := environment.AddTool(ctx, name, spec); err != nil {
			return fmt.Errorf("add ghxd tool %s: %w", spec, err)
		}
	}
	return nil
}

// Bootstrap ensures a named Smoke environment exists and composes ghxd into
// it. An empty name selects DefaultEnvironment. Re-running bootstrap is
// intentionally idempotent: an existing environment is updated through the
// same Go-native tool path rather than rejected.
func Bootstrap(ctx context.Context, name string) (environment.Environment, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = DefaultEnvironment
	}

	env, err := environment.Require(name)
	if err != nil {
		env, err = environment.Create(ctx, name)
		if err != nil {
			return environment.Environment{}, err
		}
	}
	if err := Apply(ctx, env.Name); err != nil {
		return environment.Environment{}, err
	}
	return env, nil
}
