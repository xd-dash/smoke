# ghxd

`ghxd` is Smoke's optional, Go-native GitHub tool environment. It is the stable composition surface for GitHub utilities used by Smoke providers and operator workflows.

It intentionally lives inside the `xd-dash/smoke` Go module. There is no `dash-xd/github-tools` repository and no `gxd` compatibility name.

## Bootstrap

The shortest path is:

```bash
smoke ghxd bootstrap
```

This creates the default Smoke environment named `ghxd` and installs the GitHub tools using the same native Go mechanism as `smoke env tool add`:

```text
smoke ghxd bootstrap
        |
        v
Smoke environment: ghxd
        |
        +-- go get -tool github.com/dash-xd/github-cdn@go
        +-- go get -tool github.com/dash-xd/github-device-auth/cmd/github-device-auth@main
        |
        v
immutable Smoke workspace snapshots
```

Inspect the composition without creating anything:

```bash
smoke ghxd show
```

Use another environment name when useful:

```bash
smoke ghxd bootstrap github-operator
```

Or add the GitHub provider toolset to an environment that already exists:

```bash
smoke env create dev
smoke ghxd apply dev
```

The standalone command mirrors the same surface:

```bash
go run ./cmd/ghxd show
go run ./cmd/ghxd bootstrap
```

`cmd/ghxd` is an adapter over the same in-module composition; it is not a separate package manager or repository.

## Current composition

The default `ghxd` tool environment contains:

```text
github.com/dash-xd/github-cdn@go
github.com/dash-xd/github-device-auth/cmd/github-device-auth@main
```

Agni is deliberately not part of `ghxd`. Cloud/CoreOS/QEMU tooling remains independently composable:

```bash
smoke env tool add <environment> github.com/dash-xd/agni@main
```

This keeps GitHub provider capabilities separate from cloud-building capabilities.

## Go owns the composition

`ghxd` is not an Android Repo composition. Every current component is a Go module/tool, so Go remains the authority for module queries, pseudo-versions, downloads, checksums, `go.mod`, and tool directives.

Smoke owns only the environment and execution lifecycle.

```text
GitHub Go utilities
        |
        v
      ghxd
        |
        v
Go tool directives
        |
        v
Smoke environment/snapshot
```

Do not introduce Android Repo merely because the GitHub capabilities originate in multiple Go repositories. Repo becomes relevant only if the final composition genuinely spans independently versioned non-Go ecosystems that cannot naturally remain one Go dependency/tool graph.

## Provider direction

`ghxd` is the tool environment from which an optional Smoke GitHub provider can draw capabilities. Provider policy remains separate from tool installation.

```text
Huram runtime token
        |
        v
Smoke GitHub provider
        |
        v
      ghxd
     /    \
 auth     cdn
```

No token, organization name, tenant value, repository target, project ID, or other business-specific value belongs in `ghxd`.

## Growth

Future GitHub capabilities should continue to be ordinary Go packages/tools. The expected logical shape inside Smoke is:

```text
xd-dash/smoke
├── go.mod
├── cmd/
│   └── ghxd/
│       └── main.go
└── ghxd/
    ├── ghxd.go
    ├── auth/       # when a reusable adapter is justified
    ├── cdn/        # when a reusable adapter is justified
    ├── worktree/   # later
    ├── webhook/    # later
    └── workflow/   # later
```

Do not create empty abstraction packages in advance. Add each package when there is reusable Go behavior to put behind it. Until then, the installable upstream Go tools themselves are the composition units.

For worktree automation specifically, reusable Git mechanics should migrate into an ordinary Go package/tool first; the Huram GitHub Action and `ghxd` can then consume that same implementation rather than making GitHub Actions YAML the reusable primitive.
