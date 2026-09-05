//go:build windows

package logmash

import "os"

func stdoutDetachSignal() (<-chan os.Signal, func()) {
	return nil, func() {}
}
