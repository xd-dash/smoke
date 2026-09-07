# Smoke / Logmash idioms

This file is the maintenance contract for `xd-dash/smoke`. Focused docs explain usage; this file records architectural invariants.

## Core identity

Smoke is a self-composed Go executable plus named execution environments. It is not a runtime plugin host and it is not a replacement package manager.

- Compiled-in commands/providers are ordinary Go packages selected by Go imports.
- Imported command packages register through `command.Register` during process initialization.
- The composition entrypoint owns the normalized component import list and `identity.SetComponents(...)`.
- `commands`, `compose`, `env`, and `inspect` are core names.
- Do not reintroduce PATH-based compiled-command discovery, `.so` plugins, or a resident plugin daemon.
- Ordinary environment tools are intentionally different from compiled-in Smoke commands: they are Go `tool` dependencies executed with `go tool` inside an environment snapshot.

The stock composition imports Logmash. The system Go toolchain remains the self-composition primitive.

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

Smoke has separate identities:

```text
composition digest
    = compiled component set

workspace digest
    = immutable environment Go workspace/tool snapshot
```

These identities are observational and complement, not replace, exact Git SHA/build qualification.

- A composition digest comes from the currently running process, not mutable on-disk composition state.
- A workspace digest covers the snapshot `go.work`, `tools/go.mod`, and `tools/go.sum` when present.
- `smoke inspect` and `smoke env inspect` expose these identities for correlation.
- Exact candidate/source/tool identities remain the responsibility of the qualifying workflow, normally Huram.

## Environment/workspace invariants

A Smoke environment is a **named execution composition whose dependency authority is Go workspace/tool state**. It is first-class Smoke tooling, not a runtime provider.

Canonical mutable state remains intentionally small:

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
- Canonical mutation is protected by the existing cross-process environment lock.
- Long-lived execution snapshots canonical state under a short shared lock, releases the lock, and runs against an immutable content-addressed snapshot.
- Equal Go workspace/tool state reuses the same digest/path; old snapshots are not mutated.
- Child activation uses `GOWORK`, `SMOKE_ENV`, and environment workspace identity. Never mutate process-global cwd/GOWORK to implement environment execution.

An environment can also orchestrate **ordinary source roots and native tools**, but those must not become an implicit second dependency graph. Until a future resource-root primitive explicitly includes external assets in the environment digest, callers pass the source root explicitly (for example `--dir <terraform-root>`) and qualification records that recipe/source identity separately.

This distinction is deliberate:

```text
Go modules/tools
    -> canonical environment dependency state

Terraform recipe/root
    -> ordinary source selected by the caller

Terraform executable
    -> external native prerequisite
```

### Generic tool execution

Environment tools are run through the immutable workspace with:

```bash
smoke env tool run <env> <tool> [args ...]
```

This is a thin `go tool <tool> ...` execution boundary. Smoke does not reinterpret the tool's domain language.

### Generic Terraform execution

Terraform is a native contract, not a Go library dependency of Smoke:

```bash
smoke env terraform <env> --dir <terraform-root> -- init
smoke env terraform <env> --dir <terraform-root> -- plan
smoke env terraform <env> --dir <terraform-root> -- apply
smoke env terraform <env> --dir <terraform-root> -- output
smoke env terraform <env> --dir <terraform-root> -- destroy
```

Smoke snapshots the environment, locates the installed `terraform` executable, sets the child environment, and forwards Terraform arguments unchanged. Terraform owns `.tf`, providers, variables, backends, state, plans, and lifecycle semantics.

Do not add deployment names such as Astrochicken to this generic command.

### Environment recipes

A named deployment/probe such as Astrochicken is a recipe **above** Smoke core:

```text
Astrochicken environment
    +-- selected exact Go modules
    +-- selected exact Go tools
    |     +-- agni-terraform
    |     `-- astrochicken-root (transitional recipe tool)
    +-- materialized ordinary Terraform root
    `-- native terraform
```

The recipe name and deployment policy do not belong in Smoke's generic environment implementation. A recipe tool may temporarily live in the Smoke repository for exact-source convenience, but it is not linked into the stock Smoke binary and may move to its own module/repository later without changing environment semantics.

## Composition versus environment

