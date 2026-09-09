# astrochicken-xd-run

`astrochicken-xd-run` is a named Smoke composition. It composes two independently owned infrastructure profiles plus mutable deployment configuration:

```text
astrochicken-xd-run
├── agni/probe
├── cfxd/dns-txt
└── config
    ├── probe.tfvars
    └── xd-run.routes
```

Smoke composition does not imply Terraform state composition. `agni/probe` and `cfxd/dns-txt` are seeded into sibling Terraform roots and are planned, applied, locked, and destroyed independently.

## Bootstrap exact profile tools

The composition refuses movable profile refs. Supply the exact qualified commit for each profile implementation:

```sh
smoke astrochicken-xd-run bootstrap \
  --agni-sha <40-character-agni-sha> \
  --cfxd-sha <40-character-smoke-cfxd-sha>
```

This creates or updates the `astrochicken-xd-run` Smoke environment through the existing Go tool mechanism with:

```text
github.com/dash-xd/agni/cmd/probe@<agni-sha>
github.com/xd-dash/smoke/cmd/cfxd-dns-txt@<cfxd-sha>
```

## Seed one composition workspace

Deployment values remain separate from the profile implementations:

```sh
smoke astrochicken-xd-run seed /tmp/astrochicken-xd-run \
  --probe-vars ./probe.tfvars \
  --routes ./xd-run.routes
```

The resulting layout is:

```text
/tmp/astrochicken-xd-run/
├── agni-probe/
│   ├── main.tf
│   ├── variables.tf
│   └── modules/
├── cfxd-dns-txt/
│   ├── main.tf
│   ├── variables.tf
│   └── outputs.tf
└── config/
    ├── probe.tfvars
    └── xd-run.routes
```

`xd-run.routes` is provider-neutral JSON containing `zone` and `routes` (`[]xdroute.Route`). Cloudflare zone IDs and credentials are not part of that file.

Example:

```json
{
  "zone": "xd.run",
  "routes": [
    {
      "Service": "logmash",
      "Role": "callback",
      "Provider": "axiom",
      "Region": "us-east",
      "Edge": "us-east-1.aws",
      "Host": "us-east-1.aws.edge.axiom.co"
    }
  ]
}
```

## Operate the Terraform roots independently

Enter the same Smoke environment at the composition root:

```sh
smoke env shell astrochicken-xd-run /tmp/astrochicken-xd-run
```

Operate Probe without touching DNS state:

```sh
terraform -chdir=agni-probe init
terraform -chdir=agni-probe plan -var-file=../config/probe.tfvars
terraform -chdir=agni-probe apply -var-file=../config/probe.tfvars
```

Render and operate DNS without planning Probe:

```sh
go tool cfxd-dns-txt render ../config/xd-run.routes \
  > cfxd-dns-txt/routes.auto.tfvars.json

terraform -chdir=cfxd-dns-txt init
terraform -chdir=cfxd-dns-txt plan
terraform -chdir=cfxd-dns-txt apply
```

The cfxd Terraform root still receives Cloudflare-specific values such as `zone_id` through normal Terraform configuration (`TF_VAR_zone_id`, another var file, or equivalent). Those provider values are deliberately not represented as `xdroute` data.

## Lifecycle rule

Use one Smoke environment for capabilities intended to work together. Use one profile for one owned deployable capability. Use one Terraform root/state for resources that should normally be planned, locked, applied, and destroyed together.

For this composition that means:

```text
Smoke environment:  astrochicken-xd-run
Terraform state A:  agni-probe
Terraform state B:  cfxd-dns-txt
```

Changing an Axiom callback route should therefore require only a new route render and a `cfxd-dns-txt` plan/apply. It must not require planning the Probe root.
