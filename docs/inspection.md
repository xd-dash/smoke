# Smoke runtime inspection

Smoke exposes two identities for correlating a runtime with the code and workspace state it used.

```text
composition digest
    = logical set of compiled Smoke components

workspace digest
    = immutable content-addressed environment snapshot
```

The pair is useful when a smoke test fails because it tells you both what the Smoke binary was composed to do and which Go workspace/tool state the runtime saw.

## Inspect the running Smoke binary

```sh
smoke inspect
```

Example shape:

```text
Smoke composition
  digest: 5c8...
  executable: /Users/me/go/bin/smoke
  go: go1.26.7
  components:
    github.com/xd-dash/smoke/cmd/logmash
Runtime
  environment: (none)
  workspace digest: (none)
  workspace: (none)
```

When Smoke itself is running under `smoke env run`, the Runtime section reports the inherited `SMOKE_ENV`, workspace digest, and exact snapshot `go.work` path.

The composition digest is intentionally a logical component-set identity. It is not a byte hash of the executable and does not replace a Git SHA or CI qualification run.

## Inspect an environment

```sh
smoke env inspect infra
```

This reports both the mutable canonical environment and the immutable snapshot Smoke would use for a new runtime:

```text
Environment infra
  canonical work: ~/.local/share/smoke/envs/infra/go.work
  canonical tools: ~/.local/share/smoke/envs/infra/tools/go.mod
Runtime snapshot
  digest: 9c1...
  work: ~/.cache/smoke/env-workspaces/infra/9c1.../go.work
  tools: ~/.cache/smoke/env-workspaces/infra/9c1.../tools/go.mod
```

Inspection takes the same short coherent snapshot used by `shell`, `exec`, `build`, and `run`; it does not hold the canonical environment lock after the snapshot is materialized.

## Inspect unattended Logmash sessions

Compact session listing remains:

```sh
smoke logmash list
```

For correlation/debug metadata:

```sh
smoke logmash list --verbose
```

Verbose output adds:

```text
composition: <digest>
environment: <name or none>
workspace digest: <digest or none>
workspace: <snapshot go.work path or none>
lease: <session lease path>
started: <timestamp>
```

Session records capture these values when the unattended runtime starts. An environment-backed unattended child therefore stays associated with the exact snapshot it inherited even if the canonical environment is changed later.

## Qualification model

Smoke runtime identity complements, rather than replaces, exact-source qualification:

```text
Git / CI identity
    exact candidate SHA
        +
Smoke runtime identity
    composition digest
        +
Environment identity
    workspace digest
```

Together those let a higher-level workflow such as Huram record the exact source candidate plus the logical Smoke composition and runtime workspace used during a smoke test.
