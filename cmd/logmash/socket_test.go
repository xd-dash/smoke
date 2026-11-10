package logmash

import (
	"context"
	"testing"
)

func TestSocketOverridePreservesIdentityAndACLAuth(t *testing.T) {
	t.Setenv("LOGMASH_REDIS_US_VM1_USERNAME", "subscriber")
	t.Setenv("LOGMASH_REDIS_US_VM1_PASSWORD", "test-password")
	cfg, err := parseArgs([]string{"us:vm1:tenant:qualification", "--redis-socket", "/private/test.sock", "--auth-provider", "acl-env"})
	if err != nil {
		t.Fatal(err)
	}
	sources, err := resolveSources(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 {
		t.Fatal(sources)
	}
	source := sources[0]
	if source.Target.Network != "unix" || source.Target.Socket != "/private/test.sock" || source.Target.Username != "subscriber" || source.Target.Source != "us:vm1" || source.Channels[0] != "tenant:qualification" {
		t.Fatal("socket override lost identity, selector or auth")
	}
}
func TestSocketOverrideRejectsAmbiguousSources(t *testing.T) {
	for _, args := range [][]string{{"us:vm1:events", "--redis-socket", "relative"}, {"us:vm1:events", "us:vm2:events", "--redis-socket", "/private/test.sock"}} {
		cfg, err := parseArgs(args)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = resolveSources(context.Background(), cfg); err == nil {
			t.Fatal("accepted invalid socket mapping")
		}
	}
}
