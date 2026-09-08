package logmash

import (
	"reflect"
	"testing"

	"github.com/xd-dash/smoke/callback"
)

func TestParseArgsSourceQualified(t *testing.T) {
	got, err := parseArgs([]string{"us:west:events", "us:east:events", "--pattern", "us:east:worker:*", "--into", "axiom", "east", "redis-events", "--callback-policy", "fail-fast", "--auth-provider", "acl-env", "--attached"})
	if err != nil { t.Fatal(err) }
	if got.Policy != callback.FailFast || got.AuthProvider != "acl-env" || !got.Attached || !got.Stdout { t.Fatalf("unexpected config: %#v", got) }
	if len(got.Sources) != 3 { t.Fatalf("sources=%#v", got.Sources) }
}

func TestSourceProfileHierarchy(t *testing.T) {
	if got := sourceProfile("us", "west"); got != "west.us.logma.sh" { t.Fatalf("sourceProfile=%q", got) }
	if got := logicalSource("US", "West"); got != "us:west" { t.Fatalf("logicalSource=%q", got) }
}

func TestResolveIntoAxiomAliases(t *testing.T) {
	got, err := resolveInto([]intoSpec{{Provider:"axiom", Profile:"east", Target:"one"}, {Provider:"axiom", Profile:"eu", Target:"two"}})
	if err != nil { t.Fatal(err) }
	want := []string{"axiom://one?profile=axiom-us-east-1.logma.sh", "axiom://two?profile=axiom-eu-central-1.logma.sh"}
	if !reflect.DeepEqual(got, want) { t.Fatalf("got=%#v want=%#v", got, want) }
}

func TestNoStdoutLifetime(t *testing.T) {
	got, err := parseArgs([]string{"us:west:events", "--no-stdout", "--callback", "https://example.com/hook"})
	if err != nil { t.Fatal(err) }
	if got.Stdout || got.Attached { t.Fatalf("unexpected lifetime: %#v", got) }
}
