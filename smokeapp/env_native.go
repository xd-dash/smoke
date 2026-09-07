package smokeapp

import (
	"context"
	"fmt"
	"os/exec"
)

func terraformInEnv(ctx context.Context, args []string) error {
	name, dir, terraformArgs, err := parseEnvTerraform(args)
	if err != nil {
		return err
	}
	workspace, err := snapshotEnvironment(ctx, name)
	if err != nil {
		return err
	}
	terraformBin, err := exec.LookPath("terraform")
	if err != nil {
		return fmt.Errorf("Smoke environment Terraform execution requires a preinstalled terraform executable: %w", err)
	}
	return runCommand(workspace.Command(ctx, dir, terraformBin, terraformArgs...))
}

func parseEnvTerraform(args []string) (name, dir string, terraformArgs []string, err error) {
	if len(args) < 2 {
		return "", "", nil, fmt.Errorf("usage: smoke env terraform <name> [--dir <terraform-root>] [--] <terraform-args ...>")
	}
	name = args[0]
	i := 1
	if i < len(args) && args[i] == "--dir" {
		if i+1 >= len(args) {
			return "", "", nil, fmt.Errorf("--dir requires a Terraform root")
		}
		dir = args[i+1]
		i += 2
	}
	if i < len(args) && args[i] == "--" {
		i++
	}
	if i >= len(args) {
		return "", "", nil, fmt.Errorf("Terraform arguments are required")
	}
	return name, dir, args[i:], nil
}

func runEnvironmentTool(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: smoke env tool run <name> <tool> [args ...]")
	}
	workspace, err := snapshotEnvironment(ctx, args[0])
	if err != nil {
		return err
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("Smoke environment tools require a preinstalled Go toolchain: %w", err)
	}
	toolArgs := append([]string{"tool", args[1]}, args[2:]...)
	return runCommand(workspace.Command(ctx, workspace.ToolsDir, goBin, toolArgs...))
}
