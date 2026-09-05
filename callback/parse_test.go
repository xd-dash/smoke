package callback

import "testing"

func TestParseStdoutAndWebhook(t *testing.T) {
	d, err := Parse([]string{"stdout", "https://example.com/hook"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Empty() {
		t.Fatal("expected callbacks")
	}
}

func TestParseRejectsUnsupportedCallback(t *testing.T) {
	if _, err := Parse([]string{"file:///tmp/messages"}); err == nil {
		t.Fatal("expected unsupported callback error")
	}
}
