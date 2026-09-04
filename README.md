# smoke

`smoke` is a composable URL-provider runtime. URL schemes select providers; providers produce behavior; callbacks consume provider messages.

The first transport provider is Redis Pub/Sub. Subscriptions are intentionally ephemeral: nothing is stored in Redis as durable Logma graph state. Process lifetime is subscription lifetime.

Terminology in this repository and its Huram integration:

- `Logma` means `xd-dash/logma`, including its durable Channel/Subscriber/Callback/Publisher graph and runtime reconciliation.
- `Logmash` / `logma.sh` means the client implemented here in Smoke for direct receiving/routing without requiring the Logma HTTP control plane.

## CLI

Subscribe and print payloads to stdout:

```sh
smoke 'redis://127.0.0.1:6379?channel=events&callback=stdout'
```

Pattern-subscribe:

```sh
smoke 'redis://127.0.0.1:6379?pattern=dev:*&callback=stdout'
```

Webhook only:

```sh
smoke 'redis://127.0.0.1:6379?channel=events&callback=https%3A%2F%2Fexample.com%2Fhook'
```

Because stdout is not a callback, the CLI starts a detached child and immediately returns control to the shell. Background errors are appended to the user cache at `smoke/background.log`.

Stdout and webhook together:

```sh
smoke 'redis://127.0.0.1:6379?channel=events&callback=stdout&callback=https%3A%2F%2Fexample.com%2Fhook'
```

Callback failures are non-fatal by default. A failed callback is reported while the Redis subscription remains active. Use `callback-policy=fail-fast` only when callback failure should terminate the provider session.

Multiple direct and pattern subscriptions can share one Redis Pub/Sub connection:

```sh
smoke 'redis://127.0.0.1:6379?channel=events&channel=deployments&pattern=worker:*&callback=stdout'
```

TLS and Unix sockets use the same transport provider:

```sh
smoke 'rediss://user:password@redis.example.com:6380?channel=events&callback=stdout'
smoke 'redis+unix:///run/redis/redis.sock?channel=events&callback=stdout'
```

## Logmash

`cmd/logmash` is the human-facing wrapper for DNS-resolved receiving and routing:

```sh
logmash west events
logmash west events deployments
logmash west events --pattern 'worker:*'
logmash west events --callback https://example.com/hook
logmash west events --no-stdout --callback https://example.com/hook
```

A shorthand profile such as `west` resolves as `west.logma.sh`. The DNS profile supplies provider connection metadata; channels, patterns and callbacks remain invocation-time inputs.

`logma.sh` is intentionally topology-agnostic. A DNS SRV record may target a Fatline service alias, a standalone Redis host, or another compatible endpoint. Smoke and Logmash do not inspect naming to infer deployment type.

```text
west.logma.sh
    -> TXT/SRV
    -> host + port + TLS + auth-provider + auth-profile
    -> Redis Target
    -> resolve credentials locally
    -> SUBSCRIBE / PSUBSCRIBE
```

Logmash is deliberately lighter than durable Logma running inside Fatline. It does not create durable Channel/Subscriber/Publisher graph resources, but it supervises its own live client processes so running subscriptions can be inspected and explicitly stopped.

## Logmash session supervision

Each running Logmash subscription writes a small local process-bound session record containing its session ID, PID, profile, resolved endpoint summary, channels/patterns, auth provider, and sanitized callback descriptors. This is operational supervision, not durable Logma state.

```sh
logmash list
logmash stop <session-id>
```

Session records live under the user's cache directory and are removed when the process exits. `logmash list` removes stale records whose PID is no longer alive. Webhook userinfo/query strings and callback secret selectors are not persisted as secret values.

## Axiom callback provider

Axiom is a first-class callback provider alongside `stdout` and HTTP(S) webhooks. The important identity split is:

```text
DNS profile
  deployment domain
  optional non-secret auth profile

runtime callback
  dataset id
  optional ingest parameters
```

Dataset names are never DNS state.

A managed Axiom profile looks like:

```text
axiom.logma.sh TXT "smoke=v1;provider=axiom;domain=eu-central-1.aws.edge.axiom.co;auth=axiom-default"
```

There is no dataset in that record. The dataset is selected only when the callback is constructed:

