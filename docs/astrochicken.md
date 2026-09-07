# Astrochicken through Smoke

Astrochicken is an Agni installation profile used through a generic Smoke environment. Smoke does not own the profile implementation or its internal Terraform module graph.

```text
Smoke environment
    |
    `-- github.com/dash-xd/agni/cmd/astrochicken
            |
            +-- Astrochicken HCL/config
            +-- transient probe lifecycle policy
            `-- shared Agni Terraform library
                    |
                    `-- native Terraform
```

## Bootstrap

The environment name is local and need not match the profile name:

```bash
smoke env create probe
smoke env tool add probe \
  github.com/dash-xd/agni/cmd/astrochicken@<agni-sha>
```

Seed the complete profile with one operation:

```bash
root="$PWD/.probe-tf"
smoke env tool run probe astrochicken seed "$root"
```

There is no second `tf seed --module ...` command. Astrochicken's Terraform source declares its module imports; the profile seed merely makes Agni's shared module library available under `modules/`.

Then invoke native Terraform through Smoke:

```bash
smoke env terraform probe --dir "$root" -- init
smoke env terraform probe --dir "$root" -- plan
smoke env terraform probe --dir "$root" -- apply
smoke env terraform probe --dir "$root" -- output
smoke env terraform probe --dir "$root" -- destroy
```

## Profile distinction

Astrochicken is the small probe design:

```text
IPv4          /29, 4 GCP-usable addresses
lifecycle     transient systemd smoke-testing lifecycle
frontends     Nginx execution + Squid egress
Logma         not required
Fatline       no full durable Fatline requirement
serverless    optional Gen1/Gen2 shadow functions
```

Gateway is a separate durable Agni profile: `/28`/12 usable addresses, FCOS/Quadlet startup, Nginx/Squid, Logma, and the full Fatline runtime. They may share lower-level Agni primitives without exposing those primitives as extra Smoke steps.
