# Smoke / Logmash idioms

This file is the maintenance contract for `xd-dash/smoke`. The README explains how to use Smoke; this file records the design invariants that future changes should preserve unless the design is intentionally revised.

## Core identity

Smoke is a self-composed Go executable, not a runtime plugin host.

- Optional commands/providers are ordinary Go packages.
- A local composition imports the selected packages.
- Imported command packages register through `command.Register` during process initialization.
- Normal command dispatch is an in-process Go function call.
- `commands`, `compose`, and `env` are reserved core command names and optional packages must not register them.
- Compiled-in command names are single non-whitespace tokens. Invalid/duplicate/reserved registrations are programmer errors and should fail the composition during initialization.
- Do not reintroduce PATH-based command discovery, separately installed Logmash executables, dynamic `.so` loading, or a resident plugin daemon unless a new requirement specifically needs them.

The default composition imports:

```text
github.com/xd-dash/smoke/cmd/logmash
```

The system Go toolchain is the composition/rebuild primitive.

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

Core packages must not import every optional provider merely to make them discoverable. The composition root is the authority for what is linked.

## Smoke environment/workspace invariants

A Smoke environment is a named Go workspace. It is first-class Smoke tooling, not a runtime provider.

The canonical Go-side representation is:

```text
<env>/
├── go.work
└── tools/
    ├── go.mod
    └── go.sum
```

Rules:

- `go.work` is the environment/workspace manifest.
- `tools/go.mod` exists to carry environment-scoped Go `tool` directives and their ordinary module requirements.
- Do not mirror Go tool dependencies into a Smoke JSON/YAML dependency graph.
- Use normal Go commands to mutate workspace/tool state so Go remains authoritative for version selection, sums, replacements, exclusions, and tool resolution.
- Environment activation is scoped to child processes with `GOWORK` plus `SMOKE_ENV`; never mutate process-global `GOWORK` or cwd to implement `env run`.
- `smoke env shell` creates a child shell; `smoke env exec` creates a child process.
- `smoke env run` re-execs Smoke itself under the selected environment and then uses the normal compiled-in command registry in that child. This process boundary prevents parent-process environment/cwd races without introducing PATH discovery or a separate Logmash binary.
- `smoke env build` is a thin `go build` invocation under the selected workspace. Smoke must not invent a second build graph over `go.work`.
- Workspace `use` targets are local Go module directories containing `go.mod`.
- The environment tools module is always part of the workspace so its declared tools are visible to `go tool` in workspace mode.
- Environment mutation (`create`, `use`, `drop`, tool add/remove) takes an exclusive cross-process environment lock. Commands that consume the environment take a shared lock, preventing partially observed `go.work`/tools-module edits.
- The current shared-lock model intentionally means a long-lived `env shell`, `env exec`, `env build`, or `env run` can delay environment mutation. Do not remove those locks merely for convenience. If long-lived live environments must remain mutable, first introduce a session-specific workspace/tools snapshot and then relax the lifetime lock against that immutable snapshot.

Composition and environment are separate axes:

```text
smoke compose
    = which optional packages are linked into the Smoke binary

smoke env
    = which local modules/tools are active while that binary is used
```

An environment cannot make an optional Smoke command available unless that command is already present in the current binary's import-time composition.

Logmash follows the same rule. `smoke env run <env> -- logmash ...` must execute Smoke itself and dispatch the already-compiled Logmash handler in the child. It must not resolve, install, or launch a separate `logmash` executable. Unattended Logmash children inherit `GOWORK`/`SMOKE_ENV`.

A running Smoke process and the installed Smoke filesystem entry are distinct after an atomic recomposition: the running process keeps its old image while the path may now name a newer composition. Therefore a re-exec concurrent with composition replacement may legitimately start the newly installed composition. Do not claim exact parent-image identity unless an immutable executable snapshot or OS-specific self-exec primitive is added.

## Logmash source grammar

A source selector is always:

```text
COUNTRY:REGION:CHANNEL
```

Pattern selectors use the same source qualification:

