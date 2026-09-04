package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/xd-dash/smoke/command"
)

func main() {
	if len(os.Args) < 2 {
		die("usage: smoke <registered-command> [args ...]")
	}

	registry, err := command.New(
		command.Command{
			Name:      "logmash",
			GoPackage: "github.com/xd-dash/smoke/cmd/logmash",
		},
	)
	if err != nil {
		die("registry: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	path, err := registry.Resolve(ctx, os.Args[1])
	if err != nil {
		die("resolve %s: %v", os.Args[1], err)
	}

	// On Unix this replaces Smoke with the resolved command. The command owns
	// its terminal/process lifecycle exactly as though it had been invoked
	// directly. Smoke has no knowledge of that command's provider grammar.
	if err := command.Exec(path, os.Args[2:]); err != nil {
		die("run %s: %v", os.Args[1], err)
	}
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "smoke: "+format+"\n", args...)
	os.Exit(1)
}
