# Smoke execution and tool-composition contract — v20261115

**Candidate ownership contract.** This document must be read with
`xd-dash/nix.xd/docs/architecture-v20261115.md` and
`dash-xd/agni/local/fatline/qualification/architecture-v20261115.md`.
CalVer is used for candidate branches and exact Git SHAs for execution.
Changes are zero-version changes; no backwards-compatibility promise.

## Smoke is an executable capability provider

Smoke owns reusable Go workflows and their direct tests: GHXD device
authorization, encrypted credential operations, SHA-locked repository
materialization, source selection, test runner composition, evidence format.
A Nix flake supplies Go/build dependencies and an executable wrapper, not
new behavioral ownership. Agni chooses which applications and revisions to
qualify, not how GHXD refreshes tokens.

The main compositional interface is
`nix run /workspaces/smoke...#source-build` followed by an immutable Smoke
binary and a versioned, small action such as `smoke ghxd ...`.
Do not replace a Go package with parallel Python or Bash business logic in
nix.xd or Agni. Thin shell adapters are permitted for Linux volume/credential
mount setup only until the generic execution contract is qualified.

## Current boundary violation and its retirement

`local/ghxd/host-operator.sh` currently performs Podman commands on the
macOS host. It validates and creates dedicated secret/control volumes,
builds the Go executable inside the Nix-Alpine owner, launches a sibling
container for GHXD, and stages its exact-SHA sources back into the owner.
`local/ghxd/host-actions/*.sh` plus nix.xd
`project-host-run` are **transitional adapters**, not canonical workflows.
They must not become examples for other packages.

Required migration:
1. A generic Nix-Alpine execution/grant contract must expose preprovisioned
   GHXD secret, home, source, and read-only Nix-store volumes to the proper
   Linux identity; ownership/SELinux remain enforced at the provider.
2. The selected Smoke flake owns the GHXD executable and action dispatcher.
   The Mac does not construct or administer the recurring GHXD container.
3. Reconcile exact source generations by SHA within Smoke-owned state.
   An immutable generation can be selected by the Nix-Alpine execution
   authority without host Git-archive staging.
4. Test auth-refresh, source-sync, wrong-SHA refusal, invalid manifest,
   offline state, independent store writers, and teardown. Retire host-actions
   only after this live gate proves its capabilities.

This branch intentionally preserves host actions until replacement is
qualified; their existence is not evidence of full architectural cleanup.

## Security and data paths

Credential state remains a dedicated SOPS+age volume, never Nix/store
or a derivation input. Source generations are immutable and explicit.
Qualification logs never print secrets; errors are categorized. Only the
required read-only source/store views and narrowly scoped writable
credential/state volumes may be granted. Never mount a global Podman socket
or make the owner container privileged merely to eliminate one host helper.

## Reusable qualification interface

Smoke tools return nonzero on failure, emit a stable result marker on success,
and include version/source/mode/effect evidence. Tests should validate
structured semantics and real negative cases rather than literal formatting.
For the disposable macOS sandbox, no distributed failover, long-term
migration, or backwards-compatibility behavior is required. Ensure candidate
process cleanup and leave unrelated running applications unchanged.

`make architecture-check` is the static contract gate on this branch.
A live Nix-Alpine-only GHXD gate is **not yet implemented**. Do not equate
static checks with that gate.
