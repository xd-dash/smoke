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
	return os.WriteFile(path, data, 0o600)
}

func Add(importPath string) (Manifest, error) {
	manifest, err := Load()
	if err != nil {
		return Manifest{}, err
	}
	manifest.Components = append(manifest.Components, importPath)
	manifest.Components = normalize(manifest.Components)
	return manifest, Save(manifest)
}

func Remove(importPath string) (Manifest, error) {
	manifest, err := Load()
	if err != nil {
		return Manifest{}, err
	}
	needle := strings.TrimSpace(importPath)
	out := manifest.Components[:0]
	for _, component := range manifest.Components {
		if component != needle {
			out = append(out, component)
		}
	}
	manifest.Components = normalize(out)
	return manifest, Save(manifest)
}

func Rebuild(ctx context.Context) (string, error) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("Smoke recomposition requires a preinstalled Go toolchain: %w", err)
	}
	manifest, err := Load()
	if err != nil {
		return "", err
	}
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
	if info.Main.Path == "github.com/xd-dash/smoke" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/xd-dash/smoke" && dep.Version != "" && dep.Version != "(devel)" {
			return dep.Version
		}
	}
	return ""
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
