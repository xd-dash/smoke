# Smoke / Logmash idioms

This file is the maintenance contract for `xd-dash/smoke`. The README and focused docs explain usage; this file records architectural invariants that future changes should preserve unless the design is intentionally revised.

## Core identity

Smoke is a self-composed Go executable, not a runtime plugin host.

- Optional commands/providers are ordinary Go packages selected by Go imports.
- Imported command packages register through `command.Register` during process initialization.
- Optional top-level composition packages register their logical component identity through `identity.RegisterComponent(importPath)` during initialization.
- Normal command dispatch is an in-process Go function call.
- `commands`, `compose`, `env`, and `inspect` are reserved core command names and optional packages must not register them.
- Compiled-in command names are single non-whitespace tokens. Invalid, duplicate, or reserved registrations are programmer errors and should fail during initialization.
- Do not reintroduce PATH-based command discovery, separately installed Logmash executables, dynamic `.so` loading, or a resident plugin daemon unless a new requirement specifically needs them.

The default composition imports `github.com/xd-dash/smoke/cmd/logmash`. The system Go toolchain remains the composition/rebuild primitive.

## Package direction

Keep dependencies pointed inward toward small core contracts:

```text
optional command/provider package
            │
            ▼
      Smoke core contracts

local composition
      │ imports selected optional packages
      ▼
  final smoke binary
```

Core packages must not import every optional provider merely to make them discoverable. The composition root remains the authority for what is linked.

## Runtime identity and inspection

Smoke has two distinct runtime identities:

```text
composition digest
    = logical set of self-registered compiled components

workspace digest
    = content-addressed immutable environment snapshot
```

Together they identify the two axes relevant to a smoke-test runtime:

```text
(composition digest, workspace digest)
        = what Smoke could do
          + what Go workspace/tool state it saw
```

Rules:

- The composition digest must be derived from the component set owned by the currently running process, not from the mutable on-disk composition manifest. An old process after recomposition must not accidentally report the new manifest's identity.
- Component import paths are normalized, deduplicated, sorted, and hashed deterministically.
- The composition digest is a **logical composition identity**, not a byte-for-byte executable hash and not a substitute for exact Git SHA/build qualification.
- Optional top-level composition packages must self-register their import-path identity. This is analogous to command registration but serves inspectability rather than dispatch.
- `smoke inspect` reports the running composition digest, executable path, Go version, component imports, and any inherited environment/workspace identity.
- `smoke env inspect <name>` reports canonical environment paths plus the immutable runtime snapshot digest/path Smoke would use.
- `SMOKE_ENV_WORKSPACE` identifies the exact workspace snapshot path; its content-addressed directory name is the workspace digest.
- Runtime identity is observational metadata. Do not turn it into another dependency graph, package resolver, persistent control plane, or authorization mechanism.
- Exact deployment/test qualification still belongs to the external workflow that selected the repository SHA/build candidate. Smoke identity should correlate with that workflow, not replace it.

## Smoke environment/workspace invariants

A Smoke environment is a named Go workspace. It is first-class Smoke tooling, not a runtime provider.

Canonical mutable state is:

```text
<env>/
├── go.work
└── tools/
    ├── go.mod
    └── go.sum
```

Rules:

- `go.work` is the canonical environment/workspace manifest.
- `tools/go.mod` carries environment-scoped Go `tool` directives and their normal module requirements.
- Do not mirror Go tool dependencies into a Smoke JSON/YAML dependency graph.
- Use normal Go commands to mutate workspace/tool state so Go remains authoritative for version selection, sums, replacements, exclusions, and tool resolution.
- Workspace `use` targets are local Go module directories containing `go.mod`.
- The environment tools module is always part of the workspace so declared tools are visible to `go tool` in workspace mode.
- Canonical mutation (`create`, `use`, `drop`, tool add/remove) takes an exclusive cross-process environment lock.
- Long-lived consumers (`shell`, `exec`, `build`, `run`, and tool listing) take only a short shared lock to snapshot one coherent environment state, release the lock, and then run against that immutable snapshot.
- A runtime snapshot includes `go.work` plus `tools/go.mod` and `tools/go.sum` when present. Snapshotting only `go.work` is insufficient because later tool mutation would otherwise change the running tool graph.
- Snapshot `go.work` preserves Go/toolchain/godebug/use/replace semantics. Local project paths are normalized; the tools `use` entry points at the copied snapshot-local tools module.
- Runtime snapshots are content-addressed cache state. Equal canonical state reuses one digest/path; changed canonical state creates a new immutable digest/path and never mutates older snapshots.
- Do not delete a snapshot merely because the launcher exits. Unattended Logmash descendants may still inherit its `GOWORK`. Future GC must prove a snapshot is unreferenced or use a conservative retention policy.
- Environment activation is scoped to child processes with `GOWORK`, `SMOKE_ENV`, and `SMOKE_ENV_WORKSPACE`; never mutate process-global `GOWORK` or cwd to implement `env run`.
- `smoke env shell` creates a child shell; `smoke env exec` creates an arbitrary child process; `smoke env build` remains a thin system `go build`; `smoke env run` re-execs Smoke itself and then uses normal compiled-in command dispatch.

