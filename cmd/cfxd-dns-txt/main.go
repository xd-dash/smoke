package main

import (
	"fmt"
	"os"

	"github.com/xd-dash/smoke/cfxd/profiles/dnstxt"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "seed":
		if len(os.Args) != 3 {
			usage()
		}
		if err := dnstxt.Seed(os.Args[2]); err != nil {
			fail(err)
		}
	case "render":
		if len(os.Args) != 3 {
			usage()
		}
		f, err := os.Open(os.Args[2])
		if err != nil {
			fail(err)
		}
		defer f.Close()
		if err := dnstxt.Render(f, os.Stdout); err != nil {
			fail(err)
		}
	default:
		usage()
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "cfxd-dns-txt: %v\n", err)
	os.Exit(1)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: cfxd-dns-txt <seed <destination>|render <xdroute-config>>")
	os.Exit(2)
}
