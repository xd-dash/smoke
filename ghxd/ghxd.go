// Package ghxd defines Smoke's optional GitHub tool environment.
//
// ghxd is a Go-native composition surface: each capability remains an ordinary
// Go tool, while Smoke owns the named environment, immutable snapshot, and
// execution lifecycle. Credentials and organization-specific values are never
// part of this package.
package ghxd

import (
	"context"
	"fmt"
	"strings"

	"github.com/xd-dash/smoke/environment"
)

const DefaultEnvironment = "ghxd"

// ToolSpecs is the default GitHub capability set. Future GitHub utilities such
// as worktree, webhook, and workflow tooling should be added here only when
// they expose ordinary installable Go tool surfaces.
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

// Bootstrap creates a named Smoke environment and composes ghxd into it. An
// empty name selects DefaultEnvironment.
func Bootstrap(ctx context.Context, name string) (environment.Environment, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = DefaultEnvironment
	}
	env, err := environment.Create(ctx, name)
	if err != nil {
		return environment.Environment{}, err
	}
	if err := Apply(ctx, env.Name); err != nil {
		return environment.Environment{}, err
	}
	return env, nil
}
