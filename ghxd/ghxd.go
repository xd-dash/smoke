// Package ghxd defines Smoke's GitHub tool environment.
//
// ghxd is intentionally GitHub-specific. Other source-control providers belong
// beside ghxd as their own Smoke provider/tool environments rather than behind
// a forge-neutral abstraction inside this package.
//
// Each external capability remains an ordinary Go tool. Smoke owns the named
// environment, immutable snapshot, and execution lifecycle. GitHub credential
// persistence is owned by the caller; ghxd only transforms credential bundles
// in memory for the duration of one invocation.
package ghxd

import (
	"context"
	"fmt"
	"strings"

	"github.com/xd-dash/smoke/environment"
)

const DefaultEnvironment = "ghxd"

const defaultGitHubCDNToolSpec = "github.com/dash-xd/github-cdn@6c00e9533d91906c97da7ebfb262104466da27ed"
const defaultDeviceAuthToolSpec = "github.com/dash-xd/github-device-auth/cmd/github-device-auth@85a1a81050198e429611cd58d336090868f0adfb"
const defaultWorktreeToolSpec = "github.com/xd-dash/smoke/cmd/github-worktree@599b3ffb7b0437ed10c80e8677d15a40e954901c"

var ToolSpecs = []string{
	defaultGitHubCDNToolSpec,
	defaultDeviceAuthToolSpec,
	defaultWorktreeToolSpec,
}

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
