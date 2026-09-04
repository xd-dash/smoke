# smoke

`smoke` is the bootstrap/resolution front-end for registered commands. Once a command is resolved, Smoke gets out of the way.

The first registered command is `logmash`.

```sh
smoke logmash west events
```

and, once `logmash` is installed/resolved:

```sh
logmash west events
```

have the same Logmash semantics. On Unix, Smoke replaces itself with the resolved command using `exec`, so terminal ownership and signal behavior are the same as invoking Logmash directly.

Smoke does not know Redis, Axiom, Cloudflare, Fatline, or Logmash DNS grammar. Those are owned below the registered command boundary.

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

The explicit environment registration makes it possible to select between multiple Logmash builds/wrappers on one machine without adding Logmash-specific selection logic to Smoke.

If Logmash is not already present, Smoke installs the registered Go command into `~/.local/bin` (or `SMOKE_BIN_DIR`). When Smoke itself has module build-version identity, the installer uses the same module version for Logmash so the bootstrapped command matches the registering Smoke build.

The intended bootstrap chain is:

```text
install.xd.run/smoke
        ↓
install/verify smoke
        ↓
smoke logmash ...
        ↓
resolve or install logmash
        ↓
exec logmash ...
```

After that first resolution, users may call `logmash` directly.

## Logmash

`logmash` owns DNS/provider resolution and Pub/Sub session behavior.

Basic use:

```sh
logmash west events
logmash west events deployments
logmash west events --pattern 'worker:*'
```

A shorthand Redis profile such as `west` resolves as `west.logma.sh`.

```text
west.logma.sh
    -> TXT/SRV
    -> Redis-compatible host + port + TLS + auth metadata
    -> SUBSCRIBE / PSUBSCRIBE
```

The DNS may be managed in Cloudflare, but Cloudflare is a Logmash deployment/resolution concern. Smoke has no Cloudflare provider or Cloudflare API dependency.

## Human-readable routing grammar

Managed callback destinations use `--into` rather than forcing users to spell URLs and query strings.

```sh
logmash west events \
  --into axiom east mydataset \
  --into axiom eu mydataset
```

The same command through Smoke is identical after command resolution:

```sh
smoke logmash west events \
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

An explicit FQDN may be used in the profile position when needed.

Internally, for example:

```text
--into axiom eu mydataset
        ↓
axiom://mydataset?profile=axiom-eu-central-1.logma.sh
        ↓
DNS profile resolves deployment domain + non-secret auth selector
        ↓
POST https://eu-central-1.aws.edge.axiom.co/v1/ingest/mydataset
```

Dataset names remain runtime input. They are never stored in DNS or Terraform state.

The managed Axiom TXT shape is:

```text
axiom.logma.sh TXT "smoke=v1;provider=axiom;domain=eu-central-1.aws.edge.axiom.co;auth=axiom-default"
```

The optional `auth=` field is a non-secret token-profile selector. `auth=axiom-default` resolves to an environment variable such as:

```text
LOGMASH_AXIOM_AXIOM_DEFAULT_TOKEN
```

Token values never belong in DNS.

The older `--callback URL` form remains as a compatibility/ad-hoc escape hatch, but managed destinations should prefer `--into`.

## Foreground and detached lifecycle

Detachment is explicit.

Foreground:

```sh
logmash west events --into axiom eu mydataset
```

This remains attached to the caller's shell/terminal. Ctrl+C terminates the subscription runtime. On Unix, normal terminal/session teardown also terminates the foreground process through normal process/session signal semantics. No callback choice implicitly daemonizes the process.

Detached:

```sh
logmash west events \
  --detached \
  --into axiom east mydataset \
  --into axiom eu mydataset
```

Only `--detached` creates the separate supervised process/session. Detached mode does not attach the default stdout callback and requires at least one destination.

The legacy `--no-stdout` option is retained temporarily as a compatibility alias for `--detached`.

## Session supervision

Logmash keeps lightweight local process-bound supervision state, not durable Logma graph state.

```sh
logmash list
logmash stop <session-id>
```

A session record contains the PID, Redis profile, target summary, channels/patterns, auth provider, and sanitized callback descriptors. Stale PID records are cleaned during listing.

Example display:

```text
4c1d8eaa9321 pid=4127 profile=west.logma.sh target=fatline.example:6380 tls=true
  channels: events
  callbacks: axiom:mydataset@axiom-eu-central-1.logma.sh
  auth: acl-env
```

This is intentionally lighter than `xd-dash/logma`, which remains the durable Channel/Subscriber/Callback/Publisher resource/control-plane implementation.

## Redis auth providers

Redis transport and authentication remain separate composition boundaries.

```text
none
password-env
acl-env
auto-env
```

A Fatline-backed Logmash endpoint should normally use a scoped observation principal that can subscribe to the intended channel family without publish/key/admin authority.

## Package boundaries

The intended ownership is:

```text
smoke
  registered-command resolution
  install/bootstrap
  exec

logmash
  provider grammar
  managed DNS resolution
  Redis subscription lifecycle
  callback composition
  local supervision

xd-dash/logma
  durable resource graph
  retained subscriber/publisher/channel state
  activation/reconciliation
```

The lower-level Go packages remain importable for programs that want typed Redis targets or callback implementations, but the `smoke` CLI itself is no longer a raw provider-URL runtime.
