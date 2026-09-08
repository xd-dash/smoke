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
		Provider:    "axiom",
		Role:        "callback",
		Domain:      "eu-central-1.aws.edge.axiom.co",
		Profile:     "axiom.logma.sh",
		AuthProfile: "axiom-default",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Axiom = %#v, want %#v", got, want)
	}
}

func TestParseAxiomCallbackWithRouteConstraints(t *testing.T) {
	cb, err := parseAxiomURL(mustURL(t, "axiom://redis-events?route-zone=xd.run&region=us-east"))
	if err != nil {
		t.Fatal(err)
	}
	got := cb.(Axiom)
	if got.Provider != "axiom" {
		t.Fatalf("provider = %q", got.Provider)
	}
	if got.Role != "callback" {
		t.Fatalf("role = %q", got.Role)
	}
	if got.RouteZone != "xd.run" {
		t.Fatalf("route zone = %q", got.RouteZone)
	}
	if got.Constraints.Region != "us-east" {
		t.Fatalf("region = %q", got.Constraints.Region)
	}
}

func TestResolveCallbackRouteByRegion(t *testing.T) {
	old := callbackLookupTXT
	t.Cleanup(func() { callbackLookupTXT = old })
	callbackLookupTXT = func(_ context.Context, name string) ([]string, error) {
		if name != "_axiom._callback.logmash.xd.run" {
			t.Fatalf("lookup name = %q", name)
		}
		return []string{
			"xd-route=v1;region=eu-central;edge=eu-central-1.aws;host=eu-central-1.aws.edge.axiom.co",
			"xd-route=v1;region=us-east;edge=us-east-1.aws;host=us-east-1.aws.edge.axiom.co",
		}, nil
	}

	route, err := resolveCallbackRoute("axiom", "callback", "xd.run", RouteConstraints{Region: "us-east"})
	if err != nil {
		t.Fatal(err)
	}
	if route.Host != "us-east-1.aws.edge.axiom.co" {
		t.Fatalf("host = %q", route.Host)
	}
}

func TestResolveCallbackRouteRejectsAmbiguousMatches(t *testing.T) {
	old := callbackLookupTXT
	t.Cleanup(func() { callbackLookupTXT = old })
	callbackLookupTXT = func(_ context.Context, _ string) ([]string, error) {
		return []string{
			"xd-route=v1;region=us-east;edge=us-east-1.aws;host=a.example.com",
			"xd-route=v1;region=us-east;edge=us-east-2.aws;host=b.example.com",
		}, nil
	}

	if _, err := resolveCallbackRoute("axiom", "callback", "xd.run", RouteConstraints{Region: "us-east"}); err == nil {
		t.Fatal("expected ambiguous route error")
	}
}

func TestParseAxiomRequiresDataset(t *testing.T) {
	if _, err := Parse([]string{"axiom://"}); err == nil {
		t.Fatal("expected missing dataset error")
	}
}

func TestAxiomRouteSourcesAreExclusive(t *testing.T) {
	_, err := parseAxiomURL(mustURL(t, "axiom://events?route-zone=xd.run&domain=example.com"))
	if err == nil {
		t.Fatal("expected route source ambiguity error")
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
