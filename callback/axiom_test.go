package callback

import (
	"context"
	"net/url"
	"reflect"
	"testing"
)

func TestParseAxiomCallbackWithDirectDomain(t *testing.T) {
	dispatcher, err := Parse([]string{
		"axiom://events?domain=us-east-1.aws.edge.axiom.co&token-env=MY_AXIOM_TOKEN",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dispatcher.Empty() {
		t.Fatal("expected Axiom callback")
	}
}

func TestParseAxiomCallbackWithDNSProfile(t *testing.T) {
	old := axiomLookupTXT
	t.Cleanup(func() { axiomLookupTXT = old })
	axiomLookupTXT = func(_ context.Context, name string) ([]string, error) {
		if name != "axiom.logma.sh" {
			t.Fatalf("lookup name = %q", name)
		}
		return []string{"smoke=v1;provider=axiom;domain=eu-central-1.aws.edge.axiom.co;auth=axiom-default"}, nil
	}

	cb, err := parseAxiomURL(mustURL(t, "axiom://redis-events?profile=axiom.logma.sh"))
	if err != nil {
		t.Fatal(err)
	}
	got := cb.(Axiom)
	want := Axiom{
		Dataset:     "redis-events",
		Domain:      "eu-central-1.aws.edge.axiom.co",
		Profile:     "axiom.logma.sh",
		AuthProfile: "axiom-default",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Axiom = %#v, want %#v", got, want)
	}
}

func TestParseAxiomRequiresDataset(t *testing.T) {
	if _, err := Parse([]string{"axiom://"}); err == nil {
		t.Fatal("expected missing dataset error")
	}
}

func TestAxiomProfileAndDomainAreExclusive(t *testing.T) {
	_, err := parseAxiomURL(mustURL(t, "axiom://events?profile=axiom.logma.sh&domain=example.com"))
	if err == nil {
		t.Fatal("expected profile/domain ambiguity error")
	}
}

func TestParseProfileFields(t *testing.T) {
	got := parseProfileFields("smoke=v1; provider=axiom; domain=example.com; auth=default")
	if got["provider"] != "axiom" || got["domain"] != "example.com" || got["auth"] != "default" {
		t.Fatalf("fields = %#v", got)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
