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
// Component identity is owned by the running process rather than mutable
// composition-manifest state.
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

// SetComponents installs component identities embedded by an entrypoint. Calls
// are additive and idempotent so packages may also self-register their identity.
func SetComponents(components ...string) {
	for _, component := range components {
		RegisterComponent(component)
	}
}

// RegisterComponent records one optional package in the running composition.
// This is the fallback for generated compositions until all entrypoints embed the
// normalized component list directly.
func RegisterComponent(component string) {
	component = strings.TrimSpace(component)
	if component == "" || strings.ContainsAny(component, " \t\r\n") {
		panic(fmt.Sprintf("smoke identity: invalid component %q", component))
	}
	state.Lock()
	state.components[component] = struct{}{}
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

func Environment() string {
	return strings.TrimSpace(os.Getenv("SMOKE_ENV"))
}

func Workspace() string {
	return strings.TrimSpace(os.Getenv("SMOKE_ENV_WORKSPACE"))
}

func WorkspaceDigest() string {
	work := Workspace()
	if work == "" {
		return ""
	}
	return filepath.Base(filepath.Dir(work))
}
