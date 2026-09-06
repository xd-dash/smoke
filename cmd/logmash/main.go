package logmash

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/xd-dash/smoke/callback"
	"github.com/xd-dash/smoke/command"
	redisprovider "github.com/xd-dash/smoke/provider/redis"
	redisauth "github.com/xd-dash/smoke/provider/redis/auth"
	"github.com/xd-dash/smoke/session"
)

type sourceSelector struct { Country, Region, Value string; Pattern bool }
type sourceSubscription struct { Source, Profile string; Channels, Patterns []string; Target redisprovider.Target; AuthProvider string }
type cliArgs struct { Sources []sourceSelector; Into []intoSpec; Callbacks []string; Attached, Stdout bool; Policy callback.FailurePolicy; AuthProvider string }

func init() { command.Register("logmash", Run) }

func Run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "list", "ls":
			verbose := false
			if len(args) == 2 && args[1] == "--verbose" { verbose = true } else if len(args) != 1 { return fmt.Errorf("usage: logmash list [--verbose]") }
			return listSessions(verbose)
		case "stop", "end":
			if len(args) != 2 { return fmt.Errorf("usage: logmash stop <session-id>") }
			record, err := session.Stop(args[1]); if err != nil { return err }
			fmt.Printf("stopping %s pid=%d profile=%s\n", record.ID, record.PID, record.Profile)
			return nil
		}
	}
	cfg, err := parseArgs(args); if err != nil { return err }
	intoValues, err := resolveInto(cfg.Into); if err != nil { return fmt.Errorf("into: %w", err) }
	callbackValues := append(intoValues, cfg.Callbacks...)
	if cfg.Stdout { callbackValues = append([]string{"stdout"}, callbackValues...) }
	if len(callbackValues) == 0 { return fmt.Errorf("at least one callback is required") }
	unattended := !cfg.Stdout && !cfg.Attached
	if unattended && os.Getenv("LOGMASH_BACKGROUND") != "1" {
		pid, err := startBackground(args); if err != nil { return fmt.Errorf("start unattended runtime: %w", err) }
		fmt.Fprintf(os.Stderr, "logmash: started unattended pid=%d; use `smoke logmash list` or `smoke logmash stop <session-id>`\n", pid)
		return nil
	}
	dispatcher, err := callback.Parse(callbackValues); if err != nil { return fmt.Errorf("callbacks: %w", err) }
	if dispatcher.Empty() { return fmt.Errorf("at least one callback is required") }
	if err := dispatcher.SetFailurePolicy(cfg.Policy); err != nil { return fmt.Errorf("callback policy: %w", err) }
	dispatcher.SetErrorHandler(func(_ context.Context, message callback.Message, err error) { fmt.Fprintf(os.Stderr, "logmash: callback failure source=%s provider=%s channel=%s: %v\n", message.Source, message.Provider, message.Channel, err) })
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM); defer stop()
	subscriptions, err := resolveSources(ctx, cfg); if err != nil { return fmt.Errorf("sources: %w", err) }
	if os.Getenv("LOGMASH_BACKGROUND") == "1" {
		qualifiedChannels, qualifiedPatterns, profiles := sessionSelectors(subscriptions)
		handle, record, err := session.Begin(session.Record{Profile: strings.Join(profiles, ","), Target: fmt.Sprintf("sources=%d", len(subscriptions)), Channels: qualifiedChannels, Patterns: qualifiedPatterns, Callbacks: callbackValues, AuthProvider: sessionAuthProvider(subscriptions)})
		if err != nil { return fmt.Errorf("session registry: %w", err) }
		defer handle.Close()
		fmt.Fprintf(os.Stderr, "logmash: session=%s pid=%d sources=%d\n", record.ID, record.PID, len(subscriptions))
	}
	if err := runSources(ctx, subscriptions, dispatcher); err != nil && ctx.Err() == nil { return err }
	return nil
}

