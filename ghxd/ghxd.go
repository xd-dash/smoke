// Package ghxd defines Smoke's GitHub tool environment.
//
// ghxd is intentionally GitHub-specific. Other source-control providers belong
// beside ghxd as their own Smoke provider/tool environments rather than behind
// a forge-neutral abstraction inside this package.
//
// Each external capability remains an ordinary Go tool. Smoke owns only the
// named environment, immutable snapshot, and execution lifecycle. Credentials
// and organization-specific values are never persisted here.
package ghxd

import (
	"context"
	"fmt"
	"strings"

	"github.com/xd-dash/smoke/environment"
)

const DefaultEnvironment = "ghxd"

const defaultGitHubCDNToolSpec = "github.com/dash-xd/github-cdn@6c00e9533d91906c97da7ebfb262104466da27ed"
const defaultDeviceAuthToolSpec = "github.com/dash-xd/github-device-auth/cmd/github-device-auth@113339509308abf42efce2e0305f67c65f6910be"
const defaultWorktreeToolSpec = "github.com/xd-dash/smoke/cmd/github-worktree@599b3ffb7b0437ed10c80e8677d15a40e954901c"

// ToolSpecs is the default GitHub capability set installed into the ghxd
// workspace. Every default is pinned to an exact Git commit; role branches such
// as github-cdn@go and movable refs such as @main are discovery/provenance
// selectors, not runtime authority for a durable Smoke environment.
var ToolSpecs = []string{
	defaultGitHubCDNToolSpec,
	defaultDeviceAuthToolSpec,
	defaultWorktreeToolSpec,
}

// Apply composes ghxd into an existing Smoke environment using Go's native
// tool dependency mechanism. The tool manifest update is transactional: if one
// capability cannot be added, the environment's pre-call go.mod/go.sum are
// restored instead of leaving a partially updated ghxd composition.
func Apply(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("environment name is required")
	}
	if err := environment.AddTools(ctx, name, ToolSpecs); err != nil {
		return fmt.Errorf("compose ghxd tools: %w", err)
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
