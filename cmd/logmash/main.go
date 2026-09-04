package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/xd-dash/smoke/callback"
	redisprovider "github.com/xd-dash/smoke/provider/redis"
)

type stringsFlag []string

func (s *stringsFlag) String() string { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	var patterns stringsFlag
	var callbacks stringsFlag
	var noStdout bool
	var policy string

	flag.Var(&patterns, "pattern", "Redis PSUBSCRIBE pattern; repeatable")
	flag.Var(&callbacks, "callback", "callback URL; repeatable")
	flag.BoolVar(&noStdout, "no-stdout", false, "disable the default stdout callback")
	flag.StringVar(&policy, "callback-policy", string(callback.Continue), "continue or fail-fast")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		die("usage: logmash <profile|profile.logma.sh> [channel ...] [--pattern glob] [--callback URL] [--no-stdout]")
	}
	profile := normalizeProfile(args[0])
	channels := args[1:]
	if len(channels) == 0 && len(patterns) == 0 {
		die("at least one channel or --pattern is required")
	}

	callbackValues := append([]string(nil), callbacks...)
	if !noStdout {
		callbackValues = append([]string{"stdout"}, callbackValues...)
	}
	dispatcher, err := callback.Parse(callbackValues)
	if err != nil {
		die("callbacks: %v", err)
	}
	if dispatcher.Empty() {
		die("at least one callback is required")
	}
	if err := dispatcher.SetFailurePolicy(callback.FailurePolicy(policy)); err != nil {
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

	target, err := (redisprovider.DNSResolver{}).Resolve(ctx, profile)
	if err != nil {
		die("resolve %s: %v", profile, err)
	}
	applyEnvironmentAuth(&target)

	provider := redisprovider.New()
	err = provider.RunSubscription(ctx, redisprovider.Subscription{
		Target:   target,
		Channels: channels,
		Patterns: patterns,
	}, dispatcher)
	if err != nil && ctx.Err() == nil {
		die("%v", err)
	}
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