func resolveSources(ctx context.Context, cfg cliArgs) ([]sourceSubscription, error) {
	type grouped struct { country, region string; channels, patterns map[string]struct{} }
	groups := map[string]*grouped{}
	for _, selector := range cfg.Sources {
		source := logicalSource(selector.Country, selector.Region); group := groups[source]
		if group == nil { group = &grouped{country: selector.Country, region: selector.Region, channels: map[string]struct{}{}, patterns: map[string]struct{}{}}; groups[source] = group }
		if selector.Pattern { group.patterns[selector.Value] = struct{}{} } else { group.channels[selector.Value] = struct{}{} }
	}
	sources := make([]string, 0, len(groups)); for source := range groups { sources = append(sources, source) }; sort.Strings(sources)
	authRegistry, err := redisauth.New(redisauth.None{}, redisauth.PasswordEnv{}, redisauth.ACLEnv{}, redisauth.AutoEnv{}); if err != nil { return nil, fmt.Errorf("auth registry: %w", err) }
	out := make([]sourceSubscription, 0, len(sources))
	for _, source := range sources {
		group := groups[source]; profile := sourceProfile(group.country, group.region)
		target, err := (redisprovider.DNSResolver{}).Resolve(ctx, profile); if err != nil { return nil, fmt.Errorf("resolve %s (%s): %w", source, profile, err) }
		authProvider := cfg.AuthProvider; if authProvider == "" { authProvider = target.AuthProvider }; if authProvider == "" { authProvider = "auto-env" }
		authProfile := target.AuthProfile; if authProfile == "" { authProfile = strings.ReplaceAll(source, ":", "-") }
		credentials, err := authRegistry.Resolve(ctx, authProvider, authProfile); if err != nil { return nil, fmt.Errorf("%s auth provider %s: %w", source, authProvider, err) }
		target = credentials.Apply(target); target.Source = source
		out = append(out, sourceSubscription{Source: source, Profile: profile, Channels: sortedKeys(group.channels), Patterns: sortedKeys(group.patterns), Target: target, AuthProvider: authProvider})
	}
	return out, nil
}

func runSources(parent context.Context, subscriptions []sourceSubscription, dispatcher *callback.Dispatcher) error {
	ctx, cancel := context.WithCancel(parent); defer cancel(); errCh := make(chan error, len(subscriptions)); var wg sync.WaitGroup
	for _, source := range subscriptions { source := source; wg.Add(1); go func() { defer wg.Done(); err := redisprovider.New().RunSubscription(ctx, redisprovider.Subscription{Target: source.Target, Channels: source.Channels, Patterns: source.Patterns}, dispatcher); if err != nil && ctx.Err() == nil { errCh <- fmt.Errorf("source %s: %w", source.Source, err); cancel() } }() }
	done := make(chan struct{}); go func() { wg.Wait(); close(done) }()
	select {
	case err := <-errCh: <-done; return err
	case <-parent.Done(): cancel(); <-done; return nil
	case <-done: select { case err := <-errCh: return err; default: return nil }
	}
}

func sessionSelectors(subscriptions []sourceSubscription) (channels, patterns, profiles []string) {
	for _, source := range subscriptions { profiles = append(profiles, source.Profile); for _, channel := range source.Channels { channels = append(channels, source.Source+":"+channel) }; for _, pattern := range source.Patterns { patterns = append(patterns, source.Source+":"+pattern) } }
	return channels, patterns, profiles
}
func sessionAuthProvider(subscriptions []sourceSubscription) string { providers := map[string]struct{}{}; for _, source := range subscriptions { providers[source.AuthProvider] = struct{}{} }; return strings.Join(sortedKeys(providers), ",") }
func sortedKeys(values map[string]struct{}) []string { out := make([]string, 0, len(values)); for value := range values { out = append(out, value) }; sort.Strings(out); return out }

