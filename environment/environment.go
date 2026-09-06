package environment

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/xd-dash/smoke/internal/filelock"
)

const goVersion = "1.26"

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Environment struct {
	Name     string
	Dir      string
	WorkFile string
	ToolsDir string
	ToolsMod string
	LockFile string
}

func Root() (string, error) {
	if root := strings.TrimSpace(os.Getenv("SMOKE_ENV_DIR")); root != "" {
		return filepath.Abs(root)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "smoke", "envs"), nil
}

func Resolve(name string) (Environment, error) {
	name = strings.TrimSpace(name)
	if !validName.MatchString(name) {
		return Environment{}, fmt.Errorf("invalid environment name %q", name)
	}
	root, err := Root()
	if err != nil {
		return Environment{}, err
	}
	dir := filepath.Join(root, name)
	return Environment{
		Name:     name,
		Dir:      dir,
		WorkFile: filepath.Join(dir, "go.work"),
		ToolsDir: filepath.Join(dir, "tools"),
		ToolsMod: "smoke.local/env/" + name + "/tools",
		LockFile: filepath.Join(root, ".locks", name+".lock"),
	}, nil
}

// AcquireShared prevents workspace mutation while an environment-backed process
// is using the workspace. Multiple readers may coexist.
func AcquireShared(ctx context.Context, env Environment) (*filelock.Lock, error) {
	return filelock.Acquire(ctx, env.LockFile, filelock.Shared)
}

func acquireExclusive(ctx context.Context, env Environment) (*filelock.Lock, error) {
	return filelock.Acquire(ctx, env.LockFile, filelock.Exclusive)
}

func Create(ctx context.Context, name string) (Environment, error) {
	env, err := Resolve(name)
	if err != nil {
		return Environment{}, err
	}
	lock, err := acquireExclusive(ctx, env)
	if err != nil {
		return Environment{}, err
	}
	defer lock.Close()

	if _, err := os.Stat(env.WorkFile); err == nil {
		return Environment{}, fmt.Errorf("environment %q already exists", name)
	} else if !os.IsNotExist(err) {
		return Environment{}, err
	}
	if err := os.MkdirAll(env.ToolsDir, 0o700); err != nil {
		return Environment{}, err
	}
	mod := fmt.Sprintf("module %s\n\ngo %s\n", env.ToolsMod, goVersion)
	if err := os.WriteFile(filepath.Join(env.ToolsDir, "go.mod"), []byte(mod), 0o600); err != nil {
		return Environment{}, err
	}
	work := fmt.Sprintf("go %s\n\nuse ./tools\n", goVersion)
	if err := os.WriteFile(env.WorkFile, []byte(work), 0o600); err != nil {
		return Environment{}, err
	}
	return env, nil
}

func List() ([]Environment, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Environment
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == ".locks" {
			continue
		}
		env, err := Resolve(entry.Name())
		if err != nil {
			continue
		}
		if _, err := os.Stat(env.WorkFile); err == nil {
			out = append(out, env)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func Require(name string) (Environment, error) {
	env, err := Resolve(name)
	if err != nil {
		return Environment{}, err
	}
	if _, err := os.Stat(env.WorkFile); err != nil {
		if os.IsNotExist(err) {
			return Environment{}, fmt.Errorf("environment %q does not exist", name)
		}
		return Environment{}, err
	}
	if _, err := os.Stat(filepath.Join(env.ToolsDir, "go.mod")); err != nil {
		return Environment{}, fmt.Errorf("environment %q tools module: %w", name, err)
	}
	return env, nil
}

func Use(ctx context.Context, name, moduleDir string) error {
	env, err := Require(name)
	if err != nil {
		return err
	}
	moduleDir, err = filepath.Abs(moduleDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(moduleDir, "go.mod")); err != nil {
		return fmt.Errorf("workspace module %s: %w", moduleDir, err)
	}
	lock, err := acquireExclusive(ctx, env)
	if err != nil {
		return err
	}
	defer lock.Close()
	return runGo(ctx, env.Dir, env.WorkFile, "work", "use", moduleDir)
}

func DropUse(ctx context.Context, name, moduleDir string) error {
	env, err := Require(name)
	if err != nil {
		return err
	}
	moduleDir, err = filepath.Abs(moduleDir)
	if err != nil {
		return err
	}
	lock, err := acquireExclusive(ctx, env)
	if err != nil {
		return err
	}
	defer lock.Close()
	return runGo(ctx, env.Dir, env.WorkFile, "work", "edit", "-dropuse="+moduleDir)
}

func AddTool(ctx context.Context, name, packageSpec string) error {
	env, err := Require(name)
	if err != nil {
		return err
	}
	packageSpec = strings.TrimSpace(packageSpec)
	if packageSpec == "" {
		return fmt.Errorf("tool package is required")
	}
	lock, err := acquireExclusive(ctx, env)
	if err != nil {
		return err
	}
	defer lock.Close()
	return runGo(ctx, env.ToolsDir, "off", "get", "-tool", packageSpec)
}

func RemoveTool(ctx context.Context, name, packagePath string) error {
	env, err := Require(name)
	if err != nil {
		return err
	}
	packagePath = strings.TrimSpace(packagePath)
	if packagePath == "" {
		return fmt.Errorf("tool package is required")
	}
	lock, err := acquireExclusive(ctx, env)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := runGo(ctx, env.ToolsDir, "off", "mod", "edit", "-droptool="+packagePath); err != nil {
		return err
	}
	return runGo(ctx, env.ToolsDir, "off", "mod", "tidy")
}

func Command(ctx context.Context, env Environment, dir, program string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, program, args...)
	if strings.TrimSpace(dir) == "" {
		dir = env.Dir
	}
	cmd.Dir = dir
	cmd.Env = withEnv(os.Environ(), "GOWORK", env.WorkFile)
	cmd.Env = withEnv(cmd.Env, "SMOKE_ENV", env.Name)
	return cmd
}

func runGo(ctx context.Context, dir, gowork string, args ...string) error {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("Smoke environments require a preinstalled Go toolchain: %w", err)
	}
	cmd := exec.CommandContext(ctx, goBin, args...)
	cmd.Dir = dir
	cmd.Env = withEnv(os.Environ(), "GOWORK", gowork)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func withEnv(values []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(values)+1)
	for _, item := range values {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return append(out, prefix+value)
}
