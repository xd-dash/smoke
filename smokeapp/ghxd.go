package smokeapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/xd-dash/smoke/command"
	"github.com/xd-dash/smoke/environment"
	"github.com/xd-dash/smoke/ghxd"
)

const ghxdDeviceAuthTool = "github-device-auth"
const ghxdDefaultSafetyMargin = 5 * time.Minute

func init() {
	command.Register("ghxd", runGHXD)
}

func runGHXD(args []string) error {
	ctx := context.Background()
	if len(args) == 0 {
		return ghxdUsage()
	}
	switch args[0] {
	case "show":
		if len(args) != 1 {
			return fmt.Errorf("usage: smoke ghxd show")
		}
		fmt.Printf("environment\t%s\n", ghxd.DefaultEnvironment)
		for _, spec := range ghxd.ToolSpecs {
			fmt.Printf("tool\t%s\n", spec)
		}
		return nil
	case "bootstrap":
		if len(args) > 2 {
			return fmt.Errorf("usage: smoke ghxd bootstrap [environment]")
		}
		name := ""
		if len(args) == 2 {
			name = args[1]
		}
		env, err := ghxd.Bootstrap(ctx, name)
		if err != nil {
			return err
		}
		fmt.Printf("ghxd environment %s\n%s\n", env.Name, env.WorkFile)
		return nil
	case "apply":
		if len(args) != 2 {
			return fmt.Errorf("usage: smoke ghxd apply <environment>")
		}
		return ghxd.Apply(ctx, args[1])
	case "tool":
		name, rest, err := parseGHXDEnvironment(args[1:])
		if err != nil || len(rest) == 0 {
			return fmt.Errorf("usage: smoke ghxd tool [--env <environment>] <go-tool> [args ...]")
		}
		return runGHXDTool(ctx, name, rest...)
	case "auth":
		return runGHXDAuth(ctx, args[1:])
	case "worktree":
		return runGHXDWorktree(ctx, args[1:])
	default:
		return fmt.Errorf("unknown ghxd operation %q", args[0])
	}
}

func runGHXDAuth(ctx context.Context, args []string) error {
	name, rest, err := parseGHXDEnvironment(args)
	if err != nil || len(rest) == 0 {
		return ghxdAuthUsage()
	}

	switch rest[0] {
	case "login":
		if len(rest) != 2 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] login <client-id>")
		}
		return ghxdLogin(ctx, name, rest[1])
	case "ensure":
		bundle, margin, err := ghxdBundleInput(rest[1:])
		if err != nil {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] ensure --bundle-env <environment-variable> [safety-margin-seconds]")
		}
		return ghxdEnsure(ctx, name, bundle, margin, false)
	case "refresh":
		bundle, _, err := ghxdBundleInput(rest[1:])
		if err != nil {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] refresh --bundle-env <environment-variable>")
		}
		return ghxdEnsure(ctx, name, bundle, 0, true)
	case "token":
		bundle, _, err := ghxdBundleInput(rest[1:])
		if err != nil {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] token --bundle-env <environment-variable>")
		}
		if !bundle.AccessFresh(time.Now(), 0) {
			return fmt.Errorf("access token is expired; run auth ensure first")
		}
		fmt.Fprintln(os.Stdout, bundle.AccessToken)
		return nil
	case "device":
		if len(rest) != 2 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] device <client-id>")
		}
		return runGHXDTool(ctx, name, ghxdDeviceAuthTool, "device", rest[1])
	case "poll":
		if len(rest) != 3 {
			return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] poll <client-id> <device-code>")
		}
		return runGHXDTool(ctx, name, ghxdDeviceAuthTool, "poll", rest[1], rest[2])
	default:
		return fmt.Errorf("unknown ghxd auth operation %q", rest[0])
	}
}

func ghxdLogin(ctx context.Context, name, clientID string) error {
	deviceJSON, err := runGHXDToolOutput(ctx, name, ghxdDeviceAuthTool, "device", clientID)
	if err != nil {
		return err
	}
	var device struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
	}
	if err := json.Unmarshal(deviceJSON, &device); err != nil {
		return fmt.Errorf("decode device response: %w", err)
	}
	if device.DeviceCode == "" || device.UserCode == "" || device.VerificationURI == "" {
		return fmt.Errorf("incomplete device response")
	}
	fmt.Fprintf(os.Stderr, "Open %s and enter code %s\n", device.VerificationURI, device.UserCode)
	tokenJSON, err := runGHXDToolOutput(ctx, name, ghxdDeviceAuthTool, "poll", clientID, device.DeviceCode)
	if err != nil {
		return err
	}
	var token ghxd.DeviceTokenResponse
	if err := json.Unmarshal(tokenJSON, &token); err != nil {
		return fmt.Errorf("decode token response: %w", err)
	}
	bundle, err := ghxd.NewCredentialBundle(clientID, token, time.Now())
	if err != nil {
		return err
	}
	return writeGHXDBundle(bundle)
}

