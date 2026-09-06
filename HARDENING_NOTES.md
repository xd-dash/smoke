# Smoke hardening sweep

This branch is an implementation branch for a repository-wide correctness and architecture pass. The durable architectural rules belong in `SMOKE_IDIOMS.md`; this file is temporary review context and should be removed before merge.

Confirmed hazards addressed so far:

- Environment mutations and environment-backed commands accessed the same `go.work` / tools module without cross-process coordination.
- `smoke env run` mutated process-global `GOWORK` and cwd for in-process command dispatch, which is unsafe for embedding/concurrent invocation.
- Smoke recomposition could inherit an active environment's `GOWORK`, contaminating the composition module build.
- Composition add/remove required a locked read-modify-build-write transaction to avoid lost updates.
- Failed composition builds could leave generated `main.go`, `go.mod`, or `go.sum` partially advanced.
- Fixed temp-file names increased collision risk in session/composition atomic writes.
- Logmash session liveness based only on PID existence could misidentify a reused PID after stale metadata.
- Callback dispatcher policy/error-handler configuration was data-racy with concurrent dispatch.
- Redis pattern-only subscriptions should initialize with `PSubscribe` directly instead of relying on zero-channel `Subscribe` behavior.

Remaining review areas before merge:

- environment long-lived shell/workspace mutation semantics
- exact-self re-exec semantics across concurrent recomposition
- callback HTTP response-size / lifecycle limits
- Redis reconnect/failure semantics
- command/provider registry invariants
- CI (`go test -race`, vet/static checks where appropriate)
- final idiom and user-facing documentation updates
