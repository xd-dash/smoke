package smokeapp

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	ghxdworktree "github.com/xd-dash/smoke/ghxd/worktree"
)

func runGHXDWorktree(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "seed" {
		return ghxdWorktreeUsage()
	}
	fs := flag.NewFlagSet("ghxd worktree seed", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repository := fs.String("repository", "", "GitHub repository in owner/name form")
	sha := fs.String("sha", "", "exact 40-character commit SHA")
	roleRef := fs.String("role-ref", "", "optional branch used only for provenance")
	destination := fs.String("destination", "", "destination for detached worktree")
	objectRoot := fs.String("object-root", "", "shared bare-object-database root")
	tokenEnv := fs.String("token-env", "GH_TOKEN", "environment variable containing GitHub token")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return ghxdWorktreeUsage()
	}
	token := ""
	if *tokenEnv != "" {
		token = os.Getenv(*tokenEnv)
	}
	result, err := ghxdworktree.Seed(ctx, ghxdworktree.Options{
		Repository:         *repository,
		SHA:                *sha,
		RoleRef:            *roleRef,
		Destination:        *destination,
		Token:              token,
		ObjectDatabaseRoot: *objectRoot,
	})
	if err != nil {
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return fmt.Errorf("encode worktree seed result: %w", err)
	}
	return nil
}

func ghxdWorktreeUsage() error {
	return fmt.Errorf("usage: smoke ghxd worktree seed --repository <owner/name> --sha <40-char-sha> --destination <path> [--role-ref <branch>] [--object-root <path>] [--token-env <name>]")
}
