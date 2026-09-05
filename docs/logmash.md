# logmash

`logmash` is compiled into Smoke and provides ephemeral multi-source Redis receive/route behavior. It resolves `logma.sh` provider profiles through DNS, composes channels/patterns and callbacks locally, and connects directly to the resolved Redis ingress.

DNS is discovery only. It does not carry topics, callback destinations, or credentials.

A source selector is:

```text
COUNTRY:REGION:CHANNEL
```

For example:

```sh
smoke logmash us:west:events us:east:events
```

`us:west` maps to `west.us.logma.sh`; `us:east` maps to `east.us.logma.sh`.

Pattern subscription uses the same source qualification:

```sh
smoke logmash us:west:events --pattern 'us:east:worker:*'
```

Managed callback destinations compose with stdout:

```sh
smoke logmash \
  us:west:events \
  --into axiom east mydataset \
  --callback https://example.com/hook
```

Callback failure is non-fatal by default. Use `--callback-policy fail-fast` when callback delivery is itself the required operation.

## Attached and unattended lifecycle

Stdout is included by default. When stdout is present, Logmash stays attached to the shell:

```text
Redis ──┬──> stdout
        ├──> Axiom
        └──> webhook

output is visible
shell waits
Ctrl+C stops the composition
```

Attached does not mean interactive. Logmash does not require stdin; it simply writes output while the shell waits for the runtime to finish.

A normal invocation is therefore attached:

```sh
smoke logmash us:west:events --into axiom east mydataset
```

To run only non-terminal callbacks, remove stdout:

```sh
smoke logmash \
  us:west:events \
  --no-stdout \
  --into axiom east mydataset
```

Without stdout, Logmash is unattended by default. Smoke starts another process from the same executable and returns the shell prompt. On Unix that child starts in a new OS session.

```text
Redis ──> Axiom

shell returns
runtime continues
```

Only unattended runtimes are registered for later process control:

```sh
smoke logmash list
smoke logmash stop <session-id>
```

`stop` sends `SIGTERM` on Unix, cancels the root context, and lets Logmash shut down subscriptions and callbacks before process exit.

For debugging, a no-stdout runtime can still be kept attached explicitly:

```sh
smoke logmash \
  us:west:events \
  --no-stdout \
  --attached \
  --into axiom east mydataset
```

The lifecycle primitive is intentionally only:

```text
stdout present
    -> attached
    -> output visible
    -> shell waits
    -> Ctrl+C stops it

stdout absent
    -> unattended
    -> shell returns
    -> callbacks continue
    -> stop later by session ID
```

There is no output reattachment operation, `SIGUSR1`, output socket, persisted stdout log, `tail`, `nohup`, or resident Smoke daemon.

## Authentication profiles

Redis DNS may advertise a non-secret auth profile. Credentials are resolved locally by the selected auth provider; credentials are never stored in DNS.

## Package composition

The Redis provider boundary remains typed:

```go
target, err := (redisprovider.DNSResolver{}).Resolve(ctx, "west.us.logma.sh")
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

Logmash owns subscription and callback supervision. Smoke owns composition and the small OS-process start/list/stop boundary used only for unattended runtimes.
