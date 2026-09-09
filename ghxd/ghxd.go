// Package ghxd defines Smoke's GitHub tool environment.
//
// ghxd is intentionally GitHub-specific. Other source-control providers belong
// beside ghxd as their own Smoke provider/tool environments rather than behind
// a forge-neutral abstraction inside this package.
//
// Each external capability remains an ordinary Go tool. Smoke owns the named
// environment, immutable snapshot, and execution lifecycle. Mutable GitHub
// credential checkpoints live under Smoke's local data root, outside the Go
// workspace snapshot, and are never committed into environment manifests.
package ghxd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xd-dash/smoke/environment"
)

const DefaultEnvironment = "ghxd"
const DefaultCredentialSecret = "HURAM_GITHUB_DEVICE_TOKEN"
const DefaultCredentialRecoverySecret = "HURAM_GITHUB_DEVICE_TOKEN_RECOVERY"

const defaultGitHubCDNToolSpec = "github.com/dash-xd/github-cdn@6c00e9533d91906c97da7ebfb262104466da27ed"
const defaultDeviceAuthToolSpec = "github.com/dash-xd/github-device-auth/cmd/github-device-auth@bc3fcf6b341f5c2beaf4d46fb5aaf2fc9c6212a8"
const defaultWorktreeToolSpec = "github.com/xd-dash/smoke/cmd/github-worktree@599b3ffb7b0437ed10c80e8677d15a40e954901c"

var ToolSpecs = []string{
	defaultGitHubCDNToolSpec,
	defaultDeviceAuthToolSpec,
	defaultWorktreeToolSpec,
}

func CredentialPath() (string, error) {
	root := strings.TrimSpace(os.Getenv("SMOKE_DATA_HOME"))
	if root == "" {
		if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
			root = filepath.Join(xdg, "smoke")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve home directory: %w", err)
			}
			root = filepath.Join(home, ".local", "share", "smoke")
		}
	}
	return filepath.Join(root, "ghxd", "credentials", "github-device.json"), nil
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