```text
COUNTRY:REGION:PATTERN
```

Examples:

```text
us:west:events
us:east:events
us:east:worker:*
```

The CLI identity is broad-to-narrow (`country:region`). Managed DNS is hierarchical (`region.country.logma.sh`). Callback provenance uses the logical source such as `us:west`, not the DNS spelling or physical Redis host.

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
    → stop later with `smoke logmash stop <id>`
```

"Attached" does not mean interactive. Logmash does not require stdin. It means the process lifetime remains owned by the invoking shell and stdout is visible.

`--no-stdout` is the normal transition to an unattended callback runtime. `--attached` is only the explicit override for a no-stdout runtime that should still remain shell-owned for debugging/testing.

Do not add a second public lifecycle axis such as `--detached`, stdout reattachment, `logmash detach`, `SIGUSR1`, output sockets, `tail -f`, `nohup`, persisted stdout logs, or a resident Smoke daemon unless a real requirement cannot be expressed by attached/unattended ownership.

Only unattended runtimes are written to the local session registry. Attached runs are not sessions.

## Runtime ownership and cancellation

Logmash owns subscriptions and callbacks. Smoke only owns composition plus the small unattended start/list/stop boundary.

For a Logmash invocation:

- resolve all source groups before entering the receive loop;
- create one goroutine per source subscription;
- use one child context shared by those source goroutines;
- the first unexpected source error cancels sibling sources;
- normal parent cancellation is not promoted to a provider error;
- wait for every source goroutine before returning.

Do not introduce shared mutable source state when channel ownership/context cancellation is sufficient.

## Callback semantics

The callback set is fixed after startup. Dispatcher policy and error-handler configuration are synchronized so concurrent configuration is race-safe, but configure-before-run remains the normal lifecycle.

Every provider message is fanned out to all configured callbacks for that invocation. The current dispatcher waits for all callbacks for one message before receiving the next message from that source. This is deliberate backpressure, not a delivery queue.

Therefore:

- a slow webhook/Axiom callback can slow that source;
- `continue` reports callback failures and keeps the subscription alive;
- `fail-fast` returns the callback error and ends that source/runtime;
- nil callback entries must fail as callback errors rather than panic the dispatcher;
- if independent buffering/retry is ever needed, add it as an explicit bounded delivery primitive rather than silently spawning unbounded goroutines.

HTTP callback response bodies must be drained and closed so the shared transport can reuse connections.

## DNS discovery invariants

DNS is provider discovery, not runtime state.

Redis discovery:

```text
west.us.logma.sh TXT
  smoke=v1;provider=redis;...

_rediss._tcp.west.us.logma.sh SRV
  ...
