package redisauth

import (
	"context"
	"fmt"
	"os"

	redisprovider "github.com/xd-dash/smoke/provider/redis"
)

// AutoEnv preserves the original Logmash environment behavior while making
// the selected Redis AUTH shape explicit. ACL credentials win when both a
// profile username and password exist. Otherwise a profile password or
// REDISCLI_AUTH is used as password-only/default-user AUTH.
type AutoEnv struct{}

func (AutoEnv) Name() string { return "auto-env" }

func (AutoEnv) Resolve(_ context.Context, profile string) (redisprovider.Credentials, error) {
	key := profileKey(profile)
	username := os.Getenv("LOGMASH_REDIS_" + key + "_USERNAME")
	password := os.Getenv("LOGMASH_REDIS_" + key + "_PASSWORD")
	if username != "" && password != "" {
		return redisprovider.Credentials{Username: username, Password: password}, nil
	}
	if username != "" && password == "" {
		return redisprovider.Credentials{}, fmt.Errorf("redis auth: auto-env found LOGMASH_REDIS_%s_USERNAME without password", key)
	}
	if password != "" {
		return redisprovider.Credentials{Password: password}, nil
	}
	if password = os.Getenv("REDISCLI_AUTH"); password != "" {
		return redisprovider.Credentials{Password: password}, nil
	}
	return redisprovider.Credentials{}, fmt.Errorf("redis auth: auto-env found no credentials for profile %q", profile)
}
