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
  Cloudflare projection + Terraform root

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

Cloudflare account IDs, zone IDs, API tokens, callback credentials, dataset IDs, channels, and other secrets/runtime values do not belong in the `xdroute` configuration.

`cfxd-dns-txt render <route-config>` converts the route interface into Terraform variable JSON. The generated TXT owner/value pairs are a provider projection; they are not the public configuration interface.

## Composition and state

A Smoke composition may include `cfxd/dns-txt` beside other profiles without merging their Terraform state. In particular:

```text
astrochicken-xd-run
├── agni/probe       -> Terraform state A
├── cfxd/dns-txt     -> Terraform state B
└── xd.run config
```

Changing DNS discovery metadata must not require planning or locking the Probe Terraform root. Profiles should share a Terraform root/state only when their resources must normally be planned, applied, locked, and destroyed together.

## Provider boundary

Terraform is an implementation language, not an ownership namespace. Cloudflare DNS reconciliation belongs to cfxd even when another composed profile also happens to use Terraform.
