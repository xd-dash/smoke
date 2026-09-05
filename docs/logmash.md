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

## Process and callback lifecycle

Background runtime is the default. The first Smoke invocation starts another process from the same Smoke executable and then returns the shell prompt. The child is placed in a new OS session on Unix, but it inherits the launcher's stdout/stderr descriptors.

```text
Smoke launcher
   └── same Smoke executable
         └── Logmash runtime
               └── callbacks
                     ├── stdout
                     ├── Axiom
                     └── webhook
```

Process detachment and stdout detachment are separate. Stdout remains an ordinary callback until explicitly removed or until its inherited descriptor fails.

List runtimes:

```sh
smoke logmash list
```

Detach only stdout on Unix:

```sh
smoke logmash detach <session-id>
```

This sends `SIGUSR1`. The running Logmash process atomically removes `stdout` from its callback snapshot. Redis subscriptions and all other callbacks continue.

Stop the entire runtime:

```sh
smoke logmash stop <session-id>
```

This sends `SIGTERM`, cancels the root context, and lets Logmash shut down subscriptions and callbacks before process exit.

For explicit foreground ownership:

```sh
smoke logmash --foreground us:west:events
```

`--no-stdout` starts without the stdout callback. `--detached` remains accepted for compatibility but background launch is already the default.

There is no output socket, tail process, persisted stdout log, or resident Smoke daemon in this model.

## Lockless callback membership

The dispatcher holds callback membership as an immutable atomic snapshot. `detach` replaces that snapshot without a mutex. An in-flight dispatch may finish against its existing snapshot; subsequent messages observe stdout as removed.

If stdout delivery itself fails under the default `continue` policy, stdout is removed automatically and the remaining callbacks continue.

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

Logmash owns its subscription and callback supervision. Smoke owns composition and the small OS-process control surface around the runtime.
