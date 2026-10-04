package xdroute

import (
	"reflect"
	"testing"
)

func TestIdentityDNSName(t *testing.T) {
	name, err := (Identity{
		Service:  "logmash",
		Role:     "callback",
		Provider: "axiom",
	}).DNSName("xd.run")
	if err != nil {
		t.Fatal(err)
	}
	if name != "_axiom._callback.logmash.xd.run" {
		t.Fatalf("DNSName = %q", name)
	}
}

func TestRouteTXTAndParseRoundTrip(t *testing.T) {
	want := Route{
		Version:  Version,
		Service:  "logmash",
		Role:     "callback",
		Provider: "axiom",
		Region:   "us-east",
		Edge:     "us-east-1.aws",
		Host:     "us-east-1.aws.edge.axiom.co",
	}

	txt, err := want.TXT()
	if err != nil {
		t.Fatal(err)
	}
	if txt != "xd-route=v1;region=us-east;edge=us-east-1.aws;host=us-east-1.aws.edge.axiom.co" {
		t.Fatalf("TXT = %q", txt)
	}

	got, err := Parse("_axiom._callback.logmash.xd.run.", txt)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse = %#v, want %#v", got, want)
	}
}

func TestParseAllTreatsRouteRecordsAsAlternatives(t *testing.T) {
	records := []string{
		"google-site-verification=unrelated",
		"xd-route=v1;region=us-east;edge=us-east-1.aws;host=us-east-1.aws.edge.axiom.co",
		"xd-route=v1;region=eu-central;edge=eu-central-1.aws;host=eu-central-1.aws.edge.axiom.co",
	}

	routes, err := ParseAll("_axiom._callback.logmash.xd.run", records)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 {
		t.Fatalf("len(routes) = %d, want 2", len(routes))
	}
	if routes[0].Region != "us-east" || routes[1].Region != "eu-central" {
		t.Fatalf("routes = %#v", routes)
	}
}

func TestParseIdentityReturnsZone(t *testing.T) {
	identity, zone, err := ParseIdentity("_axiom._callback.logmash.xd.run")
	if err != nil {
		t.Fatal(err)
	}
	want := Identity{Service: "logmash", Role: "callback", Provider: "axiom"}
	if !reflect.DeepEqual(identity, want) || zone != "xd.run" {
		t.Fatalf("identity = %#v, zone = %q", identity, zone)
	}
}

func TestRouteRejectsUnknownVersion(t *testing.T) {
	_, err := Parse(
		"_axiom._callback.logmash.xd.run",
		"xd-route=v2;host=us-east-1.aws.edge.axiom.co",
	)
	if err == nil {
		t.Fatal("expected unsupported version error")
	}
}

func TestRouteRequiresHost(t *testing.T) {
	_, err := Parse(
		"_axiom._callback.logmash.xd.run",
		"xd-route=v1;region=us-east",
	)
	if err == nil {
		t.Fatal("expected missing host error")
	}
}

func TestDNSNameRejectsInvalidIdentity(t *testing.T) {
	_, err := (Identity{Service: "logmash", Role: "call_back", Provider: "axiom"}).DNSName("xd.run")
	if err == nil {
		t.Fatal("expected invalid role error")
	}
}
