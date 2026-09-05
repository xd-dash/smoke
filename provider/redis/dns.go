package redisprovider

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// DNSResolver resolves provider-oriented TXT + SRV discovery records.
// Credentials are intentionally not resolved from DNS. DNS may select a
// non-secret auth provider and profile name used by the caller's secret layer.
type DNSResolver struct {
	Resolver *net.Resolver
}

func (r DNSResolver) Resolve(ctx context.Context, name string) (Target, error) {
	resolver := r.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	name = strings.TrimSuffix(strings.TrimSpace(name), ".")
	if name == "" {
		return Target{}, fmt.Errorf("redis DNS profile is empty")
	}

	txts, err := resolver.LookupTXT(ctx, name)
	if err != nil {
		return Target{}, fmt.Errorf("redis TXT resolve %q: %w", name, err)
	}

	meta, ok := redisProfileMetadata(txts)
	if !ok {
		return Target{}, fmt.Errorf("no smoke redis profile for %q", name)
	}

	tlsEnabled := strings.EqualFold(meta["tls"], "true")
	service := "redis"
	if tlsEnabled {
		service = "rediss"
	}
	_, srvs, err := resolver.LookupSRV(ctx, service, "tcp", name)
	if err != nil {
		return Target{}, fmt.Errorf("redis SRV resolve %q: %w", name, err)
	}
	if len(srvs) == 0 {
		return Target{}, fmt.Errorf("redis SRV resolve %q: no records", name)
	}

	// net.Resolver applies RFC 2782 priority/weight ordering. Use the first
	// returned target for one ephemeral client session.
	srv := srvs[0]
	host := strings.TrimSuffix(srv.Target, ".")
	return Target{
		Network:      "tcp",
		Host:         host,
		Port:         int(srv.Port),
		TLS:          tlsEnabled,
		ServerName:   host,
		AuthProvider: meta["auth-provider"],
		AuthProfile:  meta["auth"],
		Source:       name,
	}, nil
}

func redisProfileMetadata(txts []string) (map[string]string, bool) {
	for _, record := range txts {
		if !strings.HasPrefix(record, "smoke=v1;") && record != "smoke=v1" {
			continue
		}
		meta := map[string]string{}
		for _, field := range strings.Split(record, ";") {
			k, v, ok := strings.Cut(field, "=")
			if ok {
				meta[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
		if meta["smoke"] == "v1" && meta["provider"] == "redis" {
			return meta, true
		}
	}
	return nil, false
}
