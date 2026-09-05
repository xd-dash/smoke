package main

import (
	"os"

	_ "github.com/xd-dash/smoke/cmd/logmash"
	"github.com/xd-dash/smoke/smokeapp"
)

func main() {
	smokeapp.Main(os.Args[1:])
}
