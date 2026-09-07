package main

import (
	"fmt"
	"os"

	"github.com/xd-dash/smoke/smokeapp"
)

func main() {
	args := append([]string{"ghxd"}, os.Args[1:]...)
	if err := smokeapp.Run(args); err != nil {
		fmt.Fprintln(os.Stderr, "ghxd:", err)
		os.Exit(1)
	}
}