```sh
logmash west events \
  --callback 'axiom://redis-events?profile=axiom.logma.sh'

logmash west events \
  --callback 'axiom://stocks?profile=axiom.logma.sh'

logmash west events \
  --callback 'axiom://brand-new-dataset?profile=axiom.logma.sh'
```

All three callbacks resolve the same `axiom.logma.sh` deployment profile. At runtime they construct, respectively:

```text
https://eu-central-1.aws.edge.axiom.co/v1/ingest/redis-events
https://eu-central-1.aws.edge.axiom.co/v1/ingest/stocks
https://eu-central-1.aws.edge.axiom.co/v1/ingest/brand-new-dataset
```

Creating or choosing a new Axiom dataset therefore requires no DNS or Terraform change.

The optional DNS `auth=` field is a non-secret token-profile selector. For example:

```text
auth=axiom-default
    -> LOGMASH_AXIOM_AXIOM_DEFAULT_TOKEN
```

An explicit callback `token-env=` can override that selector. Without either, the callback falls back to `AXIOM_TOKEN`. Token values never belong in DNS or callback URLs.

Direct `domain=` remains available as an unmanaged/ad-hoc escape hatch:

```sh
logmash west events \
  --callback 'axiom://redis-events?domain=us-east-1.aws.edge.axiom.co'
```

A callback may use `profile=` or `domain=`, not both.

Optional Axiom ingest parameters remain runtime callback inputs:

```text
timestamp-field=
timestamp-format=
event-labels=
```

Each Pub/Sub message is ingested as the normal Smoke callback envelope rather than assuming the Redis payload itself is JSON:

```json
{
  "provider": "redis",
  "source": "west.logma.sh",
  "channel": "events",
  "pattern": "",
  "payload": "hello"
}
```

Callback failure follows the same dispatcher policy as webhooks: report and continue by default, or terminate the subscription with `--callback-policy fail-fast`.

## Redis auth providers

Redis transport and Redis authentication are separate composition boundaries. `provider/redis` owns Pub/Sub and connection options. `provider/redis/auth` owns how credentials are resolved.

Built-in Logmash auth providers:

```text
none
    no Redis AUTH

password-env
    LOGMASH_REDIS_<PROFILE>_PASSWORD
    fallback: REDISCLI_AUTH

acl-env
    LOGMASH_REDIS_<PROFILE>_USERNAME
    LOGMASH_REDIS_<PROFILE>_PASSWORD

auto-env
    ACL username+password when both profile variables exist
    otherwise profile password-only
    otherwise REDISCLI_AUTH password-only
    never silently falls back to no-auth
```

Example:

```sh
export LOGMASH_REDIS_WEST_USERNAME=observer
export LOGMASH_REDIS_WEST_PASSWORD='...'
logmash west events --auth-provider acl-env
```

A Redis provider can also be selected non-secretly by DNS:

```text
west.logma.sh TXT "smoke=v1;provider=redis;auth-provider=acl-env;auth=west;tls=true"
```

The auth provider only supplies credentials. It does not grant capabilities. On Fatline-backed endpoints, Logmash should normally authenticate as a scope-materialized observation principal with subscribe-side authority and without publish/key/admin authority.

## Go composition

Providers are ordinary imports. Another program can build a registry with only the transports it wants:

```go
registry, err := smoke.New(redisprovider.New())
```

Typed Redis targets are the preferred in-process composition boundary when the caller has already resolved connection metadata. Raw URLs remain the portable CLI/interchange form.

Callbacks remain ordinary `callback.Callback` implementations. A caller that has already resolved deployment metadata may construct Axiom directly:

```go
dispatcher := callback.New(
    callback.Stdout{},
    callback.Axiom{
        Dataset: "redis-events",
        Domain:  "us-east-1.aws.edge.axiom.co",
    },
)
```

`callback.New` defaults to `callback.Continue`. Embedded callers can opt into fail-fast explicitly.

## Provider contract

A transport provider implements:

```go
type Provider interface {
    Schemes() []string
    Run(context.Context, *url.URL, *callback.Dispatcher) error
}
```

The package registry owns only URL-scheme dispatch. Provider-specific connection semantics stay inside provider packages. CLI-specific backgrounding and local supervision stay in command/session packages, so importing a provider never unexpectedly daemonizes or registers the caller.
