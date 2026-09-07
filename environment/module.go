package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Module describes a Go module query after Go has resolved it to a concrete
// module version and materialized it in the module cache.
type Module struct {
	Requested string
	Path      string
	Version   string
	Dir       string
	Sum       string
	GoModSum  string
}

type goListModule struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
}

type goDownloadModule struct {
	Path     string `json:"Path"`
	Version  string `json:"Version"`
	Dir      string `json:"Dir"`
	Sum      string `json:"Sum"`
	GoModSum string `json:"GoModSum"`
	Error    *struct {
		Err string `json:"Err"`
	} `json:"Error"`
}

// AddModule resolves moduleSpec with the system Go command, materializes the
// resolved module through Go's module cache, and adds that local module
// directory to the environment's ordinary go.work file.
//
// Smoke deliberately does not interpret Git branches, tags, pseudo-versions,
// proxies, sums, or replacements here. Those are Go module semantics.
func AddModule(ctx context.Context, name, moduleSpec string) (Module, error) {
	env, err := Require(name)
	if err != nil {
		return Module{}, err
	}
	moduleSpec = strings.TrimSpace(moduleSpec)
	if moduleSpec == "" {
		return Module{}, fmt.Errorf("module query is required")
	}
	if !strings.Contains(moduleSpec, "@") {
		return Module{}, fmt.Errorf("module query must include @version or @revision: %q", moduleSpec)
	}

	lock, err := acquireExclusive(ctx, env)
	if err != nil {
		return Module{}, err
	}
	defer lock.Close()

	goBin, err := exec.LookPath("go")
	if err != nil {
		return Module{}, fmt.Errorf("Smoke environments require a preinstalled Go toolchain: %w", err)
	}

	resolvedOut, err := goOutput(ctx, goBin, env.ToolsDir, "list", "-m", "-json", moduleSpec)
	if err != nil {
		return Module{}, fmt.Errorf("resolve Go module %s: %w", moduleSpec, err)
	}
	var resolved goListModule
	if err := json.Unmarshal(resolvedOut, &resolved); err != nil {
		return Module{}, fmt.Errorf("decode resolved Go module %s: %w", moduleSpec, err)
	}
	if strings.TrimSpace(resolved.Path) == "" || strings.TrimSpace(resolved.Version) == "" {
		return Module{}, fmt.Errorf("Go resolved %s without a concrete module path and version", moduleSpec)
	}

	resolvedSpec := resolved.Path + "@" + resolved.Version
	downloadOut, err := goOutput(ctx, goBin, env.ToolsDir, "mod", "download", "-json", resolvedSpec)
	if err != nil {
		return Module{}, fmt.Errorf("download Go module %s: %w", resolvedSpec, err)
	}
	var downloaded goDownloadModule
	if err := json.Unmarshal(downloadOut, &downloaded); err != nil {
		return Module{}, fmt.Errorf("decode downloaded Go module %s: %w", resolvedSpec, err)
	}
	if downloaded.Error != nil && strings.TrimSpace(downloaded.Error.Err) != "" {
		return Module{}, fmt.Errorf("download Go module %s: %s", resolvedSpec, downloaded.Error.Err)
	}
	if strings.TrimSpace(downloaded.Dir) == "" {
		return Module{}, fmt.Errorf("Go downloaded %s without a module directory", resolvedSpec)
	}
	if _, err := os.Stat(filepath.Join(downloaded.Dir, "go.mod")); err != nil {
		return Module{}, fmt.Errorf("downloaded Go module %s: %w", downloaded.Dir, err)
	}

	if err := runGo(ctx, env.Dir, env.WorkFile, "work", "use", downloaded.Dir); err != nil {
		return Module{}, err
	}
	return Module{
		Requested: moduleSpec,
		Path:      downloaded.Path,
		Version:   downloaded.Version,
		Dir:       downloaded.Dir,
		Sum:       downloaded.Sum,
		GoModSum:  downloaded.GoModSum,
	}, nil
}

// DropModule removes the go.work use entry whose module path matches
// modulePath. The module cache itself remains owned by Go.
func DropModule(ctx context.Context, name, modulePath string) error {
	env, err := Require(name)
	if err != nil {
		return err
	}
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" {
		return fmt.Errorf("module path is required")
	}

	lock, err := acquireExclusive(ctx, env)
	if err != nil {
		return err
	}
	defer lock.Close()

	work, err := readGoWork(ctx, env)
	if err != nil {
		return err
	}
	for _, use := range work.Use {
		if use.ModulePath == modulePath {
			return runGo(ctx, env.Dir, env.WorkFile, "work", "edit", "-dropuse="+workspacePath(env.Dir, use.DiskPath))
		}
	}
	return fmt.Errorf("module %q is not used by environment %q", modulePath, name)
}

// Modules returns the modules currently composed through go.work. The tools
// module is omitted because it is an implementation detail of env tool.
func Modules(ctx context.Context, name string) ([]goWorkUse, error) {
	env, err := Require(name)
	if err != nil {
		return nil, err
	}
	lock, err := AcquireShared(ctx, env)
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	work, err := readGoWork(ctx, env)
	if err != nil {
		return nil, err
	}
	out := make([]goWorkUse, 0, len(work.Use))
	for _, use := range work.Use {
		if samePath(workspacePath(env.Dir, use.DiskPath), env.ToolsDir) {
			continue
		}
		out = append(out, use)
	}
	return out, nil
}

func readGoWork(ctx context.Context, env Environment) (goWorkJSON, error) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return goWorkJSON{}, fmt.Errorf("Smoke environments require a preinstalled Go toolchain: %w", err)
	}
	output, err := goOutput(ctx, goBin, env.Dir, "work", "edit", "-json", env.WorkFile)
	if err != nil {
		return goWorkJSON{}, fmt.Errorf("read workspace %s: %w", env.WorkFile, err)
	}
	var parsed goWorkJSON
	if err := json.Unmarshal(output, &parsed); err != nil {
		return goWorkJSON{}, fmt.Errorf("decode workspace %s: %w", env.WorkFile, err)
	}
	return parsed, nil
}

func goOutput(ctx context.Context, goBin, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, goBin, args...)
	cmd.Dir = dir
	cmd.Env = withEnv(os.Environ(), "GOWORK", "off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return nil, fmt.Errorf("%w: %s", err, message)
		}
		return nil, err
	}
	return output, nil
}
