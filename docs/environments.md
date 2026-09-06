# Smoke environments

Smoke environments are named Go workspaces used to compose local modules and Go tool dependencies without installing those tools globally or changing the module manifests of the projects being exercised.

The environment itself is first-class Smoke tooling. It is not a Smoke provider. Providers remain runtime transports/resources selected by optional Smoke commands.

## Representation

Each canonical environment is stored under the Smoke data directory (or `SMOKE_ENV_DIR` when overridden):

```text
~/.local/share/smoke/envs/<name>/
├── go.work
└── tools/
    ├── go.mod
    └── go.sum      # created when dependencies require it
```

`go.work` is the mutable workspace manifest. The `tools` module carries environment-scoped `tool` directives and their normal Go module requirements.

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

Canonical mutations (`create`, `use`, `drop`, tool add/remove) remain serialized with an exclusive cross-process environment lock.

## Immutable runtime workspace snapshots

Long-lived environment commands no longer hold the canonical environment lock for their lifetime.

Before `shell`, `exec`, `build`, `run`, or `tool list`, Smoke takes a short shared lock and reads one coherent canonical state. It then writes an immutable, content-addressed snapshot under the user cache containing:

```text
<cache>/smoke/env-workspaces/<env>/<digest>/
├── go.work
└── tools/
    ├── go.mod
    └── go.sum      # when present in the canonical environment
```

The snapshot `go.work` preserves the canonical workspace's Go/toolchain/godebug/use/replace semantics. Local project paths are made absolute, while the environment tools module points at the copied `./tools` directory inside the snapshot.

The lock is released **before** the child command starts:

```text
canonical environment
        │
        │ short shared lock
        ▼
content-addressed snapshot
        │
        ├── release canonical lock
        │
        └── shell / exec / build / run
```

This means a shell can keep using the exact environment state it started with while another process changes the canonical environment:

```sh
# terminal A
smoke env shell infra ~/src/agni

# terminal B -- no need to wait for terminal A to exit
smoke env use infra ~/src/firekv
smoke env tool add infra golang.org/x/tools/cmd/stringer
```

Terminal A remains pinned to its old snapshot. A later environment command gets a new snapshot containing the new canonical state.

Unchanged canonical state reuses the same digest/path. Old snapshots are deliberately cache state rather than temporary launch files, so unattended Logmash descendants can continue inheriting a valid `GOWORK` after their launcher exits.

## Running in an environment

Run an arbitrary process with the snapshot's `GOWORK`, plus `SMOKE_ENV` and `SMOKE_ENV_WORKSPACE`:

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

`smoke env build` is deliberately thin: it invokes the system `go build` under the selected snapshot and leaves package/build semantics to Go.

## Compiled-in Smoke commands and Logmash

Use `smoke env run` when the thing being executed is another compiled-in Smoke command:

```sh
smoke env run infra -- logmash us:west:events
```

or from a project directory:

```sh
smoke env run infra --dir ~/src/agni -- logmash us:west:events
```

`env run` does not mutate the parent Smoke process's working directory or environment. It snapshots the environment, releases the canonical lock, then starts Smoke itself as a child with the snapshot `GOWORK`. That child dispatches `logmash` through the ordinary compiled-in command registry.

This still does **not** discover or start a separate `logmash` executable and does not use PATH-based command discovery.

```text
parent Smoke
    │
    ├── snapshot selected environment
    │
    └── child Smoke
            │
            ├── compiled-in command registry
            │       └── logmash
            │
            └── immutable GOWORK snapshot
```

An unattended Logmash runtime may re-exec Smoke again to cross the attached/unattended process-lifetime boundary. `GOWORK`, `SMOKE_ENV`, and `SMOKE_ENV_WORKSPACE` are inherited, so background lifetime stays pinned to the same environment snapshot.

Atomic recomposition replaces the installed Smoke filesystem entry, while already-running Smoke processes keep their existing image. Therefore a child spawn racing with a completed recomposition may start the newly installed composition. The guarantee is that Smoke re-execs Smoke itself, not that it always reproduces the exact parent process image.

## Composition versus environment

The two concepts are intentionally separate:

```text
smoke compose
    = which optional Go packages are linked into the Smoke executable

smoke env
    = which Go workspace/modules/tools are active while Smoke is used
```

For example, an installed Smoke composition may include Logmash, while `infra` and `dev` environments expose different project modules and different Go tools. An environment cannot make a command available if that command was not compiled into the Smoke executable it starts.

This preserves import-time composition and keeps workspace selection out of the provider/runtime transport layer.
