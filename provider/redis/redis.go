package redisprovider

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/xd-dash/smoke/callback"
)

// Provider implements ephemeral Redis Pub/Sub subscriptions. It stores no
// subscription state; process lifetime is subscription lifetime.
type Provider struct{}

func New() Provider { return Provider{} }

func (Provider) Schemes() []string { return []string{"redis", "rediss", "redis+unix"} }

func (p Provider) Run(ctx context.Context, u *url.URL, dispatcher *callback.Dispatcher) error {
	target, channels, patterns, err := targetFromURL(u)
	if err != nil {
		return err
	}
	return p.RunSubscription(ctx, Subscription{
		Target:   target,
		Channels: channels,
		Patterns: patterns,
	}, dispatcher)
}

func targetFromURL(u *url.URL) (Target, []string, []string, error) {
	q := u.Query()
	db := 0
	if raw := q.Get("db"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return Target{}, nil, nil, fmt.Errorf("invalid redis db %q", raw)
		}
		db = parsed
	}

	username := ""
	password := ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	target := Target{Username: username, Password: password, DB: db, Source: publicSource(u)}
	switch u.Scheme {
	case "redis", "rediss":
		host := u.Hostname()
		if host == "" {
			return Target{}, nil, nil, fmt.Errorf("redis URL requires host")
		}
		port := 6379
		if raw := u.Port(); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed <= 0 || parsed > 65535 {
				return Target{}, nil, nil, fmt.Errorf("invalid redis port %q", raw)
			}
			port = parsed
		}
		target.Network = "tcp"
		target.Host = host
		target.Port = port
		target.TLS = u.Scheme == "rediss"
		target.ServerName = host
	case "redis+unix":
		if u.Path == "" {
			return Target{}, nil, nil, fmt.Errorf("redis+unix URL requires socket path")
		}
		target.Network = "unix"
		target.Socket = u.Path
	default:
		return Target{}, nil, nil, fmt.Errorf("unsupported redis scheme %q", u.Scheme)
	}

	channels := clean(q["channel"])
	patterns := clean(q["pattern"])
	if len(channels) == 0 && len(patterns) == 0 {
		return Target{}, nil, nil, fmt.Errorf("redis provider requires channel= or pattern=")
	}
	return target, channels, patterns, nil
}

func clean(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

// publicSource keeps transport identity while stripping credentials and callback
// destinations from the event envelope.
func publicSource(u *url.URL) string {
	copy := *u
	copy.User = nil
	q := copy.Query()
	q.Del("callback")
	copy.RawQuery = q.Encode()
	return copy.String()
}
