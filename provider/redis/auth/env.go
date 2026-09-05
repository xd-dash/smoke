package redisauth

import (
	"context"
	"fmt"
	"os"
	"strings"

	redisprovider "github.com/xd-dash/smoke/provider/redis"
)

// PasswordEnv implements password-only Redis AUTH. It first looks for the
// Logmash profile-specific password and then falls back to REDISCLI_AUTH, which
// matches the existing Huram Redis automation convention.
type PasswordEnv struct{}

func (PasswordEnv) Name() string { return "password-env" }

func (PasswordEnv) Resolve(_ context.Context, profile string) (redisprovider.Credentials, error) {
	key := profileKey(profile)
	password := os.Getenv("LOGMASH_REDIS_" + key + "_PASSWORD")
	if password == "" {
		password = os.Getenv("REDISCLI_AUTH")
	}
	if password == "" {
		return redisprovider.Credentials{}, fmt.Errorf("redis auth: password-env has no LOGMASH_REDIS_%s_PASSWORD or REDISCLI_AUTH", key)
	}
	return redisprovider.Credentials{Password: password}, nil
}

// ACLEnv implements Redis ACL username+password authentication using the
// profile-specific Logmash environment convention.
type ACLEnv struct{}

func (ACLEnv) Name() string { return "acl-env" }

func (ACLEnv) Resolve(_ context.Context, profile string) (redisprovider.Credentials, error) {
	key := profileKey(profile)
	username := os.Getenv("LOGMASH_REDIS_" + key + "_USERNAME")
	password := os.Getenv("LOGMASH_REDIS_" + key + "_PASSWORD")
	if username == "" || password == "" {
		return redisprovider.Credentials{}, fmt.Errorf("redis auth: acl-env requires LOGMASH_REDIS_%s_USERNAME and LOGMASH_REDIS_%s_PASSWORD", key, key)
	}
	return redisprovider.Credentials{Username: username, Password: password}, nil
}

func profileKey(profile string) string {
	key := strings.ToUpper(strings.TrimSpace(profile))
	return strings.NewReplacer("-", "_", ".", "_", ":", "_").Replace(key)
}
