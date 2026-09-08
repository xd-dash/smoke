# Smoke / Logmash idioms

Smoke is a self-composed Go executable plus named execution environments. It is not an installation-profile repository, runtime plugin host, infrastructure package manager, or replacement dependency language.

## Core identity

- Compiled commands/providers are ordinary Go packages selected by imports.
- Environment tools are Go `tool` dependencies executed with `go tool` inside immutable environment snapshots.
- `commands`, `compose`, `env`, and `inspect` are core names.
- Do not reintroduce PATH-based compiled-command discovery, `.so` plugins, or a resident plugin daemon.

## Go package layout

Repository layout follows normal Go package boundaries rather than product-layer folders or duplicated command/library names:

```text
cmd/
├── smoke/              # package main; canonical Smoke executable
└── github-worktree/    # package main; independently installable tool

cli/                    # public CLI composition surface used by cmd/smoke
                        # and generated smoke.local/composition programs
logmash/                # importable optional Smoke command package

environment/            # reusable environment/workspace API
ghxd/                   # reusable GitHub composition API
callback/               # reusable callback API
provider/               # reusable typed runtime providers
command/                # compiled-command registration contract
identity/               # runtime/composition identity
selfbuild/              # self-composition/rebuild implementation
session/                # Logmash session state
internal/
└── filelock/           # repository-private implementation detail
```

Package rules:

- `cmd/<name>` contains only an executable `package main`. Importable implementation MUST NOT live under `cmd`.
- Add a standalone command only when it has a meaningful independently installable contract. Do not duplicate a namespace already exposed by `smoke`; `smoke ghxd` is canonical, so there is no separate `cmd/ghxd`.
- Reusable domain logic lives in a package named for its responsibility (`environment`, `ghxd`, `logmash`, etc.), not in generic buckets such as `smokeapp`, `util`, or `common`.
- `cli` is intentionally public because generated composition modules are separate Go modules and must be able to call `cli.Main`/`cli.Run`. It owns argument dispatch and process wiring, not domain implementation.
- Use `internal` only when code is genuinely inaccessible to external consumers and generated composition modules. Do not move a required composition contract under `internal` merely for visual tidiness.
- Keep files within a package grouped by responsibility (`cli.go`, `compose.go`, `environment.go`, `terraform.go`, `ghxd.go`, `inspect.go`) rather than accumulating unrelated behavior in a monolithic `app.go`.
- Package names remain short, lowercase, and non-stuttering. Directory/package identity should make imports read naturally.

The root module package `github.com/xd-dash/smoke` remains the reusable runtime `Provider`/`Registry` API. The executable is `github.com/xd-dash/smoke/cmd/smoke`; these are different Go concepts and should not be collapsed merely because both carry the project name.

## Environment identity

A Smoke environment is a local execution composition whose dependency authority is Go workspace/tool state:

```text
<env>/
├── go.work
└── tools/
    ├── go.mod
    └── go.sum
```

The environment name is an operator-local role/lifecycle label. It does not imply an Agni profile/component.

```text
astrochicken
gateway
us-west1
experiment-7
```

are ordinary names. Smoke MUST NOT infer installation policy from them.

## Immutable workspace invariants

- `go.work` owns environment modules.
- `tools/go.mod` owns environment tools.
- Go owns versions, sums, replacements, exclusions, and tool resolution.
- Canonical mutation remains protected by the environment lock.
- Long-running children use immutable content-addressed snapshots after releasing the canonical lock.
- Equal Go workspace/tool state reuses the same snapshot.
- Child activation uses `GOWORK`, `SMOKE_ENV`, and explicit snapshot identity rather than process-global cwd/environment mutation.

The child environment contract is:

```text
SMOKE_ENV             local environment name
SMOKE_ENV_WORKSPACE   immutable snapshot directory
SMOKE_ENV_WORKFILE    immutable snapshot go.work path
GOWORK                immutable snapshot go.work path
```

`SMOKE_ENV_WORKSPACE` MUST remain a directory. Do not overload it with the `go.work` file path. This keeps the contract compatible with a future declared `roots/` area inside the same content-addressed snapshot.

External roots such as Terraform installations remain profile/caller-owned until an explicit future root primitive includes them in the Smoke digest. Until then, the Smoke digest identifies only Go workspace/tool state.

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

Terraform remains authoritative for HCL, module imports, providers, variables, backends, state, plans, and lifecycle.

## Installation profiles live above Smoke

Smoke does not own Probe, Gateway, or other Agni installation components. They are exact environment tools from their owning module.

Current Probe use:

