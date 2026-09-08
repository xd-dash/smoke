# ghxd

`ghxd` is Smoke's Go-native GitHub tool environment and stable GitHub operator surface. `gh` means GitHub: other Git providers belong beside `ghxd`, not underneath it.

The package and command surface are deliberately different Go concepts:

```text
github.com/xd-dash/smoke/ghxd
    reusable GitHub composition package

smoke ghxd ...
    canonical operator surface
```

There is no separate `cmd/ghxd` executable. A second binary would duplicate the namespace already exposed by Smoke without providing an independent implementation contract.

## Default Smoke shape

The stock Smoke binary compiles Logmash and includes the `ghxd` orchestration commands. External GitHub capabilities live in the default `ghxd` Go workspace:

```text
Smoke composition
├── logmash
└── ghxd orchestration

Smoke workspace: ghxd
├── github-cdn
├── github-device-auth
└── github-worktree
```

Composition identity and workspace identity remain separate. The `ghxd` command surface being present in the binary does not mean its external tools have been installed yet.

## Bootstrap

The normal GitHub bootstrap is:

```bash
smoke ghxd bootstrap
```

It idempotently ensures the default environment named `ghxd` exists and composes the GitHub tools using the same native Go tool mechanism as `smoke env tool add`:

```text
smoke ghxd bootstrap
        |
        v
Smoke environment: ghxd
        |
        +-- github-cdn
        +-- github-device-auth
        +-- github-worktree
        |
        v
immutable Smoke workspace snapshots
```

`github-worktree` is an ordinary installable Go tool backed by the reusable `ghxd/worktree` package. Its default tool spec is pinned to an exact qualified Smoke commit rather than a movable `main` selector, so an older Smoke binary cannot silently bootstrap a newer seeding implementation.

Inspect the intended default tool set without mutating anything:

```bash
smoke ghxd show
```

Use another environment when useful:

```bash
smoke ghxd bootstrap github-operator
```

or apply the same GitHub tool set to an existing environment:

```bash
smoke env create dev
smoke ghxd apply dev
```

## One execution model

After bootstrap, GitHub tools are always executed through an immutable Smoke workspace snapshot. The high-level façades and the generic tool command use the same underlying path:

```bash
smoke ghxd tool github-cdn ...
smoke ghxd auth device <client-id>
smoke ghxd auth poll <client-id> <device-code>
smoke ghxd auth refresh <client-id> <refresh-token> [client-secret]
smoke ghxd worktree seed --repository owner/repo --sha <sha> --destination <path>
```

A non-default workspace can be selected consistently:

```bash
smoke ghxd tool --env github-operator github-cdn ...
smoke ghxd auth --env github-operator device <client-id>
smoke ghxd worktree --env github-operator seed ...
```

These façades do not reimplement the external tool semantics. They resolve the selected `ghxd` environment, take an immutable snapshot, and execute the installed Go tool inside that snapshot.

Runtime credentials are inherited by child execution and are never persisted in Smoke workspace state. `GH_TOKEN`, access tokens, and refresh tokens remain runtime inputs.

## Worktree seed contract

The reusable library remains:

```text
github.com/xd-dash/smoke/ghxd/worktree
```

The independently installable tool is:

```text
github.com/xd-dash/smoke/cmd/github-worktree
```

This `cmd`/library pair is intentional: `cmd/github-worktree` is a real standalone tool, while `ghxd/worktree` contains reusable Git mechanics.

The user-facing Smoke surface remains:

```bash
smoke ghxd worktree seed \
  --repository xd-dash/example \
  --sha <40-char-sha> \
  --role-ref <optional-branch> \
  --destination /tmp/example
```

The exact SHA is execution authority. `--role-ref` is provenance only: it is fetched independently and may verify that the requested SHA is equal to or an ancestor of the current role-ref head, but it never replaces the requested SHA.

The tool maintains one shared bare object database per GitHub repository, fetches the exact commit, optionally verifies role-ref ancestry, seeds a detached worktree, then verifies exact HEAD identity and cleanliness before returning JSON. `GH_TOKEN` is read at runtime by default and is never written into Smoke state or evidence.

The returned JSON is generic execution evidence. Organization-specific evidence schemas remain with callers such as Huram.

## Capability growth

The conceptual GitHub capability families are:

```text
ghxd
├── auth
├── cdn
├── worktree
├── webhook
└── workflow
```

Do not create empty packages merely to reserve those names. Add reusable Go libraries/tools only when real behavior exists.

Other Git providers, if needed later, remain siblings:

```text
Smoke
├── ghxd
├── gitea      # later, only if needed
└── forgejo    # later, only if needed
```

No forge-neutral provider framework is required in advance.

## Ownership

Go owns module queries, pseudo-versions, checksums, `go.mod`, `go.sum`, `go.work`, tool directives, and ordinary reusable Go implementations.

Smoke owns named environments, immutable snapshots, and the GitHub operator façades that execute those tools.

Huram owns credentials, exact candidate selection, organization-specific evidence, qualification, and promotion.

Do not introduce Android Repo merely because GitHub capabilities originate in multiple Go repositories. Use native Go composition while the final graph remains Go-native.
