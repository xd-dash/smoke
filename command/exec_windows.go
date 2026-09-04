//go:build windows

package command

import (
	"os"
	"os/exec"
)

func Exec(path string, args []string) error {
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
