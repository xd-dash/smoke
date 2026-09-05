# Smoke / Logmash idioms

This file is the maintenance contract for `xd-dash/smoke`. The README explains how to use Smoke; this file records the design invariants that future changes should preserve unless the design is intentionally revised.

## Core identity

Smoke is a self-composed Go executable, not a runtime plugin host.

- Optional commands/providers are ordinary Go packages.
- A local composition imports the selected packages.
- Imported command packages register through `command.Register` during process initialization.
- Normal command dispatch is an in-process Go function call.
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

The callback set is fixed after startup. Configure the dispatcher before provider goroutines begin; do not mutate its failure policy or error handler concurrently with dispatch.

Every provider message is fanned out to all configured callbacks for that invocation. The current dispatcher waits for all callbacks for one message before receiving the next message from that source. This is deliberate backpressure, not a delivery queue.

Therefore:

- a slow webhook/Axiom callback can slow that source;
- `continue` reports callback failures and keeps the subscription alive;
- `fail-fast` returns the callback error and ends that source/runtime;
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

## Redis provider invariants

`redisprovider.Target` and `redisprovider.Subscription` are the typed in-process boundary.

- validate target, callbacks, and selectors before entering the receive loop;
- PING before subscribing so connection/auth failures surface early;
- exact channels use `SUBSCRIBE`;
- patterns use `PSUBSCRIBE`;
- cancellation must close Pub/Sub/client resources;
- `Target.Source` is provenance and must remain credential-free.

Observation credentials should be least-privilege: subscribe/read-only operations needed for observation, never publish/key mutation/admin authority.

## Session registry

The session registry is lightweight local supervision metadata for unattended Logmash processes. It is not durable Logma state.

Current records contain a random session ID, PID, source/callback summaries, auth-provider name, and start timestamp. Callback metadata is sanitized before persistence.

`list` removes records whose PID is no longer alive. `stop` resolves the session and signals the PID.

Known limitation: PID liveness is best-effort. A stale record surviving an abnormal process exit could theoretically coincide with PID reuse. Do not treat the current JSON registry as a security boundary or durable process-identity oracle. If stronger identity becomes necessary, add a process-start token/OS identity check explicitly rather than expanding the registry into a daemon.

## Self-composition/rebuild invariants

Composition state is a sorted, deduplicated list of Go import paths.

Apply order:

```text
normalize desired imports
        ↓
generate local main.go + go.mod
        ↓
go mod tidy
        ↓
go build staged candidate
        ↓
build succeeded?
   no → leave installed binary + manifest unchanged
   yes
        ↓
snapshot previous manifest
        ↓
save desired manifest
        ↓
atomic rename candidate over installed Smoke
        ↓
rename failed? restore previous manifest
```

The installed binary is replaced only with a successfully built candidate. On Unix/macOS the executable rename is atomic at the filesystem entry level. Windows in-place replacement remains unsupported.

Do not run multiple `smoke compose add/remove/rebuild` operations concurrently against the same composition directory. The generated source directory and manifest are intentionally a single-writer local composition workspace. If concurrent composition becomes a real requirement, add an explicit cross-process lock rather than relying on timing.

When build info is available, generated `go.mod` should pin the same `github.com/xd-dash/smoke` module version that produced the running binary. `SMOKE_MODULE_VERSION` is the explicit override.

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
4. Keep stdout default and attached unless the caller explicitly removes stdout.
5. Keep unattended supervision limited to start/list/stop.
6. Keep DNS discovery free of credentials and runtime dataset/channel state.
7. Add focused tests for parser, lifecycle, resolver, provider, or rebuild invariants touched by the change.
8. Run `go test ./...` on the exact final `main` head.
9. Update `README.md` for user-visible behavior changes.
10. Update this idiom file when an architectural invariant changes, not for incidental implementation details.

Before adding a new daemon, IPC channel, output persistence layer, runtime plugin mechanism, or control-plane state, first verify that the requirement cannot be expressed through the existing Go composition, attached/unattended lifetime, typed provider boundary, callback fan-out, or session start/list/stop primitives.
