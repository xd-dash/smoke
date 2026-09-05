package redisauth

import (
	"context"
	"testing"
)

func TestPasswordEnvUsesProfilePassword(t *testing.T) {
	t.Setenv("LOGMASH_REDIS_WEST_PASSWORD", "secret")
	t.Setenv("REDISCLI_AUTH", "fallback")
	got, err := (PasswordEnv{}).Resolve(context.Background(), "west")
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "" || got.Password != "secret" {
		t.Fatalf("credentials = %#v", got)
	}
}

func TestPasswordEnvFallsBackToRedisCLIAuth(t *testing.T) {
	t.Setenv("REDISCLI_AUTH", "fallback")
	got, err := (PasswordEnv{}).Resolve(context.Background(), "west")
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "" || got.Password != "fallback" {
		t.Fatalf("credentials = %#v", got)
	}
}

func TestACLEnvRequiresUsernameAndPassword(t *testing.T) {
	t.Setenv("LOGMASH_REDIS_WEST_USERNAME", "observer")
	t.Setenv("LOGMASH_REDIS_WEST_PASSWORD", "secret")
	got, err := (ACLEnv{}).Resolve(context.Background(), "west")
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "observer" || got.Password != "secret" {
		t.Fatalf("credentials = %#v", got)
	}
}

func TestAutoEnvPrefersACL(t *testing.T) {
	t.Setenv("LOGMASH_REDIS_WEST_USERNAME", "observer")
	t.Setenv("LOGMASH_REDIS_WEST_PASSWORD", "secret")
	t.Setenv("REDISCLI_AUTH", "fallback")
	got, err := (AutoEnv{}).Resolve(context.Background(), "west")
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "observer" || got.Password != "secret" {
		t.Fatalf("credentials = %#v", got)
	}
}

func TestAutoEnvRejectsUsernameWithoutPassword(t *testing.T) {
	t.Setenv("LOGMASH_REDIS_WEST_USERNAME", "observer")
	if _, err := (AutoEnv{}).Resolve(context.Background(), "west"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNone(t *testing.T) {
	got, err := (None{}).Resolve(context.Background(), "west")
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "" || got.Password != "" {
		t.Fatalf("credentials = %#v", got)
	}
}
