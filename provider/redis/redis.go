package redisprovider

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	redis "github.com/redis/go-redis/v9"
	"github.com/xd-dash/smoke/callback"
)

// Provider implements ephemeral Redis Pub/Sub subscriptions. It stores no
// subscription state; process lifetime is subscription lifetime.
type Provider struct{}

func New() Provider { return Provider{} }

func (Provider) Schemes() []string { return []string{"redis", "rediss", "redis+unix"} }

func (Provider) Run(ctx context.Context, u *url.URL, dispatcher *callback.Dispatcher) error {
	if dispatcher == nil || dispatcher.Empty() {
		return fmt.Errorf("redis provider requires at least one callback")
	}

	opts, err := options(u)
	if err != nil {
		return err
	}

	channels := clean(u.Query()["channel"])
	patterns := clean(u.Query()["pattern"])
	if len(channels) == 0 && len(patterns) == 0 {
		return fmt.Errorf("redis provider requires channel= or pattern=")
	}

	client := redis.NewClient(opts)
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}

	pubsub := client.Subscribe(ctx, channels...)
	defer pubsub.Close()

	if len(patterns) > 0 {
		if err := pubsub.PSubscribe(ctx, patterns...); err != nil {
			return fmt.Errorf("redis psubscribe: %w", err)
		}
	}

	// Receive forces Redis to acknowledge the subscription before Run enters
	// the message loop. This is useful to callers supervising an ephemeral
	// subscription process.
	if _, err := pubsub.Receive(ctx); err != nil {
		return fmt.Errorf("redis subscribe: %w", err)
	}

	source := publicSource(u)
	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("redis receive: %w", err)
		}

		if err := dispatcher.Dispatch(ctx, callback.Message{
			Provider: "redis",
			Source:   source,
			Channel:  msg.Channel,
			Pattern:  msg.Pattern,
			Payload:  msg.Payload,
		}); err != nil {
			return err
		}
	}
}

func options(u *url.URL) (*redis.Options, error) {
	q := u.Query()
	db := 0
	if raw := q.Get("db"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return nil, fmt.Errorf("invalid redis db %q", raw)
		}
		db = parsed
	}

	username := ""
	password := ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	switch u.Scheme {
	case "redis", "rediss":
		host := u.Hostname()
		if host == "" {
			return nil, fmt.Errorf("redis URL requires host")
		}
		port := u.Port()
		if port == "" {
			port = "6379"
		}

		opts := &redis.Options{
			Network:  "tcp",
			Addr:     net.JoinHostPort(host, port),
			Username: username,
			Password: password,
			DB:       db,
		}
		if u.Scheme == "rediss" {
			opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}
		}
		return opts, nil

	case "redis+unix":
		if u.Path == "" {
			return nil, fmt.Errorf("redis+unix URL requires socket path")
		}
		return &redis.Options{
			Network:  "unix",
			Addr:     u.Path,
			Username: username,
			Password: password,
			DB:       db,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported redis scheme %q", u.Scheme)
	}
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
