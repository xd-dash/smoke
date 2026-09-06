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
// The composition entrypoint installs the component list before command dispatch,
// so this identity belongs to the running binary rather than mutable manifest state.
type Info struct {
	Digest     string
	Executable string
	GoVersion  string
	Components []string
}

var state = struct {
	sync.RWMutex
	set        bool
	components []string
}{}

// SetComponents installs the normalized component list embedded into this Smoke
// entrypoint. It may be called once; a second call is a programmer error.
func SetComponents(components ...string) {
	normalized := normalize(components)
	state.Lock()
	defer state.Unlock()
	if state.set {
		panic("smoke identity: composition already set")
	}
	state.set = true
	state.components = normalized
}

func Components() []string {
	state.RLock()
	defer state.RUnlock()
	return append([]string(nil), state.components...)
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

// Environment returns the named canonical environment inherited by this process.
func Environment() string {
	return strings.TrimSpace(os.Getenv("SMOKE_ENV"))
}

// Workspace returns the exact immutable go.work snapshot inherited by an
// environment-backed runtime. Empty means this process is not snapshot-backed.
func Workspace() string {
	return strings.TrimSpace(os.Getenv("SMOKE_ENV_WORKSPACE"))
}

// WorkspaceDigest returns the content-addressed workspace identity inherited by
// an environment-backed runtime. Empty means this process is not running from a
// Smoke environment snapshot.
func WorkspaceDigest() string {
	work := Workspace()
	if work == "" {
		return ""
	}
	return filepath.Base(filepath.Dir(work))
}

func normalize(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.ContainsAny(value, " \t\r\n") {
			panic(fmt.Sprintf("smoke identity: invalid component %q", value))
		}
		set[value] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
