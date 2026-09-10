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
const probotRuntimeToolSpec = "github.com/xd-dash/probot-runtime/cmd/probot-runtime@7a01a3ec135f52940c01358ec5da3c8ff116eaec"

// ToolSpecs is the required ghxd capability set. Optional capabilities belong
// to Presets so their absence never invalidates a normal ghxd environment.
var ToolSpecs = []string{
	"github.com/dash-xd/github-cdn@go",
	"github.com/dash-xd/github-device-auth/cmd/github-device-auth@main",
	defaultWorktreeToolSpec,
}

// Presets are optional groups of ordinary Go tools. A tool may internally own
// another runtime (for example Node/Probot), but that implementation detail is
// deliberately outside Smoke's environment model.
var Presets = map[string][]string{
	"probot-runtime": {probotRuntimeToolSpec},
}

func Apply(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("environment name is required")
	}
	return addTools(ctx, name, ToolSpecs)
}

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
