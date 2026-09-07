# Astrochicken environment recipe

Astrochicken is a named Smoke environment/recipe, not a compiled-in Smoke command and not an Agni provider.

Its responsibility is to compose ordinary Terraform source with generic Agni Terraform modules and environment-specific policy:

```text
Astrochicken recipe
    +-- ordinary .tf root
    +-- Smoke environment/tool graph
    +-- Agni generic Terraform modules
    `-- native terraform executable
```

## Bootstrap

The transitional recipe source still lives in this repository so an exact Smoke SHA can identify both the Smoke runtime and the recipe tool. It is exposed through the optional Go tool `github.com/xd-dash/smoke/cmd/astrochicken-root` rather than linked into the stock Smoke binary.

```bash
smoke env create astrochicken

smoke env tool add astrochicken \
  github.com/xd-dash/smoke/cmd/astrochicken-root@<smoke-sha>

smoke env tool add astrochicken \
  github.com/dash-xd/agni/cmd/agni-terraform@<agni-sha>
```

Seed the ordinary Terraform root from exact recipe/module sources:

```bash
root="$PWD/.astrochicken-tf"

smoke env tool run astrochicken astrochicken-root seed "$root"

smoke env tool run astrochicken agni-terraform seed \
  --module regional-network \
  --module regional-internal-addresses \
  --module coreos-node \
  --module regional-cell \
  --module cloud-function-v1-http \
  --module cloud-function-v2-http \
  "$root"
```

Then use generic Terraform execution:

```bash
smoke env terraform astrochicken --dir "$root" -- init
smoke env terraform astrochicken --dir "$root" -- plan
smoke env terraform astrochicken --dir "$root" -- apply
smoke env terraform astrochicken --dir "$root" -- output
smoke env terraform astrochicken --dir "$root" -- destroy
```

No Terraform lifecycle operation is implemented by Astrochicken Go code.

## Seed semantics

`seed` is the same preparation idiom used by `github-worktree seed`: an exact source/tool writes authoritative source into a caller-owned destination. It does not create another dependency graph or reinterpret the source language.

## Network recipe

The HCL currently describes a representative `/29` regional cell:

```text
host offset 2  gateway node primary
host offset 3  world node primary
host offset 4  execution service alias
host offset 5  egress service alias
```

The recipe enables Private Google Access and may add internal-only Gen1/Gen2 shadow functions. Those are Astrochicken policy choices expressed in the ordinary Terraform root. Agni only supplies generic modules.

## Request topology

```text
public caller
    -> Cloudflare
    -> Farcaster VM
       -> local gospace / pyspace / simple-router-builder
       -> or authenticated internal Gen1 / Gen2 shadow
    <- response
    <- VM
    <- Cloudflare
```

The `.4`/`.5` service addresses remain Farcaster-owned VM aliases. They are not Cloud Function addresses.

## Responsibility boundary

```text
Smoke
  environment creation, immutable Go workspace/tool snapshots,
  generic `go tool` and Terraform child execution

Agni
  generic reusable Terraform modules and `agni-terraform seed`

Astrochicken
  ordinary Terraform root and domain policy

Huram
  exact Smoke/Agni/recipe identities, credentials, tfvars/backend inputs,
  qualification evidence, plan/apply authorization, promotion
```

The eventual clean endpoint is for the Astrochicken recipe/tool to live in its own module or repository. Moving it later must not require changes to Smoke environment or Agni tool semantics.
