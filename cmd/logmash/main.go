package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/xd-dash/smoke/callback"
	redisprovider "github.com/xd-dash/smoke/provider/redis"
	redisauth "github.com/xd-dash/smoke/provider/redis/auth"
	"github.com/xd-dash/smoke/session"
)

type cliArgs struct {
	Profile      string
	Channels     []string
	Patterns     []string
	Callbacks    []string
	NoStdout     bool
	Policy       callback.FailurePolicy
	AuthProvider string
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "list", "ls":
			listSessions()
			return
		case "stop", "end":
			if len(os.Args) != 3 {
				die("usage: logmash stop <session-id>")
			}
			record, err := session.Stop(os.Args[2])
			if err != nil {
				die("%v", err)
			}
			fmt.Printf("stopping %s pid=%d profile=%s\n", record.ID, record.PID, record.Profile)
			return
		}
	}

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
		fmt.Fprintf(os.Stderr, "logmash: started pid=%d log=%s; use `logmash list` for session id\n", pid, logPath)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	profile := normalizeProfile(cfg.Profile)
	target, err := (redisprovider.DNSResolver{}).Resolve(ctx, profile)
	if err != nil {
		die("resolve %s: %v", profile, err)
	}

	authProvider := cfg.AuthProvider
	if authProvider == "" {
		authProvider = target.AuthProvider
	}
	if authProvider == "" {
		authProvider = "auto-env"
	}
	authProfile := target.AuthProfile
	if authProfile == "" {
		authProfile = cfg.Profile
	}

	authRegistry, err := redisauth.New(
		redisauth.None{},
		redisauth.PasswordEnv{},
		redisauth.ACLEnv{},
		redisauth.AutoEnv{},
	)
	if err != nil {
		die("auth registry: %v", err)
	}
	credentials, err := authRegistry.Resolve(ctx, authProvider, authProfile)
	if err != nil {
		die("auth provider %s: %v", authProvider, err)
	}
	target = credentials.Apply(target)

	handle, record, err := session.Begin(session.Record{
		Profile:      profile,
		Target:       displayTarget(target),
		Channels:     append([]string(nil), cfg.Channels...),
		Patterns:     append([]string(nil), cfg.Patterns...),
		Callbacks:    callbackValues,
		AuthProvider: authProvider,
	})
	if err != nil {
		die("session registry: %v", err)
	}
	defer handle.Close()
	fmt.Fprintf(os.Stderr, "logmash: session=%s pid=%d\n", record.ID, record.PID)

	err = redisprovider.New().RunSubscription(ctx, redisprovider.Subscription{
		Target:   target,
		Channels: cfg.Channels,
		Patterns: cfg.Patterns,
	}, dispatcher)
	if err != nil && ctx.Err() == nil {
		die("%v", err)
	}
}

func listSessions() {
	records, err := session.List()
	if err != nil {
		die("list sessions: %v", err)
	}
	if len(records) == 0 {
		fmt.Println("no active logmash sessions")
		return
	}
	for _, record := range records {
		fmt.Printf("%s pid=%d profile=%s target=%s\n", record.ID, record.PID, record.Profile, record.Target)
		if len(record.Channels) > 0 {
			fmt.Printf("  channels: %s\n", strings.Join(record.Channels, ", "))
		}
		if len(record.Patterns) > 0 {
			fmt.Printf("  patterns: %s\n", strings.Join(record.Patterns, ", "))
		}
		fmt.Printf("  callbacks: %s\n", strings.Join(record.Callbacks, ", "))
		if record.AuthProvider != "" {
			fmt.Printf("  auth: %s\n", record.AuthProvider)
		}
	}
}

func displayTarget(target redisprovider.Target) string {
	if target.Network == "unix" {
		return "unix:" + target.Socket
	}
	scheme := "redis"
	if target.TLS {
		scheme = "rediss"
	}
	return scheme + "://" + net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port))
}

func parseArgs(args []string) (cliArgs, error) {
	cfg := cliArgs{Policy: callback.Continue}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--pattern", "-p":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("%s requires a value", args[i-1])
			}
			cfg.Patterns = append(cfg.Patterns, args[i])
		case "--callback", "-c":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("%s requires a value", args[i-1])
			}
			cfg.Callbacks = append(cfg.Callbacks, args[i])
		case "--callback-policy":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("--callback-policy requires a value")
			}
			cfg.Policy = callback.FailurePolicy(args[i])
		case "--auth-provider":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("--auth-provider requires a value")
			}
			cfg.AuthProvider = args[i]
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
		return cfg, fmt.Errorf("usage: logmash <profile|profile.logma.sh> [channel ...] [--pattern glob] [--callback URL] [--auth-provider NAME] [--no-stdout]")
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

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "logmash: "+format+"\n", args...)
	os.Exit(1)
}
