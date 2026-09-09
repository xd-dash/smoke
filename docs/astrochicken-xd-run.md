# astrochicken-xd-run

`astrochicken-xd-run` is a named Smoke composition. It combines capabilities that are intended to operate together without merging their Terraform lifecycle or state.

```text
astrochicken-xd-run
├── agni/probe
├── cfxd/dns-txt
└── xd.run route configuration
```

## Bootstrap

```bash
smoke astrochicken-xd-run bootstrap /tmp/astrochicken-xd-run
```

The command creates or updates the `astrochicken-xd-run` Smoke environment with exact profile-tool revisions, then materializes:

```text
/tmp/astrochicken-xd-run/
├── agni-probe/
│   ├── main.tf
│   ├── variables.tf
│   └── modules/
├── cfxd-dns-txt/
│   ├── main.tf
│   └── variables.tf
└── config/
    ├── probe.tfvars
    └── xd-run.routes
```

Existing files beneath `config/` are not overwritten by a repeated seed. Profile source may be re-seeded; mutable deployment configuration remains a separate component.

## Independent Terraform roots

The two profile roots are intentionally siblings. Neither Terraform root imports the other, so each may use its own backend, state, locking, credentials, plan/apply cadence, and destroy lifecycle.

```bash
smoke env shell astrochicken-xd-run /tmp/astrochicken-xd-run

terraform -chdir=agni-probe init
terraform -chdir=agni-probe plan -var-file=../config/probe.tfvars
terraform -chdir=agni-probe apply -var-file=../config/probe.tfvars
```

DNS may be updated without re-planning Probe:

```bash
go tool cfxd-dns-txt render config/xd-run.routes \
  > cfxd-dns-txt/routes.auto.tfvars.json

terraform -chdir=cfxd-dns-txt init
terraform -chdir=cfxd-dns-txt plan
terraform -chdir=cfxd-dns-txt apply
```

`zone_id` is intentionally not part of `xd-run.routes`; supply it using normal Terraform input such as `TF_VAR_zone_id` or a provider-specific tfvars file. The route file remains provider-neutral.

## Route configuration

`config/xd-run.routes` is ordinary mutable JSON text describing the `xdroute` interface, not exact Cloudflare TXT records:

```json
{
  "zone": "xd.run",
  "routes": [
    {
      "service": "logmash",
      "role": "callback",
      "provider": "axiom",
      "region": "us-east",
      "edge": "us-east-1.aws",
      "host": "us-east-1.aws.edge.axiom.co"
    }
  ]
}
```

`cfxd-dns-txt render` validates the route through `xdroute`, derives the DNS owner and TXT serialization, and emits Terraform input. cfxd owns the Cloudflare projection; `xdroute` owns route meaning and serialization.

## Ownership rule

```text
one Smoke environment
    things intended to operate together

one profile
    one owned deployable capability

one Terraform root/state
    resources normally planned, locked, applied, and destroyed together
```

Smoke composition therefore does not imply Terraform state composition.
