package callback

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/xd-dash/smoke/xdroute"
)

// RouteConstraints describe delivery intent without naming a concrete provider
// endpoint. Empty fields do not constrain selection.
type RouteConstraints struct {
	Region string
	Edge   string
}

var callbackLookupTXT = net.DefaultResolver.LookupTXT

func resolveCallbackRoute(provider, role, zone string, constraints RouteConstraints) (xdroute.Route, error) {
	provider = strings.TrimSpace(provider)
	role = strings.TrimSpace(role)
	zone = strings.TrimSuffix(strings.TrimSpace(zone), ".")
	if provider == "" {
		return xdroute.Route{}, fmt.Errorf("callback route provider is required")
	}
	if role == "" {
		return xdroute.Route{}, fmt.Errorf("callback route role is required")
	}
	if zone == "" {
		return xdroute.Route{}, fmt.Errorf("callback route zone is required")
	}

	identity := xdroute.Identity{Service: "logmash", Role: role, Provider: provider}
	name, err := identity.DNSName(zone)
	if err != nil {
		return xdroute.Route{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	records, err := callbackLookupTXT(ctx, name)
	if err != nil {
		return xdroute.Route{}, fmt.Errorf("lookup callback route %s: %w", name, err)
	}
	routes, err := xdroute.ParseAll(name, records)
	if err != nil {
		return xdroute.Route{}, fmt.Errorf("parse callback routes %s: %w", name, err)
	}

	matches := make([]xdroute.Route, 0, len(routes))
	for _, route := range routes {
		if constraints.Region != "" && route.Region != constraints.Region {
			continue
		}
		if constraints.Edge != "" && route.Edge != constraints.Edge {
			continue
		}
		matches = append(matches, route)
	}

	switch len(matches) {
	case 0:
		return xdroute.Route{}, fmt.Errorf("no callback route for provider=%s role=%s region=%s edge=%s", provider, role, constraints.Region, constraints.Edge)
	case 1:
		return matches[0], nil
	default:
		return xdroute.Route{}, fmt.Errorf("ambiguous callback route for provider=%s role=%s region=%s edge=%s: %d matches", provider, role, constraints.Region, constraints.Edge, len(matches))
	}
}
