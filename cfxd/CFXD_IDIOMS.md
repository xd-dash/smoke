# cfxd provider/profile idioms

cfxd owns Cloudflare-specific infrastructure profiles and provider reconciliation source. It is parallel to Agni, not a sub-layer of Agni.

## Authority

```text
Huram
  exact candidates + values + credentials + evidence + promotion
      |
      v
Smoke environment/session
  generic composition + immutable Go tool execution
      |
      +---------------------------+
      |                           |
      v                           v
Agni profile                 cfxd profile
  probe                        dns-txt
  machine/runtime substrate    Cloudflare DNS projection
      |                           |
      v                           v
Terraform / native tools     Terraform / Cloudflare provider
```

A Smoke environment such as `astrochicken` may compose both profiles, but neither profile becomes a dependency of the other merely because they participate in the same deployment.

## Profiles own provider-specific source

cfxd profiles own their complete native provider source. Cloudflare Terraform must not be placed in Agni simply because both domains use Terraform.

The first profile is:

```text
cfxd/profiles/dnstxt/
├── seed.go
└── terraform/
    ├── main.tf
    ├── variables.tf
    └── outputs.tf
```

Its installable Go tool is:

```text
github.com/xd-dash/smoke/cmd/cfxd-dns-txt
```

and its preparation contract is:

```bash
cfxd-dns-txt seed <root>
```

Terraform remains authoritative for provider configuration, plan/apply/destroy, state, and resource reconciliation.

## dns-txt

`dns-txt` is an optional cfxd profile for durable Cloudflare TXT records carrying provider-neutral `xdroute` discovery metadata.

It is not part of Agni Probe and is not required for an Astrochicken CoreOS deployment to exist. It may be composed after the base deployment when lifecycle/observability routing is wanted, for example Logmash callback discovery into Axiom.

```text
Astrochicken
├── required: agni/probe
│   └── CoreOS/network/runtime substrate
└── optional: cfxd/dns-txt
    └── xdroute TXT discovery metadata
        └── Logmash -> Axiom routing
```

`xdroute` owns the provider-neutral DNS grammar and serialization. cfxd owns durable Cloudflare projection/reconciliation. Logmash consumes the resulting discovery metadata. Secrets, callback datasets, channels, and runtime credentials do not belong in DNS.

Multiple TXT records at the same owner are unordered route alternatives; consumers must not infer preference from DNS ordering.

## Smoke composition

cfxd profiles are composed into an existing Smoke environment after that environment is created. They do not redefine the environment or the Agni profile already used by it.

Example:

```bash
smoke env tool add astrochicken github.com/xd-dash/smoke/cmd/cfxd-dns-txt@<exact-sha>
smoke env shell astrochicken <session-root>

go tool cfxd-dns-txt seed ./cfxd-dns-txt
terraform -chdir=./cfxd-dns-txt init
terraform -chdir=./cfxd-dns-txt plan
terraform -chdir=./cfxd-dns-txt apply
```

The cfxd Terraform root and Agni Probe Terraform root should remain separate state/reconciliation roots unless a concrete cross-provider requirement proves that coupling necessary.

## Change protocol

1. Keep Cloudflare-specific profiles and reconciliation source under cfxd ownership.
2. Keep Agni focused on machine/runtime/cloud infrastructure profiles it actually owns.
3. Keep `xdroute` provider-neutral; do not duplicate its route grammar in Terraform.
4. Treat `dns-txt` as optional composition, not an Astrochicken dependency.
5. Keep native Terraform authoritative; do not mirror provider resources into a Go DSL.
6. Keep credentials and runtime secrets outside DNS and profile source.
7. Preserve independent Terraform state/lifecycle boundaries for independently owned profiles by default.
8. Keep cfxd profile tools directly usable through normal Go/native tooling; Smoke is composition/execution convenience, not a mandatory runtime dependency.
