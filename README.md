# smoke

`smoke` is a composable URL-provider runtime. URL schemes select providers; providers produce behavior; callbacks consume provider messages.

The first provider is Redis Pub/Sub. Subscriptions are intentionally ephemeral: nothing is stored in Redis or in a smoke registry. Process lifetime is subscription lifetime.

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

TLS and Unix sockets use the same provider:

```sh
smoke 'rediss://user:password@redis.example.com:6380?channel=events&callback=stdout'
smoke 'redis+unix:///run/redis/redis.sock?channel=events&callback=stdout'
```

## Logmash

`cmd/logmash` is the human-facing wrapper for DNS-resolved ephemeral receiving and routing:

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
    -> host + port + TLS + auth profile
    -> Redis Target
    -> SUBSCRIBE / PSUBSCRIBE
```

Farcaster host naming and Fatline service naming belong to deployment/DNS configuration, not to the Redis provider.

Logmash is also deliberately lighter than durable Logma running inside Fatline. Logmash receives and routes messages directly over provider transport and does not require an HTTP control plane or create durable Channel/Subscriber/Publisher graph state.

## Go composition

Providers are ordinary imports. Another program can build a registry with only the transports it wants:

```go
package main

import (
    "context"
    "log"

    "github.com/xd-dash/smoke"
    "github.com/xd-dash/smoke/callback"
    redisprovider "github.com/xd-dash/smoke/provider/redis"
)

func main() {
    registry, err := smoke.New(redisprovider.New())
    if err != nil {
        panic(err)
    }

    dispatcher := callback.New(callback.Stdout{})
    dispatcher.SetErrorHandler(func(_ context.Context, message callback.Message, err error) {
        log.Printf("provider=%s channel=%s callback error: %v", message.Provider, message.Channel, err)
    })

    err = registry.Run(
        context.Background(),
        "redis://127.0.0.1:6379?channel=events",
        dispatcher,
    )
    if err != nil {
        panic(err)
    }
}
```

Typed Redis targets are the preferred in-process composition boundary when the caller has already resolved connection metadata. Raw URLs remain the portable CLI/interchange form.

`callback.New` defaults to `callback.Continue`. Embedded callers can opt into fail-fast explicitly:

```go
if err := dispatcher.SetFailurePolicy(callback.FailFast); err != nil {
    panic(err)
}
```

Callback parsing is reusable:

```go
dispatcher, err := callback.Parse([]string{
    "stdout",
    "https://example.com/hook",
})
```

## Provider contract

A provider implements:

```go
type Provider interface {
    Schemes() []string
    Run(context.Context, *url.URL, *callback.Dispatcher) error
}
```

The package registry owns only URL-scheme dispatch. Provider-specific connection semantics stay inside provider packages. CLI-specific backgrounding stays in command packages, so importing a provider never unexpectedly daemonizes the caller.

Callback failure policy also stays outside providers. That gives Redis, SSE, and future providers the same callback semantics without duplicating error-handling behavior in each transport.

## Redis callback envelope

Webhook callbacks receive JSON:

```json
{
  "provider": "redis",
  "source": "redis://redis.example.com:6379?channel=events",
  "channel": "events",
  "pattern": "",
  "payload": "hello"
}
```

Credentials and `callback=` destinations are removed from `source`. Stdout intentionally prints only the Redis payload, one message per line.
