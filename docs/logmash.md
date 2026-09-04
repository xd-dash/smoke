# logmash

`logmash` is the user-facing wrapper over Smoke's Redis provider. It resolves a `logma.sh` profile through DNS, composes channels/patterns and callbacks locally, and connects directly to the resolved Farcaster/World Redis ingress.

DNS is discovery only. It does not carry topics, callback destinations, or credentials.

Expected records:

```text
west.logma.sh TXT "smoke=v1;provider=redis;auth=west;tls=true"
_rediss._tcp.west.logma.sh SRV 10 10 6380 farcaster.xd.run.
```

The SRV target resolves normally with A/AAAA. When Cloudflare is authoritative DNS, Redis-facing endpoint records are DNS-only; nginx stream on the Farcaster/World is the TCP/TLS ingress.

## CLI

Stdout is the default callback:

```sh
logmash west events
logmash west events deployments
logmash prod.us-west1 prices trades
```

Pattern subscription:

```sh
logmash west --pattern 'worker:*'
logmash west events --pattern 'worker:*'
```

Add webhooks while retaining stdout:

```sh
logmash west events \
  --callback https://example.com/hook \
  --callback https://example.net/hook
```

Webhook-only subscriptions detach and return the shell immediately:

```sh
logmash west events \
  --no-stdout \
  --callback https://example.com/hook
```

Callback failure is non-fatal by default. Use `--callback-policy fail-fast` when callback delivery is itself the required operation.

Shorthand profile names are normalized under `logma.sh`; `west` becomes `west.logma.sh`. Full FQDNs are accepted unchanged.

## Authentication profiles

The TXT record may contain a non-secret selector such as `auth=west`. The current CLI resolves that selector from environment variables:

```text
LOGMASH_REDIS_WEST_USERNAME
LOGMASH_REDIS_WEST_PASSWORD
```

Dots, dashes and colons in the profile name are normalized to underscores for the environment key. This is an initial credential adapter; future SOPS/Marai resolvers can populate the same typed Redis target without changing Pub/Sub semantics.

## Package composition

The provider API is typed:

```go
target, err := (redisprovider.DNSResolver{}).Resolve(ctx, "west.logma.sh")
if err != nil {
    return err
}

sub := redisprovider.Subscription{
    Target:   target,
    Channels: []string{"events", "deployments"},
    Patterns: []string{"worker:*"},
}

return redisprovider.New().RunSubscription(ctx, sub, dispatcher)
```

Raw `redis://`, `rediss://`, and `redis+unix://` Smoke URLs delegate into the same `RunSubscription` implementation. DNS/URLs are interchange forms; `Target` and `Subscription` are the preferred in-process composition boundary.
