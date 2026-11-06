package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/xd-dash/smoke/ghxd"
	"github.com/xd-dash/smoke/smokeapp"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "ghxd" {
		if len(os.Args) != 3 || os.Args[2] != "bootstrap" {
			fmt.Fprintln(os.Stderr, "usage: smoke ghxd bootstrap")
			os.Exit(2)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		env, err := ghxd.Bootstrap(ctx, "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "ghxd bootstrap:", err)
			os.Exit(1)
		}
		fmt.Printf("ghxd environment: %s\n", env.Name)
		return
	}
	smokeapp.Main(os.Args[1:])
}
