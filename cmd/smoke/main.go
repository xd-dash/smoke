package main

import (
	"os"

	_ "github.com/xd-dash/smoke/logmash"
	"github.com/xd-dash/smoke/identity"
	"github.com/xd-dash/smoke/internal/cli"
)

func main() {
	identity.SetComponents("github.com/xd-dash/smoke/logmash")
	cli.Main(os.Args[1:])
}
