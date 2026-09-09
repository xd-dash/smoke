package astrochickenxdrun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/xd-dash/smoke/environment"
)

const routeConfigTemplate = `{
  "zone": "xd.run",
  "routes": []
}
`

const probeTFVarsTemplate = `# Deployment-specific Probe values belong here.
# The Agni Probe profile owns variables.tf; this file owns only this instance's values.
`

// SeedWorkspace materializes the two profile implementations into sibling
// Terraform roots and creates mutable configuration files beside them.
// Terraform state remains independent because neither root imports the other.
func SeedWorkspace(ctx context.Context, environmentName, root string) error {
	if environmentName == "" {
		environmentName = DefaultEnvironment
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	env, err := environment.Require(environmentName)
	if err != nil {
		return err
	}
	workspace, err := environment.Snapshot(ctx, env)
	if err != nil {
		return err
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("astrochicken-xd-run requires a preinstalled Go toolchain: %w", err)
	}

	if err := run(workspace.Command(ctx, workspace.ToolsDir, goBin, "tool", "probe", "seed", filepath.Join(root, "agni-probe"))); err != nil {
		return fmt.Errorf("seed agni/probe: %w", err)
	}
	if err := run(workspace.Command(ctx, workspace.ToolsDir, goBin, "tool", "cfxd-dns-txt", "seed", filepath.Join(root, "cfxd-dns-txt"))); err != nil {
		return fmt.Errorf("seed cfxd/dns-txt: %w", err)
	}

	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	if err := writeConfigIfMissing(filepath.Join(configDir, "probe.tfvars"), probeTFVarsTemplate); err != nil {
		return err
	}
	if err := writeConfigIfMissing(filepath.Join(configDir, "xd-run.routes"), routeConfigTemplate); err != nil {
		return err
	}
	return nil
}

func writeConfigIfMissing(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

func run(cmd *exec.Cmd) error {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
