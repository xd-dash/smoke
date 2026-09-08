package main

import (
	"fmt"
	"os"

	"github.com/xd-dash/smoke/cli"
)

func main() {
	args := append([]string{"ghxd"}, os.Args[1:]...)
	if err := cli.Run(args); err != nil {
		fmt.Fprintln(os.Stderr, "ghxd:", err)
		os.Exit(1)
	}
}
