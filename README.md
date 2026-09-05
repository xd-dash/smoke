# smoke

`smoke` is the bootstrap/resolution front-end for registered commands. Once a command is resolved, Smoke gets out of the way.

The first registered command is `logmash`:

```sh
smoke logmash us:west:events us:east:events
```

and, once `logmash` is installed/resolved:

```sh
logmash us:west:events us:east:events
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

A Redis subscription is one atomic geographic source relationship:

```text
COUNTRY:REGION:CHANNEL
```

The country is a two-letter country code. Country and region are normalized to lowercase.

Examples:

```sh
logmash us:west:events
logmash us:west:events us:west:ratelimiters
logmash us:west:events us:west:ratelimiters us:east:events
```

The human source `us:west` resolves through the DNS hierarchy:

```text
us:west
    ↓
west.us.logma.sh
```

Likewise:

```text
us:east
    ↓
east.us.logma.sh
```

The CLI is broad-to-narrow (`country:region:channel`) while DNS is naturally hierarchical (`region.country.logma.sh`). The DNS encoding is an implementation detail: callback provenance remains the logical source `us:west` or `us:east`.

One invocation can fan in from several independently resolved Redis-compatible sources:

```sh
logmash \
  us:west:events \
  us:west:ratelimiters \
  us:east:events
```

Selectors are grouped by `country:region` before connecting:

```text
us:west:events
us:west:ratelimiters
        ↓
one west.us.logma.sh Redis Pub/Sub connection
  SUBSCRIBE events ratelimiters

us:east:events
        ↓
one east.us.logma.sh Redis Pub/Sub connection
  SUBSCRIBE events
```

Exact duplicate selectors are deduplicated within each source group.

Older ambiguous forms such as:

```text
logmash west events
logmash west:events
```

are intentionally rejected. Country, region, and channel association must be explicit.

Pattern subscriptions use the same hierarchy:

```sh
logmash us:west:events --pattern 'us:east:worker:*'
```

which means direct `us:west:events` plus `PSUBSCRIBE worker:*` on `east.us.logma.sh`.

## Source provenance

Every incoming callback message carries the logical geographic source independently of the physical Redis endpoint:

```json
{
  "provider": "redis",
  "source": "us:west",
  "channel": "events",
  "pattern": "",
  "payload": "hello"
}
```

DNS can therefore move `west.us.logma.sh` between Fatlines/hosts without changing event identity.

Foreground stdout preserves the full relationship:

```text
us:west:events	hello
us:west:ratelimiters	allowed
us:east:events	deployed
```

## Human-readable routing grammar

Managed callback destinations use `--into` rather than requiring URLs:

```sh
logmash \
  us:west:events \
  us:west:ratelimiters \
  us:east:events \
  --into axiom east mydataset \
  --into axiom eu mydataset
```

The same command through Smoke is equivalent after resolution:

```sh
smoke logmash \
  us:west:events \
  us:west:ratelimiters \
  us:east:events \
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

Dataset names remain runtime input and never enter DNS/Terraform state. Axiom receives the normal callback envelope, including the logical `country:region` source.

## Foreground and detached lifecycle

Foreground:

```sh
logmash us:west:events us:east:events --into axiom eu mydataset
```

remains attached to the caller's shell/terminal. Ctrl+C cancels every source subscription and callback in the invocation.

Detached:

```sh
logmash \
  us:west:events \
  us:east:events \
  --detached \
  --into axiom eu mydataset
```

creates one supervised Logmash process containing all source subscriptions. If one source fails unexpectedly, sibling source subscriptions are cancelled instead of leaving a partial fan-in running.

## Session supervision

Logmash keeps lightweight local process-bound supervision state:

```sh
logmash list
logmash stop <session-id>
```

Example shape:

```text
4c1d8eaa9321 pid=4127 profiles=east.us.logma.sh,west.us.logma.sh sources=2
  channels: us:east:events, us:west:events, us:west:ratelimiters
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

Auth is resolved independently for each geographic source profile. If DNS does not specify an auth profile, `us:west` falls back to the local auth profile name `us-west`.

## Package boundaries

```text
smoke
  registered-command resolution
  install/bootstrap
  exec

logmash
  COUNTRY:REGION:CHANNEL grammar
  geographic multi-source fan-in
  provider DNS resolution
  Redis subscription lifecycle
  callback composition
  local supervision

xd-dash/logma
  durable resource graph
  retained subscriber/publisher/channel state
  activation/reconciliation
```
