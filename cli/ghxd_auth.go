package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dash-xd/github-device-auth/deviceauth"
)

func runGHXDAuth(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return ghxdAuthUsage()
	}

	var value any
	var err error
	switch args[0] {
	case "device":
		if len(args) != 2 {
			return fmt.Errorf("usage: smoke ghxd auth device <client-id>")
		}
		value, err = deviceauth.RequestDeviceCode(ctx, args[1])
	case "poll":
		if len(args) != 3 {
			return fmt.Errorf("usage: smoke ghxd auth poll <client-id> <device-code>")
		}
		value, err = deviceauth.PollForToken(ctx, args[1], args[2], 5*time.Second)
	case "refresh":
		if len(args) != 3 && len(args) != 4 {
			return fmt.Errorf("usage: smoke ghxd auth refresh <client-id> <refresh-token> [client-secret]")
		}
		secret := ""
		if len(args) == 4 {
			secret = args[3]
		}
		value, err = deviceauth.RefreshAccessToken(ctx, args[1], secret, args[2])
	default:
		return fmt.Errorf("unknown ghxd auth operation %q", args[0])
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(value)
}

func ghxdAuthUsage() error {
	return fmt.Errorf("usage: smoke ghxd auth <device|poll|refresh> ...")
}
