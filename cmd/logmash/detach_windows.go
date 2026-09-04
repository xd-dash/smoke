//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func detach(args []string) (int, string, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, "", err
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return 0, "", err
	}
	logDir := filepath.Join(cacheDir, "smoke")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return 0, "", err
	}
	logPath := filepath.Join(logDir, "logmash-background.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, "", err
	}
	defer logFile.Close()
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return 0, "", err
	}
	defer devNull.Close()

	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "LOGMASH_DETACHED=1")
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return 0, "", fmt.Errorf("start detached child: %w", err)
	}
	return cmd.Process.Pid, logPath, nil
}
