package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// Info describes the logical composition of the currently running Smoke binary.
// Components are populated by the binary's composition entrypoint, not by the
// mutable on-disk composition manifest.
type Info struct {
	Digest     string
	Executable string
	GoVersion  string
	Components []string
}

var state = struct {
	sync.RWMutex
	components []string
}{}

// SetComponents records the exact logical component list compiled into this
// entrypoint. Generated compositions and the default cmd/smoke entrypoint call
// this before dispatching commands.
func SetComponents(values ...string) {
	components := normalize(values)
	state.Lock()
	state.components = components
	state.Unlock()
}

func Components() []string {
	state.RLock()
	defer state.RUnlock()
	return append([]string(nil), state.components...)
}

func CompositionDigest() string {
	components := Components()
	h := sha256.New()
	for _, component := range components {
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
	dir := filepath.Dir(work)
	return filepath.Base(dir)
}

func normalize(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
