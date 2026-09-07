# Smoke environments

Smoke environments are named Go workspaces and tool sets used to compose execution without installing every capability globally or changing the manifests of projects being exercised.

Canonical state:

```text
~/.local/share/smoke/envs/<name>/
├── go.work
└── tools/
    ├── go.mod
    └── go.sum
```

Go remains authoritative for module/tool resolution. Smoke does not create a second JSON/YAML dependency graph.

## Create and compose

```bash
smoke env create infra
smoke env use infra ~/src/agni
smoke env module add infra example.com/module@<version-or-sha>
smoke env tool add infra example.com/cmd/tool@<version-or-sha>
```

Local modules are added with `env use`; versioned modules can be composed through `env module add`. Environment tools use Go's `tool` directives.

## Immutable snapshots

Before `shell`, `exec`, `build`, `run`, tool execution, or Terraform execution, Smoke takes a short shared lock and writes/reuses a content-addressed snapshot of `go.work` plus the tools module. The canonical lock is released before the child starts.

A running command therefore keeps the exact Go workspace/tool graph it started with while later canonical mutations affect only later snapshots.

Children receive explicit snapshot identity:

```text
SMOKE_ENV             environment name
SMOKE_ENV_WORKSPACE   immutable snapshot directory
SMOKE_ENV_WORKFILE    immutable snapshot go.work path
GOWORK                same immutable snapshot go.work path
```

`SMOKE_ENV_WORKSPACE` is a directory by contract. This leaves room for future snapshot-owned resource roots without overloading a file-path variable.

## Execute environment tools

```bash
smoke env tool list infra
smoke env tool run infra <tool> [args ...]
```

`tool run` is a thin invocation of `go tool <tool> ...` under the immutable environment snapshot.

## Execute native programs

```bash
smoke env exec infra -- <program> [args ...]
smoke env exec infra --dir ~/src/project -- <program> [args ...]
```

Use `env run` only for commands compiled into Smoke itself:

```bash
smoke env run infra -- logmash us:west:events
```

The distinction is deliberate:

```text
compiled Smoke capability -> env run
Go environment tool       -> env tool run
arbitrary native program  -> env exec
```

## Terraform

Terraform remains an external native prerequisite. Smoke provides a generic convenience façade that snapshots the environment and forwards Terraform arguments unchanged:

```bash
smoke env terraform infra --dir ./terraform -- init
smoke env terraform infra --dir ./terraform -- plan
smoke env terraform infra --dir ./terraform -- apply
smoke env terraform infra --dir ./terraform -- output
smoke env terraform infra --dir ./terraform -- destroy
```

Smoke does not parse HCL or replace Terraform state/provider semantics.

A deployment environment can compose an external Agni profile/component tool without compiling that deployment into Smoke. Current Probe usage is:

```bash
smoke env create astrochicken
smoke env tool add astrochicken \
  github.com/dash-xd/agni/cmd/probe@<exact-agni-sha>

root="$PWD/.astrochicken-probe"
smoke env tool run astrochicken probe seed "$root"

smoke env terraform astrochicken --dir "$root" -- init
smoke env terraform astrochicken --dir "$root" -- plan
```

There is no second `tf seed`, module list, `materialize`, or Smoke-owned profile command. Probe's HCL declares its Terraform module imports; Probe's seeder supplies its own shared implementation library.

The word `astrochicken` here is only the Smoke environment name. `probe` is the Agni component/tool identity. Smoke does not infer one from the other.

A future durable Gateway follows the same generic Smoke pattern:

```bash
smoke env create gateway
smoke env tool add gateway github.com/dash-xd/agni/cmd/gateway@<exact-agni-sha>
smoke env tool run gateway gateway seed "$root"
```

That command should exist only after Agni's Gateway profile contains the complete durable graph.

## Source roots and future resource snapshots

Today Terraform/profile roots are explicit external source directories. Their exact source identity should be recorded by the qualifying workflow along with the Smoke environment digest.

The current Smoke environment digest covers only the Go workspace/tool state that Smoke snapshots. It does **not** claim to hash externally seeded Terraform/profile roots.

If Smoke later gains declared snapshot-owned resource roots, those resources MUST participate in the content digest before the snapshot can claim to identify them. Do not silently copy arbitrary files into the environment and leave them outside identity accounting.

Any such future root should live beneath the snapshot directory identified by `SMOKE_ENV_WORKSPACE`; `SMOKE_ENV_WORKFILE` remains the explicit Go workspace file path.

## Composition versus environment

```text
smoke compose
    = compiled optional Go packages in the Smoke executable

smoke env
    = Go modules/tools plus child execution context
```

Environment tools do not need to become compiled Smoke commands. This is the preferred model for external profile seeders and similar build/deployment helpers.
