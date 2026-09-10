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

const defaultWorktreeToolSpec = "github.com/xd-dash/smoke/cmd/github-worktree@599b3ffb7b0437ed10c80e8677d15a40e954901c"
const probotRuntimeToolSpec = "github.com/xd-dash/probot-runtime/cmd/probot-runtime@53a730434a696cd1e1bf72afe85d9bd485969774"

// ToolSpecs is the required GitHub capability set installed into every ghxd
// workspace. Optional capabilities belong to explicit presets and must not be
// added here.
var ToolSpecs = []string{
	"github.com/dash-xd/github-cdn@go",
	"github.com/dash-xd/github-device-auth/cmd/github-device-auth@main",
	defaultWorktreeToolSpec,
}

var Presets = map[string][]string{
	"probot-runtime": {probotRuntimeToolSpec},
}

// Apply composes the required ghxd capability set into an existing Smoke
// environment using Go's native tool dependency mechanism.
func Apply(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("environment name is required")
	}
	return addTools(ctx, name, ToolSpecs)
}

// ApplyPreset adds an optional ghxd capability set. Presets are ordinary Go
// tools from Smoke's point of view; a tool may internally orchestrate another
// runtime without teaching Smoke about that runtime's package manager.
func ApplyPreset(ctx context.Context, name, preset string) error {
	name = strings.TrimSpace(name)
	preset = strings.TrimSpace(preset)
	if name == "" {
		return fmt.Errorf("environment name is required")
	}
	specs, ok := Presets[preset]
	if !ok {
		return fmt.Errorf("unknown ghxd preset %q", preset)
	}
	return addTools(ctx, name, specs)
}

func addTools(ctx context.Context, name string, specs []string) error {
	for _, spec := range specs {
		if err := environment.AddTool(ctx, name, spec); err != nil {
			return fmt.Errorf("add ghxd tool %s: %w", spec, err)
		}
	}
	return nil
}

// Bootstrap ensures a named Smoke environment exists and composes the required
// ghxd tools into it. Optional presets are applied separately so plain ghxd
// remains usable without them.
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
