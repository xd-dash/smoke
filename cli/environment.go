package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/xd-dash/smoke/command"
	"github.com/xd-dash/smoke/environment"
)

func runEnv(args []string) error {
	if len(args) == 0 {
		return envUsage()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch args[0] {
	case "create":
		if len(args) != 2 {
			return fmt.Errorf("usage: smoke env create <name>")
		}
		env, err := environment.Create(ctx, args[1])
		if err != nil {
			return err
		}
		fmt.Println(env.WorkFile)
		return nil
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("usage: smoke env list")
		}
		envs, err := environment.List()
		if err != nil {
			return err
		}
		for _, env := range envs {
			fmt.Println(env.Name)
		}
		return nil
	case "show":
		return showEnvironment(ctx, args[1:])
	case "inspect":
		return inspectEnvironment(ctx, args[1:])
	case "use":
		if len(args) != 3 {
			return fmt.Errorf("usage: smoke env use <name> <module-dir>")
		}
		return environment.Use(ctx, args[1], args[2])
	case "drop":
		if len(args) != 3 {
			return fmt.Errorf("usage: smoke env drop <name> <module-dir>")
		}
		return environment.DropUse(ctx, args[1], args[2])
	case "module":
		return runEnvModule(ctx, args[1:])
	case "tool":
		return runEnvTool(ctx, args[1:])
	case "terraform":
		return terraformInEnv(ctx, args[1:])
	case "run":
		return runSmokeInEnv(ctx, args[1:])
	case "exec":
		return execInEnv(ctx, args[1:])
	case "shell":
		return shellInEnv(ctx, args[1:])
	case "build":
		return buildInEnv(ctx, args[1:])
	default:
		return fmt.Errorf("unknown env operation %q", args[0])
	}
}

func showEnvironment(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: smoke env show <name>")
	}
	env, err := environment.Require(args[0])
	if err != nil {
		return err
	}
	lock, err := environment.AcquireShared(ctx, env)
	if err != nil {
		return err
	}
	defer lock.Close()

	work, err := os.ReadFile(env.WorkFile)
	if err != nil {
		return err
	}
	mod, err := os.ReadFile(filepath.Join(env.ToolsDir, "go.mod"))
	if err != nil {
		return err
	}
	fmt.Printf("env %s\nwork %s\ntools %s\n\n%s\n%s", env.Name, env.WorkFile, env.ToolsDir, work, mod)
	return nil
}

func runEnvModule(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: smoke env module <add|drop|list> <name> [module]")
	}
	name := args[1]
	switch args[0] {
	case "add":
		if len(args) != 3 {
			return fmt.Errorf("usage: smoke env module add <name> <module@version-or-revision>")
		}
		module, err := environment.AddModule(ctx, name, args[2])
		if err != nil {
			return err
		}
		fmt.Printf("%s@%s\n", module.Path, module.Version)
		return nil
	case "drop":
		if len(args) != 3 {
			return fmt.Errorf("usage: smoke env module drop <name> <module-path>")
		}
		return environment.DropModule(ctx, name, args[2])
	case "list":
		if len(args) != 2 {
			return fmt.Errorf("usage: smoke env module list <name>")
		}
		modules, err := environment.Modules(ctx, name)
		if err != nil {
			return err
		}
		for _, module := range modules {
			fmt.Printf("%s\t%s\n", module.ModulePath, module.DiskPath)
		}
		return nil
	default:
		return fmt.Errorf("unknown env module operation %q", args[0])
	}
}

func runEnvTool(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: smoke env tool <add|remove|list|run> <name> [package-or-tool] [args ...]")
	}
	name := args[1]
	switch args[0] {
	case "add":
		if len(args) != 3 {
			return fmt.Errorf("usage: smoke env tool add <name> <package[@version]>")
		}
		return environment.AddTool(ctx, name, args[2])
	case "remove":
		if len(args) != 3 {
			return fmt.Errorf("usage: smoke env tool remove <name> <package>")
		}
		return environment.RemoveTool(ctx, name, args[2])
	case "list":
		if len(args) != 2 {
			return fmt.Errorf("usage: smoke env tool list <name>")
		}
		env, err := environment.Require(name)
		if err != nil {
			return err
		}
		workspace, err := environment.Snapshot(ctx, env)
		if err != nil {
			return err
		}
		goBin, err := exec.LookPath("go")
		if err != nil {
			return fmt.Errorf("Smoke environments require a preinstalled Go toolchain: %w", err)
		}
		return runCommand(workspace.Command(ctx, workspace.ToolsDir, goBin, "tool"))
	case "run":
		return runEnvironmentTool(ctx, args[1:])
	default:
		return fmt.Errorf("unknown env tool operation %q", args[0])
	}
}

