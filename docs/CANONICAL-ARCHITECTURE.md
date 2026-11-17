# Smoke canonical execution contract (2026-11-16)

Status: design requirement for new changes; host-operated GHXD compatibility is still present and native execution **has not been live-qualified**. Read `SMOKE_IDIOMS.md`, `docs/composition-contract-v20261115.md` and this file before changing GHXD.

## Ownership and entrypoint

Smoke owns reusable Go executable composition and business workflows: GitHub device authorization, SOPS+age secrets, bounded refresh and retry, SHA-locked source materialization, and generic process/stream qualification primitives. Agni owns topology, application selection and acceptance. nix.xd owns Nix-Alpine execution transport, stores and provider grants. macOS is a one-time bootstrap and operator surface, not a GHXD runtime.

Go package imports define optional command composition. Keep dependency footprint low; do not add PATH-based plugin discovery, a new service daemon, or a parallel Python/Bash implementation of GitHub auth and source reconciliation. Nix packages expose a pinned binary and required tools; they must not own credential logic.

Canonical target: Agni selects an exact Smoke SHA and invokes a Nix-Alpine `#ghxd-native` or a narrower replacement through nix.xd's generic `run` interface; Smoke operates on pregranted runtime mounts. Never invoke GHXD from a recurring host-side Podman script as a new path.

## Explicitly denied new patterns

- **DENY** new `local/*/host-actions`, host `project-host-run`, `host-operator.sh` callers, macOS Podman sidecars, or ad hoc tar uploads as source authority. Existing GHXD code is quarantine-only until native qualification passes.
- **DENY** credentials or private keys in `/nix/store`, Nix build environment, source archives, command line output and logs.
- **DENY** mutable branch/tag resolution as final execution identity; commit SHA must be verified before and after materialization.
- **DENY** multiple concurrently writable credential controllers or unconditional refresh loops; acquire a single writer, tolerate invalidation races with reread-latest and bounded retry.
- **DENY** a broad Podman socket or privileged container as a solution to mount access, and do not silently assume UID or SELinux labels.
- **DENY** static string matching as a substitute for an observed live grant, credential or source-sync test.

## Native GHXD live acceptance

On a disposable, independently bootstrapped Nix-Alpine owner, inspect/read/write the three declared GHXD volumes; verify UID/GID and SELinux denial from an unauthorized principal. Test the precise declared security model: writable secret grant to the selected trusted owner is a deliberate change from the sidecar isolation boundary.

Confirm: valid SOPS+age state, device authentication availability without logging a token, a successful bounded credential refresh, invalid/stale credential behavior, exact-SHA materialization and revision evidence, wrong SHA and malformed manifest failure, idempotent repeat, clean owner-scoped teardown, and absence of a concurrent host adapter. Each real command must return zero and emit stable evidence; negative cases must return nonzero.

Keep `local/ghxd/host-operator.sh` and `host-actions` quarantined until the above is observed. Once the canonical path is qualified, remove legacy files, old callers and checks requiring their presence in one reviewed transition. Do not present `SMOKE_ARCHITECTURE_CHECK=passed` as live GHXD approval.

## Consumer interface and evidence

Provide bounded, typed commands and stable markers with source SHA, action, mode and evidence location. Exit nonzero on failure and preserve stderr. The application selects acceptance gates; Smoke provides reusable mechanism. Historical `#architecture-check` is structural; actual end-to-end credential and source evidence must come from a separate live command. CalVer workspace names aid development only; SHA pins establish runtime identity.
