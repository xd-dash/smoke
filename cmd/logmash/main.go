package main

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
	redisprovider "github.com/xd-dash/smoke/provider/redis"
	redisauth "github.com/xd-dash/smoke/provider/redis/auth"
	"github.com/xd-dash/smoke/session"
)

type sourceSelector struct {
	Country string
	Region  string
	Value   string
	Pattern bool
}

type sourceSubscription struct {
	Source       string
	Profile      string
	Channels     []string
	Patterns     []string
	Target       redisprovider.Target
	AuthProvider string
}

type cliArgs struct {
	Sources      []sourceSelector
	Into         []intoSpec
	Callbacks    []string // legacy URL form; --into is preferred
	Detached     bool
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

	intoValues, err := resolveInto(cfg.Into)
	if err != nil {
		die("into: %v", err)
	}
	callbackValues := append(intoValues, cfg.Callbacks...)
	if !cfg.Detached {
		callbackValues = append([]string{"stdout"}, callbackValues...)
	}
	if cfg.Detached && len(callbackValues) == 0 {
		die("--detached requires at least one --into destination")
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
		fmt.Fprintf(os.Stderr, "logmash: callback failure source=%s provider=%s channel=%s: %v\n", message.Source, message.Provider, message.Channel, err)
	})

	if cfg.Detached && os.Getenv("LOGMASH_DETACHED") != "1" {
		pid, logPath, err := detach(os.Args[1:])
		if err != nil {
			die("detach: %v", err)
		}
		fmt.Fprintf(os.Stderr, "logmash: started detached pid=%d log=%s; use `logmash list` for session id\n", pid, logPath)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	subscriptions, err := resolveSources(ctx, cfg)
	if err != nil {
		die("sources: %v", err)
	}

	qualifiedChannels, qualifiedPatterns, profiles := sessionSelectors(subscriptions)
	handle, record, err := session.Begin(session.Record{
		Profile:      strings.Join(profiles, ","),
		Target:       fmt.Sprintf("sources=%d", len(subscriptions)),
		Channels:     qualifiedChannels,
		Patterns:     qualifiedPatterns,
		Callbacks:    callbackValues,
		AuthProvider: sessionAuthProvider(subscriptions),
	})
	if err != nil {
		die("session registry: %v", err)
	}
	defer handle.Close()
	fmt.Fprintf(os.Stderr, "logmash: session=%s pid=%d sources=%d detached=%t\n", record.ID, record.PID, len(subscriptions), cfg.Detached)

	if err := runSources(ctx, subscriptions, dispatcher); err != nil && ctx.Err() == nil {
		die("%v", err)
	}
}

func resolveSources(ctx context.Context, cfg cliArgs) ([]sourceSubscription, error) {
	type grouped struct {
		country  string
		region   string
		channels map[string]struct{}
		patterns map[string]struct{}
	}
	groups := map[string]*grouped{}
	for _, selector := range cfg.Sources {
		source := logicalSource(selector.Country, selector.Region)
		group := groups[source]
		if group == nil {
			group = &grouped{
				country: selector.Country,
				region: selector.Region,
				channels: map[string]struct{}{},
				patterns: map[string]struct{}{},
			}
			groups[source] = group
		}
		if selector.Pattern {
			group.patterns[selector.Value] = struct{}{}
		} else {
			group.channels[selector.Value] = struct{}{}
		}
	}

	sources := make([]string, 0, len(groups))
	for source := range groups {
		sources = append(sources, source)
	}
	sort.Strings(sources)

	authRegistry, err := redisauth.New(
		redisauth.None{},
		redisauth.PasswordEnv{},
		redisauth.ACLEnv{},
		redisauth.AutoEnv{},
	)
	if err != nil {
		return nil, fmt.Errorf("auth registry: %w", err)
	}

	out := make([]sourceSubscription, 0, len(sources))
	for _, source := range sources {
		group := groups[source]
		profile := sourceProfile(group.country, group.region)
		target, err := (redisprovider.DNSResolver{}).Resolve(ctx, profile)
		if err != nil {
			return nil, fmt.Errorf("resolve %s (%s): %w", source, profile, err)
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
			authProfile = strings.ReplaceAll(source, ":", "-")
		}
		credentials, err := authRegistry.Resolve(ctx, authProvider, authProfile)
		if err != nil {
			return nil, fmt.Errorf("%s auth provider %s: %w", source, authProvider, err)
		}
		target = credentials.Apply(target)
		// Preserve the human/logical source relationship in callback envelopes.
		// DNS profile and physical Redis endpoint remain independently movable.
		target.Source = source
		out = append(out, sourceSubscription{
			Source:       source,
			Profile:      profile,
			Channels:     sortedKeys(group.channels),
			Patterns:     sortedKeys(group.patterns),
			Target:       target,
			AuthProvider: authProvider,
		})
	}
	return out, nil
}

