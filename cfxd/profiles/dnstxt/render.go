package dnstxt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/xd-dash/smoke/xdroute"
)

// RouteConfig is deployment data consumed by the dns-txt profile. It is
// provider-neutral: Cloudflare account/zone IDs and credentials do not belong
// here.
type RouteConfig struct {
	Zone   string          `json:"zone"`
	Routes []xdroute.Route `json:"routes"`
}

type terraformRecord struct {
	Name    string   `json:"name"`
	Content string   `json:"content"`
	TTL     int      `json:"ttl,omitempty"`
	Comment string   `json:"comment,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type terraformVars struct {
	Records map[string]terraformRecord `json:"records"`
}

// RenderTerraformVars projects xdroute configuration into the generic
// Terraform variables consumed by the cfxd dns-txt root.
func RenderTerraformVars(configJSON []byte) ([]byte, error) {
	var cfg RouteConfig
	dec := json.NewDecoder(bytes.NewReader(configJSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode route config: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode route config: multiple JSON values")
		}
		return nil, fmt.Errorf("decode route config: trailing data: %w", err)
	}

	cfg.Zone = strings.TrimSpace(cfg.Zone)
	if cfg.Zone == "" {
		return nil, fmt.Errorf("route config zone is required")
	}

	records := make(map[string]terraformRecord, len(cfg.Routes))
	for i, route := range cfg.Routes {
		name, err := route.Identity().DNSName(cfg.Zone)
		if err != nil {
			return nil, fmt.Errorf("route %d dns name: %w", i, err)
		}
		content, err := route.TXT()
		if err != nil {
			return nil, fmt.Errorf("route %d txt: %w", i, err)
		}
		key := recordKey(name, content)
		if _, exists := records[key]; exists {
			return nil, fmt.Errorf("route %d duplicates an existing route", i)
		}
		records[key] = terraformRecord{
			Name:    name,
			Content: content,
		}
	}

	// Terraform resource identity is derived from route content, not list
	// position. Reordering routes therefore cannot churn resource addresses.
	// encoding/json sorts map keys, but keep the explicit ordering step so the
	// projection remains deterministic if the encoder changes later.
	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]terraformRecord, len(records))
	for _, key := range keys {
		ordered[key] = records[key]
	}

	out, err := json.MarshalIndent(terraformVars{Records: ordered}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func recordKey(name, content string) string {
	sum := sha256.Sum256([]byte(name + "\x00" + content))
	return hex.EncodeToString(sum[:])
}
