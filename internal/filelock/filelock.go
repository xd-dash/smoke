package filelock

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Mode int

const (
	Shared Mode = iota
	Exclusive
)

// Lock is an advisory cross-process file lock. The underlying file remains on
// disk, while ownership is tied to the open file descriptor/handle so a crash
// releases the lock automatically.
type Lock struct {
	file *os.File
}

// TryAcquire attempts to lock path once. ok is false when another process owns
// an incompatible lock. The returned lock must be closed when ok is true.
func TryAcquire(path string, mode Mode) (lock *Lock, ok bool, err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, false, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	locked, err := tryLock(file, mode)
	if err != nil {
		_ = file.Close()
		return nil, false, fmt.Errorf("lock %s: %w", path, err)
	}
	if !locked {
		_ = file.Close()
		return nil, false, nil
	}
	return &Lock{file: file}, true, nil
}

// Acquire waits until path can be locked in the requested mode or ctx is canceled.
func Acquire(ctx context.Context, path string, mode Mode) (*Lock, error) {
	for {
		lock, ok, err := TryAcquire(path, mode)
		if err != nil {
			return nil, err
		}
		if ok {
			return lock, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	unlockErr := unlock(file)
	closeErr := file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
