package redisprovider

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	redis "github.com/redis/go-redis/v9"
	"github.com/xd-dash/smoke/callback"
)

// Target is the typed Redis connection boundary used by imported callers.
type Target struct {
	Network      string
	Host         string
	Port         int
	Socket       string
	TLS          bool
	ServerName   string
	Username     string
	Password     string
	DB           int
	AuthProvider string
	AuthProfile  string
	Source       string
}

// Subscription is intentionally ephemeral. No registration is persisted.
type Subscription struct {
	Target   Target
	Channels []string
	Patterns []string
}

type subscriptionKind int

const (
	subscriptionChannels subscriptionKind = iota + 1
	subscriptionPatterns
	subscriptionMixed
)

func classifySubscription(channels, patterns []string) subscriptionKind {
	switch {
	case len(channels) > 0 && len(patterns) > 0:
		return subscriptionMixed
	case len(patterns) > 0:
		return subscriptionPatterns
	default:
		return subscriptionChannels
	}
}

func (p Provider) RunSubscription(ctx context.Context, sub Subscription, dispatcher *callback.Dispatcher) error {
	if dispatcher == nil || dispatcher.Empty() {
		return fmt.Errorf("redis provider requires at least one callback")
	}
	if len(sub.Channels) == 0 && len(sub.Patterns) == 0 {
		return fmt.Errorf("redis provider requires at least one channel or pattern")
	}

	opts, err := optionsForTarget(sub.Target)
	if err != nil {
		return err
	}

	client := redis.NewClient(opts)
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}

	var pubsub *redis.PubSub
	switch classifySubscription(sub.Channels, sub.Patterns) {
	case subscriptionChannels:
		pubsub = client.Subscribe(ctx, sub.Channels...)
	case subscriptionPatterns:
		pubsub = client.PSubscribe(ctx, sub.Patterns...)
	case subscriptionMixed:
		pubsub = client.Subscribe(ctx, sub.Channels...)
		if err := pubsub.PSubscribe(ctx, sub.Patterns...); err != nil {
			_ = pubsub.Close()
			return fmt.Errorf("redis psubscribe: %w", err)
		}
	}
	defer pubsub.Close()

	if _, err := pubsub.Receive(ctx); err != nil {
		return fmt.Errorf("redis subscribe: %w", err)
	}

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
			Source:   sub.Target.Source,
			Channel:  msg.Channel,
			Pattern:  msg.Pattern,
			Payload:  msg.Payload,
		}); err != nil {
			return err
		}
	}
}

func optionsForTarget(target Target) (*redis.Options, error) {
	if target.DB < 0 {
		return nil, fmt.Errorf("invalid redis db %d", target.DB)
	}
	if target.Network == "unix" {
		if target.Socket == "" {
			return nil, fmt.Errorf("redis unix target requires socket")
		}
		return &redis.Options{Network: "unix", Addr: target.Socket, Username: target.Username, Password: target.Password, DB: target.DB}, nil
	}
	if target.Host == "" || target.Port <= 0 || target.Port > 65535 {
		return nil, fmt.Errorf("redis tcp target requires valid host and port")
	}
	opts := &redis.Options{
		Network:  "tcp",
		Addr:     net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port)),
		Username: target.Username,
		Password: target.Password,
		DB:       target.DB,
	}
	if target.TLS {
		serverName := target.ServerName
		if serverName == "" {
			serverName = target.Host
		}
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}
	}
	return opts, nil
}
