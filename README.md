# smoke

`smoke` is a self-composed Go executable. Optional commands and providers are linked into the `smoke` binary by importing their Go packages into a tiny local composition program and rebuilding that program with the system Go toolchain.

The default composition includes Logmash directly. There is no separate `logmash` executable to resolve or install.

```sh
go install github.com/xd-dash/smoke/cmd/smoke@latest

smoke logmash us:west:events us:east:events
```

Conceptually the installed binary is built from a composition like:

```go
package main

import (
    "os"

    _ "github.com/xd-dash/smoke/cmd/logmash"
    "github.com/xd-dash/smoke/smokeapp"
)

func main() {
    smokeapp.Main(os.Args[1:])
}
```

The blank import is the composition boundary. Importing the Logmash package causes its `init` function to register the in-process `logmash` command. Packages that are not imported are not linked into that Smoke executable.

## Self composition

Smoke requires a preinstalled Go toolchain for recomposition.

The local desired composition is stored as a list of Go import paths. By default it contains:

```text
github.com/xd-dash/smoke/cmd/logmash
```

Inspect it with:

```sh
smoke compose show
```

Add another optional command/provider package and rebuild Smoke:

```sh
smoke compose add example.com/acme/smoke-provider
```

Remove one:

```sh
smoke compose remove example.com/acme/smoke-provider
```

Or rebuild the current composition without changing it:

```sh
smoke compose rebuild
```

Recomposition performs:

```text
composition manifest
        ↓
generate local main.go
        ↓
generate local go.mod
        ↓
go mod tidy
        ↓
go build candidate Smoke binary
        ↓
build succeeded?
   no ──> keep current manifest + current binary
   yes
        ↓
persist composition manifest
        ↓
atomically rename candidate over current smoke executable
```

The generated composition lives under the user's local data directory by default and has its own `go.mod` (`module smoke.local/composition`). When the running Smoke binary has module version information, that version of `github.com/xd-dash/smoke` is pinned in the generated `go.mod`; `SMOKE_MODULE_VERSION` can explicitly select the version for development or controlled recomposition.

This is intentionally normal Go dependency composition rather than a runtime plugin system. Optional packages depend on Smoke's core interfaces; Smoke's local composition chooses which optional packages to import.

## Compiled-in command registry

Optional command packages register an in-process handler:

```go
func init() {
    command.Register("logmash", Run)
}
```

Smoke dispatches directly into that handler. For a Logmash launch, the handler performs the one OS-level lifetime transition that cannot be represented by a goroutine alone: it starts the same Smoke executable as the background runtime, then the launcher exits.

There is no PATH lookup and no separately installed Logmash executable.

List the commands present in the current binary with:

```sh
smoke commands
```

## Logmash source grammar

A Redis subscription is one atomic geographic source relationship:

```text
COUNTRY:REGION:CHANNEL
```

Examples:

```sh
smoke logmash us:west:events
smoke logmash us:west:events us:west:ratelimiters
smoke logmash us:west:events us:west:ratelimiters us:east:events
```

The country is a two-letter country code. Country and region are normalized to lowercase.

The human source maps to managed DNS as:

```text
us:west
    ↓
west.us.logma.sh

us:east
    ↓
east.us.logma.sh
```

The CLI remains broad-to-narrow (`country:region:channel`) while DNS remains hierarchical (`region.country.logma.sh`). Callback provenance keeps the logical source (`us:west`), not the DNS spelling or physical Redis host.

Selectors are grouped by `country:region` before connecting:

```text
us:west:events
us:west:ratelimiters
us:west:ratelimiters
        ↓
one west.us.logma.sh Redis Pub/Sub connection
  SUBSCRIBE events ratelimiters

us:east:events
        ↓
one east.us.logma.sh Redis Pub/Sub connection
  SUBSCRIBE events
```

Exact duplicate selectors are deduplicated within each source group.

Pattern subscriptions use the same hierarchy:

```sh
smoke logmash us:west:events --pattern 'us:east:worker:*'
```

## Source provenance

Every incoming callback message carries the logical source independently of the physical Redis endpoint:

```json
{
  "provider": "redis",
  "source": "us:west",
  "channel": "events",
  "pattern": "",
  "payload": "hello"
}
```

Stdout preserves the relationship:

```text
us:west:events	hello
us:west:ratelimiters	allowed
us:east:events	deployed
```

## Human-readable routing grammar

Managed callback destinations use `--into` rather than requiring callback URLs:

```sh
smoke logmash \
  us:west:events \
  us:east:events \
  --into axiom east mydataset \
  --into axiom eu mydataset
```

Current Axiom aliases include:

```text
axiom east <dataset>
    -> axiom-us-east-1.logma.sh

axiom eu <dataset>
    -> axiom-eu-central-1.logma.sh

axiom default <dataset>
    -> axiom.logma.sh
```

Dataset names remain runtime inputs and never enter DNS/Terraform state.

## Logmash lifecycle

Background runtime is the default:

```sh
smoke logmash \
  us:west:events \
  us:east:events \
  --into axiom eu mydataset
```

The launcher starts another process from the same Smoke executable in a new OS session and immediately returns the shell prompt. The child inherits the launcher's stdout and stderr, so stdout remains a normal Logmash callback even though the runtime process is detached from the launcher.

```text
shell
  │
  ├── Smoke launcher ── exits
  │
  └── detached same Smoke executable
          └── Logmash runtime
                └── callback supervisor
                      ├── stdout -> inherited terminal fd
                      ├── Axiom
                      └── webhook
```

No socket, output log, tail process, daemon, or data IPC is required.

List running runtimes:

```sh
smoke logmash list
```

Detach only stdout from a running session on Unix:

```sh
smoke logmash detach <session-id>
```

This sends `SIGUSR1` to the Logmash process. Logmash atomically removes the `stdout` callback while keeping Redis subscriptions and the remaining callbacks alive:

```text
before                   after detach
stdout   ✓                stdout   ✕
Axiom    ✓                Axiom    ✓
webhook  ✓                webhook  ✓
```

The callback set is represented by immutable atomic snapshots, so stdout removal does not require a callback registry mutex and does not race an in-flight dispatch. A dispatch already holding the old snapshot may finish once; subsequent dispatches use the new set.

If an inherited stdout write fails while callback policy is `continue`, Logmash removes stdout automatically and keeps the other callbacks running.

Stop the entire runtime separately:

```sh
smoke logmash stop <session-id>
```

That sends `SIGTERM`, cancels the root runtime context, closes the Redis subscriptions, lets Logmash finish its callback supervision, and exits the process.

For debugging or explicit shell-owned lifetime, bypass the default background launch with:

```sh
smoke logmash --foreground us:west:events
```

`--detached` remains accepted as a compatibility spelling but is no longer required. `--no-stdout` starts the runtime without the stdout callback.

## Package direction

The intended dependency direction is:

```text
smoke core
  command registry
  selfbuild/recomposition
  application dispatcher
        ↑
        │ imports core contracts
        │
optional command/provider packages
  cmd/logmash
  future providers/commands
        ↑
        │ selected by local composition imports
        │
local smoke composition main.go
```

Optional packages depend on core contracts. The core does not need to import every optional provider. The composition root chooses what becomes part of the executable.

`xd-dash/logma` remains a separate durable service/resource graph. Logmash is the ephemeral multi-source receive/route command compiled into Smoke. Logmash itself owns subscription and callback supervision; Smoke only establishes the detached process lifetime and provides the small list/detach/stop control surface.
