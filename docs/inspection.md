# Smoke runtime inspection

Smoke exposes two identities for correlating a runtime with the code and workspace state it used.

```text
composition digest
    = logical set of compiled Smoke components

workspace digest
    = immutable content-addressed environment snapshot
```

The pair is useful when a smoke test fails because it tells you both what the Smoke binary was composed to do and which Go workspace/tool state the runtime saw.

## Composition identity ownership

The composition entrypoint owns the complete logical component list. Both the canonical `cmd/smoke` entrypoint and self-generated compositions call `identity.SetComponents(...)` with the normalized component imports they link.

Optional packages still register commands/providers during Go package initialization, but they do **not** need a separate component-identity registration hook. This keeps identity generation coupled to the same manifest/import list that actually builds the binary and removes the possibility that an optional component is linked successfully but omitted from inspection metadata.

`identity.RegisterComponent` remains available for source compatibility with packages written against the earlier convention. The entrypoint's `SetComponents` call is authoritative and replaces any compatibility registrations made during package initialization.

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

For automation and qualification, use the versioned JSON representation:

```sh
smoke inspect --json
```

```json
{
  "schema": 1,
  "kind": "smoke.runtime",
  "composition": {
    "digest": "5c8...",
    "executable": "/Users/me/go/bin/smoke",
    "go_version": "go1.26.7",
    "components": ["github.com/xd-dash/smoke/cmd/logmash"]
  },
  "runtime": {
    "environment": "",
    "workspace_digest": "",
    "workspace": ""
  }
}
```

JSON uses empty strings for unavailable runtime values rather than display strings such as `(none)`. `schema` is the compatibility boundary for machine consumers; field names within schema 1 are intended to remain stable.

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

The corresponding machine-readable form is:

```sh
smoke env inspect infra --json
```

```json
{
  "schema": 1,
  "kind": "smoke.environment",
  "environment": {
    "name": "infra",
    "canonical_work": "/.../envs/infra/go.work",
    "canonical_tools": "/.../envs/infra/tools/go.mod"
  },
  "runtime_snapshot": {
    "digest": "9c1...",
    "work": "/.../env-workspaces/infra/9c1.../go.work",
    "tools": "/.../env-workspaces/infra/9c1.../tools/go.mod"
  }
}
```

`smoke env inspect --json infra` is accepted as an equivalent spelling.

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

Together those let a higher-level workflow such as Huram record the exact source candidate plus the logical Smoke composition and runtime workspace used during a smoke test. Machine consumers should capture the JSON documents verbatim when practical and derive indexed fields from schema 1 rather than parsing human-oriented output.
