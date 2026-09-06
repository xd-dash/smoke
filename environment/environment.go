package environment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

// Workspace is an immutable, content-addressed snapshot of an Environment's
// go.work state. Long-lived commands use the snapshot so canonical environment
// mutation never has to wait for the command lifetime.
type Workspace struct {
	Environment Environment
	WorkFile    string
	Digest      string
}

type goWorkJSON struct {
	Go        string          `json:"Go"`
	Toolchain string          `json:"Toolchain"`
	Godebug   []goWorkGodebug `json:"Godebug"`
	Use       []goWorkUse     `json:"Use"`
	Replace   []goWorkReplace `json:"Replace"`
}

type goWorkGodebug struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type goWorkUse struct {
	DiskPath   string `json:"DiskPath"`
	ModulePath string `json:"ModulePath"`
}

type goWorkReplace struct {
	Old goWorkModule `json:"Old"`
	New goWorkModule `json:"New"`
}

type goWorkModule struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
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

// AcquireShared protects a short canonical workspace read while a snapshot or
// inspection is being made. Long-lived child processes must not retain it.
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

// Snapshot reads the canonical go.work under a short shared lock, normalizes
// local paths to absolute paths, and writes an immutable content-addressed
// workspace into the user cache. Once returned, the canonical lock is released.
func Snapshot(ctx context.Context, env Environment) (Workspace, error) {
	lock, err := AcquireShared(ctx, env)
	if err != nil {
		return Workspace{}, err
	}
	defer lock.Close()

	goBin, err := exec.LookPath("go")
	if err != nil {
		return Workspace{}, fmt.Errorf("Smoke environments require a preinstalled Go toolchain: %w", err)
	}
	cmd := exec.CommandContext(ctx, goBin, "work", "edit", "-json", env.WorkFile)
	cmd.Dir = env.Dir
	cmd.Env = withEnv(os.Environ(), "GOWORK", "off")
	output, err := cmd.Output()
	if err != nil {
		return Workspace{}, fmt.Errorf("read workspace %s: %w", env.WorkFile, err)
	}
	var parsed goWorkJSON
	if err := json.Unmarshal(output, &parsed); err != nil {
		return Workspace{}, fmt.Errorf("decode workspace %s: %w", env.WorkFile, err)
	}
	body, err := renderSnapshot(env, parsed)
	if err != nil {
		return Workspace{}, err
	}
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return Workspace{}, fmt.Errorf("user cache directory: %w", err)
	}
	dir := filepath.Join(cacheRoot, "smoke", "env-workspaces", env.Name, digest)
	workFile := filepath.Join(dir, "go.work")
	if err := writeImmutable(workFile, body); err != nil {
		return Workspace{}, err
	}
	return Workspace{Environment: env, WorkFile: workFile, Digest: digest}, nil
}

func renderSnapshot(env Environment, work goWorkJSON) ([]byte, error) {
	var b strings.Builder
	if strings.TrimSpace(work.Go) == "" {
		return nil, fmt.Errorf("workspace %s has no go directive", env.WorkFile)
	}
	fmt.Fprintf(&b, "go %s\n", work.Go)
	if work.Toolchain != "" {
		fmt.Fprintf(&b, "\ntoolchain %s\n", work.Toolchain)
	}
	for _, setting := range work.Godebug {
		fmt.Fprintf(&b, "\ngodebug %s=%s\n", setting.Key, setting.Value)
	}
	if len(work.Use) == 1 {
		fmt.Fprintf(&b, "\nuse %s\n", strconv.Quote(workspacePath(env.Dir, work.Use[0].DiskPath)))
	} else if len(work.Use) > 1 {
		b.WriteString("\nuse (\n")
		for _, use := range work.Use {
			fmt.Fprintf(&b, "\t%s\n", strconv.Quote(workspacePath(env.Dir, use.DiskPath)))
		}
		b.WriteString(")\n")
	}
	for _, replacement := range work.Replace {
		old := moduleToken(replacement.Old)
		newToken := moduleToken(replacement.New)
		if replacement.New.Version == "" {
			newToken = strconv.Quote(workspacePath(env.Dir, replacement.New.Path))
		}
		fmt.Fprintf(&b, "\nreplace %s => %s\n", old, newToken)
	}
	return []byte(b.String()), nil
}

func workspacePath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
}

func moduleToken(module goWorkModule) string {
	if module.Version == "" {
		return module.Path
	}
	return module.Path + "@" + module.Version
}

func writeImmutable(path string, body []byte) error {
	if current, err := os.ReadFile(path); err == nil {
		if string(current) != string(body) {
			return fmt.Errorf("workspace snapshot collision at %s", path)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".go.work-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("commit workspace snapshot: %w", err)
	}
	return nil
}

// Command scopes a child to the canonical workspace. It is retained for short,
// explicitly locked operations; long-lived execution should use Workspace.Command.
func Command(ctx context.Context, env Environment, dir, program string, args ...string) *exec.Cmd {
	return commandForWorkFile(ctx, env, env.WorkFile, dir, program, args...)
}

// Command creates a child process pinned to this immutable workspace snapshot.
func (workspace Workspace) Command(ctx context.Context, dir, program string, args ...string) *exec.Cmd {
	return commandForWorkFile(ctx, workspace.Environment, workspace.WorkFile, dir, program, args...)
}

func commandForWorkFile(ctx context.Context, env Environment, workFile, dir, program string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, program, args...)
	if strings.TrimSpace(dir) == "" {
		dir = env.Dir
	}
	cmd.Dir = dir
	cmd.Env = withEnv(os.Environ(), "GOWORK", workFile)
	cmd.Env = withEnv(cmd.Env, "SMOKE_ENV", env.Name)
	cmd.Env = withEnv(cmd.Env, "SMOKE_ENV_WORKSPACE", workFile)
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
