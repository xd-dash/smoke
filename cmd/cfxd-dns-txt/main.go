package main

import (
	"fmt"
	"os"

	"github.com/xd-dash/smoke/cfxd/profiles/dnstxt"
)

func main() {
	if len(os.Args) != 3 || os.Args[1] != "seed" {
		fmt.Fprintln(os.Stderr, "usage: cfxd-dns-txt seed <destination>")
		os.Exit(2)
	}
	if err := dnstxt.Seed(os.Args[2]); err != nil {
		fmt.Fprintf(os.Stderr, "cfxd-dns-txt: %v\n", err)
		os.Exit(1)
	}
}
