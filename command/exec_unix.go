//go:build !windows

package command

import (
	"os"
	"syscall"
)

// Exec replaces Smoke with the resolved command. The invoked command therefore
// owns the terminal and signal lifecycle exactly as if it had been run directly.
func Exec(path string, args []string) error {
	return syscall.Exec(path, append([]string{path}, args...), os.Environ())
}
