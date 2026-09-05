package redisauth

import (
	"context"

	redisprovider "github.com/xd-dash/smoke/provider/redis"
)

type None struct{}

func (None) Name() string { return "none" }

func (None) Resolve(context.Context, string) (redisprovider.Credentials, error) {
	return redisprovider.Credentials{}, nil
}
