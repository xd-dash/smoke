package agni

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/xd-dash/smoke/astrochicken"
	"github.com/xd-dash/smoke/command"
)

func init() { command.Register("agni", Run) }

func Run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch args[0] {
	case "astrochicken":
		return astrochicken.Run(ctx, args[1:])
	case "help", "-h", "--help":
		return usage()
	default:
		return fmt.Errorf("unknown agni command %q", args[0])
	}
}

func usage() error {
	return fmt.Errorf("usage: smoke agni astrochicken <deploy|plan|destroy|output> [terraform-args...]")
}
