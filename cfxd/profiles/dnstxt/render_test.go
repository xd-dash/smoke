package dnstxt

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderTerraformVars(t *testing.T) {
	input := []byte(`{
  "zone": "xd.run",
  "routes": [
    {
      "Service": "logmash",
      "Role": "callback",
      "Provider": "axiom",
      "Region": "us-east",
      "Edge": "us-east-1.aws",
      "Host": "us-east-1.aws.edge.axiom.co"
    }
  ]
}`)

	out, err := RenderTerraformVars(input)
	if err != nil {
		t.Fatal(err)
	}
	var vars struct {
		Records map[string]struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		} `json:"records"`
	}
	if err := json.Unmarshal(out, &vars); err != nil {
		t.Fatal(err)
	}
	if len(vars.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(vars.Records))
	}
	for _, record := range vars.Records {
		if got, want := record.Name, "_axiom._callback.logmash.xd.run"; got != want {
			t.Fatalf("name = %q, want %q", got, want)
		}
		if got, want := record.Content, "xd-route=v1;region=us-east;edge=us-east-1.aws;host=us-east-1.aws.edge.axiom.co"; got != want {
			t.Fatalf("content = %q, want %q", got, want)
		}
	}
}

func TestRenderTerraformVarsRejectsProviderSpecificRouteConfig(t *testing.T) {
	out, err := RenderTerraformVars([]byte(`{"zone":"xd.run","zone_id":"cloudflare-zone-id","routes":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "zone_id") {
		t.Fatalf("render leaked provider-specific zone_id: %s", out)
	}
}