Ownership model:

```text
mutable canonical environment
        │
        │ short shared lock
        ▼
immutable content-addressed snapshot
        │
        ├── release canonical lock
        │
        └── shell / exec / build / run

canonical use/drop/tool mutation
        │
        └── exclusive lock only for mutation duration
```

A running command keeps the environment state it started with while later canonical mutation proceeds immediately and affects only later snapshots.

Composition and environment remain separate axes:

```text
smoke compose
    = which optional packages are linked into the Smoke binary

smoke env
    = which local modules/tools are active while that binary is used
```

An environment cannot make an optional command available unless that command is already linked into the current binary.

Logmash follows the same rule. `smoke env run <env> -- logmash ...` must execute Smoke itself and dispatch the compiled Logmash handler; it must not resolve or install a separate `logmash` executable. Unattended Logmash children inherit the immutable runtime snapshot.

A running Smoke process and the installed Smoke filesystem entry are distinct after atomic recomposition. A re-exec racing with a completed replacement may start the newly installed composition. Do not claim exact parent-image identity unless an immutable executable snapshot or OS-specific self-exec primitive is introduced.

## Logmash source grammar

A source selector is `COUNTRY:REGION:CHANNEL`; pattern selectors use `COUNTRY:REGION:PATTERN`.

Examples:

```text
us:west:events
us:east:events
us:east:worker:*
```

The CLI identity is broad-to-narrow (`country:region`). Managed DNS is hierarchical (`region.country.logma.sh`). Callback provenance uses the logical source such as `us:west`, not DNS spelling or the physical Redis host.

Selectors are grouped by logical source. One source group owns one Redis Pub/Sub connection and may contain multiple exact channels and patterns. Exact duplicate selectors are deduplicated before connecting.

## Attached versus unattended lifecycle

Stdout is included by default.

```text
stdout present
    → attached runtime
    → output visible
    → shell waits
    → Ctrl+C cancels the composition

stdout absent
    → unattended runtime
    → shell returns
    → callbacks continue
    → runtime gets a session ID
```

`--no-stdout` is the normal transition to unattended callback runtime. `--attached` is only the explicit override for a no-stdout runtime that should remain shell-owned for debugging/testing.

Do not add a second public lifecycle axis such as `--detached`, stdout reattachment, output sockets, persisted logs, or a resident Smoke daemon unless a real requirement cannot be expressed by attached/unattended ownership.

Only unattended runtimes are written to the local session registry.

## Runtime ownership and cancellation

Logmash owns subscriptions and callbacks. Smoke owns composition plus the small unattended start/list/stop boundary.

For one invocation:

- resolve all source groups before entering the receive loop;
- create one goroutine per source subscription;
- share one child context across source goroutines;
- the first unexpected source error cancels siblings;
- normal parent cancellation is not promoted to a provider error;
- wait for every source goroutine before returning.

Prefer context cancellation and channel/process ownership over shared mutable source state.

## Callback semantics

The callback set is fixed after startup. Dispatcher policy and error-handler configuration are synchronized, but configure-before-run remains the normal lifecycle.

Every provider message is fanned out to every configured callback. The dispatcher waits for all callbacks for one message before receiving the next message from that source. This is deliberate backpressure, not a delivery queue.

Therefore:

- a slow callback can slow that source;
- `continue` reports callback failures and keeps the source alive;
- `fail-fast` returns the callback error and ends that source/runtime;
- nil callback entries fail as callback errors rather than panicking;
- if buffering/retry is needed later, add an explicit bounded delivery primitive with queue capacity, overflow policy, retries, timeout, and counters rather than silently spawning unbounded goroutines.

HTTP response bodies must be drained and closed for connection reuse.

## DNS discovery invariants

DNS is provider discovery, not runtime state.

- credentials never live in DNS;
- channels/patterns never live in DNS;
- Axiom dataset IDs never live in DNS/Terraform state;
- auth provider/profile names may be non-secret DNS metadata;
- multiple unrelated TXT records may coexist;
- resolver code must select a valid typed Smoke provider record rather than assume the first TXT record belongs to that provider.

Service identity should come from typed metadata rather than arbitrary hostname shape when typed metadata exists.

## Provider registry invariants

Provider schemes are typed runtime dispatch, not plugin discovery.

