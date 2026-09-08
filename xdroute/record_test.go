package xdroute

import "testing"

func TestRouteRecord(t *testing.T) {
	record, err := (Route{
		Service:  "logmash",
		Role:     "callback",
		Provider: "axiom",
		Region:   "us-east",
		Edge:     "us-east-1.aws",
		Host:     "us-east-1.aws.edge.axiom.co",
	}).Record("xd.run")
	if err != nil {
		t.Fatal(err)
	}
	if record.Name != "_axiom._callback.logmash.xd.run" {
		t.Fatalf("Name = %q", record.Name)
	}
	if record.Content != "xd-route=v1;region=us-east;edge=us-east-1.aws;host=us-east-1.aws.edge.axiom.co" {
		t.Fatalf("Content = %q", record.Content)
	}
}
