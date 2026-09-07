package astrochicken

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	probe "github.com/xd-dash/smoke/astrochicken"
	"github.com/xd-dash/smoke/command"
)

func init() { command.Register("astrochicken", Run) }

func Run(args []string) error {
	provider, rest, err := parseArgs(args)
	if err != nil {
		return err
	}

	if provider == "" {
		names := probe.Names()
		if len(names) == 0 {
			return fmt.Errorf("no astrochicken provider is compiled into this smoke")
		}
		if len(names) != 1 {
			return fmt.Errorf("multiple astrochicken providers are compiled in (%s); select one with --provider", strings.Join(names, ", "))
		}
		provider = names[0]
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return probe.Run(ctx, provider, rest)
}

func parseArgs(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("usage: smoke astrochicken [--provider <name>] <provider-args...>")
	}
	if args[0] != "--provider" {
		return "", append([]string(nil), args...), nil
	}
	if len(args) < 3 || strings.TrimSpace(args[1]) == "" {
		return "", nil, fmt.Errorf("usage: smoke astrochicken --provider <name> <provider-args...>")
	}
	return args[1], append([]string(nil), args[2:]...), nil
}
