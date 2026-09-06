package command

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Handler func([]string) error

var global = struct {
	sync.RWMutex
	handlers map[string]Handler
}{handlers: map[string]Handler{}}

var reserved = map[string]struct{}{
	"commands": {},
	"compose":  {},
	"env":      {},
	"inspect":  {},
}

func Register(name string, handler Handler) {
	if name == "" { panic("smoke command: empty name") }
	if strings.TrimSpace(name) != name || strings.ContainsAny(name, " \t\r\n") { panic(fmt.Sprintf("smoke command %q: name must be one non-whitespace token", name)) }
	if _, exists := reserved[name]; exists { panic(fmt.Sprintf("smoke command %q: reserved by Smoke", name)) }
	if handler == nil { panic(fmt.Sprintf("smoke command %q: nil handler", name)) }
	global.Lock(); defer global.Unlock()
	if _, exists := global.handlers[name]; exists { panic(fmt.Sprintf("smoke command %q registered twice", name)) }
	global.handlers[name] = handler
}

func Run(name string, args []string) error {
	global.RLock(); handler := global.handlers[name]; global.RUnlock()
	if handler == nil { return fmt.Errorf("command %q is not compiled into this smoke", name) }
	return handler(args)
}

func Names() []string {
	global.RLock(); defer global.RUnlock()
	out := make([]string, 0, len(global.handlers))
	for name := range global.handlers { out = append(out, name) }
	sort.Strings(out)
	return out
}
