package smokeapp

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/xd-dash/smoke/command"
	"github.com/xd-dash/smoke/selfbuild"
)

// Main runs the Smoke application using commands compiled into this binary.
// Optional commands register themselves through package init functions selected
// by the composition's imports.
func Main(args []string) {
	if err := Run(args); err != nil {
		fmt.Fprintf(os.Stderr, "smoke: %v\n", err)
		os.Exit(1)
	}
}

func Run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	switch args[0] {
	case "commands":
		for _, name := range command.Names() {
			fmt.Println(name)
		}
		return nil
	case "compose":
		return runCompose(args[1:])
	default:
		return command.Run(args[0], args[1:])
	}
}

func runCompose(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: smoke compose <show|add|remove|rebuild> [import-path]")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch args[0] {
	case "show":
		if len(args) != 1 {
			return fmt.Errorf("usage: smoke compose show")
		}
		manifest, err := selfbuild.Load()
		if err != nil {
			return err
		}
		for _, component := range manifest.Components {
			fmt.Println(component)
		}
		return nil
	case "add":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: smoke compose add <go-import-path>")
		}
		manifest, err := selfbuild.Load()
		if err != nil {
			return err
		}
		manifest = selfbuild.WithAdded(manifest, args[1])
		path, err := selfbuild.Apply(ctx, manifest)
		if err != nil {
			return err
		}
		fmt.Printf("rebuilt %s\n", path)
		return nil
	case "remove":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: smoke compose remove <go-import-path>")
		}
		manifest, err := selfbuild.Load()
		if err != nil {
			return err
		}
		manifest = selfbuild.WithRemoved(manifest, args[1])
		path, err := selfbuild.Apply(ctx, manifest)
		if err != nil {
			return err
		}
		fmt.Printf("rebuilt %s\n", path)
		return nil
	case "rebuild":
		if len(args) != 1 {
			return fmt.Errorf("usage: smoke compose rebuild")
		}
		manifest, err := selfbuild.Load()
		if err != nil {
			return err
		}
		path, err := selfbuild.Apply(ctx, manifest)
		if err != nil {
			return err
		}
		fmt.Printf("rebuilt %s\n", path)
		return nil
	default:
		return fmt.Errorf("unknown compose operation %q", args[0])
	}
}

func usageError() error {
	names := command.Names()
	if len(names) == 0 {
		return fmt.Errorf("usage: smoke <command> [args ...] | smoke compose <show|add|remove|rebuild>")
	}
	return fmt.Errorf("usage: smoke <%s> [args ...] | smoke compose <show|add|remove|rebuild>", strings.Join(names, "|"))
}
