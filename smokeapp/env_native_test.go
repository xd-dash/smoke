package smokeapp

import (
	"reflect"
	"testing"
)

func TestParseEnvTerraform(t *testing.T) {
	name, dir, args, err := parseEnvTerraform([]string{"astrochicken", "--dir", "/tmp/root", "--", "plan", "-input=false"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "astrochicken" || dir != "/tmp/root" {
		t.Fatalf("got name=%q dir=%q", name, dir)
	}
	if want := []string{"plan", "-input=false"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestParseEnvTerraformRequiresArguments(t *testing.T) {
	if _, _, _, err := parseEnvTerraform([]string{"astrochicken"}); err == nil {
		t.Fatal("expected Terraform argument error")
	}
}
