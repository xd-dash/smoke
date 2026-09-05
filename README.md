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

Smoke dispatches directly into that handler. Attached Logmash runs stay in that process. Only unattended Logmash runtimes need the OS-level lifetime transition: Smoke starts the same executable again as a background child and returns the shell prompt.

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

Stdout is included by default. Its presence means the runtime is **attached**:

```sh
smoke logmash \
  us:west:events \
  us:east:events \
  --into axiom eu mydataset
```

```text
Redis ──┬──> stdout
        └──> Axiom

output is visible
shell waits
Ctrl+C stops the whole composition
```

Attached does not mean interactive. Logmash does not need stdin; it simply owns the shell lifetime while writing output.

To create an unattended callback runtime, remove stdout:

```sh
smoke logmash \
  us:west:events \
  us:east:events \
  --no-stdout \
  --into axiom eu mydataset
```

```text
Redis ─────> Axiom

no terminal consumer
shell returns
runtime continues
```

For an unattended runtime, Smoke starts another process from the exact same executable. On Unix the child starts in a new OS session. No socket, output log, `tail`, `nohup`, resident daemon, or data IPC is required.

Only unattended runtimes are registered as sessions:

```sh
smoke logmash list
smoke logmash stop <session-id>
```

`stop` sends `SIGTERM` on Unix, cancels the root runtime context, closes the Redis subscriptions, lets Logmash finish callback supervision, and exits the process.

If stdout is disabled but shell-owned lifetime is still useful for debugging, explicitly keep the runtime attached:

```sh
smoke logmash \
  us:west:events \
  --no-stdout \
  --attached \
  --into axiom eu mydataset
```

The lifecycle rule is intentionally small:

```text
                 stdout?
                    │
           ┌────────┴────────┐
          yes                no
           │                  │
       attached          unattended
           │                  │
    output visible        shell returns
      shell waits          session ID
    Ctrl+C stops it        stop ID
```

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

`xd-dash/logma` remains a separate durable service/resource graph. Logmash is the ephemeral multi-source receive/route command compiled into Smoke. Logmash owns subscription and callback supervision; Smoke only owns composition plus the small unattended-runtime start/list/stop boundary.
