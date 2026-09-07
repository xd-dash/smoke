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

A deployment environment can therefore compose Terraform asset tools without compiling the deployment into Smoke:

```bash
smoke env create astrochicken
smoke env tool add astrochicken github.com/dash-xd/agni/cmd/agni-terraform@<sha>
smoke env tool add astrochicken github.com/xd-dash/smoke/cmd/astrochicken-root@<sha>

root="$PWD/.astrochicken-tf"
smoke env tool run astrochicken astrochicken-root "$root"
smoke env tool run astrochicken agni-terraform materialize \
  --module regional-network \
  --module regional-internal-addresses \
  --module coreos-node \
  --module regional-cell \
  --module cloud-function-v1-http \
  --module cloud-function-v2-http \
  "$root"

smoke env terraform astrochicken --dir "$root" -- init
smoke env terraform astrochicken --dir "$root" -- plan
```

The word `astrochicken` here is an environment/recipe name, not a Smoke core command or Agni primitive.

## Source roots and future resource snapshots

Today ordinary Terraform recipe roots are explicit source directories. Their exact source identity should be recorded by the qualifying workflow along with the Smoke environment digest.

If Smoke later gains snapshot-owned resource roots, those resources MUST participate in the content digest before the snapshot can claim to identify them. Do not silently copy arbitrary files into the environment and leave them outside identity accounting.

## Composition versus environment

```text
smoke compose
    = compiled optional Go packages in the Smoke executable

smoke env
    = Go modules/tools plus child execution context
```

Environment tools do not need to become compiled Smoke commands. This is the preferred model for Terraform asset materializers and similar build/deployment helpers.
