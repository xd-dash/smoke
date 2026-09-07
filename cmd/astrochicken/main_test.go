package astrochicken

import "testing"

func TestParseArgsUsesOnlyProviderArgs(t *testing.T) {
	provider, args, err := parseArgs([]string{"--provider", "agni", "deploy", "--auto-approve"})
	if err != nil {
		t.Fatal(err)
	}
	if provider != "agni" {
		t.Fatalf("provider = %q", provider)
	}
	if len(args) != 2 || args[0] != "deploy" || args[1] != "--auto-approve" {
		t.Fatalf("args = %#v", args)
	}
}

func TestParseArgsAllowsImplicitProvider(t *testing.T) {
	provider, args, err := parseArgs([]string{"deploy"})
	if err != nil {
		t.Fatal(err)
	}
	if provider != "" || len(args) != 1 || args[0] != "deploy" {
		t.Fatalf("provider=%q args=%#v", provider, args)
	}
}

func TestParseArgsRequiresProviderArgs(t *testing.T) {
	if _, _, err := parseArgs([]string{"--provider", "agni"}); err == nil {
		t.Fatal("expected usage error")
	}
}
