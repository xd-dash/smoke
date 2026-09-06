package main

import (
	"os"

	_ "github.com/xd-dash/smoke/cmd/logmash"
	"github.com/xd-dash/smoke/identity"
	"github.com/xd-dash/smoke/smokeapp"
)

func main() {
	identity.SetComponents("github.com/xd-dash/smoke/cmd/logmash")
	smokeapp.Main(os.Args[1:])
}
