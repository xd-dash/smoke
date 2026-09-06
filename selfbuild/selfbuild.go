package selfbuild

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/xd-dash/smoke/internal/filelock"
)

const DefaultLogmash = "github.com/xd-dash/smoke/cmd/logmash"

type Manifest struct {
	Components []string `json:"components"`
}

type fileSnapshot struct {
	path   string
	data   []byte
	exists bool
}

func Load() (Manifest, error) {
	path, err := manifestPath()
	if err != nil {
		return Manifest{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Manifest{Components: []string{DefaultLogmash}}, nil
	}
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode %s: %w", path, err)
	}
	manifest.Components = normalize(manifest.Components)
	return manifest, nil
}

func Save(manifest Manifest) error {
	path, err := manifestPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	manifest.Components = normalize(manifest.Components)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".composition-manifest-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace composition manifest: %w", err)
	}
	return nil
}

func WithAdded(manifest Manifest, importPath string) Manifest {
	manifest.Components = append(manifest.Components, strings.TrimSpace(importPath))
	manifest.Components = normalize(manifest.Components)
	return manifest
}

func WithRemoved(manifest Manifest, importPath string) Manifest {
	needle := strings.TrimSpace(importPath)
	out := make([]string, 0, len(manifest.Components))
	for _, component := range manifest.Components {
		if strings.TrimSpace(component) != needle {
			out = append(out, component)
		}
	}
	manifest.Components = normalize(out)
	return manifest
}

// Update performs a locked read-modify-rebuild transaction. It prevents two
// compose operations from both reading an old manifest and silently losing one
// writer's change.
func Update(ctx context.Context, transform func(Manifest) Manifest) (string, error) {
	lock, err := acquire(ctx, filelock.Exclusive)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	manifest, err := Load()
	if err != nil {
		return "", err
	}
	return applyLocked(ctx, transform(manifest))
}

// Rebuild rebuilds the current desired composition under the same lock used by
// mutation, so a rebuild cannot overwrite a concurrent add/remove with stale state.
func Rebuild(ctx context.Context) (string, error) {
	lock, err := acquire(ctx, filelock.Exclusive)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	manifest, err := Load()
	if err != nil {
		return "", err
	}
	return applyLocked(ctx, manifest)
}

// Apply replaces the installed executable with exactly manifest. Prefer Update
// for read-modify-write operations.
func Apply(ctx context.Context, manifest Manifest) (string, error) {
	lock, err := acquire(ctx, filelock.Exclusive)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	return applyLocked(ctx, manifest)
}

// AcquireSpawnLock prevents composition replacement while a process is resolving
// and starting the currently installed Smoke executable. Callers should release
// it immediately after cmd.Start succeeds; it is not a lifetime lock.
func AcquireSpawnLock(ctx context.Context) (*filelock.Lock, error) {
	return acquire(ctx, filelock.Shared)
}

func applyLocked(ctx context.Context, manifest Manifest) (result string, retErr error) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("Smoke recomposition requires a preinstalled Go toolchain: %w", err)
	}
	manifest.Components = normalize(manifest.Components)

	sourceDir, err := compositionDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(sourceDir, 0o700); err != nil {
		return "", err
	}

	// go mod tidy and go build are allowed to mutate the generated composition
	// module. Snapshot it so a failed composition cannot leak dependency/version
	// changes into a later rebuild.
	snapshots, err := snapshotComposition(sourceDir)
	if err != nil {
		return "", err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if restoreErr := restoreComposition(snapshots); restoreErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("restore composition source state: %w", restoreErr))
		}
	}()

	if err := os.WriteFile(filepath.Join(sourceDir, "main.go"), []byte(renderMain(manifest)), 0o600); err != nil {
		return "", err
	}
	if err := ensureGoMod(ctx, sourceDir, goBin); err != nil {
		return "", err
	}
	if err := runGo(ctx, sourceDir, goBin, "mod", "tidy"); err != nil {
		return "", err
	}

	target, err := os.Executable()
	if err != nil {
		return "", err
	}
	target, err = filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	staged := filepath.Join(filepath.Dir(target), fmt.Sprintf(".%s.new-%d", filepath.Base(target), os.Getpid()))
	defer os.Remove(staged)
	if err := runGo(ctx, sourceDir, goBin, "build", "-o", staged, "."); err != nil {
		return "", err
	}
	if current, err := os.Stat(target); err == nil {
		_ = os.Chmod(staged, current.Mode().Perm())
	}

	if runtime.GOOS == "windows" {
		return staged, fmt.Errorf("built replacement at %s; in-place replacement of the running executable is not yet supported on Windows", staged)
	}

	manifestPath, previousManifest, previousManifestExists, err := snapshotManifest()
	if err != nil {
		return "", err
	}
	if err := Save(manifest); err != nil {
		return "", err
	}
	if err := os.Rename(staged, target); err != nil {
		if rollbackErr := restoreManifest(manifestPath, previousManifest, previousManifestExists); rollbackErr != nil {
			return "", fmt.Errorf("replace %s: %w; restore previous composition manifest: %v", target, err, rollbackErr)
		}
		return "", fmt.Errorf("replace %s: %w", target, err)
	}
	committed = true
	return target, nil
}

