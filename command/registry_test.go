package command

import (
	"fmt"
	"reflect"
	"testing"
)

func TestRegisterAndRunCompiledCommand(t *testing.T) {
	name := "test-compiled-command"
	Register(name, func(args []string) error {
		if !reflect.DeepEqual(args, []string{"one", "two"}) {
			return fmt.Errorf("args = %#v", args)
		}
		return nil
	})
	if err := Run(name, []string{"one", "two"}); err != nil { t.Fatal(err) }
}

func TestUnknownCompiledCommandRejected(t *testing.T) {
	if err := Run("definitely-not-compiled", nil); err == nil { t.Fatal("expected unregistered command error") }
}

func TestNamesContainsRegisteredCommand(t *testing.T) {
	name := "test-listed-command"
	Register(name, func([]string) error { return nil })
	for _, got := range Names() { if got == name { return } }
	t.Fatalf("Names() does not contain %q: %#v", name, Names())
}

func TestRegisterRejectsReservedAndWhitespaceNames(t *testing.T) {
	for _, name := range []string{"env", "compose", "commands", "inspect", "bad name", " padded"} {
		t.Run(name, func(t *testing.T) {
			defer func() { if recover() == nil { t.Fatalf("Register(%q) did not panic", name) } }()
			Register(name, func([]string) error { return nil })
		})
	}
}
