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

// Dispatcher fans each message out to all callbacks. Callback failures are
// collected after every configured callback gets a chance to run.
type Dispatcher struct {
	callbacks []Callback
	stdout    bool
}

func New(callbacks ...Callback) *Dispatcher {
	d := &Dispatcher{callbacks: append([]Callback(nil), callbacks...)}
	for _, cb := range callbacks {
		if cb.Name() == "stdout" {
			d.stdout = true
		}
	}
	return d
}

func (d *Dispatcher) HasStdout() bool { return d != nil && d.stdout }

func (d *Dispatcher) Empty() bool { return d == nil || len(d.callbacks) == 0 }

func (d *Dispatcher) Dispatch(ctx context.Context, message Message) error {
	if d == nil || len(d.callbacks) == 0 {
		return nil
	}

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
	for err := range errCh {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
