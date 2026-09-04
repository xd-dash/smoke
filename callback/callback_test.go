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
