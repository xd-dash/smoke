package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// Info describes the logical composition of the currently running Smoke binary.
// Components self-register during package initialization, so this identity is
// owned by the running process rather than the mutable composition manifest.
type Info struct {
	Digest     string
	Executable string
	GoVersion  string
	Components []string
}

var state = struct {
	sync.RWMutex
	components map[string]struct{}
}{components: map[string]struct{}{}}

// RegisterComponent records one optional package as part of the running Smoke
// composition. Optional component packages should call this from init().
func RegisterComponent(importPath string) {
	importPath = strings.TrimSpace(importPath)
	if importPath == "" || strings.ContainsAny(importPath, " \t\r\n") {
		panic(fmt.Sprintf("smoke identity: invalid component %q", importPath))
	}
	state.Lock()
	state.components[importPath] = struct{}{}
	state.Unlock()
}

func Components() []string {
	state.RLock()
	defer state.RUnlock()
	out := make([]string, 0, len(state.components))
	for component := range state.components {
		out = append(out, component)
	}
	sort.Strings(out)
	return out
}

func CompositionDigest() string {
	h := sha256.New()
	for _, component := range Components() {
		_, _ = h.Write([]byte(component))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func Current() Info {
	exe, _ := os.Executable()
	if exe != "" {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
	}
	return Info{
		Digest:     CompositionDigest(),
		Executable: exe,
		GoVersion:  runtime.Version(),
		Components: Components(),
	}
}

// WorkspaceDigest returns the content-addressed workspace identity inherited by
// an environment-backed runtime. Empty means this process is not running from a
// Smoke environment snapshot.
func WorkspaceDigest() string {
	work := strings.TrimSpace(os.Getenv("SMOKE_ENV_WORKSPACE"))
	if work == "" {
		return ""
	}
	return filepath.Base(filepath.Dir(work))
}
