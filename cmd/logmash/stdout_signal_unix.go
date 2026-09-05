//go:build !windows

package logmash

import (
	"os"
	"os/signal"
	"syscall"
)

func stdoutDetachSignal() (<-chan os.Signal, func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	return ch, func() { signal.Stop(ch) }
}
