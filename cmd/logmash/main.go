package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/xd-dash/smoke/callback"
	redisprovider "github.com/xd-dash/smoke/provider/redis"
)

type cliArgs struct {
	Profile   string
	Channels  []string
	Patterns  []string
	Callbacks []string
	NoStdout  bool
	Policy    callback.FailurePolicy
}

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		die("%v", err)
	}

	callbackValues := append([]string(nil), cfg.Callbacks...)
	if !cfg.NoStdout {
		callbackValues = append([]string{"stdout"}, callbackValues...)
	}
	dispatcher, err := callback.Parse(callbackValues)
	if err != nil {
		die("callbacks: %v", err)
	}
	if dispatcher.Empty() {
		die("at least one callback is required")
	}
	if err := dispatcher.SetFailurePolicy(cfg.Policy); err != nil {
		die("callback policy: %v", err)
	}
	dispatcher.SetErrorHandler(func(_ context.Context, message callback.Message, err error) {
		fmt.Fprintf(os.Stderr, "logmash: callback failure provider=%s channel=%s: %v\n", message.Provider, message.Channel, err)
	})

	if !dispatcher.HasStdout() && os.Getenv("LOGMASH_DETACHED") != "1" {
		pid, logPath, err := detach(os.Args[1:])
		if err != nil {
			die("detach: %v", err)
		}
		fmt.Fprintf(os.Stderr, "logmash: started pid=%d log=%s\n", pid, logPath)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	profile := normalizeProfile(cfg.Profile)
	target, err := (redisprovider.DNSResolver{}).Resolve(ctx, profile)
	if err != nil {
		die("resolve %s: %v", profile, err)
	}
	applyEnvironmentAuth(&target)

	err = redisprovider.New().RunSubscription(ctx, redisprovider.Subscription{
		Target:   target,
		Channels: cfg.Channels,
		Patterns: cfg.Patterns,
	}, dispatcher)
	if err != nil && ctx.Err() == nil {
		die("%v", err)
	}
}

func parseArgs(args []string) (cliArgs, error) {
	cfg := cliArgs{Policy: callback.Continue}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--pattern", "-p":
			i++
			if i >= len(args) { return cfg, fmt.Errorf("%s requires a value", args[i-1]) }
			cfg.Patterns = append(cfg.Patterns, args[i])
		case "--callback", "-c":
			i++
			if i >= len(args) { return cfg, fmt.Errorf("%s requires a value", args[i-1]) }
			cfg.Callbacks = append(cfg.Callbacks, args[i])
		case "--callback-policy":
			i++
			if i >= len(args) { return cfg, fmt.Errorf("--callback-policy requires a value") }
			cfg.Policy = callback.FailurePolicy(args[i])
		case "--no-stdout", "-q":
			cfg.NoStdout = true
		default:
			if strings.HasPrefix(args[i], "-") {
				return cfg, fmt.Errorf("unknown option %q", args[i])
			}
			if cfg.Profile == "" {
				cfg.Profile = args[i]
			} else {
				cfg.Channels = append(cfg.Channels, args[i])
			}
		}
	}
	if cfg.Profile == "" {
		return cfg, fmt.Errorf("usage: logmash <profile|profile.logma.sh> [channel ...] [--pattern glob] [--callback URL] [--no-stdout]")
	}
	if len(cfg.Channels) == 0 && len(cfg.Patterns) == 0 {
		return cfg, fmt.Errorf("at least one channel or --pattern is required")
	}
	if cfg.NoStdout && len(cfg.Callbacks) == 0 {
		return cfg, fmt.Errorf("--no-stdout requires at least one --callback")
	}
	return cfg, nil
}

func normalizeProfile(name string) string {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".")
	if strings.Contains(name, ".") {
		return name
	}
	return name + ".logma.sh"
}

func applyEnvironmentAuth(target *redisprovider.Target) {
	if target.AuthProfile == "" {
		return
	}
	key := strings.ToUpper(target.AuthProfile)
	key = strings.NewReplacer("-", "_", ".", "_", ":", "_").Replace(key)
	target.Username = os.Getenv("LOGMASH_REDIS_" + key + "_USERNAME")
	target.Password = os.Getenv("LOGMASH_REDIS_" + key + "_PASSWORD")
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "logmash: "+format+"\n", args...)
	os.Exit(1)
}
