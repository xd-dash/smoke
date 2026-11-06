# Smoke in ARM Nix-Alpine

## Preferred macOS provider: rootless Podman Machine

The canonical host adapter is now `xd-dash/nix.xd` branch
`codex/podman-unified-bootstrap`. It delegates to the upstream
`dash-xd/nix-alpine/local/bootstrap.sh` Quadlet, so `/workspaces` is
read-write with `UserNS=keep-id:uid=1000,gid=1000`, and `/nix` is
a scoped single-owner named volume.

Clone Smoke into `nix.xd/compositions/smoke`; it then appears directly at
`/workspaces/compositions/smoke` inside the runner. Smoke's
`tools/nix-alpine.sh` uses `/workspaces/.smoke-state` for its persistent
binary, composition state, and Go workspace environment. No dedicated
`/smoke` or `/exchange` mounts are needed for the normal Podman path.

```sh
nix develop /workspaces/nix-alpine/nix-alpine/toolbox#go -c \
  bash /workspaces/compositions/smoke/tools/nix-alpine.sh refresh-test
nix develop /workspaces/nix-alpine/nix-alpine/toolbox#go -c \
  bash /workspaces/compositions/smoke/tools/nix-alpine.sh build
```

The older Apple Container producer/reader design below is retained only
for migration reference. A second Podman container can mount the same
workspace, but until its source mount is explicitly read-only, it must not
mutate Smoke's canonical workspace or composition concurrently. Nix store
outputs are transferred between scoped stores through
`/workspaces/.cache/nix`, not through a shared writable `/nix`.

---


The host owns the container lifecycle. Nix-Alpine supplies a persistent Linux
Nix store and a Go toolchain. Smoke owns its Git refresh, Go workspace,
composition, test selection, and runtime identity. No Nix-Alpine Makefile
needs to know Smoke's command graph.

The macOS adapter in `xd-dash/nix.xd` accepts these bind mounts:

| Mount | Writer container | Reader container |
| --- | --- | --- |
| `/workspaces` | read-only pinned Nix inputs | read-only |
| `/smoke` | read-write Smoke Git checkout | read-only |
| `/exchange` | read-write artifact/file-cache exchange | read-only |
| `/nix` | private persistent named volume | a **different** private volume |

The Smoke checkout stays on the Mac. Edits and Git fast-forwards inside the
writer appear on the host and in the reader's mount. The reader should verify
source commit/digest before executing any newly published artifact. Treat
`/exchange` as trusted, host-owned data, not as a shared mutable Nix store.

**Never attach one writable Nix volume to two running Apple containers.** Nix
has database and store-writer assumptions. To reuse build outputs, the writer
publishes their closure to a local file binary cache under
`/exchange/nix-cache`, and the reader imports it to its own `/nix` store.
Importing an unsigned local cache with `--no-check-sigs` is suitable only for
a trusted producer; it is not validation of an untrusted source.

## Smoke workflow inside Nix-Alpine

Run with `/smoke` mounted read-write and the existing Nix-Alpine Go toolbox
available under `/workspaces/nix-alpine/nix-alpine/toolbox`:

```sh
nix develop /workspaces/nix-alpine/nix-alpine/toolbox#go -c \
  bash /smoke/tools/nix-alpine.sh refresh-test

nix develop /workspaces/nix-alpine/nix-alpine/toolbox#go -c \
  bash /smoke/tools/nix-alpine.sh vet

nix develop /workspaces/nix-alpine/nix-alpine/toolbox#go -c \
  bash /smoke/tools/nix-alpine.sh build

nix develop /workspaces/nix-alpine/nix-alpine/toolbox#go -c \
  bash /smoke/tools/nix-alpine.sh commands
```

`refresh-test` refuses dirty tracked files and detached HEAD, fetches the
checkout's current upstream branch, fast-forwards, and runs Smoke's Go tests.
It does not restart or replace Nix-Alpine or discard developer changes.
`test` and `vet` explicitly use `GOWORK=off` to check the repository's
module independently of a mutable Smoke environment. The lightweight
`CGO_ENABLED=0` test here is not a replacement for the Go race CI job.

Smoke's existing `env` and `compose` subcommands remain the canonical
self-composition mechanism. Invoke them using the same Go shell:

```sh
nix develop /workspaces/nix-alpine/nix-alpine/toolbox#go -c \
  bash /smoke/tools/nix-alpine.sh env
```

Smoke installs its Linux executable at `/exchange/smoke-state/bin/smoke`.
The wrapper sets `SMOKE_ENV_DIR`, `SMOKE_COMPOSITION_MANIFEST`, and
`SMOKE_COMPOSITION_DIR` into that shared state, while `SMOKE_LOCAL_SOURCE=/smoke`
keeps recomposition linked to the edited local checkout instead of an
unrelated published Go module version. In particular, Smoke `compose` runs
its persistent installed executable, never the ephemeral executable produced
by `go run`. Smoke environment snapshots and composition directories have a
**single mutation owner**. Readers should consume immutable exported outputs
rather than modify canonical environment files simultaneously.
Native cross-VM file locking and cache coherence must be qualified before
allowing concurrent Smoke environment writers.

## Store handoff

The Nix-Alpine *writer* builds a Nix store output (for example the reusable
Go devShell derivation), then publishes its exact store path:

```sh
nix copy --to file:///exchange/nix-cache /nix/store/<exact-store-path>
```

The *reader*, with its own persistent Nix store, imports that output:

```sh
nix copy --no-check-sigs --from file:///exchange/nix-cache \
  /nix/store/<exact-store-path>
```

`nix.xd` provides logged `make store-export` and `make store-import`
wrappers around those commands. A Nix output path is not interchangeable
with an unrelated architecture's output. Both containers here target
`aarch64-linux`.

The initial integration must still be verified on the actual Apple host,
including read-write bind permissions, Go builds, cache publication, import,
and read-only source access.
