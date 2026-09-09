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
		data, err := os.ReadFile(os.Args[2])
		if err != nil {
			fail(err)
		}
		out, err := dnstxt.RenderTerraformVars(data)
		if err != nil {
			fail(err)
		}
		if _, err := os.Stdout.Write(out); err != nil {
			fail(err)
		}
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: cfxd-dns-txt <seed <destination>|render <xdroute-config.json>>")
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "cfxd-dns-txt: %v\n", err)
	os.Exit(1)
}