```text
smoke compose
    = which optional Go packages are linked into the Smoke executable

smoke env
    = which Go modules/tools and child execution context are selected
```

An environment cannot make a compiled Smoke command appear. Conversely, a capability does not need to be compiled into Smoke when it is naturally an ordinary Go tool, such as Terraform asset materialization.

`smoke env run` re-execs Smoke for compiled-in commands. `smoke env tool run` executes environment tools. `smoke env exec` executes arbitrary native processes. Keep those boundaries distinct.

## Provider registry invariants

Provider registries are for typed **runtime dispatch capabilities**, not generic tool discovery or Terraform composition.

- providers are supplied by Go composition/callers, never discovered from PATH;
- schemes are normalized and duplicates invalid;
- keep provider contracts small and capability-specific;
- prefer narrow optional interfaces over one catch-all provider;
- do not use a Smoke provider merely to materialize `.tf` files or forward Terraform CLI arguments when an ordinary environment tool/native command is sufficient.

This rule intentionally retires the experimental Smoke↔Agni Terraform provider bridge. Agni Terraform modules are composed through an ordinary Go tool instead.

## Logmash source/runtime invariants

A source selector is `COUNTRY:REGION:CHANNEL`; patterns use `COUNTRY:REGION:PATTERN`. Selectors are grouped by logical source and exact duplicates are removed.

Stdout remains enabled by default:

```text
stdout present -> attached -> shell waits -> cancellation is shell-owned
stdout absent  -> unattended -> session identity -> callbacks continue
```

- `--no-stdout` is the normal unattended transition.
- Do not add a second detached/supervision axis without a requirement that cannot fit attached/unattended ownership.
- One invocation shares cancellation across source goroutines and waits for them before returning.
- Callback fan-out remains bounded by synchronous backpressure per source unless an explicit queue primitive is introduced.
- HTTP callback bodies must be drained and closed.

## DNS/provider security

DNS is provider discovery, not secret/runtime state.

- credentials never live in DNS;
- runtime channel/dataset state does not live in DNS;
- observation credentials remain least privilege;
- typed metadata should beat hostname-shape inference when available.

## Session registry

The unattended session registry is lightweight local supervision metadata, not durable Logma state. Lease ownership is the liveness authority; PID existence alone is insufficient. Composition/workspace identities are correlation metadata only.

## Self-composition/rebuild

Composition state remains a sorted, deduplicated import set. Recomposition is isolated from named environments with `GOWORK=off`, serialized under one cross-process composition lock, and installs only a successfully built candidate via atomic replacement where supported.

Do not let environment tools/resources become hidden inputs to self-composition.

## Durable Logma boundary

`xd-dash/logma` remains the durable Fatline service/resource graph. Logmash remains ephemeral receive/route/callback runtime. Do not collapse durable Logma state into Smoke environments or unattended session metadata.

## Cross-repository authority boundary

For cloud/deployment work the preferred chain is:

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
  generic infrastructure modules/tools
       |
       v
native Terraform/gcloud/Butane/QEMU
```

Smoke must not persist Huram credentials/business values. Agni must not own Smoke environment recipes. Huram must not duplicate generic Smoke environment or Agni infrastructure implementation.

## Change protocol

When modifying Smoke:

1. Preserve the smallest native composition primitive that satisfies the requirement.
2. Keep Go as authority for Go modules/tools and Terraform as authority for Terraform.
3. Prefer environment tools over new compiled providers when the capability is naturally a CLI/materializer.
4. Keep deployment recipes outside generic Smoke core.
5. Preserve immutable snapshots and short canonical locks.
6. Keep process-global cwd/environment mutation out of reusable execution paths.
7. Preserve exact-source qualification outside Smoke runtime identity.
8. Add focused tests for parser/environment/tool/provider/lifecycle invariants touched.
9. Run `go vet ./...` and `go test -race ./...` on the exact final candidate.
10. Update this maintenance contract when a responsibility boundary changes.

Before adding a daemon, custom dependency graph, runtime plugin mechanism, asset package manager, or provider abstraction, first verify the requirement cannot be expressed through Go composition, named environments, `go.work`/`go.mod`, environment tools, immutable snapshots, `env exec`, or the authoritative native tool itself.
