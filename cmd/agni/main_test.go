package agni

import "testing"

func TestRunRejectsUnknownCommand(t *testing.T) {
	if err := Run([]string{"unknown"}); err == nil {
		t.Fatal("expected unknown command error")
	}
}

func TestRunRequiresSubcommand(t *testing.T) {
	if err := Run(nil); err == nil {
		t.Fatal("expected usage error")
	}
}
