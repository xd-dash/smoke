package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/xd-dash/smoke/selfbuild"
)

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
		path, err := selfbuild.Update(ctx, func(manifest selfbuild.Manifest) selfbuild.Manifest {
			return selfbuild.WithAdded(manifest, args[1])
		})
		if err != nil {
			return err
		}
		fmt.Printf("rebuilt %s\n", path)
		return nil
	case "remove":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: smoke compose remove <go-import-path>")
		}
		path, err := selfbuild.Update(ctx, func(manifest selfbuild.Manifest) selfbuild.Manifest {
			return selfbuild.WithRemoved(manifest, args[1])
		})
		if err != nil {
			return err
		}
		fmt.Printf("rebuilt %s\n", path)
		return nil
	case "rebuild":
		if len(args) != 1 {
			return fmt.Errorf("usage: smoke compose rebuild")
		}
		path, err := selfbuild.Rebuild(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("rebuilt %s\n", path)
		return nil
	default:
		return fmt.Errorf("unknown compose operation %q", args[0])
	}
}
