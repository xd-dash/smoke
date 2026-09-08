package main

import (
	"os"

	"github.com/xd-dash/smoke/cli"
	"github.com/xd-dash/smoke/identity"
	_ "github.com/xd-dash/smoke/logmash"
)

func main() {
	identity.SetComponents("github.com/xd-dash/smoke/logmash")
	cli.Main(os.Args[1:])
}
