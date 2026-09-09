package smokeapp

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/xd-dash/smoke/command"
	"github.com/xd-dash/smoke/environment"
	"github.com/xd-dash/smoke/ghxd"
)

const ghxdDeviceAuthTool = "github-device-auth"

func init() {
	command.Register("ghxd", runGHXD)
}

func runGHXD(args []string) error {
	ctx := context.Background()
	if len(args) == 0 {
		return ghxdUsage()
	}
	switch args[0] {
	case "show":
		if len(args) != 1 {
			return fmt.Errorf("usage: smoke ghxd show")
		}
		fmt.Printf("environment\t%s\n", ghxd.DefaultEnvironment)
		for _, spec := range ghxd.ToolSpecs {
			fmt.Printf("tool\t%s\n", spec)
		}
		return nil
	case "bootstrap":
		if len(args) > 2 {
			return fmt.Errorf("usage: smoke ghxd bootstrap [environment]")
		}
		name := ""
		if len(args) == 2 {
			name = args[1]
		}
		env, err := ghxd.Bootstrap(ctx, name)
		if err != nil {
			return err
		}
		fmt.Printf("ghxd environment %s\n%s\n", env.Name, env.WorkFile)
		return nil
	case "apply":
		if len(args) != 2 {
			return fmt.Errorf("usage: smoke ghxd apply <environment>")
		}
		return ghxd.Apply(ctx, args[1])
	case "tool":
		name, rest, err := parseGHXDEnvironment(args[1:])
		if err != nil || len(rest) == 0 {
			return fmt.Errorf("usage: smoke ghxd tool [--env <environment>] <go-tool> [args ...]")
		}
		return runGHXDTool(ctx, name, rest...)
	case "auth":
		return runGHXDAuth(ctx, args[1:])
	case "worktree":
		return runGHXDWorktree(ctx, args[1:])
	default:
		return fmt.Errorf("unknown ghxd operation %q", args[0])
	}
}

func runGHXDAuth(ctx context.Context, args []string) error {
	name, rest, err := parseGHXDEnvironment(args)
	if err != nil || len(rest) == 0 {
		return ghxdAuthUsage()
	}
	store, err := ghxd.CredentialPath()
	if err != nil {
		return err
	}

	switch rest[0] {
	case "login":
		if len(rest) != 2 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] login <client-id>")
		}
		return runGHXDTool(ctx, name, ghxdDeviceAuthTool, "login", rest[1], store)
	case "import":
		if len(rest) != 3 || rest[1] != "--repo-secret-env" {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] import --repo-secret-env <environment-variable>")
		}
		return runGHXDTool(ctx, name, ghxdDeviceAuthTool, "import-env", rest[2], store)
	case "status":
		if len(rest) != 1 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] status")
		}
		return runGHXDTool(ctx, name, ghxdDeviceAuthTool, "status", store)
	case "ensure":
		if len(rest) > 2 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] ensure [safety-margin-seconds]")
		}
		toolArgs := []string{ghxdDeviceAuthTool, "ensure", store}
		if len(rest) == 2 {
			toolArgs = append(toolArgs, rest[1])
		}
		return runGHXDTool(ctx, name, toolArgs...)
	case "refresh":
		if len(rest) > 2 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] refresh [safety-margin-seconds]")
		}
		toolArgs := []string{ghxdDeviceAuthTool, "refresh-local", store}
		if len(rest) == 2 {
			toolArgs = append(toolArgs, rest[1])
		}
		return runGHXDTool(ctx, name, toolArgs...)
	case "token":
		if len(rest) != 1 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] token")
		}
		return runGHXDTool(ctx, name, ghxdDeviceAuthTool, "token", store)
	case "export":
		if len(rest) != 1 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] export")
		}
		return runGHXDTool(ctx, name, ghxdDeviceAuthTool, "export", store)
	case "sync":
		if len(rest) != 3 && len(rest) != 5 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] sync --repo <owner/repo> [--secret <name>]")
		}
		if rest[1] != "--repo" {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] sync --repo <owner/repo> [--secret <name>]")
		}
		secret := ghxd.DefaultCredentialSecret
		if len(rest) == 5 {
			if rest[3] != "--secret" || rest[4] == "" {
				return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] sync --repo <owner/repo> [--secret <name>]")
			}
			secret = rest[4]
		}
		if err := runGHXDTool(ctx, name, ghxdDeviceAuthTool, "ensure", store); err != nil {
			return err
		}
		return runGHXDTool(ctx, name, ghxdDeviceAuthTool, "sync-secret", store, rest[2], secret)
	case "device":
		if len(rest) != 2 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] device <client-id>")
		}
		return runGHXDTool(ctx, name, ghxdDeviceAuthTool, "device", rest[1])
	case "poll":
		if len(rest) != 3 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] poll <client-id> <device-code>")
		}
		return runGHXDTool(ctx, name, ghxdDeviceAuthTool, "poll", rest[1], rest[2])
	default:
		return fmt.Errorf("unknown ghxd auth operation %q", rest[0])
	}
}

func runGHXDTool(ctx context.Context, name string, args ...string) error {
	if name == "" {
		name = ghxd.DefaultEnvironment
	}
	env, err := environment.Require(name)
	if err != nil {
		return fmt.Errorf("ghxd environment %q is not bootstrapped; run `smoke ghxd bootstrap%s`: %w", name, bootstrapSuffix(name), err)
	}
	workspace, err := environment.Snapshot(ctx, env)
	if err != nil {
		return err
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("ghxd requires a preinstalled Go toolchain: %w", err)
	}
	toolArgs := append([]string{"tool"}, args...)
	return runCommand(workspace.Command(ctx, workspace.ToolsDir, goBin, toolArgs...))
}

func parseGHXDEnvironment(args []string) (string, []string, error) {
	name := ghxd.DefaultEnvironment
	if len(args) >= 1 && args[0] == "--env" {
		if len(args) < 2 || args[1] == "" {
			return "", nil, fmt.Errorf("missing environment")
		}
		name = args[1]
		args = args[2:]
	}
	return name, args, nil
}

func bootstrapSuffix(name string) string {
	if name == ghxd.DefaultEnvironment {
		return ""
	}
	return " " + name
}

func ghxdAuthUsage() error {
	return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] <login|import|status|ensure|refresh|token|export|sync|device|poll> ...")
}

func ghxdUsage() error {
	return fmt.Errorf("usage: smoke ghxd <show|bootstrap|apply|tool|auth|worktree> ...")
}
