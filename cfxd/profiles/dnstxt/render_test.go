package dnstxt

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderXDRouteConfig(t *testing.T) {
	config := `{
  "zone": "xd.run",
  "routes": [
    {
      "service": "logmash",
      "role": "callback",
      "provider": "axiom",
      "region": "us-east",
      "edge": "us-east-1.aws",
      "host": "us-east-1.aws.edge.axiom.co"
    }
  ]
}`
	var out bytes.Buffer
	if err := Render(strings.NewReader(config), &out); err != nil {
		t.Fatal(err)
	}

	var got terraformInput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	record, ok := got.Records["route-000"]
	if !ok {
		t.Fatalf("missing route-000 in %#v", got.Records)
	}
	if want := "_axiom._callback.logmash.xd.run"; record.Name != want {
		t.Fatalf("name = %q, want %q", record.Name, want)
	}
	if want := "xd-route=v1;region=us-east;edge=us-east-1.aws;host=us-east-1.aws.edge.axiom.co"; record.Content != want {
		t.Fatalf("content = %q, want %q", record.Content, want)
	}
}

func TestRenderRejectsProviderFields(t *testing.T) {
	config := `{"zone":"xd.run","zone_id":"cloudflare-specific","routes":[]}`
	if err := Render(strings.NewReader(config), &bytes.Buffer{}); err == nil {
		t.Fatal("Render accepted provider-specific zone_id")
	}
}
