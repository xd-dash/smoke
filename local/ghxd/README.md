# GHXD operator, versioned under Smoke

This source-level interface belongs to Smoke, not to the nix.xd
or Nix-Alpine bootstrap. The dated source checkout is
`smoke20261112`, on branch `v20261112`; the GHXD Go implementation
remains part of Smoke. Exact commit SHAs, not branch names, select
runtime binaries and downstream qualification sources.

## Infrastructure prerequisites

Nix-Alpine already exists, with its owner and writable Nix store.
The existing Podman Machine can run short-lived rootless helper
containers with that store mounted read-only. The host has its SSH/
Podman connectivity configured. The operator's private age identity
and encrypted SOPS token are stored in the existing labeled GHXD
secret volume, never in the Nix-Alpine writer or Nix store.

Use nix.xd's existing **generic** workspace/stage/project-host-run
targets. From `nix.xd20261112`:

```sh
make workspace WORKSPACE_REPO=xd-dash/smoke WORKSPACE_VERSION=20261112
make workspace WORKSPACE_REPO=dash-xd/agni WORKSPACE_VERSION=20261112
make stage CHECKOUT=smoke20261112

make project-host-run PROJECT=smoke20261112 APPLICATION=ghxd ACTION=provision
make project-host-run PROJECT=smoke20261112 APPLICATION=ghxd ACTION=inventory
```

`provision` delegates to the generic `state-volume-provision` capability.
It reuses the existing `nix-xd-state-secret-ghxd`,
`nix-xd-state-control-ghxd-home`, and
`nix-xd-state-control-ghxd-workspaces` volumes without resetting or
copying credentials.

A caller-supplied application source selection is validated by Smoke.
For the existing VM1 candidate, Agni owns the selected exact revisions:

```sh
make project-host-run \
  PROJECT=smoke20261112 APPLICATION=ghxd ACTION=source-sync \
  SMOKE_GHXD_SOURCE_MANIFEST="$HOME/workspace/agni20261112/local/fatline/qualification/vm1-source-selection-20261112.json"
```

This action validates `<repository>YYYYMMDD` checkout paths, a common
valid calendar date, exact 40-hex source pins and duplicate paths.
It decrypts the OAuth token only in a restricted short-lived container
and exports a source archive without Git administrative state to
the Nix-Alpine writer. The writer gets the exact source bytes and
`.nix-xd-source-revision`, never the OAuth token.

If the existing credential is absent and GitHub device authorization
is required, use the Smoke-owned actions:

```sh
make project-host-run PROJECT=smoke20261112 APPLICATION=ghxd \
  ACTION=auth-start SMOKE_GHXD_CLIENT_ID=Iv1.YOUR_PUBLIC_CLIENT_ID
# Authorize the displayed device code in the browser.
make project-host-run PROJECT=smoke20261112 APPLICATION=ghxd ACTION=auth-finish
```

The operator also exposes `ACTION=auth-refresh` and
`ACTION=bootstrap`. Do not run `auth-start` to replace an existing
credential; the operator deliberately refuses that condition.

## Source versus state versioning

Each daily workspace has its own `vYYYYMMDD` branch and dated
checkout name. Immutable source SHA pins remain separate from the date.
Persistent credential volumes and Nix GC roots are **not** renamed
daily. A new Smoke version must not overwrite a staged checkout with
different source contents; select a new date/generation instead.

The root Smoke flake exposes `#source-build` and the optional
`#credential-runtime-adapter` package. These build and wrap the same
selected Smoke source. There is no independently versioned GHXD binary
or separate GHXD flake. GitHub commands are dispatched through the
compiled `smoke ghxd` interface and its Go package implementations.
The optional shell adapter handles runtime SOPS/age credential storage,
Podman volume isolation, and source export; those deployment concerns are
not an additional GHXD tool framework. nix.xd provides only the
selected host/runtime context.

## Qualification

`python3 local/ghxd/test-source-version.py` checks dated source
contracts, and the Smoke GHXD CI workflow checks the operator and
host action syntax. This is static qualification, not proof of
a successful live GHXD source-sync or token refresh on the Mac.
