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
	default:
		return fmt.Errorf("unknown ghxd operation %q", args[0])
	}
}

func runGHXDAuth(ctx context.Context, args []string) error {
	name, rest, err := parseGHXDEnvironment(args)
	if err != nil || len(rest) == 0 {
		return ghxdAuthUsage()
	}

	switch rest[0] {
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
	case "refresh":
		if len(rest) != 3 && len(rest) != 4 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] refresh <client-id> <refresh-token> [client-secret]")
		}
		toolArgs := append([]string{ghxdDeviceAuthTool, "refresh"}, rest[1:]...)
		return runGHXDTool(ctx, name, toolArgs...)
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
	return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] <device|poll|refresh> ...")
}

func ghxdUsage() error {
	return fmt.Errorf("usage: smoke ghxd <show|bootstrap|apply|tool|auth> ...")
}
