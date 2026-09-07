# Agni / Astrochicken composition

Astrochicken is Smoke domain vocabulary. Agni does not implement an Astrochicken environment or Terraform root.

The composed command is:

```text
smoke agni astrochicken deploy
```

The dependency boundary is:

```text
Smoke cmd/agni
    |
    +-- Smoke Astrochicken recipe
    |      - chooses a two-node representative topology
    |      - owns gateway/world test meaning
    |      - owns outside-vs-environment lifecycle policy
    |      - writes the transient Terraform root
    |
    v
Smoke agni.Provider contract
    |
    v
compiled dash-xd/agni provider
    |      - materializes only requested reusable Terraform modules
    |      - invokes Terraform unchanged
    v
Agni generic modules
    regional-network
    coreos-node
    regional-cell
```

Agni module names and implementations are generic. They do not contain `astrochicken`, `farcaster`, `world`, `fatline`, or Logma topology policy. The same `regional-cell` module is intended to scale from the two nodes selected by Smoke to the full regional /28 composition.

## Scope

Outside a named Smoke environment, Astrochicken may run Terraform lifecycle operations (`deploy`, `plan`, `destroy`, `output`). Inside `smoke env run`, the initial contract is deliberately constrained to `output`; mutation stays outside the environment boundary. Later qualification-only operations can be added without granting environment-scoped infrastructure mutation.

## Terraform state

Astrochicken does not replace Terraform state semantics. Its persistent transient root defaults to `~/.smoke/agni/astrochicken`, or `SMOKE_ASTROCHICKEN_WORKSPACE` when explicitly set. Terraform therefore owns state and lifecycle within that root. A future remote backend can be supplied by the Smoke recipe without changing Agni's provider contract.

## Composition

Smoke core never imports Agni. A client composes the Agni provider package in the normal Go-import manner. That package may blank-import `github.com/xd-dash/smoke/cmd/agni` so adding the provider also adds the `smoke agni` command.

The provider receives a selection of Agni module names rather than an Agni environment name. This is intentional: Agni can contain other reusable modules and Terraform roots while each Smoke recipe materializes only the implementation it needs.
