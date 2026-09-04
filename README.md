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

Multiple direct and pattern subscriptions can share one Redis Pub/Sub connection:

```sh
smoke 'redis://127.0.0.1:6379?channel=events&channel=deployments&pattern=worker:*&callback=stdout'
```

TLS and Unix sockets use the same provider:

```sh
smoke 'rediss://user:password@redis.example.com:6380?channel=events&callback=stdout'
smoke 'redis+unix:///run/redis/redis.sock?channel=events&callback=stdout'
```

Use `db=` for a Redis database number. Do not put webhook credentials in DNS aliases; resolve them from a local/secret configuration layer before constructing the final provider URL.

## Go composition

Providers are ordinary imports. Another program can build a registry with only the transports it wants:

```go
package main

import (
    "context"

    "github.com/xd-dash/smoke"
    "github.com/xd-dash/smoke/callback"
    redisprovider "github.com/xd-dash/smoke/provider/redis"
)

func main() {
    registry, err := smoke.New(
        redisprovider.New(),
    )
    if err != nil {
        panic(err)
    }

    dispatcher := callback.New(callback.Stdout{})

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

Callback parsing is also reusable:

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

The package registry owns only URL-scheme dispatch. Provider-specific connection semantics stay inside provider packages. CLI-specific backgrounding stays in `cmd/smoke`, so importing a provider never unexpectedly daemonizes the caller.

The intended next providers can therefore be added independently, for example HTTP artifact resolution, SSE, Git/forge operations, or other stream transports.

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
