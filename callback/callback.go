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
// Logmash runtime is composed. Callback membership is fixed for that runtime.
type Dispatcher struct {
	callbacks    []Callback
	policy       FailurePolicy
	errorHandler ErrorHandler
}

func New(callbacks ...Callback) *Dispatcher {
	return &Dispatcher{callbacks: append([]Callback(nil), callbacks...), policy: Continue}
}

func (d *Dispatcher) Empty() bool { return d == nil || len(d.callbacks) == 0 }

func (d *Dispatcher) FailurePolicy() FailurePolicy {
	if d == nil || d.policy == "" { return Continue }
	return d.policy
}

func (d *Dispatcher) SetFailurePolicy(policy FailurePolicy) error {
	if d == nil { return errors.New("nil dispatcher") }
	switch policy {
	case Continue, FailFast:
		d.policy = policy
		return nil
	default:
		return fmt.Errorf("unsupported callback failure policy %q", policy)
	}
}

func (d *Dispatcher) SetErrorHandler(handler ErrorHandler) {
	if d != nil { d.errorHandler = handler }
}

func (d *Dispatcher) Dispatch(ctx context.Context, message Message) error {
	if d == nil || len(d.callbacks) == 0 { return nil }
	var wg sync.WaitGroup
	errCh := make(chan error, len(d.callbacks))
	for _, cb := range d.callbacks {
		cb := cb
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := cb.Handle(ctx, message); err != nil {
				errCh <- fmt.Errorf("%s callback: %w", cb.Name(), err)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	var errs []error
	for err := range errCh { errs = append(errs, err) }
	joined := errors.Join(errs...)
	if joined == nil { return nil }
	if d.FailurePolicy() == FailFast { return joined }
	if d.errorHandler != nil { d.errorHandler(ctx, message, joined) }
	return nil
}
