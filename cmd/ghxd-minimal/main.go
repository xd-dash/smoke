package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/xd-dash/smoke/smokeapp"
 "github.com/xd-dash/smoke/identity"
 _ "github.com/xd-dash/smoke/cmd/secrets"
)

func main() {
 identity.SetComponents("github.com/xd-dash/smoke/cmd/secrets")
	if len(os.Args) >= 3 && os.Args[1] == "ghxd" {
		switch os.Args[2] {
		case "auth":
			if len(os.Args) != 3 {
				fmt.Fprintln(os.Stderr, "use `smoke ghxd auth`; raw token/device-code subcommands are disabled")
				os.Exit(2)
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := bootstrapGitHubToken(ctx); err != nil {
				fmt.Fprintln(os.Stderr, "ghxd auth:", err)
				os.Exit(1)
			}
			return
		case "status":
			if len(os.Args) != 3 {
				fmt.Fprintln(os.Stderr, "usage: smoke ghxd status")
				os.Exit(2)
			}
			if err := checkEncryptedCredential(context.Background()); err != nil {
				fmt.Fprintln(os.Stderr, "ghxd status:", err)
				os.Exit(1)
			}
			return
		}
	}
	smokeapp.Main(os.Args[1:])
}
