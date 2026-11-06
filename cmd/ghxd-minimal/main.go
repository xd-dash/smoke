package main

import (
 "os"

 _ "github.com/xd-dash/smoke/cmd/ghxd-secrets"
 _ "github.com/xd-dash/smoke/cmd/secrets"
 "github.com/xd-dash/smoke/identity"
 "github.com/xd-dash/smoke/smokeapp"
)

func main() {
 identity.SetComponents(
  "github.com/xd-dash/smoke/cmd/ghxd-secrets",
  "github.com/xd-dash/smoke/cmd/secrets",
 )
 smokeapp.Main(os.Args[1:])
}
