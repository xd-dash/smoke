package redisprovider

import (
	"net/url"
	"testing"
)

func TestOptionsTCP(t *testing.T) {
	u, err := url.Parse("rediss://user:secret@example.com:6380?db=2&channel=events&callback=stdout")
	if err != nil {
		t.Fatal(err)
	}

	opts, err := options(u)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Addr != "example.com:6380" || opts.DB != 2 || opts.Username != "user" || opts.Password != "secret" {
		t.Fatalf("unexpected options: %+v", opts)
	}
	if opts.TLSConfig == nil {
		t.Fatal("expected TLS config")
	}
}

func TestPublicSourceStripsSecretsAndCallbacks(t *testing.T) {
	u, err := url.Parse("redis://user:secret@example.com:6379?channel=events&callback=https%3A%2F%2Fexample.net%2Fhook")
	if err != nil {
		t.Fatal(err)
	}

	got := publicSource(u)
	want := "redis://example.com:6379?channel=events"
	if got != want {
		t.Fatalf("publicSource() = %q, want %q", got, want)
	}
}
