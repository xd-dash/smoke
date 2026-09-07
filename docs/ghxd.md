# ghxd

`ghxd` is Smoke's optional, Go-native GitHub tool environment. It is the stable GitHub composition surface for provider and operator workflows.

It intentionally lives inside the `xd-dash/smoke` Go module. `gh` means GitHub here: other Git providers should live beside `ghxd` as separate provider/tool environments rather than underneath it.

## Default Smoke shape

The stock Smoke binary compiles Logmash and exposes `ghxd` as its GitHub tool workspace bootstrap:

```text
cmd/smoke
├── compiled Logmash command
└── ghxd workspace orchestration
```

The two axes remain distinct:

```text
Smoke composition
    └── logmash

Smoke workspace
    └── ghxd
        ├── github-cdn
        └── github-device-auth
```

This preserves the existing rule that compiled Smoke capabilities and Go workspace/tool state are separate identities.

## Bootstrap

The shortest path is:

```bash
smoke ghxd bootstrap
```

This ensures the default Smoke environment named `ghxd` exists and installs or updates the GitHub tools using the same native Go mechanism as `smoke env tool add`:

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

Bootstrap is idempotent and may be repeated by CI or operator workflows.

Inspect the composition without creating anything:

```bash
smoke ghxd show
```

Use another environment name when useful:

```bash
smoke ghxd bootstrap github-operator
```

Or add the GitHub toolset to an environment that already exists:

```bash
smoke env create dev
smoke ghxd apply dev
```

## Running GitHub tools through Smoke

`ghxd` tools are executed through an immutable Smoke workspace snapshot:

```bash
smoke ghxd tool github-cdn snapshot xd-dash huram-abi-master automation
```

A non-default environment may be selected explicitly:

```bash
smoke ghxd tool --env github-operator github-cdn ...
```

Runtime credentials are inherited by the child process. They are not persisted in the Smoke environment. For example, `github-cdn` consumes `GH_TOKEN` / `GITHUB_TOKEN` normally.

## GitHub authentication

Authentication is a GitHub capability family within `ghxd`, not a generic forge adapter. The current device-auth tool is exposed through a thin Smoke façade:

```bash
smoke ghxd auth device <client-id>
smoke ghxd auth poll <client-id> <device-code>
smoke ghxd auth refresh <client-id> <refresh-token> [client-secret]
```

These commands forward to the installed `github-device-auth` Go tool inside the immutable `ghxd` workspace. Smoke does not reimplement GitHub's OAuth/device protocol.

## Exact worktree seeding

Reusable GitHub worktree mechanics live in the ordinary Go package `github.com/xd-dash/smoke/ghxd/worktree`. The operator surface is:

```bash
smoke ghxd worktree seed \
  --repository xd-dash/example \
  --sha <40-char-sha> \
  --role-ref <optional-branch> \
  --destination /tmp/example
```

The exact SHA is execution authority. `--role-ref` is fetched independently and, when present, only verifies that the requested SHA is equal to or an ancestor of the current role-ref head. It can never replace the requested SHA.

The primitive maintains one shared bare object database per GitHub repository, fetches the exact commit, optionally verifies role-ref ancestry, seeds a detached worktree, then verifies exact HEAD identity and cleanliness before returning JSON. `GH_TOKEN` is read at runtime by default and is never persisted into Smoke state or included as a command argument.

The returned JSON is transport-neutral execution evidence from the primitive. Organization-specific evidence schemas remain with the caller. Huram, for example, wraps the result in its `huram.git_component_seed` evidence instead of making that schema part of Smoke.

The intended growth shape is:

```text
ghxd/
├── auth/
│   ├── device/     # current device-flow capability
│   ├── oauth/      # when reusable behavior exists
│   ├── wif/        # when reusable behavior exists
│   └── ...
├── cdn/
├── worktree/       # reusable exact-Git seeding
├── webhook/
└── workflow/
```

Do not create empty packages merely to reserve names. Add a package when reusable Go behavior exists.

## Provider direction

`ghxd` is itself the GitHub provider/tool environment.

```text
Huram credential/bootstrap authority
        |
        | runtime access/refresh token
        v
Smoke
        |
        v
      ghxd
     /    |     \
 auth    cdn   worktree
```

No token, organization name, tenant value, repository target, project ID, or other business-specific value belongs in `ghxd`.

Other Git providers, if needed later, sit at the same level as `ghxd`:

```text
Smoke
├── ghxd
├── gitea      # later, only if needed
└── forgejo    # later, only if needed
```

No forge-neutral provider framework is required in advance.

## Go owns the composition

Every current `ghxd` component is ordinary Go package/tool behavior, so Go remains authoritative for module queries, pseudo-versions, downloads, checksums, `go.mod`, tool directives, and reusable worktree implementation.

Smoke owns only environment, snapshot, execution, and GitHub operator orchestration lifecycle.

Do not introduce Android Repo merely because GitHub capabilities originate in multiple Go repositories. Repo is only relevant if a final composition genuinely spans independently versioned non-Go ecosystems that cannot naturally remain one Go dependency/tool graph.