func runSources(parent context.Context, subscriptions []sourceSubscription, dispatcher *callback.Dispatcher) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	errCh := make(chan error, len(subscriptions))
	var wg sync.WaitGroup
	for _, source := range subscriptions {
		source := source
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := redisprovider.New().RunSubscription(ctx, redisprovider.Subscription{
				Target:   source.Target,
				Channels: source.Channels,
				Patterns: source.Patterns,
			}, dispatcher)
			if err != nil && ctx.Err() == nil {
				errCh <- fmt.Errorf("source %s: %w", source.Source, err)
				cancel()
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		<-done
		return err
	case <-parent.Done():
		cancel()
		<-done
		return nil
	case <-done:
		select {
		case err := <-errCh:
			return err
		default:
			return nil
		}
	}
}

func sessionSelectors(subscriptions []sourceSubscription) (channels, patterns, profiles []string) {
	for _, source := range subscriptions {
		profiles = append(profiles, source.Profile)
		for _, channel := range source.Channels {
			channels = append(channels, source.Source+":"+channel)
		}
		for _, pattern := range source.Patterns {
			patterns = append(patterns, source.Source+":"+pattern)
		}
	}
	return channels, patterns, profiles
}

func sessionAuthProvider(subscriptions []sourceSubscription) string {
	providers := map[string]struct{}{}
	for _, source := range subscriptions {
		providers[source.AuthProvider] = struct{}{}
	}
	values := sortedKeys(providers)
	return strings.Join(values, ",")
}

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
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
		fmt.Printf("%s pid=%d profiles=%s %s\n", record.ID, record.PID, record.Profile, record.Target)
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
		return "target=" + target.Socket + " network=unix"
	}
	return fmt.Sprintf("target=%s tls=%t", net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port)), target.TLS)
}

func parseArgs(args []string) (cliArgs, error) {
	cfg := cliArgs{Policy: callback.Continue}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--pattern", "-p":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("%s requires COUNTRY:REGION:PATTERN", args[i-1])
			}
			selector, err := parseSourceSelector(args[i], true)
			if err != nil {
				return cfg, err
			}
			cfg.Sources = append(cfg.Sources, selector)
		case "--into":
			if i+3 >= len(args) {
				return cfg, fmt.Errorf("--into requires PROVIDER PROFILE TARGET")
			}
			cfg.Into = append(cfg.Into, intoSpec{Provider: args[i+1], Profile: args[i+2], Target: args[i+3]})
			i += 3
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
		case "--detached":
			cfg.Detached = true
		case "--no-stdout", "-q":
			cfg.Detached = true
		default:
			if strings.HasPrefix(args[i], "-") {
				return cfg, fmt.Errorf("unknown option %q", args[i])
			}
			selector, err := parseSourceSelector(args[i], false)
			if err != nil {
				return cfg, err
			}
			cfg.Sources = append(cfg.Sources, selector)
		}
	}
	if len(cfg.Sources) == 0 {
		return cfg, fmt.Errorf("usage: logmash COUNTRY:REGION:CHANNEL [COUNTRY:REGION:CHANNEL ...] [--pattern COUNTRY:REGION:GLOB] [--detached] [--into PROVIDER PROFILE TARGET]")
	}
	if cfg.Detached && len(cfg.Into) == 0 && len(cfg.Callbacks) == 0 {
		return cfg, fmt.Errorf("--detached requires at least one --into destination")
	}
	return cfg, nil
}

func parseSourceSelector(value string, pattern bool) (sourceSelector, error) {
	value = strings.TrimSpace(value)
	parts := strings.SplitN(value, ":", 3)
	if len(parts) != 3 {
		return invalidSourceSelector(value, pattern)
	}
	country := strings.ToLower(strings.TrimSpace(parts[0]))
	region := strings.ToLower(strings.TrimSpace(parts[1]))
	item := strings.TrimSpace(parts[2])
	if country == "" || region == "" || item == "" || len(country) != 2 {
		return invalidSourceSelector(value, pattern)
	}
	return sourceSelector{Country: country, Region: region, Value: item, Pattern: pattern}, nil
}

func invalidSourceSelector(value string, pattern bool) (sourceSelector, error) {
	kind := "CHANNEL"
	if pattern {
		kind = "PATTERN"
	}
	return sourceSelector{}, fmt.Errorf("%q must be COUNTRY:REGION:%s with a two-letter country code", value, kind)
}

func logicalSource(country, region string) string {
	return strings.ToLower(strings.TrimSpace(country)) + ":" + strings.ToLower(strings.TrimSpace(region))
}

func sourceProfile(country, region string) string {
	// DNS hierarchy is reversed relative to the human grammar: the broader
	// country scope is the parent of the region label.
	return strings.ToLower(strings.TrimSpace(region)) + "." + strings.ToLower(strings.TrimSpace(country)) + ".logma.sh"
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "logmash: "+format+"\n", args...)
	os.Exit(1)
}
