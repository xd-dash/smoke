//go:build windows

package logmash

import (
	"fmt"
	"os"
	"os/exec"
)

func detach(args []string) (int, error) {
	exe, err := os.Executable()
	if err != nil { return 0, err }
	childArgs := append([]string{"logmash"}, args...)
	cmd := exec.Command(exe, childArgs...)
	cmd.Env = append(os.Environ(), "LOGMASH_BACKGROUND=1")
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil { return 0, fmt.Errorf("start background child: %w", err) }
	return cmd.Process.Pid, nil
}
