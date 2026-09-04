package callback

import (
	"context"
	"fmt"
	"io"
	"os"
)

type Stdout struct {
	Writer io.Writer
}

func (Stdout) Name() string { return "stdout" }

func (s Stdout) Handle(_ context.Context, message Message) error {
	w := s.Writer
	if w == nil {
		w = os.Stdout
	}
	_, err := fmt.Fprintln(w, message.Payload)
	return err
}
