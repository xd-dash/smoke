# Smoke — binding contributor and agent policy

**MANDATORY first reads:** [docs/CANONICAL-ARCHITECTURE.md](docs/CANONICAL-ARCHITECTURE.md), [SMOKE_IDIOMS.md](SMOKE_IDIOMS.md), [docs/composition-contract-v20261115.md](docs/composition-contract-v20261115.md). The canonical contract supersedes old host-operator examples for new work. Existing implementation is not permission to duplicate its legacy design.

For cross-repository qualification, merges and cleanup, read Agni's `local/fatline/qualification/POST-QUALIFICATION-CLEANUP.md`. Legacy host-action inventory may only shrink and must be retired following observed native credential gates. Never delete other workloads, volumes, branches or evidence by wildcard.

- Smoke owns composed **Go** workflow behavior: GHXD device auth, SOPS+age persistence, bounded refresh, exact-SHA source materialization, reusable qualifications and evidence. Optional command composition is via Go imports, not PATH plugin dispatch, duplicated Bash/Python algorithms, or new daemons.
- nix.xd owns generic Nix-Alpine flake runner, store and grant transport. Agni owns topology, VM/Quadlet activation and application-specific success criteria. App repositories own domain effects. **DO NOT MOVE OWNERSHIP across boundaries.**
- **PROHIBITED new paths:** Mac Podman GHXD operators, `project-host-run` callers, `local/*/host-actions/*`, ad hoc tar/source-sync authority, implicit mutable branch checkout, or a broad Podman socket. Existing `local/ghxd/host-operator.sh` and `host-actions` are quarantined migration exceptions only.
- Canonical candidate is `#ghxd-native` inside a SHA-pinned Nix-Alpine owner with explicit grants; do not equate candidate existence or static `#architecture-check` with a passed live auth/refresh/source-sync test.
- Writable GHXD secret grant changes the trust boundary versus the host sidecar. Require explicit approval and observed UID/SELinux, one-writer, credential refresh, exact-SHA, malformed manifest, wrong-SHA, log-redaction and teardown gates; never run both credential controllers at once.
- Keep tokens/age identities outside `/nix/store`, Nix inputs, committed files and logs. No privileged containers. Use bounded reread-latest retry for rotation races.
- CalVer names mutable development branches, complete SHAs identify executions. Return nonzero on failures, stable result markers only after actual successful actions, and capture stdout/stderr.
- Mac sandbox is disposable; do not add cross-generation migration/failover/backwards-compatibility machinery for its qualification.

Run `nix run .#architecture-check` inside Linux Nix-Alpine, plus Go tests for touched packages and independent live qualification for credential and source changes. If legacy patterns conflict with the canonical contract, **reject the pattern**. Update the doc and checks together for an intentional architecture change.
