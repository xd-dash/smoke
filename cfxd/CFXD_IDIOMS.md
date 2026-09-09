# cfxd idioms

cfxd owns Cloudflare-specific infrastructure profiles and reconciliation source. It is parallel to Agni rather than a sub-layer of Agni.

## dns-txt

`cfxd/dns-txt` is a deployable profile with its own Terraform root/state lifecycle. It projects provider-neutral `xdroute` configuration into Cloudflare TXT resources.

Ownership is intentionally split:

```text
xdroute
  route interface + DNS/TXT serialization semantics

session/config
  concrete route instances and zone name

cfxd/dns-txt
  Cloudflare projection + Terraform root source

Terraform
  plan/apply/destroy/state semantics
```

The profile source must not contain deployment-specific route values. `Seed` writes only the generic Terraform implementation. Mutable route data is imported or rendered separately.

The route configuration is provider-neutral text data:

```json
{
  "zone": "xd.run",
  "routes": []
}
```

That JSON is a strict interface. Unknown/provider-specific fields are rejected rather than silently ignored. Cloudflare account IDs, zone IDs, API tokens, callback credentials, dataset IDs, channels, and other secrets/runtime values do not belong in the `xdroute` configuration.

`cfxd-dns-txt render <route-config>` validates the route interface and converts it into Terraform variable JSON. The generated TXT owner/value pairs are a provider projection; they are not the public configuration interface.

Terraform resource keys are derived from the rendered route identity/content, not route-list position. Reordering routes must not churn Terraform resource addresses. Exact duplicate routes are invalid rather than silently collapsing into one resource.

## Seed ownership

Within a seeded `cfxd/dns-txt` root, root-level `*.tf` files are profile-owned source. Reseeding reconciles that source set so a Terraform file removed from the selected cfxd revision cannot survive and continue influencing a later plan.

The profile does **not** own Terraform runtime/state artifacts or mutable instance input:

```text
profile-owned
  main.tf
  variables.tf
  outputs.tf
  other root *.tf shipped by the profile

not profile-owned
  .terraform/
  terraform.tfstate / backend state
  routes.auto.tfvars.json
  external xdroute config
  provider credentials
```

Do not place hand-written deployment policy in extra root `*.tf` files and then expect profile reseeding to preserve it. Deployment values belong in tfvars/environment/config components; separately owned infrastructure belongs in a separate profile/root.

## Composition and state

A Smoke composition may include `cfxd/dns-txt` beside other profiles without merging their Terraform state. In particular:

```text
astrochicken-xd-run
├── agni/probe       -> Terraform state A
├── cfxd/dns-txt     -> Terraform state B
└── xd.run config
```

Changing DNS discovery metadata must not require planning or locking the Probe Terraform root. Profiles should share a Terraform root/state only when their resources must normally be planned, applied, locked, and destroyed together.

A named Smoke composition may select exact profile tools, but exact deployment candidate selection remains outside the profile implementation. For qualification, Huram supplies exact immutable SHAs and deployment values; cfxd does not infer authority from a branch name, PR label, or moving head.

## Provider boundary

Terraform is an implementation language, not an ownership namespace. Cloudflare DNS reconciliation belongs to cfxd even when another composed profile also happens to use Terraform.