func snapshotComposition(dir string) ([]fileSnapshot, error) {
	paths := []string{
		filepath.Join(dir, "main.go"),
		filepath.Join(dir, "go.mod"),
		filepath.Join(dir, "go.sum"),
	}
	out := make([]fileSnapshot, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			out = append(out, fileSnapshot{path: path})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("snapshot %s: %w", path, err)
		}
		out = append(out, fileSnapshot{path: path, data: data, exists: true})
	}
	return out, nil
}

func restoreComposition(snapshots []fileSnapshot) error {
	var errs []error
	for _, snapshot := range snapshots {
		if !snapshot.exists {
			if err := os.Remove(snapshot.path); err != nil && !os.IsNotExist(err) {
				errs = append(errs, err)
			}
			continue
		}
		tmp, err := os.CreateTemp(filepath.Dir(snapshot.path), ".composition-restore-*.tmp")
		if err != nil {
			errs = append(errs, err)
			continue
		}
		tmpPath := tmp.Name()
		if err := tmp.Chmod(0o600); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			errs = append(errs, err)
			continue
		}
		if _, err := tmp.Write(snapshot.data); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			errs = append(errs, err)
			continue
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpPath)
			errs = append(errs, err)
			continue
		}
		if err := os.Rename(tmpPath, snapshot.path); err != nil {
			_ = os.Remove(tmpPath)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func ensureGoMod(ctx context.Context, sourceDir, goBin string) error {
	path := filepath.Join(sourceDir, "go.mod")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(renderGoMod()), 0o600); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	// Keep the composition module identity current without discarding versions
	// already selected for optional components.
	if err := runGo(ctx, sourceDir, goBin, "mod", "edit", "-module=smoke.local/composition", "-go=1.26"); err != nil {
		return err
	}
	if version := moduleVersion(); version != "" {
		if err := runGo(ctx, sourceDir, goBin, "mod", "edit", "-require=github.com/xd-dash/smoke@"+version); err != nil {
			return err
		}
	}
	return nil
}

func acquire(ctx context.Context, mode filelock.Mode) (*filelock.Lock, error) {
	dir, err := compositionDir()
	if err != nil {
		return nil, err
	}
	return filelock.Acquire(ctx, filepath.Join(dir, ".composition.lock"), mode)
}

func snapshotManifest() (string, []byte, bool, error) {
	path, err := manifestPath()
	if err != nil {
		return "", nil, false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return path, nil, false, nil
	}
	if err != nil {
		return "", nil, false, fmt.Errorf("snapshot composition manifest: %w", err)
	}
	return path, data, true, nil
}

func restoreManifest(path string, data []byte, existed bool) error {
	if !existed {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".composition-rollback-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func runGo(ctx context.Context, dir, goBin string, args ...string) error {
	cmd := exec.CommandContext(ctx, goBin, args...)
	cmd.Dir = dir
	// Recomposition is its own module operation. Never let an active Smoke
	// environment's GOWORK alter dependency selection for the Smoke binary.
	cmd.Env = withEnv(os.Environ(), "GOWORK", "off")
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

func renderMain(manifest Manifest) string {
	components := normalize(manifest.Components)
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n\t\"os\"\n\t\"github.com/xd-dash/smoke/identity\"\n\t\"github.com/xd-dash/smoke/smokeapp\"\n")
	for _, component := range components {
		fmt.Fprintf(&b, "\t_ %q\n", component)
	}
	b.WriteString(")\n\nfunc main() {\n\tidentity.SetComponents(\n")
	for _, component := range components {
		fmt.Fprintf(&b, "\t\t%q,\n", component)
	}
	b.WriteString("\t)\n\tsmokeapp.Main(os.Args[1:])\n}\n")
	return b.String()
}

func renderGoMod() string {
	var b strings.Builder
	b.WriteString("module smoke.local/composition\n\ngo 1.26\n")
	if version := moduleVersion(); version != "" {
		fmt.Fprintf(&b, "\nrequire github.com/xd-dash/smoke %s\n", version)
	}
	return b.String()
}

func moduleVersion() string {
	if version := strings.TrimSpace(os.Getenv("SMOKE_MODULE_VERSION")); version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if (info.Main.Path == "github.com/xd-dash/smoke" || strings.HasPrefix(info.Main.Path, "github.com/xd-dash/smoke/")) && usableVersion(info.Main.Version) {
		return info.Main.Version
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/xd-dash/smoke" && usableVersion(dep.Version) {
			return dep.Version
		}
	}
	return ""
}

func usableVersion(version string) bool {
	version = strings.TrimSpace(version)
	return version != "" && version != "(devel)"
}

func manifestPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv("SMOKE_COMPOSITION_MANIFEST")); path != "" {
		return path, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "smoke", "composition.json"), nil
}

func compositionDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("SMOKE_COMPOSITION_DIR")); dir != "" {
		return dir, nil
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".local", "share", "smoke", "composition"), nil
}

func normalize(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
