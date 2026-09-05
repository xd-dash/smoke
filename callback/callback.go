package callback

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// Message is the provider-neutral callback envelope.
type Message struct {
	Provider string `json:"provider"`
	Source   string `json:"source"`
	Channel  string `json:"channel,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
	Payload  string `json:"payload"`
}

// Callback consumes one provider message.
type Callback interface {
	Name() string
	Handle(context.Context, Message) error
}

// FailurePolicy controls whether callback delivery failures terminate the
// provider session. Continue is the default for ephemeral bridges.
type FailurePolicy string

const (
	Continue FailurePolicy = "continue"
	FailFast FailurePolicy = "fail-fast"
)

// ErrorHandler receives callback delivery failures when the dispatcher is in
// Continue mode. It may log, count, or otherwise record the failure.
type ErrorHandler func(context.Context, Message, error)

type callbackSet struct {
	callbacks []Callback
	stdout    bool
}

// Dispatcher fans each message out to an immutable snapshot of the current
// callback set. Callback membership is replaced atomically, so lifecycle
// operations such as detaching stdout do not require a registry lock and do
// not race an in-flight dispatch.
type Dispatcher struct {
	callbacks    atomic.Pointer[callbackSet]
	policy       FailurePolicy
	errorHandler ErrorHandler
}

func New(callbacks ...Callback) *Dispatcher {
	d := &Dispatcher{policy: Continue}
	d.callbacks.Store(newCallbackSet(callbacks))
	return d
}

func newCallbackSet(callbacks []Callback) *callbackSet {
	set := &callbackSet{callbacks: append([]Callback(nil), callbacks...)}
	for _, cb := range set.callbacks {
		if cb.Name() == "stdout" {
			set.stdout = true
		}
	}
	return set
}

func (d *Dispatcher) snapshot() *callbackSet {
	if d == nil {
		return nil
	}
	return d.callbacks.Load()
}

func (d *Dispatcher) HasStdout() bool {
	set := d.snapshot()
	return set != nil && set.stdout
}

func (d *Dispatcher) Empty() bool {
	set := d.snapshot()
	return set == nil || len(set.callbacks) == 0
}

// Remove removes every callback with the given name. An in-flight Dispatch may
// finish using the snapshot it already acquired; subsequent dispatches observe
// the replacement immediately.
func (d *Dispatcher) Remove(name string) bool {
	if d == nil || name == "" {
		return false
	}
	for {
		current := d.callbacks.Load()
		if current == nil || len(current.callbacks) == 0 {
			return false
		}
		kept := make([]Callback, 0, len(current.callbacks))
		removed := false
		for _, cb := range current.callbacks {
			if cb.Name() == name {
				removed = true
				continue
			}
			kept = append(kept, cb)
		}
		if !removed {
			return false
		}
		next := newCallbackSet(kept)
		if d.callbacks.CompareAndSwap(current, next) {
			return true
		}
	}
}

func (d *Dispatcher) FailurePolicy() FailurePolicy {
	if d == nil || d.policy == "" {
		return Continue
	}
	return d.policy
}

func (d *Dispatcher) SetFailurePolicy(policy FailurePolicy) error {
	if d == nil {
		return errors.New("nil dispatcher")
	}
	switch policy {
	case Continue, FailFast:
		d.policy = policy
		return nil
	default:
		return fmt.Errorf("unsupported callback failure policy %q", policy)
	}
}

func (d *Dispatcher) SetErrorHandler(handler ErrorHandler) {
	if d != nil {
		d.errorHandler = handler
	}
}

type callbackError struct {
	name string
	err  error
}

func (d *Dispatcher) Dispatch(ctx context.Context, message Message) error {
	set := d.snapshot()
	if set == nil || len(set.callbacks) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errCh := make(chan callbackError, len(set.callbacks))

	for _, cb := range set.callbacks {
		cb := cb
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := cb.Handle(ctx, message); err != nil {
				errCh <- callbackError{name: cb.Name(), err: fmt.Errorf("%s callback: %w", cb.Name(), err)}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for callbackErr := range errCh {
		errs = append(errs, callbackErr.err)
		// Stdout is observational rather than authoritative. If its inherited
		// descriptor disappears, detach it and keep the remaining callbacks
		// supervised by Logmash.
		if d.FailurePolicy() == Continue && callbackErr.name == "stdout" {
			d.Remove("stdout")
		}
	}
	joined := errors.Join(errs...)
	if joined == nil {
		return nil
	}

	if d.FailurePolicy() == FailFast {
		return joined
	}

	if d.errorHandler != nil {
		d.errorHandler(ctx, message, joined)
	}
	return nil
}
