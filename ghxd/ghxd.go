// Package ghxd defines Smoke's GitHub tool environment.
//
// ghxd is intentionally GitHub-specific. Other source-control providers belong
// beside ghxd as their own Smoke provider/tool environments rather than behind
// a forge-neutral abstraction inside this package.
//
// Required external capabilities remain ordinary Go tools. Optional non-Go
// capabilities are layered in through named presets and must never become a
// hard dependency of the default ghxd environment.
package ghxd

import (
	"context"
	"fmt"
	"strings"

	"github.com/xd-dash/smoke/environment"
)

const DefaultEnvironment = "ghxd"
const ProbotRuntimePreset = "probot-runtime"
const ProbotRuntimeTool = "probot-runtime"

const defaultWorktreeToolSpec = "github.com/xd-dash/smoke/cmd/github-worktree@599b3ffb7b0437ed10c80e8677d15a40e954901c"
const probotRuntimePackage = "@xd-dash/probot-runtime"
const probotRuntimeSpec = "github:xd-dash/probot-runtime#3a9934c9360b5496e2590e314e6a956ae06ba283"

// ToolSpecs is the required GitHub capability set installed into the ghxd
// workspace. The optional Probot runtime is deliberately not part of this set.
var ToolSpecs = []string{
	"github.com/dash-xd/github-cdn@go",
	"github.com/dash-xd/github-device-auth/cmd/github-device-auth@main",
	defaultWorktreeToolSpec,
}

var Presets = map[string]environment.NodeToolSpec{
	ProbotRuntimePreset: {
		Name:    ProbotRuntimeTool,
		Package: probotRuntimePackage,
		Spec:    probotRuntimeSpec,
	},
}

// Apply composes the required ghxd Go tools into an existing Smoke environment.
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

// ApplyPreset adds one optional capability to an existing ghxd environment.
func ApplyPreset(ctx context.Context, name, preset string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = DefaultEnvironment
	}
	preset = strings.TrimSpace(preset)
	spec, ok := Presets[preset]
	if !ok {
		return fmt.Errorf("unknown ghxd preset %q", preset)
	}
	if _, err := environment.InstallNodeTool(ctx, name, spec); err != nil {
		return fmt.Errorf("apply ghxd preset %s: %w", preset, err)
	}
	return nil
}

// ProbotRuntime resolves the optional Probot runtime in a ghxd environment.
// ok=false is a normal state: ghxd does not require this capability.
func ProbotRuntime(name string) (environment.NodeTool, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = DefaultEnvironment
	}
	return environment.ResolveNodeTool(name, ProbotRuntimeTool)
}

// Bootstrap ensures a named Smoke environment exists and composes only the
// required ghxd tool set. It intentionally installs no optional presets.
func Bootstrap(ctx context.Context, name string) (environment.Environment, error) {
	return BootstrapWithPresets(ctx, name, nil)
}

// BootstrapWithPresets composes ghxd and then layers in explicitly selected
// optional presets. Re-running it is idempotent for identical pinned specs.
func BootstrapWithPresets(ctx context.Context, name string, presets []string) (environment.Environment, error) {
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
	for _, preset := range presets {
		if err := ApplyPreset(ctx, env.Name, preset); err != nil {
			return environment.Environment{}, err
		}
	}
	return env, nil
}
