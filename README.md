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

The stdout callback keeps the process attached to the foreground. Each message is fanned out to both callbacks.

Callback failures are non-fatal by default. A failed webhook is reported, but the Redis subscription remains active and continues handling later messages:

```text
callback failure
    -> report error
    -> keep Redis subscription alive
```

To terminate the provider session when any callback fails, opt into fail-fast behavior:

```sh
smoke 'redis://127.0.0.1:6379?channel=events&callback=https%3A%2F%2Fexample.com%2Fhook&callback-policy=fail-fast'
```

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

`logma.sh` is intentionally topology-agnostic. A DNS SRV record may currently target a Fatline service alias such as `fatline.west.farcaster.world`, a standalone Redis host, or another compatible endpoint. Smoke and Logmash do not inspect that naming and do not need to know what deployment implements the service.

In particular, there is no `IsFatline` or `IsFarcaster` provider behavior:

```text
west.logma.sh
    -> TXT/SRV
    -> host + port + TLS + auth-provider + auth-profile
    -> Redis Target
    -> resolve credentials locally
    -> SUBSCRIBE / PSUBSCRIBE
```

Farcaster host naming and Fatline service naming belong to deployment/DNS configuration, not to the Redis provider.

Logmash is deliberately lighter than durable Logma running inside Fatline. It does not create durable Channel/Subscriber/Publisher graph resources, but it does supervise its own live client processes so running subscriptions can be inspected and explicitly stopped.

## Logmash session supervision

Each running Logmash subscription writes a small local process-bound session record containing its session ID, PID, profile, resolved endpoint summary, channels/patterns, auth provider, and sanitized callback descriptors. This is operational supervision, not durable Logma state.

List active sessions:

```sh
logmash list
```

Example output:

```text
4c1d8eaa9321 pid=4127 profile=west.logma.sh target=rediss://fatline.west.farcaster.world:6380
  channels: events, deployments
  callbacks: stdout, axiom:events@us-east-1.aws.edge.axiom.co
  auth: acl-env
```

Stop one exact subscription process and therefore all callbacks attached to it:

```sh
logmash stop 4c1d8eaa9321
```

Session records live under the user's cache directory and are removed when the process exits. `logmash list` removes stale records whose PID is no longer alive. Webhook userinfo/query strings are removed before callback descriptors are persisted.

This gives Logmash a useful subset of Logma-like operational control without introducing Redis-backed durable resource graphs or an HTTP control plane.

## Axiom callback provider

Axiom is a first-class callback URL provider alongside `stdout` and HTTP(S) webhooks. The callback URL describes non-secret ingest configuration:

```text
axiom://<dataset-id>?domain=<edge-domain>
```

For example:

```sh
export AXIOM_TOKEN='...'

logmash west events \
  --callback 'axiom://redis-events?domain=us-east-1.aws.edge.axiom.co'
```

For EU Central 1:

```sh
logmash west events \
  --callback 'axiom://redis-events?domain=eu-central-1.aws.edge.axiom.co'
```

The callback constructs:

```text
POST https://<edge-domain>/v1/ingest/<dataset-id>
Authorization: Bearer <API token>
Content-Type: application/json
```

The token defaults to environment variable `AXIOM_TOKEN`. A different environment variable can be selected without putting the secret in the URL:

```sh
logmash west events \
  --callback 'axiom://redis-events?domain=us-east-1.aws.edge.axiom.co&token-env=WEST_AXIOM_TOKEN'
```

Optional Axiom ingest parameters are represented in the callback URL:

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

This makes arbitrary Redis payloads queryable while preserving channel/source metadata. Callback failure follows the same dispatcher policy as webhooks: report and continue by default, or terminate the subscription with `--callback-policy fail-fast`.

## Redis auth providers

Redis transport and Redis authentication are separate composition boundaries. `provider/redis` owns Pub/Sub and connection options. `provider/redis/auth` owns how credentials are resolved.

Built-in Logmash auth providers:

```text
none
    no Redis AUTH
    intended only for explicitly unauthenticated/local deployments

password-env
    password-only/default-user AUTH
    LOGMASH_REDIS_<PROFILE>_PASSWORD
    fallback: REDISCLI_AUTH

acl-env
    Redis ACL AUTH username password
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

The provider can also be selected non-secretly by the DNS TXT profile:

```text
west.logma.sh TXT "smoke=v1;provider=redis;auth-provider=acl-env;auth=west;tls=true"
```

`--auth-provider` overrides DNS metadata. If neither is specified, Logmash uses `auto-env`.

The auth provider only supplies credentials. It does not grant capabilities. On Fatline-backed endpoints, Logmash should normally authenticate as a scope-materialized observation principal whose Redis ACL permits the exact subscribe-side channel families and required connection commands while denying publish, key mutation, scripting, ACL/CONFIG/MODULE/SHUTDOWN and unrelated channel families.

The existing Huram `REDISCLI_AUTH` convention maps naturally to `password-env`; scoped Redis ACL principals map naturally to `acl-env`. Secrets never belong in `logma.sh` TXT/SRV records.

## Go composition

Providers are ordinary imports. Another program can build a registry with only the transports it wants:

```go
registry, err := smoke.New(redisprovider.New())
```

Typed Redis targets are the preferred in-process composition boundary when the caller has already resolved connection metadata. Raw URLs remain the portable CLI/interchange form.

Auth providers are also ordinary imports:

```go
registry, err := redisauth.New(
    redisauth.None{},
    redisauth.PasswordEnv{},
    redisauth.ACLEnv{},
    redisauth.AutoEnv{},
)
creds, err := registry.Resolve(ctx, "acl-env", "west")
target = creds.Apply(target)
```

Callbacks remain ordinary `callback.Callback` implementations, so Axiom can also be composed directly:

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

Callback failure policy also stays outside providers. That gives Redis, SSE, and future providers the same callback semantics without duplicating error-handling behavior in each transport.
