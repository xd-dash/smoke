package selfbuild

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
)

const DefaultLogmash = "github.com/xd-dash/smoke/cmd/logmash"

type Manifest struct {
	Components []string `json:"components"`
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
	tmp := path + fmt.Sprintf(".tmp-%d", os.Getpid())
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := os.Rename(tmp, path); err != nil {
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

// Apply builds the requested composition first. Only after the Go build
// succeeds does it persist the manifest and atomically replace the current
// Smoke executable. A bad optional component therefore leaves the currently
// running Smoke binary and the previous manifest intact.
func Apply(ctx context.Context, manifest Manifest) (string, error) {
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
	if err := os.WriteFile(filepath.Join(sourceDir, "main.go"), []byte(renderMain(manifest)), 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "go.mod"), []byte(renderGoMod()), 0o600); err != nil {
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

	// Do not commit a bad desired composition. Persist only after the candidate
	// build has succeeded, then replace the executable with the exact candidate.
	if err := Save(manifest); err != nil {
		return "", err
	}
	if err := os.Rename(staged, target); err != nil {
		return "", fmt.Errorf("replace %s: %w", target, err)
	}
	return target, nil
}

func runGo(ctx context.Context, dir, goBin string, args ...string) error {
	cmd := exec.CommandContext(ctx, goBin, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func renderMain(manifest Manifest) string {
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n\t\"os\"\n\t\"github.com/xd-dash/smoke/smokeapp\"\n")
	for _, component := range normalize(manifest.Components) {
		fmt.Fprintf(&b, "\t_ %q\n", component)
	}
	b.WriteString(")\n\nfunc main() { smokeapp.Main(os.Args[1:]) }\n")
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
