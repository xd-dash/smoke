# Astrochicken through Smoke

Astrochicken is an Agni installation profile used through a generic Smoke environment. Smoke does not own the profile implementation.

```text
Smoke environment
    |
    `-- github.com/dash-xd/agni/cmd/astrochicken
            |
            +-- Astrochicken Terraform/config root
            `-- shared Agni modules selected by the profile
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

The profile owns its shared-module dependency graph. There is no second `tf seed --module ...` command.

Then invoke native Terraform through Smoke:

```bash
smoke env terraform probe --dir "$root" -- init
smoke env terraform probe --dir "$root" -- plan
smoke env terraform probe --dir "$root" -- apply
smoke env terraform probe --dir "$root" -- output
smoke env terraform probe --dir "$root" -- destroy
```

## Profile contract

Astrochicken is the small probe design:

```text
IPv4          /29, 4 GCP-usable addresses
lifecycle     transient systemd smoke-testing idiom
frontends     Nginx execution + Squid egress
Logma         not required
Fatline       no full Fatline requirement
serverless    optional Gen1/Gen2 shadow functions
```

The durable Gateway profile is a separate design: `/28`/12 usable addresses, persistent FCOS/Quadlet startup, and the full Fatline/Logma runtime. Both profiles may reuse the same lower-level Agni modules without exposing that module list to the operator.
