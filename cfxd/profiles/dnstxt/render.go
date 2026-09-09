package dnstxt

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/xd-dash/smoke/xdroute"
)

// RouteConfig is provider-neutral deployment data consumed by dns-txt.
// Zone identifies the public DNS zone; Routes are xdroute interface values.
type RouteConfig struct {
	Zone   string          `json:"zone"`
	Routes []xdroute.Route `json:"routes"`
}

type terraformInput struct {
	Records map[string]terraformRecord `json:"records"`
}

type terraformRecord struct {
	Name    string   `json:"name"`
	Content string   `json:"content"`
	TTL     int      `json:"ttl,omitempty"`
	Comment string   `json:"comment,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

// Render converts provider-neutral route configuration into Terraform variable
// input for the cfxd dns-txt profile. Cloudflare-specific credentials and
// zone_id remain ordinary Terraform/provider inputs and are not stored here.
func Render(r io.Reader, w io.Writer) error {
	var cfg RouteConfig
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return fmt.Errorf("decode xdroute config: %w", err)
	}
	if strings.TrimSpace(cfg.Zone) == "" {
		return fmt.Errorf("xdroute config zone is required")
	}

	input := terraformInput{Records: make(map[string]terraformRecord, len(cfg.Routes))}
	keys := make([]string, 0, len(cfg.Routes))
	for i, route := range cfg.Routes {
		name, err := route.Identity().DNSName(cfg.Zone)
		if err != nil {
			return fmt.Errorf("route %d DNS name: %w", i, err)
		}
		content, err := route.TXT()
		if err != nil {
			return fmt.Errorf("route %d TXT: %w", i, err)
		}
		key := fmt.Sprintf("route-%03d", i)
		input.Records[key] = terraformRecord{Name: name, Content: content, TTL: 1}
		keys = append(keys, key)
	}

	// Keep deterministic ordering in the encoder path even though Terraform's
	// map semantics do not depend on ordering.
	sort.Strings(keys)
	ordered := terraformInput{Records: make(map[string]terraformRecord, len(keys))}
	for _, key := range keys {
		ordered.Records[key] = input.Records[key]
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ordered); err != nil {
		return fmt.Errorf("encode Terraform input: %w", err)
	}
	return nil
}