func ghxdBundleInput(args []string) (ghxd.CredentialBundle, time.Duration, error) {
	if len(args) < 2 || args[0] != "--bundle-env" || strings.TrimSpace(args[1]) == "" {
		return ghxd.CredentialBundle{}, 0, fmt.Errorf("bundle env is required")
	}
	value := os.Getenv(args[1])
	if value == "" {
		return ghxd.CredentialBundle{}, 0, fmt.Errorf("environment variable %s is empty", args[1])
	}
	bundle, err := ghxd.ParseCredentialBundle([]byte(value))
	if err != nil {
		return ghxd.CredentialBundle{}, 0, err
	}
	margin := ghxdDefaultSafetyMargin
	if len(args) == 3 {
		seconds, err := strconv.Atoi(args[2])
		if err != nil || seconds < 0 {
			return ghxd.CredentialBundle{}, 0, fmt.Errorf("invalid safety margin %q", args[2])
		}
		margin = time.Duration(seconds) * time.Second
	} else if len(args) != 2 {
		return ghxd.CredentialBundle{}, 0, fmt.Errorf("unexpected arguments")
	}
	return bundle, margin, nil
}

func ghxdEnsure(ctx context.Context, name string, bundle ghxd.CredentialBundle, margin time.Duration, force bool) error {
	if !force && bundle.AccessFresh(time.Now(), margin) {
		return writeGHXDBundle(bundle)
	}
	if !time.Now().UTC().Before(bundle.RefreshTokenExpiresAt.UTC()) {
		return fmt.Errorf("refresh token is expired")
	}
	responseJSON, err := runGHXDToolOutput(ctx, name, ghxdDeviceAuthTool, "refresh", bundle.ClientID, bundle.RefreshToken)
	if err != nil {
		return err
	}
	var response ghxd.DeviceTokenResponse
	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return fmt.Errorf("decode refresh response: %w", err)
	}
	replacement, err := ghxd.NewCredentialBundle(bundle.ClientID, response, time.Now())
	if err != nil {
		return err
	}
	return writeGHXDBundle(replacement)
}

func writeGHXDBundle(bundle ghxd.CredentialBundle) error {
	data, err := ghxd.MarshalCredentialBundle(bundle)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}

func runGHXDToolOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name == "" {
		name = ghxd.DefaultEnvironment
	}
	env, err := environment.Require(name)
	if err != nil {
		return nil, fmt.Errorf("ghxd environment %q is not bootstrapped; run `smoke ghxd bootstrap%s`: %w", name, bootstrapSuffix(name), err)
	}
	workspace, err := environment.Snapshot(ctx, env)
	if err != nil {
		return nil, err
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("ghxd requires a preinstalled Go toolchain: %w", err)
	}
	toolArgs := append([]string{"tool"}, args...)
	cmd := workspace.Command(ctx, workspace.ToolsDir, goBin, toolArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("ghxd tool failed: %s: %w", strings.TrimSpace(stderr.String()), err)
		}
		return nil, err
	}
	return output, nil
}

func runGHXDTool(ctx context.Context, name string, args ...string) error {
	if name == "" {
		name = ghxd.DefaultEnvironment
	}
	env, err := environment.Require(name)
	if err != nil {
		return fmt.Errorf("ghxd environment %q is not bootstrapped; run `smoke ghxd bootstrap%s`: %w", name, bootstrapSuffix(name), err)
	}
	workspace, err := environment.Snapshot(ctx, env)
	if err != nil {
		return err
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("ghxd requires a preinstalled Go toolchain: %w", err)
	}
	toolArgs := append([]string{"tool"}, args...)
	return runCommand(workspace.Command(ctx, workspace.ToolsDir, goBin, toolArgs...))
}

func parseGHXDEnvironment(args []string) (string, []string, error) {
	name := ghxd.DefaultEnvironment
	if len(args) >= 1 && args[0] == "--env" {
		if len(args) < 2 || args[1] == "" {
			return "", nil, fmt.Errorf("missing environment")
		}
		name = args[1]
		args = args[2:]
	}
	return name, args, nil
}

func bootstrapSuffix(name string) string {
	if name == ghxd.DefaultEnvironment {
		return ""
	}
	return " " + name
}

func ghxdAuthUsage() error {
	return fmt.Errorf("usage: smoke ghxd auth [--env <environment>] <login|ensure|refresh|token|device|poll> ...")
}

func ghxdUsage() error {
	return fmt.Errorf("usage: smoke ghxd <show|bootstrap|apply|tool|auth|worktree> ...")
}
