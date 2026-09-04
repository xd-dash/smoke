# smoke

`smoke` is the bootstrap/resolution front-end for registered commands. Once a command is resolved, Smoke gets out of the way.

The first registered command is `logmash`:

```sh
smoke logmash west:events east:events
```

and, once `logmash` is installed/resolved:

```sh
logmash west:events east:events
```

has the same semantics. On Unix, Smoke replaces itself with the resolved command using `exec`, so terminal ownership and signal behavior are the same as invoking Logmash directly.

Smoke does not know Redis, Axiom, Cloudflare, Fatline, or Logmash DNS grammar. Those are owned below the registered-command boundary.

## Command resolution

For a registered command such as `logmash`, Smoke resolves in this order:

```text
SMOKE_COMMAND_LOGMASH=/explicit/path
        ↓
PATH
        ↓
sibling executable next to smoke
        ↓
registered installer
```

If Logmash is not already present, Smoke installs the registered command into `~/.local/bin` (or `SMOKE_BIN_DIR`). When Smoke has module build-version identity, the installer uses the same module version for Logmash.

## Logmash source grammar

A Redis subscription is written as one atomic source relationship:

```text
SOURCE:CHANNEL
```

Examples:

```sh
logmash west:events
logmash west:events west:ratelimiters
logmash west:events west:ratelimiters east:events
```

The source is a Logmash DNS profile. `west` resolves as `west.logma.sh`; `east` resolves as `east.logma.sh`.

One invocation can therefore fan in from several independently resolved Redis-compatible sources:

```sh
logmash \
  west:events \
  west:ratelimiters \
  east:events
```

Selectors are grouped by source before connecting:

```text
west:events
west:ratelimiters
        ↓
one west.logma.sh Redis Pub/Sub connection
  SUBSCRIBE events ratelimiters

east:events
        ↓
one east.logma.sh Redis Pub/Sub connection
  SUBSCRIBE events
```

Exact duplicate selectors are deduplicated within the source group.

The old ambiguous grammar:

```text
logmash west events
```

is intentionally rejected. Source/channel association should always be explicit.

Pattern subscriptions use the same source relationship:

```sh
logmash west:events --pattern 'east:worker:*'
```

which means direct `west:events` plus `PSUBSCRIBE worker:*` on `east.logma.sh`.

## Source provenance

Every incoming callback message carries the logical source profile independently of the physical Redis endpoint:

```json
{
  "provider": "redis",
  "source": "west.logma.sh",
  "channel": "events",
  "pattern": "",
  "payload": "hello"
}
```

This lets DNS move `west.logma.sh` between Fatlines/hosts without changing event identity.

Foreground stdout also preserves that relationship:

```text
west:events    hello
west:ratelimiters    allowed
 east:events    deployed
```

(the separator between selector and payload is a tab).

## Human-readable routing grammar

Managed callback destinations use `--into` rather than requiring URLs:

```sh
logmash \
  west:events \
  west:ratelimiters \
  east:events \
  --into axiom east mydataset \
  --into axiom eu mydataset
```

The same command through Smoke is equivalent after resolution:

```sh
smoke logmash \
  west:events \
  west:ratelimiters \
  east:events \
  --into axiom east mydataset \
  --into axiom eu mydataset
```

Current Axiom aliases are:

```text
axiom east <dataset>
    -> axiom-us-east-1.logma.sh

axiom eu <dataset>
    -> axiom-eu-central-1.logma.sh

axiom default <dataset>
    -> axiom.logma.sh
```

Dataset names remain runtime input and never enter DNS/Terraform state. Axiom receives the normal callback envelope, including `source`, so one dataset can safely ingest events from multiple Logmash sources while retaining provenance.

The older `--callback URL` form remains as an ad-hoc compatibility escape hatch.

## Foreground and detached lifecycle

Foreground:

```sh
logmash west:events east:events --into axiom eu mydataset
```

remains attached to the caller's shell/terminal. Ctrl+C cancels all source subscriptions and callback work in the invocation.

Detached:

```sh
logmash \
  west:events \
  east:events \
  --detached \
  --into axiom eu mydataset
```

creates one supervised Logmash process containing all of those source subscriptions. If one source fails unexpectedly, the invocation cancels its sibling source subscriptions instead of silently leaving a partial fan-in running.

## Session supervision

Logmash keeps lightweight local process-bound supervision state, not durable Logma graph state:

```sh
logmash list
logmash stop <session-id>
```

A multi-source session records source-qualified channel/pattern selectors, its callback descriptors, and the source profiles participating in that process. Example shape:

```text
4c1d8eaa9321 pid=4127 profiles=east.logma.sh,west.logma.sh sources=2
  channels: east:events, west:events, west:ratelimiters
  callbacks: axiom:mydataset@axiom-eu-central-1.logma.sh
  auth: acl-env
```

This remains intentionally lighter than `xd-dash/logma`, which owns durable Channel/Subscriber/Callback/Publisher resources.

## Redis auth providers

Redis transport and authentication remain separate composition boundaries:

```text
none
password-env
acl-env
auto-env
```

Auth is resolved independently for each source profile, so `west.logma.sh` and `east.logma.sh` may have different DNS auth profiles while participating in one Logmash process. A CLI `--auth-provider` override applies to all source profiles in that invocation.

## Package boundaries

```text
smoke
  registered-command resolution
  install/bootstrap
  exec

logmash
  SOURCE:CHANNEL grammar
  multi-source fan-in
  provider DNS resolution
  Redis subscription lifecycle
  callback composition
  local supervision

xd-dash/logma
  durable resource graph
  retained subscriber/publisher/channel state
  activation/reconciliation
```
