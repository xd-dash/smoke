# Astrochicken environment

`astrochicken` is the current Smoke environment name used to exercise the reusable Agni `probe` component. It is not a Smoke package, compiled command, or Agni profile identity.

The separation is:

```text
Smoke environment    astrochicken
Agni component       probe
Agni tool             github.com/dash-xd/agni/cmd/probe
```

Create the environment and compose the exact Probe tool through ordinary Go tool semantics:

```bash
smoke env create astrochicken
smoke env tool add astrochicken \
  github.com/dash-xd/agni/cmd/probe@<exact-agni-sha>
```

Seed the complete Probe root with one profile operation:

```bash
root="$PWD/.astrochicken-probe"
smoke env tool run astrochicken probe seed "$root"
```

Probe owns its `/29` topology, transient lifecycle, Nginx/Squid configuration, and optional Gen1/Gen2 shadow-function policy. Smoke does not enumerate Probe's Terraform modules or interpret its installation configuration.

Operate the completed root through native Terraform:

```bash
smoke env terraform astrochicken --dir "$root" -- init
smoke env terraform astrochicken --dir "$root" -- plan
```

The environment name is intentionally not a dependency selector. The same Probe tool could be used from a differently named environment, and a future `gateway` environment may use a separate Agni `gateway` component.

## Identity

Smoke's environment snapshot digest covers Go workspace/tool state. The external Probe root is not currently part of that digest. Qualification should therefore retain both the exact Agni SHA and a separate seeded-root digest until Smoke gains an explicit digest-owned resource-root primitive.

The child execution contract is:

```text
SMOKE_ENV             astrochicken
SMOKE_ENV_WORKSPACE   immutable Smoke snapshot directory
SMOKE_ENV_WORKFILE    immutable snapshot go.work
GOWORK                immutable snapshot go.work
```

No Astrochicken-specific implementation belongs in Smoke core. The name is only an operator-local environment role.
