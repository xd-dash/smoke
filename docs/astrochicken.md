# Astrochicken environment through Smoke

`astrochicken` is the current Smoke environment name for running Agni's reusable `probe` component. Smoke does not own Probe's implementation or its internal Terraform module graph.

```text
Smoke env: astrochicken
    |
    `-- github.com/dash-xd/agni/cmd/probe
            |
            +-- Probe HCL/config
            +-- transient probe lifecycle policy
            `-- shared Agni Terraform library
                    |
                    `-- native Terraform
```

## Bootstrap

```bash
smoke env create astrochicken
smoke env tool add astrochicken \
  github.com/dash-xd/agni/cmd/probe@<agni-sha>
```

Seed the complete Probe source tree with one operation:

```bash
root="$PWD/.astrochicken-tf"
smoke env tool run astrochicken probe seed "$root"
```

There is no second `tf seed --module ...` command. Probe's Terraform source declares its module imports; the profile seed merely makes Agni's shared module library available under `modules/`.

Then invoke native Terraform through Smoke:

```bash
smoke env terraform astrochicken --dir "$root" -- init
smoke env terraform astrochicken --dir "$root" -- plan
smoke env terraform astrochicken --dir "$root" -- apply
smoke env terraform astrochicken --dir "$root" -- output
smoke env terraform astrochicken --dir "$root" -- destroy
```

## Probe contract

```text
IPv4          /29, 4 GCP-usable addresses
lifecycle     transient systemd smoke-testing lifecycle
frontends     Nginx execution + Squid egress
Logma         not required
Fatline       no full durable Fatline requirement
serverless    optional Gen1/Gen2 shadow functions
```

## Gateway evolution

Gateway is the durable Agni composition that will grow from Probe's reusable capabilities while owning its own `/28` network and persistent FCOS/Quadlet/Logma/Fatline policy.

The intended later use is:

```bash
smoke env create gateway
smoke env tool add gateway github.com/dash-xd/agni/cmd/gateway@<agni-sha>
smoke env tool run gateway gateway seed <root>
```

The environment names are operator-local. `astrochicken` does not select Probe implicitly, and `gateway` does not select Gateway implicitly.
