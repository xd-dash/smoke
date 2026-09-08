package cli

import (
	"context"
	"fmt"
	"os/exec"
)

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
