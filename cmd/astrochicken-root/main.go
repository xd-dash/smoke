package main

import (
	"fmt"
	"os"

	"github.com/xd-dash/smoke/astrochicken"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: astrochicken-root <destination>")
		os.Exit(2)
	}
	if err := astrochicken.Materialize(os.Args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "astrochicken-root: %v\n", err)
		os.Exit(1)
	}
}
