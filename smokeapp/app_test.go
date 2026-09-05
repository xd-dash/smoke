package smokeapp

import "testing"

func TestParseEnvInvocation(t *testing.T) {
	name, dir, rest, err := parseEnvInvocation([]string{"infra", "--dir", "./project", "--", "logmash", "us:west:events"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "infra" || dir != "./project" {
		t.Fatalf("name=%q dir=%q", name, dir)
	}
	if len(rest) != 2 || rest[0] != "logmash" || rest[1] != "us:west:events" {
		t.Fatalf("rest=%q", rest)
	}
}

func TestParseEnvInvocationWithoutSeparator(t *testing.T) {
	name, dir, rest, err := parseEnvInvocation([]string{"dev", "go", "tool", "stringer"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "dev" || dir != "" {
		t.Fatalf("name=%q dir=%q", name, dir)
	}
	if len(rest) != 3 || rest[0] != "go" || rest[1] != "tool" || rest[2] != "stringer" {
		t.Fatalf("rest=%q", rest)
	}
}

func TestParseEnvInvocationRequiresCommand(t *testing.T) {
	if _, _, _, err := parseEnvInvocation([]string{"infra", "--"}); err == nil {
		t.Fatal("expected missing command error")
	}
}
