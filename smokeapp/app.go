package smokeapp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/xd-dash/smoke/command"
	"github.com/xd-dash/smoke/environment"
	"github.com/xd-dash/smoke/selfbuild"
)

// Main runs the Smoke application using commands compiled into this binary.
// Optional commands register themselves through package init functions selected
// by the composition's imports.
func Main(args []string) {
	if err := Run(args); err != nil {
		fmt.Fprintf(os.Stderr, "smoke: %v\n", err)
		os.Exit(1)
	}
}

func Run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	switch args[0] {
	case "commands":
		for _, name := range command.Names() {
			fmt.Println(name)
		}
		return nil
	case "compose":
		return runCompose(args[1:])
	case "env":
		return runEnv(args[1:])
	default:
		return command.Run(args[0], args[1:])
	}
}

func runCompose(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: smoke compose <show|add|remove|rebuild> [import-path]")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch args[0] {
	case "show":
		if len(args) != 1 {
			return fmt.Errorf("usage: smoke compose show")
		}
		manifest, err := selfbuild.Load()
		if err != nil {
			return err
		}
		for _, component := range manifest.Components {
			fmt.Println(component)
		}
		return nil
	case "add":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: smoke compose add <go-import-path>")
		}
		manifest, err := selfbuild.Load()
		if err != nil {
			return err
		}
		manifest = selfbuild.WithAdded(manifest, args[1])
		path, err := selfbuild.Apply(ctx, manifest)
		if err != nil {
			return err
		}
		fmt.Printf("rebuilt %s\n", path)
		return nil
	case "remove":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: smoke compose remove <go-import-path>")
		}
		manifest, err := selfbuild.Load()
		if err != nil {
			return err
		}
		manifest = selfbuild.WithRemoved(manifest, args[1])
		path, err := selfbuild.Apply(ctx, manifest)
		if err != nil {
			return err
		}
		fmt.Printf("rebuilt %s\n", path)
		return nil
	case "rebuild":
		if len(args) != 1 {
			return fmt.Errorf("usage: smoke compose rebuild")
		}
		manifest, err := selfbuild.Load()
		if err != nil {
			return err
		}
		path, err := selfbuild.Apply(ctx, manifest)
		if err != nil {
			return err
		}
		fmt.Printf("rebuilt %s\n", path)
		return nil
	default:
		return fmt.Errorf("unknown compose operation %q", args[0])
	}
}

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
		if len(args) != 2 {
			return fmt.Errorf("usage: smoke env show <name>")
		}
		env, err := environment.Require(args[1])
		if err != nil {
			return err
		}
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
	case "tool":
		return runEnvTool(ctx, args[1:])
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

func runEnvTool(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: smoke env tool <add|remove|list> <name> [package]")
	}
	switch args[0] {
	case "add":
		if len(args) != 3 {
			return fmt.Errorf("usage: smoke env tool add <name> <package[@version]>")
		}
		return environment.AddTool(ctx, args[1], args[2])
	case "remove":
		if len(args) != 3 {
			return fmt.Errorf("usage: smoke env tool remove <name> <package>")
		}
		return environment.RemoveTool(ctx, args[1], args[2])
	case "list":
		if len(args) != 2 {
			return fmt.Errorf("usage: smoke env tool list <name>")
		}
		env, err := environment.Require(args[1])
		if err != nil {
			return err
		}
		goBin, err := exec.LookPath("go")
		if err != nil {
			return err
		}
		cmd := environment.Command(ctx, env, env.ToolsDir, goBin, "tool")
		return runCommand(cmd)
	default:
		return fmt.Errorf("unknown env tool operation %q", args[0])
	}
}

// runSmokeInEnv runs another compiled-in Smoke command in-process under the
// selected workspace. This keeps Logmash and every other optional command on
// the same command registry/composition path. An unattended Logmash child also
// inherits GOWORK and SMOKE_ENV through os.Environ.
func runSmokeInEnv(ctx context.Context, args []string) error {
	name, dir, rest, err := parseEnvInvocation(args)
	if err != nil {
		return fmt.Errorf("usage: smoke env run <name> [--dir <path>] -- <smoke-command> [args ...]")
	}
	env, err := environment.Require(name)
	if err != nil {
		return err
	}
	restoreEnv, err := environment.Activate(env)
	if err != nil {
		return err
	}
	defer restoreEnv()
	if dir != "" {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return err
		}
		old, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := os.Chdir(dir); err != nil {
			return err
		}
		defer os.Chdir(old)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return Run(rest)
	}
}

func execInEnv(ctx context.Context, args []string) error {
	name, dir, rest, err := parseEnvInvocation(args)
	if err != nil {
		return fmt.Errorf("usage: smoke env exec <name> [--dir <path>] -- <program> [args ...]")
	}
	env, err := environment.Require(name)
	if err != nil {
		return err
	}
	cmd := environment.Command(ctx, env, dir, rest[0], rest[1:]...)
	return runCommand(cmd)
}

func shellInEnv(ctx context.Context, args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: smoke env shell <name> [dir]")
	}
	env, err := environment.Require(args[0])
	if err != nil {
		return err
	}
	dir := env.Dir
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
	cmd := environment.Command(ctx, env, dir, shell)
	return runCommand(cmd)
}

func buildInEnv(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: smoke env build <name> [--dir <path>] [-- <go-build-args ...>]")
	}
	name := args[0]
	dir := ""
	rest := args[1:]
	if len(rest) >= 2 && rest[0] == "--dir" {
		dir = rest[1]
		rest = rest[2:]
	}
	if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	env, err := environment.Require(name)
	if err != nil {
		return err
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("Smoke environments require a preinstalled Go toolchain: %w", err)
	}
	buildArgs := append([]string{"build"}, rest...)
	cmd := environment.Command(ctx, env, dir, goBin, buildArgs...)
	return runCommand(cmd)
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
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func envUsage() error {
	return fmt.Errorf("usage: smoke env <create|list|show|use|drop|tool|run|exec|shell|build> ...")
}

func usageError() error {
	names := command.Names()
	if len(names) == 0 {
		return fmt.Errorf("usage: smoke <command> [args ...] | smoke compose <show|add|remove|rebuild> | smoke env <operation> ...")
	}
	return fmt.Errorf("usage: smoke <%s> [args ...] | smoke compose <show|add|remove|rebuild> | smoke env <operation> ...", strings.Join(names, "|"))
}