func listSessions(verbose bool) error {
	records, err := session.List(); if err != nil { return fmt.Errorf("list sessions: %w", err) }
	if len(records) == 0 { fmt.Println("no active logmash sessions"); return nil }
	for _, record := range records {
		fmt.Printf("%s pid=%d profiles=%s %s\n", record.ID, record.PID, record.Profile, record.Target)
		if len(record.Channels) > 0 { fmt.Printf("  channels: %s\n", strings.Join(record.Channels, ", ")) }
		if len(record.Patterns) > 0 { fmt.Printf("  patterns: %s\n", strings.Join(record.Patterns, ", ")) }
		fmt.Printf("  callbacks: %s\n", strings.Join(record.Callbacks, ", "))
		if record.AuthProvider != "" { fmt.Printf("  auth: %s\n", record.AuthProvider) }
		if verbose {
			fmt.Printf("  composition: %s\n", emptyLabel(record.CompositionDigest))
			fmt.Printf("  environment: %s\n", emptyLabel(record.Environment))
			fmt.Printf("  workspace digest: %s\n", emptyLabel(record.WorkspaceDigest))
			fmt.Printf("  workspace: %s\n", emptyLabel(record.Workspace))
			fmt.Printf("  lease: %s\n", emptyLabel(record.Lease))
			fmt.Printf("  started: %s\n", record.StartedAt.Format("2006-01-02T15:04:05Z07:00"))
		}
	}
	return nil
}
func emptyLabel(value string) string { if strings.TrimSpace(value) == "" { return "(none)" }; return value }

func displayTarget(target redisprovider.Target) string { if target.Network == "unix" { return "target=" + target.Socket + " network=unix" }; return fmt.Sprintf("target=%s tls=%t", net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port)), target.TLS) }

func parseArgs(args []string) (cliArgs, error) {
	cfg := cliArgs{Policy: callback.Continue, Stdout: true}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--pattern", "-p": i++; if i >= len(args) { return cfg, fmt.Errorf("%s requires COUNTRY:REGION:PATTERN", args[i-1]) }; selector, err := parseSourceSelector(args[i], true); if err != nil { return cfg, err }; cfg.Sources = append(cfg.Sources, selector)
		case "--into": if i+3 >= len(args) { return cfg, fmt.Errorf("--into requires PROVIDER PROFILE TARGET") }; cfg.Into = append(cfg.Into, intoSpec{Provider: args[i+1], Profile: args[i+2], Target: args[i+3]}); i += 3
		case "--callback", "-c": i++; if i >= len(args) { return cfg, fmt.Errorf("%s requires a value", args[i-1]) }; cfg.Callbacks = append(cfg.Callbacks, args[i])
		case "--callback-policy": i++; if i >= len(args) { return cfg, fmt.Errorf("--callback-policy requires a value") }; cfg.Policy = callback.FailurePolicy(args[i])
		case "--auth-provider": i++; if i >= len(args) { return cfg, fmt.Errorf("--auth-provider requires a value") }; cfg.AuthProvider = args[i]
		case "--attached": cfg.Attached = true
		case "--no-stdout", "-q": cfg.Stdout = false
		default: if strings.HasPrefix(args[i], "-") { return cfg, fmt.Errorf("unknown option %q", args[i]) }; selector, err := parseSourceSelector(args[i], false); if err != nil { return cfg, err }; cfg.Sources = append(cfg.Sources, selector)
		}
	}
	if len(cfg.Sources) == 0 { return cfg, fmt.Errorf("usage: logmash COUNTRY:REGION:CHANNEL [COUNTRY:REGION:CHANNEL ...] [--pattern COUNTRY:REGION:GLOB] [--no-stdout] [--attached] [--into PROVIDER PROFILE TARGET]") }
	return cfg, nil
}

func parseSourceSelector(value string, pattern bool) (sourceSelector, error) { value = strings.TrimSpace(value); parts := strings.SplitN(value, ":", 3); if len(parts) != 3 { return invalidSourceSelector(value, pattern) }; country, region, item := strings.ToLower(strings.TrimSpace(parts[0])), strings.ToLower(strings.TrimSpace(parts[1])), strings.TrimSpace(parts[2]); if country == "" || region == "" || item == "" || len(country) != 2 { return invalidSourceSelector(value, pattern) }; return sourceSelector{Country: country, Region: region, Value: item, Pattern: pattern}, nil }
func invalidSourceSelector(value string, pattern bool) (sourceSelector, error) { kind := "CHANNEL"; if pattern { kind = "PATTERN" }; return sourceSelector{}, fmt.Errorf("%q must be COUNTRY:REGION:%s with a two-letter country code", value, kind) }
func logicalSource(country, region string) string { return strings.ToLower(strings.TrimSpace(country)) + ":" + strings.ToLower(strings.TrimSpace(region)) }
func sourceProfile(country, region string) string { return strings.ToLower(strings.TrimSpace(region)) + "." + strings.ToLower(strings.TrimSpace(country)) + ".logma.sh" }
