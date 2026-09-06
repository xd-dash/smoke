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

// SetComponents installs the complete component identity embedded by the
// composition entrypoint. The supplied list is authoritative: it replaces any
// compatibility registrations that may have happened during package init.
func SetComponents(components ...string) {
	next := make(map[string]struct{}, len(components))
	for _, component := range components {
		component = validComponent(component)
		next[component] = struct{}{}
	}
	state.Lock()
	state.components = next
	state.Unlock()
}

// RegisterComponent is retained for source compatibility with optional packages
// that adopted the earlier self-registration idiom. Generated and canonical
// Smoke entrypoints call SetComponents with the complete normalized composition,
// so package self-registration is no longer required for correct identity.
func RegisterComponent(component string) {
	component = validComponent(component)
	state.Lock()
	state.components[component] = struct{}{}
	state.Unlock()
}

func validComponent(component string) string {
	component = strings.TrimSpace(component)
	if component == "" || strings.ContainsAny(component, " \t\r\n") {
		panic(fmt.Sprintf("smoke identity: invalid component %q", component))
	}
	return component
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
