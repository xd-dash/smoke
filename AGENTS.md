# Smoke — contributor/agent contract

Read [SMOKE_IDIOMS.md](SMOKE_IDIOMS.md) and [docs/composition-contract-v20261115.md](docs/composition-contract-v20261115.md) before changing tool composition or GHXD.

- Smoke core is Go with optional commands selected by normal Go imports and a deterministic composition identity. No PATH-based plugin dispatch, duplicated Bash/Python business logic, or new service daemon.
- Smoke owns GHXD device authentication, SOPS+age persistence, SHA-pinned source generations, tool execution and reusable qualification primitives.
- Nix supplies packages and immutable execution environments; Agni chooses application topology and acceptance. Keep GHXD tokens and keys outside /nix/store.
- `local/ghxd/host-operator.sh` and `host-actions` are transitional macOS Podman adapters. Do not add callers. Their retirement requires an independently qualified Nix-Alpine-only credential/source/grant path.
- Run `nix run .#architecture-check` (Linux Nix-Alpine), plus appropriate Go tests before revising deployment contracts.
