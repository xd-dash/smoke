package xdroute

import (
	"fmt"
	"strings"
)

const Version uint = 1

type Identity struct {
	Service  string
	Role     string
	Provider string
}

type Route struct {
	Version uint

	Service  string
	Role     string
	Provider string

	Region string
	Edge   string
	Host   string
}

func (i Identity) DNSName(zone string) (string, error) {
	if err := i.validate(); err != nil {
		return "", err
	}
	zone = normalizeName(zone)
	if err := validateZone(zone); err != nil {
		return "", err
	}
	return "_" + i.Provider + "._" + i.Role + "." + i.Service + "." + zone, nil
}

func (r Route) Identity() Identity {
	return Identity{
		Service:  r.Service,
		Role:     r.Role,
		Provider: r.Provider,
	}
}

func (r Route) TXT() (string, error) {
	if r.Version == 0 {
		r.Version = Version
	}
	if r.Version != Version {
		return "", fmt.Errorf("xdroute: unsupported version %d", r.Version)
	}
	if err := r.Identity().validate(); err != nil {
		return "", err
	}
	if err := validateValue("region", r.Region, false); err != nil {
		return "", err
	}
	if err := validateValue("edge", r.Edge, false); err != nil {
		return "", err
	}
	host := normalizeName(r.Host)
	if err := validateHost(host); err != nil {
		return "", err
	}

	fields := []string{"xd-route=v1"}
	if r.Region != "" {
		fields = append(fields, "region="+r.Region)
	}
	if r.Edge != "" {
		fields = append(fields, "edge="+r.Edge)
	}
	fields = append(fields, "host="+host)
	return strings.Join(fields, ";"), nil
}

func Parse(name, txt string) (Route, error) {
	identity, _, err := ParseIdentity(name)
	if err != nil {
		return Route{}, err
	}

	fields, err := parseTXT(txt)
	if err != nil {
		return Route{}, err
	}
	version, ok := fields["xd-route"]
	if !ok {
		return Route{}, fmt.Errorf("xdroute: TXT record is not an xd-route record")
	}
	if version != "v1" {
		return Route{}, fmt.Errorf("xdroute: unsupported TXT version %q", version)
	}

	route := Route{
		Version:  Version,
		Service:  identity.Service,
		Role:     identity.Role,
		Provider: identity.Provider,
		Region:   fields["region"],
		Edge:     fields["edge"],
		Host:     normalizeName(fields["host"]),
	}
	if _, err := route.TXT(); err != nil {
		return Route{}, err
	}
	return route, nil
}

func ParseAll(name string, records []string) ([]Route, error) {
	routes := make([]Route, 0, len(records))
	for _, record := range records {
		if !IsTXT(record) {
			continue
		}
		route, err := Parse(name, record)
		if err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return routes, nil
}

func ParseIdentity(name string) (Identity, string, error) {
	name = normalizeName(name)
	labels := strings.Split(name, ".")
	if len(labels) < 4 {
		return Identity{}, "", fmt.Errorf("xdroute: DNS name %q does not contain provider, role, service, and zone", name)
	}
	if !strings.HasPrefix(labels[0], "_") || !strings.HasPrefix(labels[1], "_") {
		return Identity{}, "", fmt.Errorf("xdroute: DNS name %q must begin with _provider._role", name)
	}

	identity := Identity{
		Provider: strings.TrimPrefix(labels[0], "_"),
		Role:     strings.TrimPrefix(labels[1], "_"),
		Service:  labels[2],
	}
	if err := identity.validate(); err != nil {
		return Identity{}, "", err
	}
	zone := strings.Join(labels[3:], ".")
	if err := validateZone(zone); err != nil {
		return Identity{}, "", err
	}
	return identity, zone, nil
}

func IsTXT(txt string) bool {
	for _, field := range strings.Split(txt, ";") {
		key, _, ok := strings.Cut(strings.TrimSpace(field), "=")
		if ok && strings.TrimSpace(key) == "xd-route" {
			return true
		}
	}
	return false
}

func (i Identity) validate() error {
	for name, value := range map[string]string{
		"service":  i.Service,
		"role":     i.Role,
		"provider": i.Provider,
	} {
		if err := validateLabel(name, value); err != nil {
			return err
		}
	}
	return nil
}

func parseTXT(txt string) (map[string]string, error) {
	fields := make(map[string]string)
	for _, field := range strings.Split(txt, ";") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			return nil, fmt.Errorf("xdroute: malformed TXT field %q", field)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("xdroute: TXT field has empty key")
		}
		if _, exists := fields[key]; exists {
			return nil, fmt.Errorf("xdroute: duplicate TXT field %q", key)
		}
		fields[key] = value
	}
	return fields, nil
}

func validateLabel(name, value string) error {
	if value == "" {
		return fmt.Errorf("xdroute: %s is required", name)
	}
	if len(value) > 63 {
		return fmt.Errorf("xdroute: %s exceeds 63 bytes", name)
	}
	if value[0] == '-' || value[len(value)-1] == '-' {
		return fmt.Errorf("xdroute: %s %q may not begin or end with '-'", name, value)
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return fmt.Errorf("xdroute: %s %q contains invalid DNS label character %q", name, value, r)
	}
	return nil
}

func validateZone(zone string) error {
	if zone == "" {
		return fmt.Errorf("xdroute: zone is required")
	}
	if len(zone) > 253 {
		return fmt.Errorf("xdroute: zone exceeds 253 bytes")
	}
	for _, label := range strings.Split(zone, ".") {
		if err := validateLabel("zone label", label); err != nil {
			return err
		}
	}
	return nil
}

func validateHost(host string) error {
	if host == "" {
		return fmt.Errorf("xdroute: host is required")
	}
	if err := validateZone(host); err != nil {
		return fmt.Errorf("xdroute: invalid host: %w", err)
	}
	return nil
}

func validateValue(name, value string, required bool) error {
	if value == "" {
		if required {
			return fmt.Errorf("xdroute: %s is required", name)
		}
		return nil
	}
	if strings.ContainsAny(value, ";\r\n") {
		return fmt.Errorf("xdroute: %s contains a reserved character", name)
	}
	return nil
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), "."))
}
