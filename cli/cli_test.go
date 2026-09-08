package cli

import (
	"reflect"
	"testing"
)

func TestParseEnvInvocation(t *testing.T) {
	name, dir, rest, err := parseEnvInvocation([]string{"infra", "--dir", "./project", "--", "logmash", "us:west:events"})
	if err != nil { t.Fatal(err) }
	if name != "infra" || dir != "./project" { t.Fatalf("name=%q dir=%q", name, dir) }
	if !reflect.DeepEqual(rest, []string{"logmash", "us:west:events"}) { t.Fatalf("rest=%q", rest) }
}

func TestParseEnvTerraform(t *testing.T) {
	name, dir, args, err := parseEnvTerraform([]string{"astrochicken", "--dir", "/tmp/root", "--", "plan", "-input=false"})
	if err != nil { t.Fatal(err) }
	if name != "astrochicken" || dir != "/tmp/root" { t.Fatalf("got name=%q dir=%q", name, dir) }
	if want := []string{"plan", "-input=false"}; !reflect.DeepEqual(args, want) { t.Fatalf("args=%#v want=%#v", args, want) }
}

func TestParseGHXDEnvironment(t *testing.T) {
	name, rest, err := parseGHXDEnvironment([]string{"--env", "operator", "refresh", "client", "token"})
	if err != nil { t.Fatal(err) }
	if name != "operator" || len(rest) != 3 || rest[0] != "refresh" { t.Fatalf("name=%q rest=%q", name, rest) }
}
