# Astrochicken

`astrochicken` is Smoke's narrow orchestration boundary for disposable environment probes.

Smoke does not know Terraform, Agni repository layouts, cloud providers, or the topology being deployed. Those belong to a compiled-in Astrochicken provider.

```text
smoke astrochicken ...
        |
        v
Astrochicken provider contract
        |
        v
compiled provider package
        |
        v
Agni environment implementation
```

A provider is ordinary Go composition. It registers with `astrochicken.Register`; runtime provider discovery, PATH plugins, and provider subprocess lookup are not used.

The command package is optional:

```text
smoke compose add github.com/xd-dash/smoke/cmd/astrochicken
```

An implementation package can import that command package itself so adding the implementation is sufficient to make `smoke astrochicken` available.

## Scope

Every invocation carries one of two scopes:

```text
outside
    Smoke is not running under a named Smoke environment.

environment
    Smoke inherited SMOKE_ENV from `smoke env run`.
```

The provider receives the environment name when scope is `environment`. Smoke does not assign permissions to either scope. The provider owns the policy. This lets a provider expose broad lifecycle operations outside an environment while deliberately limiting what can happen from inside an environment without duplicating Smoke's environment model.

## CLI

With one provider compiled in:

```text
smoke astrochicken deploy
smoke astrochicken destroy
```

With multiple providers compiled in:

```text
smoke astrochicken --provider agni deploy
```

Arguments after provider selection are opaque to Smoke and are passed unchanged to the selected provider. This is intentional: Terraform verbs, environment names, variables, output formats, cleanup policy, and cloud-specific behavior stay in the provider.

## Agni boundary

For the first implementation, Agni should expose a small Go package that registers an `agni` Astrochicken provider and internally selects only Agni's `terraform/astrochicken` environment. Agni may contain other Terraform environments without making them Astrochicken-visible.

The initial dependency direction should remain:

```text
Agni Astrochicken adapter
        |
        v
github.com/xd-dash/smoke/astrochicken
```

Smoke must not import Agni from core packages. An Agni-aware Smoke executable opts into the adapter through normal Smoke composition.

The gateway/world CoreOS topology, Quadlets, Redis transport, Nginx/Squid configuration, Logma deployment, serverless Logma round-trip, and SSE qualification all belong to the Agni environment and its qualification workflow, not to this Smoke contract.
