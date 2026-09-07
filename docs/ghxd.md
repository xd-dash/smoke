# ghxd

`ghxd` is Smoke's GitHub-specific operator surface. It intentionally lives inside the `xd-dash/smoke` Go module. Other Git providers, if ever needed, belong beside `ghxd` as separate provider/tool environments rather than underneath it.

## Default Smoke shape

The stock `cmd/smoke` binary has a deliberate two-axis default:

```text
stock Smoke executable
├── compiled Logmash command
└── built-in ghxd orchestration
    ├── workspace bootstrap/tool execution
    └── internal GitHub capabilities
        └── worktree seed

optional/default ghxd workspace
├── github-cdn
└── github-device-auth
```

The distinction matters:

- **compiled/internal capability** is Go code already present in the Smoke executable;
- **workspace tool capability** is an ordinary external Go tool installed into a named Smoke environment and executed through an immutable workspace snapshot.

Do not install an internal `ghxd` capability into the `ghxd` tools workspace merely to make it available. Conversely, do not compile external GitHub tools into the stock Smoke binary merely to avoid workspace bootstrapping.

The composition and workspace identities remain separate:

```text
Smoke composition
    └── logmash

Smoke built-in operator surface
    └── ghxd
        └── worktree seed

Smoke workspace
    └── ghxd
        ├── github-cdn
        └── github-device-auth
```

`ghxd` itself is available from the stock Smoke binary before its workspace exists. Only operations that execute workspace tools require the workspace to be bootstrapped.

## Default bootstrap

The default GitHub workspace bootstrap is:

```bash
smoke ghxd bootstrap
```

It ensures the default Smoke environment named `ghxd` exists and installs/updates the external GitHub tools using the same native Go mechanism as `smoke env tool add`:

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

Bootstrap is intentionally idempotent. CI, Huram, and operators may invoke it unconditionally rather than pre-checking whether the environment exists.

Bootstrap is not a prerequisite for built-in `ghxd` operations such as:

```bash
smoke ghxd worktree seed ...
```

Bootstrap **is** required for operations that forward to installed Go tools, including:

```bash
smoke ghxd tool github-cdn ...
smoke ghxd auth device <client-id>
smoke ghxd auth poll <client-id> <device-code>
smoke ghxd auth refresh <client-id> <refresh-token> [client-secret]
```

Inspect the default workspace specification without creating anything:

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

## Running GitHub workspace tools through Smoke

`ghxd` workspace tools execute through an immutable Smoke workspace snapshot:

```bash
smoke ghxd tool github-cdn snapshot xd-dash huram-abi-master automation
```

A non-default environment may be selected explicitly:

```bash
smoke ghxd tool --env github-operator github-cdn ...
```

Runtime credentials are inherited by the child process. They are not persisted in the Smoke environment. For example, `github-cdn` consumes `GH_TOKEN` / `GITHUB_TOKEN` normally.

## GitHub authentication

Authentication is a GitHub capability family within `ghxd`, not a generic forge adapter. The current device-auth surface is a thin façade over the external `github-device-auth` Go tool installed by `smoke ghxd bootstrap`:

```bash
smoke ghxd auth device <client-id>
smoke ghxd auth poll <client-id> <device-code>
smoke ghxd auth refresh <client-id> <refresh-token> [client-secret]
```

These commands execute the installed tool inside an immutable `ghxd` workspace snapshot. Smoke does not reimplement GitHub's OAuth/device protocol and does not persist the access or refresh credentials.

If a credential is required before Smoke or the `ghxd` workspace can be obtained, the caller may retain a minimal bootstrap credential path. Once Smoke and `ghxd` are available, normal GitHub operations should prefer the Smoke surface.

## Exact worktree seeding

Reusable GitHub worktree mechanics live in the ordinary Go package `github.com/xd-dash/smoke/ghxd/worktree` and are linked into the stock Smoke operator surface. They do **not** depend on the external `ghxd` tools workspace.

The operator surface is:

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

## Capability placement

Place a GitHub capability according to its implementation shape, not merely its name:

```text
reusable Go behavior that belongs to Smoke orchestration
        -> ghxd/<capability> package linked into Smoke when appropriate

independently installable external Go utility
        -> ghxd ToolSpecs / named workspace

organization policy, credentials, exact candidate selection, evidence
        -> Huram / caller
```

Current shape:

```text
ghxd/
├── auth/           # capability family; current implementation forwards to workspace tool
│   ├── device/     # when reusable in-module behavior is justified
│   ├── oauth/      # later
│   ├── wif/        # later
│   └── ...
├── cdn/            # only when reusable in-module behavior is justified
├── worktree/       # current internal exact-Git seed primitive
├── webhook/        # later
└── workflow/       # later
```

Do not create empty packages merely to reserve names. Do not move a mature external tool into Smoke merely because a thin `ghxd` façade invokes it.

## Provider direction

`ghxd` is itself the GitHub provider/operator namespace.

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
 auth    tool   worktree
          |       |
     workspace   internal
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

## Native contracts remain authoritative

Go remains authoritative for module queries, pseudo-versions, downloads, checksums, `go.mod`, `go.work`, and tool directives. Git remains authoritative for exact commit/worktree semantics. Mature GitHub utilities remain authoritative for their own API/OAuth behavior.

Smoke owns named environment lifecycle, immutable snapshots, child execution, and the small built-in orchestration surfaces that compose those native contracts.

Do not introduce Android Repo merely because GitHub capabilities originate in multiple Go repositories. Repo is only relevant if a final composition genuinely spans independently versioned heterogeneous ecosystems that cannot naturally remain one native dependency/tool graph.
