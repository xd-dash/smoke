package callback

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type failingCallback struct{}

func (failingCallback) Name() string { return "failing" }
func (failingCallback) Handle(context.Context, Message) error { return errors.New("boom") }

func TestDispatcherConcurrentConfigurationAndDispatch(t *testing.T) {
	d := New(failingCallback{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_ = d.SetFailurePolicy(Continue)
			} else {
				_ = d.SetFailurePolicy(FailFast)
			}
			d.SetErrorHandler(func(context.Context, Message, error) {})
		}(i)
		go func() {
			defer wg.Done()
			_ = d.Dispatch(context.Background(), Message{Provider: "test", Payload: "x"})
		}()
	}
	wg.Wait()
}
