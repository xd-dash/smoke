package dnstxt

import (
	"encoding/json"
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
	if _, err := RenderTerraformVars([]byte(`{"zone":"xd.run","zone_id":"cloudflare-zone-id","routes":[]}`)); err == nil {
		t.Fatal("provider-specific zone_id was accepted")
	}
}

func TestRenderTerraformVarsRejectsDuplicateRoutes(t *testing.T) {
	input := []byte(`{
  "zone":"xd.run",
  "routes":[
    {"Service":"logmash","Role":"callback","Provider":"axiom","Host":"edge.example.com"},
    {"Service":"logmash","Role":"callback","Provider":"axiom","Host":"edge.example.com"}
  ]
}`)
	if _, err := RenderTerraformVars(input); err == nil {
		t.Fatal("duplicate route was accepted")
	}
}

func TestRenderTerraformVarsKeysAreStableAcrossReordering(t *testing.T) {
	first := []byte(`{
  "zone":"xd.run",
  "routes":[
    {"Service":"logmash","Role":"callback","Provider":"axiom","Region":"us-east","Host":"east.example.com"},
    {"Service":"logmash","Role":"callback","Provider":"axiom","Region":"eu-central","Host":"eu.example.com"}
  ]
}`)
	second := []byte(`{
  "zone":"xd.run",
  "routes":[
    {"Service":"logmash","Role":"callback","Provider":"axiom","Region":"eu-central","Host":"eu.example.com"},
    {"Service":"logmash","Role":"callback","Provider":"axiom","Region":"us-east","Host":"east.example.com"}
  ]
}`)

	var a, b terraformVars
	out, err := RenderTerraformVars(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &a); err != nil {
		t.Fatal(err)
	}
	out, err = RenderTerraformVars(second)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &b); err != nil {
		t.Fatal(err)
	}
	if len(a.Records) != len(b.Records) {
		t.Fatalf("record counts differ: %d vs %d", len(a.Records), len(b.Records))
	}
	for key, record := range a.Records {
		if got, ok := b.Records[key]; !ok || got != record {
			t.Fatalf("record key %s changed across reorder", key)
		}
	}
}
