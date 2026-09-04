package callback

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
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
	if message.Source != "" && message.Channel != "" {
		source := strings.TrimSuffix(message.Source, ".logma.sh")
		_, err := fmt.Fprintf(w, "%s:%s\t%s\n", source, message.Channel, message.Payload)
		return err
	}
	_, err := fmt.Fprintln(w, message.Payload)
	return err
}
