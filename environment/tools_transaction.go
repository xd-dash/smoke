package environment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type toolManifestState struct {
	body   []byte
	mode   os.FileMode
	exists bool
}

// AddTools composes multiple Go tools into one environment as one manifest
// transaction. If any go get -tool operation fails, tools/go.mod and tools/go.sum
// are restored to their exact pre-call contents. Module-cache downloads are
// intentionally not rolled back because they are cache state, not environment
// authority.
func AddTools(ctx context.Context, name string, packageSpecs []string) error {
	return addToolsWithRunner(ctx, name, packageSpecs, runGo)
}

func addToolsWithRunner(ctx context.Context, name string, packageSpecs []string, runner func(context.Context, string, string, ...string) error) error {
	env, err := Require(name)
	if err != nil {
		return err
	}
	if len(packageSpecs) == 0 {
		return nil
	}
	specs := make([]string, 0, len(packageSpecs))
	for _, spec := range packageSpecs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			return fmt.Errorf("tool package is required")
		}
		specs = append(specs, spec)
	}

	lock, err := acquireExclusive(ctx, env)
	if err != nil {
		return err
	}
	defer lock.Close()

	modPath := filepath.Join(env.ToolsDir, "go.mod")
	sumPath := filepath.Join(env.ToolsDir, "go.sum")
	modBefore, err := readToolManifestState(modPath, true)
	if err != nil {
		return err
	}
	sumBefore, err := readToolManifestState(sumPath, false)
	if err != nil {
		return err
	}

	for _, spec := range specs {
		if err := runner(ctx, env.ToolsDir, "off", "get", "-tool", spec); err != nil {
			if restoreErr := restoreToolManifest(modPath, modBefore); restoreErr != nil {
				return fmt.Errorf("add tool %s: %v; restore tools go.mod: %w", spec, err, restoreErr)
			}
			if restoreErr := restoreToolManifest(sumPath, sumBefore); restoreErr != nil {
				return fmt.Errorf("add tool %s: %v; restore tools go.sum: %w", spec, err, restoreErr)
			}
			return fmt.Errorf("add tool %s: %w", spec, err)
		}
	}
	return nil
}

func readToolManifestState(path string, required bool) (toolManifestState, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) && !required {
		return toolManifestState{}, nil
	}
	if err != nil {
		return toolManifestState{}, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return toolManifestState{}, err
	}
	return toolManifestState{body: body, mode: info.Mode().Perm(), exists: true}, nil
}

func restoreToolManifest(path string, state toolManifestState) error {
	if !state.exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeToolManifest(path, state.body, state.mode)
}

func writeToolManifest(path string, body []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tool-manifest-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
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
	return os.Rename(tmpName, path)
}
