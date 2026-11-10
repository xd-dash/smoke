package main

import (
	_ "github.com/xd-dash/smoke/cmd/logmash"
	_ "github.com/xd-dash/smoke/cmd/wisp"
	"github.com/xd-dash/smoke/identity"
	"github.com/xd-dash/smoke/smokeapp"
	"os"
)

func main() {
	identity.SetComponents("github.com/xd-dash/smoke/cmd/logmash", "github.com/xd-dash/smoke/cmd/wisp")
	smokeapp.Main(os.Args[1:])
}