```

Rules:

- credentials never live in DNS;
- channels/patterns never live in DNS;
- Axiom dataset IDs never live in DNS/Terraform state;
- auth provider/profile names may be non-secret DNS metadata;
- multiple unrelated TXT records may coexist at a name;
- resolver code must select a valid `smoke=v1;provider=<provider>` record rather than assuming the first Smoke TXT record belongs to that provider.

Service identity must not be inferred from arbitrary hostname shape when typed metadata exists.

## Provider registry invariants

Provider schemes are a typed runtime dispatch boundary, not plugin discovery.

- nil providers are invalid;
- schemes are normalized to lowercase and must satisfy normal URL-scheme syntax;
- two providers may not claim the same normalized scheme;
- a nil registry must return an error rather than panic;
- providers are supplied by Go composition/callers, never discovered from PATH or runtime directories.

## Redis provider invariants

`redisprovider.Target` and `redisprovider.Subscription` are the typed in-process boundary.

- validate target, callbacks, and selectors before entering the receive loop;
- PING before subscribing so connection/auth failures surface early;
- exact-channel-only subscriptions initialize with `SUBSCRIBE`;
- pattern-only subscriptions initialize with `PSUBSCRIBE` directly;
- mixed subscriptions initialize exact channels and then add patterns explicitly;
- cancellation must close Pub/Sub/client resources;
- `Target.Source` is provenance and must remain credential-free.

Observation credentials should be least-privilege: subscribe/read-only operations needed for observation, never publish/key mutation/admin authority.

## Session registry

The session registry is lightweight local supervision metadata for unattended Logmash processes. It is not durable Logma state.

Current records contain a random session ID, PID, source/callback summaries, auth-provider name, and start timestamp. Callback metadata is sanitized before persistence.

Each live unattended process also owns an exclusive OS-backed lease file for its session ID. The lease, not PID existence alone, is the liveness authority. `list` and `stop` require both a held lease and a live PID; stale JSON/lease files are cleaned when ownership is gone. This prevents an old session record from treating a later reused PID as sufficient proof of ownership.

Do not weaken session identity back to PID-only checks. Do not turn the lease/JSON registry into durable Logma graph state or a resident supervision daemon.

## Self-composition/rebuild invariants

Composition state is a sorted, deduplicated list of Go import paths.

All composition mutation is a cross-process transaction. `add`/`remove` perform the manifest read, transformation, dependency resolution, candidate build, manifest commit, and executable replacement under one exclusive composition lock. `rebuild` uses the same lock. This prevents lost updates from concurrent CLI invocations.

Recomposition is isolated from Smoke environments: every Go command used for composition runs with `GOWORK=off`. An active `smoke env` must never alter dependency selection for the Smoke executable itself.

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
build succeeded?
   no → restore generated source/module snapshot
        leave installed binary + manifest unchanged
   yes
        ↓
snapshot previous manifest
        ↓
save desired manifest atomically
        ↓
atomic rename candidate over installed Smoke
        ↓
rename failed? restore previous manifest
```

The installed binary is replaced only with a successfully built candidate. On Unix/macOS the executable rename is atomic at the filesystem entry level. Windows in-place replacement remains unsupported.

Generated composition files and manifests use unique temporary files for atomic replacement/rollback; do not use fixed temp names that can collide across processes.

When build info is available, generated `go.mod` should pin the same `github.com/xd-dash/smoke` module version that produced the running binary. `SMOKE_MODULE_VERSION` is the explicit override. Existing selected versions for optional composition dependencies should not be discarded on every rebuild; normal Go module operations own their evolution.

## Durable Logma boundary

`xd-dash/logma` remains the durable Fatline service/resource graph.

Logmash is intentionally ephemeral:

```text
receive + route
source subscriptions
stdout / Axiom / webhooks
attached or unattended local lifetime
```

Do not make Logmash secretly depend on the durable Logma HTTP control plane merely because the Redis endpoint is hosted by Fatline. Do not weaken durable Logma resources into process-local Smoke state.

## Change protocol

When modifying Smoke/Logmash:

1. Preserve the smallest composition primitive that satisfies the requirement.
2. Prefer import-time composition over runtime discovery.
3. Prefer context cancellation and ownership over shared mutable state.
4. Keep process-global cwd/environment mutation out of reusable execution paths.
5. Serialize shared on-disk state transitions across processes, not merely goroutines.
6. Keep stdout default and attached unless the caller explicitly removes stdout.
7. Keep unattended supervision limited to start/list/stop and preserve lease-backed process identity.
8. Keep DNS discovery free of credentials and runtime dataset/channel state.
9. Add focused tests for parser, lifecycle, resolver, provider, environment, workspace, session, callback, registry, or rebuild invariants touched by the change.
10. Run `go vet ./...` and `go test -race ./...` on the exact final candidate; then require the normal `main` CI run after merge.
11. Update `README.md` or focused docs for user-visible behavior changes.
12. Update this idiom file when an architectural invariant changes, not for incidental implementation details.

Before adding a new daemon, IPC channel, output persistence layer, runtime plugin mechanism, control-plane state, or custom environment dependency graph, first verify that the requirement cannot be expressed through the existing Go composition, `go.work`/`go.mod` workspace machinery, attached/unattended lifetime, typed provider boundary, callback fan-out, or session start/list/stop primitives.
