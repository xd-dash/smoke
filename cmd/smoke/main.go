package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/xd-dash/smoke"
	"github.com/xd-dash/smoke/callback"
	redisprovider "github.com/xd-dash/smoke/provider/redis"
)

func main() {
	if len(os.Args) != 2 {
		die("usage: smoke <provider-url>")
	}

	rawURL := os.Args[1]
	u, err := url.Parse(rawURL)
	if err != nil {
		die("parse URL: %v", err)
	}

	dispatcher, err := callback.Parse(u.Query()["callback"])
	if err != nil {
		die("callbacks: %v", err)
	}
	if dispatcher.Empty() {
		die("at least one callback is required")
	}

	if !dispatcher.HasStdout() && os.Getenv("SMOKE_DETACHED") != "1" {
		pid, logPath, err := detach(rawURL)
		if err != nil {
			die("detach: %v", err)
		}
		fmt.Fprintf(os.Stderr, "smoke: started pid=%d log=%s\n", pid, logPath)
		return
	}

	registry, err := smoke.New(
		redisprovider.New(),
	)
	if err != nil {
		die("registry: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := registry.Run(ctx, rawURL, dispatcher); err != nil && ctx.Err() == nil {
		die("%v", err)
	}
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "smoke: "+format+"\n", args...)
	os.Exit(1)
}
