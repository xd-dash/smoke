//go:build windows

package session

import (
	"os"

	"golang.org/x/sys/windows"
)

func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil { return false }
	_ = windows.CloseHandle(h)
	return true
}

func stopProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil { return err }
	return process.Kill()
}
