package session

import (
	"reflect"
	"testing"
)

func TestSanitizeCallbacks(t *testing.T) {
	got := SanitizeCallbacks([]string{
		"stdout",
		"https://user:secret@example.com/hook?token=hidden",
		"axiom://events?profile=axiom.logma.sh&token-env=SECRET_NAME",
	})
	want := []string{
		"stdout",
		"https://example.com/hook",
		"axiom:events@axiom.logma.sh",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SanitizeCallbacks() = %#v, want %#v", got, want)
	}
}
