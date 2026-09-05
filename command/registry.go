package command

import (
	"fmt"
	"sort"
	"sync"
)

// Handler is the in-process entry point for a command compiled into Smoke.
// Optional command packages register handlers from init(); a composed Smoke
// binary selects its command set purely through Go imports.
type Handler func([]string) error

var global = struct {
	sync.RWMutex
	handlers map[string]Handler
}{handlers: map[string]Handler{}}

// Register adds one compiled-in command. It panics on programmer errors so a
// bad composition fails immediately during process initialization rather than
// leaving an ambiguous command graph.
func Register(name string, handler Handler) {
	if name == "" {
		panic("smoke command: empty name")
	}
	if handler == nil {
		panic(fmt.Sprintf("smoke command %q: nil handler", name))
	}
	global.Lock()
	defer global.Unlock()
	if _, exists := global.handlers[name]; exists {
		panic(fmt.Sprintf("smoke command %q registered twice", name))
	}
	global.handlers[name] = handler
}

// Run executes a command already linked into the current Smoke binary.
func Run(name string, args []string) error {
	global.RLock()
	handler := global.handlers[name]
	global.RUnlock()
	if handler == nil {
		return fmt.Errorf("command %q is not compiled into this smoke", name)
	}
	return handler(args)
}

// Names reports the commands present in this particular Smoke composition.
func Names() []string {
	global.RLock()
	defer global.RUnlock()
	out := make([]string, 0, len(global.handlers))
	for name := range global.handlers {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
