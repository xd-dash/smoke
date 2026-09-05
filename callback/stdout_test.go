package callback

import (
	"bytes"
	"context"
	"testing"
)

func TestStdoutPreservesSourceRelationship(t *testing.T) {
	var out bytes.Buffer
	cb := Stdout{Writer: &out}
	if err := cb.Handle(context.Background(), Message{
		Source:  "us:west",
		Channel: "events",
		Payload: "hello",
	}); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "us:west:events\thello\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestStdoutFallsBackToPayloadWithoutSource(t *testing.T) {
	var out bytes.Buffer
	cb := Stdout{Writer: &out}
	if err := cb.Handle(context.Background(), Message{Payload: "hello"}); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "hello\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