func runSmokeInEnv(ctx context.Context, args []string) error {
	name, dir, rest, err := parseEnvInvocation(args)
	if err != nil {
		return fmt.Errorf("usage: smoke env run <name> [--dir <path>] -- <smoke-command> [args ...]")
	}
	if !compiledCommand(rest[0]) {
		return fmt.Errorf("command %q is not compiled into this smoke", rest[0])
	}
	workspace, err := snapshotEnvironment(ctx, name)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return runCommand(workspace.Command(ctx, dir, exe, rest...))
}

func compiledCommand(name string) bool {
	for _, candidate := range command.Names() {
		if candidate == name {
			return true
		}
	}
	return false
}

func execInEnv(ctx context.Context, args []string) error {
	name, dir, rest, err := parseEnvInvocation(args)
	if err != nil {
		return fmt.Errorf("usage: smoke env exec <name> [--dir <path>] -- <program> [args ...]")
	}
	workspace, err := snapshotEnvironment(ctx, name)
	if err != nil {
		return err
	}
	return runCommand(workspace.Command(ctx, dir, rest[0], rest[1:]...))
}

func shellInEnv(ctx context.Context, args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: smoke env shell <name> [dir]")
	}
	workspace, err := snapshotEnvironment(ctx, args[0])
	if err != nil {
		return err
	}
	dir := workspace.Environment.Dir
	if len(args) == 2 {
		dir, err = filepath.Abs(args[1])
		if err != nil {
			return err
		}
	}
	shell := os.Getenv("SHELL")
	if runtime.GOOS == "windows" {
		shell = os.Getenv("ComSpec")
	}
	if shell == "" {
		if runtime.GOOS == "windows" {
			shell = "cmd.exe"
		} else {
			shell = "/bin/sh"
		}
	}
	return runCommand(workspace.Command(ctx, dir, shell))
}

func buildInEnv(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: smoke env build <name> [--dir <path>] [-- <go-build-args ...>]")
	}
	name, dir, rest := args[0], "", args[1:]
	if len(rest) >= 2 && rest[0] == "--dir" {
		dir, rest = rest[1], rest[2:]
	}
	if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	workspace, err := snapshotEnvironment(ctx, name)
	if err != nil {
		return err
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("Smoke environments require a preinstalled Go toolchain: %w", err)
	}
	return runCommand(workspace.Command(ctx, dir, goBin, append([]string{"build"}, rest...)...))
}

func snapshotEnvironment(ctx context.Context, name string) (environment.Workspace, error) {
	env, err := environment.Require(name)
	if err != nil {
		return environment.Workspace{}, err
	}
	return environment.Snapshot(ctx, env)
}

func parseEnvInvocation(args []string) (name, dir string, rest []string, err error) {
	if len(args) < 2 {
		return "", "", nil, fmt.Errorf("missing arguments")
	}
	name = args[0]
	i := 1
	if i < len(args) && args[i] == "--dir" {
		if i+1 >= len(args) {
			return "", "", nil, fmt.Errorf("missing directory")
		}
		dir = args[i+1]
		i += 2
	}
	if i < len(args) && args[i] == "--" {
		i++
	}
	if i >= len(args) {
		return "", "", nil, fmt.Errorf("missing command")
	}
	return name, dir, args[i:], nil
}

func runCommand(cmd *exec.Cmd) error {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func envUsage() error {
	return fmt.Errorf("usage: smoke env <create|list|show|inspect|use|drop|module|tool|terraform|run|exec|shell|build> ...")
}
