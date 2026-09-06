package filelock

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSharedLocksCoexistAndBlockExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	first, err := Acquire(context.Background(), path, Shared)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := Acquire(context.Background(), path, Shared)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	if lock, ok, err := TryAcquire(path, Exclusive); err != nil {
		t.Fatal(err)
	} else if ok {
		_ = lock.Close()
		t.Fatal("exclusive lock unexpectedly acquired while shared locks are held")
	}
}

func TestAcquireHonorsCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	lock, err := Acquire(context.Background(), path, Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := Acquire(ctx, path, Exclusive); err == nil {
		t.Fatal("expected canceled lock acquisition")
	}
}
