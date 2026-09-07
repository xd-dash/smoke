# Smoke / Logmash idioms

This file is the maintenance contract for `xd-dash/smoke`. Focused docs explain usage; this file records architectural invariants.

## Core identity

Smoke is a self-composed Go executable plus named execution environments. It is not a runtime plugin host and it is not a replacement package manager.

- Compiled-in commands/providers are ordinary Go packages selected by Go imports.
- Imported command packages register through `command.Register` during initialization.
- The composition entrypoint owns the normalized component import list and `identity.SetComponents(...)`.
- `commands`, `compose`, `env`, and `inspect` are core names.
- Do not reintroduce PATH-based compiled-command discovery, `.so` plugins, or a resident plugin daemon.
- Environment tools are Go `tool` dependencies executed with `go tool` inside an environment snapshot; they are not compiled Smoke commands.

## Dependency direction

```text
compiled optional command/provider
            |
            v
      Smoke core contracts

named Smoke environment
    +-- Go workspace modules
    +-- Go tool dependencies
    +-- native executables invoked generically
```

Core packages must not import every optional capability merely for discovery. Go composition determines compiled capabilities; `go.work` and the environment tools module determine environment-scoped modules/tools.

## Runtime identity

Smoke keeps composition identity separate from environment-workspace identity:

```text
composition digest = compiled component set
workspace digest   = immutable Go workspace/tool snapshot
```

Exact Git SHA/build qualification remains outside Smoke, normally in Huram.

## Environment/workspace invariants

A Smoke environment is a named execution composition whose dependency authority is Go workspace/tool state.

Canonical mutable state is intentionally small:

```text
<env>/
├── go.work
└── tools/
    ├── go.mod
    └── go.sum
```

Rules:

- `go.work` is authoritative for local/versioned Go modules used by the environment.
- `tools/go.mod` is authoritative for environment-scoped Go `tool` dependencies.
- Do not mirror Go module/tool dependencies into a Smoke JSON/YAML graph.
- Workspace/tool mutation uses ordinary Go commands so Go owns versions, sums, replacements, exclusions, and tool resolution.
- Canonical mutation uses the cross-process environment lock.
- Long-lived execution snapshots canonical state under a short shared lock, releases the lock, and runs against an immutable content-addressed snapshot.
- Equal Go workspace/tool state reuses the same digest/path; old snapshots are never mutated.
- Child activation uses `GOWORK`, `SMOKE_ENV`, and environment workspace identity. Never mutate process-global cwd/GOWORK to implement environment execution.

External source roots such as Terraform roots are caller-owned until a future explicit resource-root primitive includes them in the environment digest. Qualification therefore records their exact source/seed identities separately.

## Seed is the preparation idiom

Use **seed** for preparing a destination from an exact source/tool. This is already the established ghxd/worktree idiom:

```bash
smoke ghxd worktree ... seed ...
github-worktree seed ...
```

The semantic contract is:

```text
exact source/tool
      |
      v
seed destination/root
      |
      v
consumer operates on seeded root
```

Use `seed` consistently for Terraform recipe roots and reusable Terraform modules as well. Do not introduce `materialize` as a competing public command, package API, or architectural term for the same operation.

## Generic tool execution

Environment tools run through the immutable workspace:

```bash
smoke env tool run <env> <tool> [args ...]
```

This is a thin `go tool <tool> ...` boundary. Smoke does not reinterpret the tool's domain language.

## Generic Terraform execution

Terraform is an authoritative native contract, not a Go library dependency of Smoke:

```bash
smoke env terraform <env> --dir <terraform-root> -- init
smoke env terraform <env> --dir <terraform-root> -- plan
smoke env terraform <env> --dir <terraform-root> -- apply
smoke env terraform <env> --dir <terraform-root> -- output
smoke env terraform <env> --dir <terraform-root> -- destroy
```

