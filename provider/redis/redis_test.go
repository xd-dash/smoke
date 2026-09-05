package redisprovider

import (
	"net/url"
	"testing"
)

func TestOptionsTCP(t *testing.T) {
	opts, err := optionsForTarget(Target{
		Network:    "tcp",
		Host:       "example.com",
		Port:       6380,
		TLS:        true,
		ServerName: "example.com",
		Username:   "user",
		Password:   "secret",
		DB:         2,
	})
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

func TestRedisProfileMetadataSkipsOtherSmokeProfiles(t *testing.T) {
	meta, ok := redisProfileMetadata([]string{
		"verification=example",
		"smoke=v1;provider=axiom;domain=example.axiom.co",
		"smoke=v1;provider=redis;tls=true;auth-provider=acl-env;auth=us-west",
	})
	if !ok {
		t.Fatal("expected Redis profile")
	}
	if meta["provider"] != "redis" || meta["tls"] != "true" || meta["auth"] != "us-west" {
		t.Fatalf("unexpected metadata: %#v", meta)
	}
}

func TestRedisProfileMetadataRejectsMissingRedisProfile(t *testing.T) {
	if _, ok := redisProfileMetadata([]string{"smoke=v1;provider=axiom"}); ok {
		t.Fatal("unexpected Redis profile")
	}
}