- nil providers are invalid;
- schemes are normalized to lowercase and must satisfy URL-scheme syntax;
- duplicate normalized schemes are invalid;
- nil registries return errors rather than panic;
- providers are supplied by Go composition/callers, never discovered from PATH or runtime directories.

Keep provider contracts small. If providers diverge in capability, prefer narrow optional interfaces such as subscriber/publisher/resolver over growing one catch-all provider interface.

## Redis provider invariants

- validate target, callbacks, and selectors before receive loops;
- PING before subscribing so connection/auth failures surface early;
- exact-only subscriptions use `SUBSCRIBE`;
- pattern-only subscriptions use `PSUBSCRIBE` directly;
- mixed subscriptions initialize exact channels and then patterns explicitly;
- cancellation closes Pub/Sub/client resources;
- `Target.Source` is credential-free provenance.

Observation credentials should be least-privilege and must not imply publish/key mutation/admin authority.

## Session registry

The session registry is lightweight local supervision metadata for unattended Logmash processes. It is not durable Logma state.

Current records include random session ID, PID, source/callback summaries, auth provider, start time, composition digest, environment name, workspace digest/path, and lease path. Callback metadata remains sanitized before persistence.

Each live unattended process owns an exclusive OS-backed lease file. Lease ownership, not PID existence alone, is the liveness authority. `list` and `stop` require a held lease and a live PID; stale records are cleaned when ownership is gone.

`smoke logmash list` should remain compact. `smoke logmash list --verbose` exposes correlation/debug metadata including composition/workspace identities and lease path.

Session identity metadata is observational. Do not turn the lease/JSON registry into durable Logma graph state, orchestration state, or a resident supervision daemon.

## Self-composition/rebuild invariants

Composition state is a sorted, deduplicated list of Go import paths.

All composition mutation is one cross-process transaction. `add`/`remove` perform manifest read, transformation, dependency resolution, candidate build, manifest commit, and executable replacement under one exclusive composition lock. `rebuild` uses the same lock.

Recomposition is isolated from Smoke environments: every Go command used for composition runs with `GOWORK=off`.

Apply order:

```text
lock composition
        ↓
load + normalize desired imports
        ↓
snapshot generated main.go/go.mod/go.sum
        ↓
generate/update composition module
        ↓
go mod tidy
        ↓
go build staged candidate
        ↓
build failed? restore generated source/module state
        ↓
save desired manifest atomically
        ↓
atomic rename candidate over installed Smoke
        ↓
rename failed? restore previous manifest
```

The installed binary is replaced only with a successfully built candidate. Unix/macOS filesystem-entry rename is atomic. Windows in-place replacement remains unsupported.

Generated composition files/manifests use unique temporary files. Do not use fixed temp names that collide across processes.

When build info is available, generated `go.mod` should pin the same `github.com/xd-dash/smoke` module version that produced the running binary; `SMOKE_MODULE_VERSION` is the explicit override. Existing selected versions for optional dependencies should not be discarded on every rebuild.

## Durable Logma boundary

`xd-dash/logma` remains the durable Fatline service/resource graph. Logmash remains intentionally ephemeral: receive/route, source subscriptions, stdout/Axiom/webhooks, and attached or unattended local lifetime.

Do not make Logmash secretly depend on the durable Logma HTTP control plane merely because Redis is hosted by Fatline. Do not weaken durable Logma resources into process-local Smoke state.

## Change protocol

When modifying Smoke/Logmash:

1. Preserve the smallest composition primitive that satisfies the requirement.
2. Prefer import-time composition over runtime discovery.
3. Prefer context cancellation and ownership over shared mutable state.
4. Keep process-global cwd/environment mutation out of reusable execution paths.
5. Serialize shared on-disk transitions across processes, not merely goroutines.
6. Prefer immutable runtime snapshots over long-lived canonical-state locks.
7. Preserve the composition/workspace identity pair through environment and unattended-runtime boundaries.
8. Keep stdout default and attached unless explicitly removed.
9. Keep unattended supervision limited to start/list/stop and preserve lease-backed process identity.
10. Keep DNS discovery free of credentials and runtime dataset/channel state.
11. Add focused tests for parser, lifecycle, resolver, provider, environment, workspace, identity, session, callback, registry, or rebuild invariants touched by the change.
12. Run `go vet ./...` and `go test -race ./...` on the exact final candidate; then require normal `main` CI after merge.
13. Update focused docs for user-visible behavior and this file for architectural invariant changes.

Before adding a daemon, IPC channel, output persistence layer, runtime plugin mechanism, control-plane state, or custom environment dependency graph, first verify that the requirement cannot be expressed through existing Go composition, `go.work`/`go.mod`, immutable snapshots, runtime identity inspection, attached/unattended lifetime, typed providers, callback fan-out, or session start/list/stop primitives.