Smoke snapshots the environment, locates the installed `terraform` executable, sets the child environment, and forwards arguments unchanged. Terraform owns `.tf`, providers, variables, backends, state, plans, and lifecycle semantics.

## Environment recipes

A deployment/probe such as Astrochicken is a recipe above Smoke core:

```text
Astrochicken environment
    +-- selected exact Go tools
    |     +-- astrochicken-root
    |     `-- agni-terraform
    +-- seeded ordinary Terraform root
    `-- native terraform
```

The recipe name and deployment policy do not belong in Smoke's generic environment implementation. The current recipe tool may live in Smoke temporarily, but it is not linked into stock Smoke.

The intended seed path is:

```bash
smoke env tool run astrochicken astrochicken-root seed <root>
smoke env tool run astrochicken agni-terraform seed --module ... <root>
```

## Composition versus environment

```text
smoke compose = optional Go packages linked into Smoke
smoke env     = Go modules/tools and child execution context
```

`smoke env run` re-execs Smoke for compiled commands. `smoke env tool run` executes environment tools. `smoke env exec` executes arbitrary native programs. Keep those boundaries distinct.

## Provider registry invariants

Provider registries are for typed runtime dispatch capabilities, not build/root seeding or generic tool discovery.

- providers are supplied by Go composition/callers, never discovered from PATH;
- schemes are normalized and duplicates invalid;
- keep provider contracts small and capability-specific;
- prefer narrow optional interfaces over a catch-all provider;
- do not use a provider merely to seed Terraform source or forward Terraform CLI arguments.

The experimental Smoke↔Agni Terraform provider bridge is retired.

## ghxd / worktree idiom

`ghxd` remains GitHub-specific. Worktree preparation uses `seed` and exact SHA inputs. `github-worktree seed` is the reference idiom for source-root preparation: exact source identity, caller-selected destination, no hidden dependency graph.

## Logmash runtime invariants

Logmash remains ephemeral receive/route/callback runtime. Stdout is enabled by default; removing stdout is the normal transition to unattended operation. Context cancellation and explicit ownership remain preferred over shared mutable state. The unattended session registry is local supervision metadata, not durable Logma state.

## Durable Logma boundary

`xd-dash/logma` remains the durable Fatline service/resource graph. Do not collapse durable Logma state into Smoke environments or unattended session metadata.

## Cross-repository authority boundary

```text
Huram
  exact candidates + credentials + evidence + promotion
       |
       v
Smoke
  environment/workspace/tool composition + immutable execution
       |
       v
Agni
  generic infrastructure modules/seed tools
       |
       v
native Terraform/gcloud/Butane/QEMU
```

Smoke must not persist Huram credentials/business values. Agni must not own Smoke environment recipes. Huram must not duplicate generic Smoke environment or Agni infrastructure implementation.

## Change protocol

When modifying Smoke:

1. Preserve the smallest native composition primitive that satisfies the requirement.
2. Keep Go authoritative for Go modules/tools and Terraform authoritative for Terraform.
3. Use `seed` for exact-source destination preparation; do not add parallel `materialize` vocabulary.
4. Prefer environment tools over new compiled providers when the capability is naturally a CLI/seed operation.
5. Keep deployment recipes outside generic Smoke core.
6. Preserve immutable snapshots and short canonical locks.
7. Keep process-global cwd/environment mutation out of reusable execution paths.
8. Preserve exact-source qualification outside Smoke runtime identity.
9. Add focused tests for parser/environment/tool/provider/lifecycle invariants touched.
10. Run `go vet ./...` and `go test -race ./...` on the exact final candidate.
11. Update this maintenance contract when a responsibility boundary changes.

Before adding a daemon, custom dependency graph, runtime plugin mechanism, asset package manager, or provider abstraction, first verify the requirement cannot be expressed through Go composition, named environments, `go.work`/`go.mod`, environment tools, immutable snapshots, `env exec`, `seed`, or the authoritative native tool itself.
