# ghxd

`ghxd` is Smoke's Go-native GitHub operator surface. GitHub capabilities that are useful in-process are ordinary Go libraries linked into Smoke; only capabilities that benefit from an executable boundary belong in the `ghxd` environment.

```text
Smoke binary
├── ghxd auth
│   └── github-device-auth/deviceauth
├── ghxd cdn
│   └── github-cdn/operations
└── ghxd worktree
    └── go tool github-worktree
```

There is no standalone `cmd/ghxd` executable.

## Built-in capabilities

Device flow is linked through the router-free package:

```text
github.com/dash-xd/github-device-auth/deviceauth
```

The operator surface is:

```bash
smoke ghxd auth device <client-id>
smoke ghxd auth poll <client-id> <device-code>
smoke ghxd auth refresh <client-id> <refresh-token> [client-secret]
```

These calls do not create or require a Smoke environment and do not execute `go tool github-device-auth`.

GitHub CDN/repository operations are linked through the existing router-independent operation layer:

```text
github.com/dash-xd/github-cdn/operations
```

The operator surface is:

```bash
smoke ghxd cdn repo-create ...
smoke ghxd cdn upload ...
smoke ghxd cdn snapshot ...
smoke ghxd cdn delete ...
smoke ghxd cdn branch-empty ...
smoke ghxd cdn branch-from ...
```

`GH_TOKEN` or `GITHUB_TOKEN` is a runtime credential and is never persisted in Smoke state.

## Bootstrap

`smoke ghxd bootstrap` now installs only executable capabilities:

```text
smoke ghxd bootstrap
        |
        v
Smoke environment: ghxd
        |
        `-- github-worktree
```

The command is idempotent. The same tool set can be applied to another existing environment with:

```bash
smoke ghxd apply <environment>
```

Inspect the distinction with:

```bash
smoke ghxd show
```

which reports built-ins separately from environment tools:

```text
builtin auth
builtin cdn
environment ghxd
tool github.com/xd-dash/smoke/cmd/github-worktree@<sha>
```

## Worktree seed contract

The reusable Git mechanics remain:

```text
github.com/xd-dash/smoke/ghxd/worktree
```

The independently installable tool remains:

```text
github.com/xd-dash/smoke/cmd/github-worktree
```

The user-facing surface is:

```bash
smoke ghxd worktree seed \
  --repository xd-dash/example \
  --sha <40-char-sha> \
  --role-ref <optional-branch> \
  --destination /tmp/example
```

The exact SHA is execution authority. `--role-ref` is provenance only. The tool uses a shared bare object database, seeds a detached worktree, and verifies exact HEAD identity and cleanliness before returning evidence.

## Composition rule

Prefer ordinary library composition when a capability can run cleanly in-process:

```text
router-free Go operation
    -> import into ghxd

independent process/tool semantics
    -> ghxd environment tool
```

Do not install a Go tool merely to call functionality that already has a narrow reusable library API. Conversely, do not pull HTTP routers, server lifecycle, or persistent service policy into `ghxd` just to reuse their underlying operations.

The conceptual capability families remain:

```text
ghxd
├── auth
├── cdn
├── worktree
├── webhook
└── workflow
```

Add a family only when real behavior exists. Other forge providers remain separate concepts rather than being forced behind a premature forge-neutral abstraction.

Huram continues to own credentials, exact candidate selection, qualification evidence, and promotion. Smoke owns the operator composition and any named execution environments required by true tool boundaries.
