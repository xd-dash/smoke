package callback

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

type callbackFunc struct {
	name string
	fn   func(context.Context, Message) error
}

func (c callbackFunc) Name() string { return c.name }
func (c callbackFunc) Handle(ctx context.Context, m Message) error {
	return c.fn(ctx, m)
}

func TestDispatcherContinuesAndReportsByDefault(t *testing.T) {
	want := errors.New("webhook down")
	d := New(callbackFunc{name: "webhook", fn: func(context.Context, Message) error { return want }})

	var reported atomic.Int32
	d.SetErrorHandler(func(_ context.Context, _ Message, err error) {
		if !errors.Is(err, want) {
			t.Errorf("reported error = %v, want wrapped %v", err, want)
		}
		reported.Add(1)
	})

	if err := d.Dispatch(context.Background(), Message{Provider: "redis", Channel: "events"}); err != nil {
		t.Fatalf("Dispatch() error = %v, want nil", err)
	}
	if got := reported.Load(); got != 1 {
		t.Fatalf("reported = %d, want 1", got)
	}
}

func TestDispatcherFailFastReturnsCallbackError(t *testing.T) {
	want := errors.New("webhook down")
	d := New(callbackFunc{name: "webhook", fn: func(context.Context, Message) error { return want }})
	if err := d.SetFailurePolicy(FailFast); err != nil {
		t.Fatal(err)
	}

	err := d.Dispatch(context.Background(), Message{Provider: "redis", Channel: "events"})
	if !errors.Is(err, want) {
		t.Fatalf("Dispatch() error = %v, want wrapped %v", err, want)
	}
}

func TestDispatcherRejectsUnknownFailurePolicy(t *testing.T) {
	d := New()
	if err := d.SetFailurePolicy(FailurePolicy("explode")); err == nil {
		t.Fatal("SetFailurePolicy() error = nil, want error")
	}
}

func TestDispatcherRemoveStdoutKeepsOtherCallbacks(t *testing.T) {
	var stdoutCalls atomic.Int32
	var webhookCalls atomic.Int32
	d := New(
		callbackFunc{name: "stdout", fn: func(context.Context, Message) error { stdoutCalls.Add(1); return nil }},
		callbackFunc{name: "webhook", fn: func(context.Context, Message) error { webhookCalls.Add(1); return nil }},
	)
	if !d.HasStdout() {
		t.Fatal("expected stdout callback")
	}
	if !d.Remove("stdout") {
		t.Fatal("Remove(stdout) = false, want true")
	}
	if d.HasStdout() {
		t.Fatal("stdout callback still present")
	}
	if err := d.Dispatch(context.Background(), Message{}); err != nil {
		t.Fatal(err)
	}
	if got := stdoutCalls.Load(); got != 0 {
		t.Fatalf("stdout calls = %d, want 0", got)
	}
	if got := webhookCalls.Load(); got != 1 {
		t.Fatalf("webhook calls = %d, want 1", got)
	}
}

func TestDispatcherStdoutFailureDetachesInContinueMode(t *testing.T) {
	stdoutErr := errors.New("terminal gone")
	var stdoutCalls atomic.Int32
	var webhookCalls atomic.Int32
	d := New(
		callbackFunc{name: "stdout", fn: func(context.Context, Message) error { stdoutCalls.Add(1); return stdoutErr }},
		callbackFunc{name: "webhook", fn: func(context.Context, Message) error { webhookCalls.Add(1); return nil }},
	)
	if err := d.Dispatch(context.Background(), Message{}); err != nil {
		t.Fatalf("first Dispatch() error = %v", err)
	}
	if d.HasStdout() {
		t.Fatal("stdout should detach after write failure")
	}
	if err := d.Dispatch(context.Background(), Message{}); err != nil {
		t.Fatalf("second Dispatch() error = %v", err)
	}
	if got := stdoutCalls.Load(); got != 1 {
		t.Fatalf("stdout calls = %d, want 1", got)
	}
	if got := webhookCalls.Load(); got != 2 {
		t.Fatalf("webhook calls = %d, want 2", got)
	}
}