```bash
smoke env create astrochicken
smoke env tool add astrochicken github.com/dash-xd/agni/cmd/probe@<sha>
smoke env tool run astrochicken probe seed <root>
smoke env terraform astrochicken --dir <root> -- plan
```

From Smoke's perspective a complete profile seed is one tool operation. Smoke MUST NOT require or encourage a second module-enumeration step.

The profile's own HCL/configuration declares its composition. If the profile internally ships a shared Terraform library, that is profile implementation detail; Smoke neither selects nor inventories those Terraform modules.

## Environment role and profile identity are orthogonal

```text
environment role    astrochicken / gateway / us-west1 / arbitrary
Agni identity       probe / gateway / future component
```

`astrochicken` is therefore a valid environment name for the reusable Agni `probe` component. Environment names must never become hidden dependency selectors.

A future `gateway` environment may use an Agni `gateway` profile that itself reuses Probe capabilities. The identical spelling of environment and tool is optional coincidence, not a Smoke rule.

## Seed vocabulary

`seed` is a semantic preparation verb, not a shared implementation contract.

```text
ghxd worktree seed
    Git/worktree implementation

Agni profile seed
    installation-root/config/filesystem implementation
```

Do not create a generic `SeedProvider` merely because both domains use the same verb.

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
- do not introduce providers merely to seed installation files or forward Terraform arguments.

## ghxd boundary

`ghxd` remains GitHub-specific. Prefer ordinary Go library composition for router-free GitHub operations that can run cleanly in-process; reserve the `ghxd` environment for capabilities that still benefit from an executable/tool boundary.

Current split:

```text
smoke ghxd auth ...
    github.com/dash-xd/github-device-auth/deviceauth
    linked directly into Smoke

smoke ghxd cdn ...
    github.com/dash-xd/github-cdn/operations
    linked directly into Smoke

smoke ghxd worktree ...
    github.com/xd-dash/smoke/cmd/github-worktree
    executed as a Go tool from an immutable ghxd environment
```

`smoke ghxd bootstrap` MUST NOT install `github-cdn` or `github-device-auth` merely to call their reusable APIs. Bootstrap installs only actual executable capabilities. At present that is `github-worktree`.

`github-worktree seed` owns repository/SHA/auth/object/ref/worktree semantics. `ghxd/worktree` is its reusable package while `cmd/github-worktree` is its independently installable executable surface. That library/command pair is intentional and does not justify a second standalone `ghxd` executable.

Do not pull HTTP routers or server lifecycle into `ghxd` to reuse lower-level behavior. If an upstream package mixes reusable operations with router imports, split or expose a router-free package at the owning repository and compose that package instead.

Installation-profile seeding remains unrelated implementation owned by the profile repository.

## Logmash / durable Logma boundary

Logmash remains ephemeral receive/route/callback runtime. `xd-dash/logma` remains the durable Fatline service/resource graph. Do not collapse durable Logma state into Smoke environments or unattended session metadata.

Probe owns its own transient systemd lifecycle and does not depend semantically on Smoke. A durable Gateway profile may include Logma/Fatline as installation policy. Smoke core remains agnostic to both designs.

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
Agni profile/component
  profile HCL/config + lifecycle + shared primitives
       |
       v
native Terraform/gcloud/Butane/QEMU
```

## Change protocol

1. Keep Smoke generic; profile names do not enter Smoke core.
2. Preserve idiomatic Go package boundaries: `cmd` is executable-only; reusable implementation is importable outside `cmd`.
3. Keep the public `cli` package narrow: command-line composition/wiring only, with domain behavior remaining in reusable packages.
4. Avoid duplicate executable surfaces and generic package names such as `smokeapp`.
5. Prefer direct Go library composition over bootstrapped tools when a capability has a router-free reusable API.
6. Keep Go authoritative for environment modules/tools and Terraform authoritative for Terraform composition/lifecycle.
7. Let installation profiles own their internal dependency graph and infrastructure metadata.
8. Never make operators restate profile dependencies through Smoke.
9. Treat one profile seed as one opaque preparation operation from Smoke's perspective.
10. Preserve environment-name/profile-identity orthogonality.
11. Preserve immutable snapshots, short canonical locks, and the directory/file distinction between `SMOKE_ENV_WORKSPACE` and `SMOKE_ENV_WORKFILE`.
12. Keep process-global cwd/environment mutation out of reusable execution paths.
13. Preserve exact-source qualification outside Smoke runtime identity.
14. Use provider registries only for genuine runtime dispatch.
15. Run `go vet ./...` and `go test -race ./...` on exact final candidates.
