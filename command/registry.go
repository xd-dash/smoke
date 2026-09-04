package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Command describes one command Smoke can resolve/install and then execute.
// Provider-specific behavior belongs to the command itself, not to Smoke.
type Command struct {
	Name      string
	GoPackage string
}

type Registry struct {
	commands map[string]Command
}

func New(commands ...Command) (*Registry, error) {
	r := &Registry{commands: make(map[string]Command, len(commands))}
	for _, command := range commands {
		if command.Name == "" {
			return nil, errors.New("command name is required")
		}
		if _, exists := r.commands[command.Name]; exists {
			return nil, fmt.Errorf("duplicate command %q", command.Name)
		}
		r.commands[command.Name] = command
	}
	return r, nil
}

// Resolve returns an executable path. Existing PATH/sibling installations win;
// otherwise a registered Go command is installed into Smoke's user bin dir.
func (r *Registry) Resolve(ctx context.Context, name string) (string, error) {
	command, ok := r.commands[name]
	if !ok {
		return "", fmt.Errorf("unregistered command %q", name)
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	if exe, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(exe), executableName(name))
		if info, err := os.Stat(sibling); err == nil && !info.IsDir() {
			return sibling, nil
		}
	}
	if command.GoPackage == "" {
		return "", fmt.Errorf("command %q is not installed and has no installer", name)
	}
	return installGoCommand(ctx, command)
}

func installGoCommand(ctx context.Context, command Command) (string, error) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("command %q is not installed; Go toolchain is required for bootstrap: %w", command.Name, err)
	}
	binDir, err := userBinDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("create Smoke bin directory: %w", err)
	}
	cmd := exec.CommandContext(ctx, goBin, "install", command.GoPackage)
	cmd.Env = append(os.Environ(), "GOBIN="+binDir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("install %s: %w", command.Name, err)
	}
	path := filepath.Join(binDir, executableName(command.Name))
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("installed command %q not found at %s: %w", command.Name, path, err)
	}
	return path, nil
}

func userBinDir() (string, error) {
	if dir := os.Getenv("SMOKE_BIN_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
