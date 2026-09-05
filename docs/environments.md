# Smoke environments

Smoke environments are named Go workspaces used to compose local modules and Go tool dependencies without installing those tools globally or changing the module manifests of the projects being exercised.

The environment itself is first-class Smoke tooling. It is not a Smoke provider. Providers remain runtime transports/resources selected by optional Smoke commands.

## Representation

Each environment is stored under the Smoke data directory (or `SMOKE_ENV_DIR` when overridden):

```text
~/.local/share/smoke/envs/<name>/
├── go.work
└── tools/
    ├── go.mod
    └── go.sum      # created when dependencies require it
```

`go.work` is the workspace manifest. The `tools` module exists only to carry environment-scoped `tool` directives and their normal Go module requirements.

Creating an environment:

```sh
smoke env create infra
```

produces the equivalent of:

```go
// go.work
go 1.26

use ./tools
```

and:

```go
// tools/go.mod
module smoke.local/env/infra/tools

go 1.26
```

There is intentionally no second JSON/YAML tool dependency graph. Go owns tool versions, requirements, replacements, exclusions, sums, and workspace resolution.

## Workspace modules

Add a local module to an environment:

```sh
smoke env use infra ~/src/agni
```

Remove it:

```sh
smoke env drop infra ~/src/agni
```

Both operations delegate to the Go workspace machinery. A `use` target must contain a `go.mod` file.

Inspect environments:

```sh
smoke env list
smoke env show infra
```

## Environment tools

Add a Go tool dependency:

```sh
smoke env tool add infra golang.org/x/tools/cmd/stringer
```

A specific version may be selected with normal Go package/version syntax:

```sh
smoke env tool add infra example.com/tool/cmd/tool@v1.2.3
```

Remove a tool:

```sh
smoke env tool remove infra example.com/tool/cmd/tool
```

List tools visible through the selected workspace:

```sh
smoke env tool list infra
```

The tool dependency is fetched and versioned by Go. It is not installed into `$GOBIN` merely because it belongs to the environment.

## Running in an environment

Run an arbitrary process with the environment's `GOWORK` and `SMOKE_ENV` values:

```sh
smoke env exec infra -- go tool stringer
smoke env exec infra --dir ~/src/agni -- go test ./...
```

Launch a scoped shell:

```sh
smoke env shell infra ~/src/agni
```

Exiting that child shell returns to the original shell without mutating the parent shell environment.

Build using the workspace:

```sh
smoke env build infra --dir ~/src/agni -- ./...
```

`smoke env build` is deliberately thin: it invokes the system `go build` under the selected workspace and leaves package/build semantics to Go.

## Compiled-in Smoke commands and Logmash

Use `smoke env run` when the thing being executed is another compiled-in Smoke command:

```sh
smoke env run infra -- logmash us:west:events
```

or from a project directory:

```sh
smoke env run infra --dir ~/src/agni -- logmash us:west:events
```

This path does **not** discover or start a separate `logmash` executable. Smoke activates the environment and dispatches `logmash` through the same in-process command registry used by a normal `smoke logmash ...` invocation.

That preserves the existing composition boundary:

```text
local Smoke binary
    │
    ├── compiled-in command registry
    │       └── logmash
    │
    └── selected Smoke environment
            ├── go.work
            ├── project modules
            └── tools/go.mod
```

An unattended Logmash run still re-execs the exact same Smoke executable. The child inherits `GOWORK` and `SMOKE_ENV`, so background lifetime does not create a second environment model.

## Composition versus environment

The two concepts are intentionally separate:

```text
smoke compose
    = which optional Go packages are linked into the Smoke executable

smoke env
    = which Go workspace/modules/tools are active while Smoke is used
```

For example, an installed Smoke composition may include Logmash, while `infra` and `dev` environments expose different project modules and different Go tools. An environment cannot make a command available if that command was not compiled into the current Smoke executable.

This preserves import-time composition and keeps workspace selection out of the provider/runtime transport layer.
