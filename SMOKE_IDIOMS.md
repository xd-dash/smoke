# Smoke / Logmash idioms

This file is the maintenance contract for `xd-dash/smoke`. Smoke is a self-composed Go executable plus named execution environments. It is not an installation-profile repository, runtime plugin host, or replacement package manager.

## Core identity

- Compiled-in commands/providers are ordinary Go packages selected by imports.
- Environment tools are Go `tool` dependencies executed with `go tool` inside immutable environment snapshots.
- `commands`, `compose`, `env`, and `inspect` are core names.
- Do not reintroduce PATH-based compiled-command discovery, `.so` plugins, or a resident plugin daemon.

## Environment identity

A Smoke environment is a local execution composition whose dependency authority is Go workspace/tool state:

```text
<env>/
├── go.work
└── tools/
    ├── go.mod
    └── go.sum
```

The environment name is an operator-local role/lifecycle label. It does not imply a recipe or profile.

```text
probe
us-west1
experiment-7
gateway-test
```

are all ordinary names. Smoke MUST NOT infer installation policy from them.

## Immutable workspace invariants

- `go.work` owns environment modules.
- `tools/go.mod` owns environment tools.
- Go owns versions, sums, replacements, exclusions, and tool resolution.
- Canonical mutation remains protected by the environment lock.
- Long-running children use immutable content-addressed snapshots after releasing the canonical lock.
- Equal Go workspace/tool state reuses the same snapshot.
- Child activation uses `GOWORK`, `SMOKE_ENV`, and workspace identity rather than process-global cwd/environment mutation.

External roots such as Terraform installations remain caller/profile-owned until an explicit future root primitive includes them in the Smoke digest.

## Generic tool execution

```bash
smoke env tool run <env> <tool> [args ...]
```

This is a thin `go tool <tool> ...` boundary. Smoke does not reinterpret the tool's domain language.

## Generic Terraform execution

```bash
smoke env terraform <env> --dir <root> -- init
smoke env terraform <env> --dir <root> -- plan
smoke env terraform <env> --dir <root> -- apply
smoke env terraform <env> --dir <root> -- output
smoke env terraform <env> --dir <root> -- destroy
```

Terraform remains the authoritative native contract for HCL, providers, variables, backends, state, plans, and lifecycle.

## Installation profiles live above Smoke

Smoke does not own Astrochicken, Gateway, or other installation recipes. Those profiles may be installed as exact environment tools from their owning module.

Example:

```bash
smoke env create probe
smoke env tool add probe github.com/dash-xd/agni/cmd/astrochicken@<sha>
smoke env tool run probe astrochicken seed <root>
smoke env terraform probe --dir <root> -- plan
```

The profile tool owns its own configuration and internal shared-module dependency graph. Smoke MUST NOT require the operator to enumerate those dependencies separately.

This means the old two-tool path is retired:

```text
Astrochicken tool
+
tf seed --module ...
```

A complete profile seed is one operation from Smoke's perspective.

## Seed vocabulary

`seed` is a semantic preparation verb, not a shared implementation contract.

```text
ghxd worktree seed
    Git/worktree implementation

Agni profile seed
    installation-root/filesystem implementation
```

Smoke MUST NOT unify unrelated seed implementations merely because they share a verb.

## Composition versus environment

```text
smoke compose = optional Go packages linked into Smoke
smoke env     = Go modules/tools and child execution context
```

`smoke env run` re-execs Smoke for compiled commands. `smoke env tool run` executes environment tools. `smoke env exec` executes arbitrary native programs.

## Provider registry invariants

Provider registries are for typed runtime dispatch capabilities, not build/root seeding or generic tool discovery.

- providers are supplied by Go composition/callers;
- schemes are normalized and duplicates invalid;
- contracts remain narrow and capability-specific;
- do not introduce providers merely to seed Terraform files or forward Terraform arguments.

## ghxd boundary

`ghxd` remains GitHub-specific. `github-worktree seed` owns Git repository/SHA/auth/object/worktree semantics. Installation-profile seeding is unrelated implementation owned by the profile repository.

## Logmash / durable Logma boundary

Logmash remains ephemeral receive/route/callback runtime. `xd-dash/logma` remains the durable Fatline service/resource graph. Do not collapse durable Logma state into Smoke environments or unattended session metadata.

## Cross-repository authority

```text
Huram
  exact candidates + credentials + evidence + promotion
       |
       v
Smoke
  generic environment/workspace/tool execution
       |
       v
Agni installation profile
  profile config + lifecycle + shared infrastructure primitives
       |
       v
native Terraform/gcloud/Butane/QEMU
```

## Change protocol

1. Keep Smoke generic; profile names do not enter Smoke core.
2. Keep Go authoritative for environment modules/tools and Terraform authoritative for Terraform.
3. Let installation profiles own their internal dependency graph.
4. Never make operators restate profile dependencies through Smoke.
5. Preserve immutable snapshots and short canonical locks.
6. Keep process-global cwd/environment mutation out of reusable execution paths.
7. Preserve exact-source qualification outside Smoke runtime identity.
8. Use provider registries only for genuine runtime dispatch.
9. Run `go vet ./...` and `go test -race ./...` on exact final candidates.
