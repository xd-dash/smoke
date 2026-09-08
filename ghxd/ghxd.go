// Package ghxd defines Smoke's GitHub tool environment.
//
// GitHub operations that are useful in-process belong in ordinary Go libraries
// composed into Smoke. The ghxd environment is reserved for capabilities that
// still benefit from an executable/tool boundary, such as exact detached
// worktree seeding.
package ghxd

import (
	"context"
	"fmt"
	"strings"

	"github.com/xd-dash/smoke/environment"
)

const DefaultEnvironment = "ghxd"

const defaultWorktreeToolSpec = "github.com/xd-dash/smoke/cmd/github-worktree@599b3ffb7b0437ed10c80e8677d15a40e954901c"

// ToolSpecs is the executable capability set installed into the ghxd
// environment. Router-free github-cdn and github-device-auth behavior is linked
// directly into Smoke and therefore does not belong here.
var ToolSpecs = []string{
	defaultWorktreeToolSpec,
}

// Apply composes the executable ghxd tool set into an existing Smoke
// environment using Go's native tool dependency mechanism.
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

// Bootstrap ensures a named Smoke environment exists and installs the
// executable ghxd tool set. An empty name selects DefaultEnvironment.
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
