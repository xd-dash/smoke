package smokeapp

import (
	"context"
	"fmt"
)

const ghxdWorktreeTool = "github-worktree"

func runGHXDWorktree(ctx context.Context, args []string) error {
	name, rest, err := parseGHXDEnvironment(args)
	if err != nil || len(rest) == 0 || rest[0] != "seed" {
		return ghxdWorktreeUsage()
	}
	return runGHXDTool(ctx, name, append([]string{ghxdWorktreeTool}, rest...)...)
}

func ghxdWorktreeUsage() error {
	return fmt.Errorf("usage: smoke ghxd worktree [--env <environment>] seed --repository <owner/name> --sha <40-char-sha> --destination <path> [--role-ref <branch>] [--object-root <path>] [--token-env <name>]")
}
