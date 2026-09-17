package logmash

import "testing"

func TestParseArgsAdvertisedRedisSource(t *testing.T) {
	got, err := parseArgs([]string{"redis://127.0.0.1:16379?channel=events&pattern=stonks:*", "--attached"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 0 || len(got.Direct) != 1 {
		t.Fatalf("sources=%d direct=%d", len(got.Sources), len(got.Direct))
	}
	if got.Direct[0].Scheme != "redis" || got.Direct[0].Hostname() != "127.0.0.1" || got.Direct[0].Port() != "16379" {
		t.Fatalf("direct source = %s", got.Direct[0])
	}
}

func TestAdvertisedRedisSourceRequiresSelector(t *testing.T) {
	if _, err := parseArgs([]string{"redis://127.0.0.1:6379"}); err == nil {
		t.Fatal("expected direct Redis source without channel or pattern to fail")
	}
}

func TestLogicalAndAdvertisedSourcesCanCoexist(t *testing.T) {
	got, err := parseArgs([]string{"us:west:events", "redis+unix:///run/redis.sock?channel=local"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 1 || len(got.Direct) != 1 {
		t.Fatalf("sources=%d direct=%d", len(got.Sources), len(got.Direct))
	}
}
