package dnstxt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("decode route config: %w", err)
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

	// encoding/json orders map keys, but sort here as an explicit part of the
	// projection contract and to make future alternate encoders easy to verify.
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
