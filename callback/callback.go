package callback

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

type ErrorHandler func(context.Context, Message, error)

// Dispatcher fans each message out to the callbacks selected when the
// Logmash runtime is composed. Callback membership is fixed for that runtime;
// policy and error reporting may be configured safely before or during use.
type Dispatcher struct {
	callbacks []Callback

	mu           sync.RWMutex
	policy       FailurePolicy
	errorHandler ErrorHandler
}

func New(callbacks ...Callback) *Dispatcher {
	return &Dispatcher{callbacks: append([]Callback(nil), callbacks...), policy: Continue}
}

func (d *Dispatcher) Empty() bool { return d == nil || len(d.callbacks) == 0 }

func (d *Dispatcher) FailurePolicy() FailurePolicy {
	if d == nil {
		return Continue
	}
	d.mu.RLock()
	policy := d.policy
	d.mu.RUnlock()
	if policy == "" {
		return Continue
	}
	return policy
}

func (d *Dispatcher) SetFailurePolicy(policy FailurePolicy) error {
	if d == nil {
		return errors.New("nil dispatcher")
	}
	switch policy {
	case Continue, FailFast:
		d.mu.Lock()
		d.policy = policy
		d.mu.Unlock()
		return nil
	default:
		return fmt.Errorf("unsupported callback failure policy %q", policy)
	}
}

func (d *Dispatcher) SetErrorHandler(handler ErrorHandler) {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.errorHandler = handler
	d.mu.Unlock()
}

func (d *Dispatcher) config() (FailurePolicy, ErrorHandler) {
	if d == nil {
		return Continue, nil
	}
	d.mu.RLock()
	policy, handler := d.policy, d.errorHandler
	d.mu.RUnlock()
	if policy == "" {
		policy = Continue
	}
	return policy, handler
}

func (d *Dispatcher) Dispatch(ctx context.Context, message Message) error {
	if d == nil || len(d.callbacks) == 0 {
		return nil
	}
	policy, errorHandler := d.config()

	var wg sync.WaitGroup
	errCh := make(chan error, len(d.callbacks))
	for _, cb := range d.callbacks {
		cb := cb
		wg.Add(1)
		go func() {
			defer wg.Done()
			if cb == nil {
				errCh <- errors.New("nil callback")
				return
			}
			if err := cb.Handle(ctx, message); err != nil {
				errCh <- fmt.Errorf("%s callback: %w", cb.Name(), err)
			}
		}()
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	joined := errors.Join(errs...)
	if joined == nil {
		return nil
	}
	if policy == FailFast {
		return joined
	}
	if errorHandler != nil {
		errorHandler(ctx, message, joined)
	}
	return nil
}
