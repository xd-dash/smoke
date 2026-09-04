package session

import (
	"reflect"
	"testing"
)

func TestSanitizeCallbacks(t *testing.T) {
	got := SanitizeCallbacks([]string{
		"stdout",
		"https://user:secret@example.com/hook?token=hidden",
		"axiom://events?domain=us-east-1.aws.edge.axiom.co&token-env=SECRET_NAME",
	})
	want := []string{
		"stdout",
		"https://example.com/hook",
		"axiom:events@us-east-1.aws.edge.axiom.co",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SanitizeCallbacks() = %#v, want %#v", got, want)
	}
}
