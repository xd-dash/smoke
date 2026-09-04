package callback

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

type axiomDNSProfile struct {
	Domain      string
	AuthProfile string
}

var axiomLookupTXT = net.DefaultResolver.LookupTXT

func resolveAxiomDNSProfile(name string) (axiomDNSProfile, error) {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".")
	if name == "" {
		return axiomDNSProfile{}, fmt.Errorf("Axiom DNS profile is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	records, err := axiomLookupTXT(ctx, name)
	if err != nil {
		return axiomDNSProfile{}, fmt.Errorf("lookup Axiom profile %s: %w", name, err)
	}

	for _, record := range records {
		fields := parseProfileFields(record)
		if fields["smoke"] != "v1" || fields["provider"] != "axiom" {
			continue
		}
		domain := strings.TrimSpace(fields["domain"])
		if domain == "" {
			return axiomDNSProfile{}, fmt.Errorf("Axiom profile %s is missing domain", name)
		}
		return axiomDNSProfile{
			Domain:      strings.TrimSuffix(domain, "."),
			AuthProfile: strings.TrimSpace(fields["auth"]),
		}, nil
	}
	return axiomDNSProfile{}, fmt.Errorf("Axiom profile %s has no smoke=v1 provider=axiom TXT record", name)
}

func parseProfileFields(record string) map[string]string {
	out := make(map[string]string)
	for _, field := range strings.Split(record, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(field), "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(value)
	}
	return out
}
