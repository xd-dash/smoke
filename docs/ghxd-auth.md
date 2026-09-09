# ghxd local GitHub authentication

`ghxd` keeps GitHub OAuth/device-flow credential lifecycle outside Smoke core. The exact-pinned `github-device-auth` Go tool owns device login, refresh, bundle validation and local persistence; Smoke provides the GitHub-specific operator surface.

The local checkpoint defaults to:

```text
~/.local/share/smoke/ghxd/credentials/github-device.json
```

`SMOKE_DATA_HOME` overrides the Smoke data root. Otherwise `XDG_DATA_HOME` is honored before the conventional `~/.local/share` fallback.

The checkpoint is one atomic JSON document containing the client ID, access token, refresh token and both expiry boundaries. Access and refresh tokens are never persisted separately because a refresh rotates the pair together.

Normal use:

```bash
smoke ghxd bootstrap
smoke ghxd auth login <github-device-flow-client-id>
smoke ghxd auth status
smoke ghxd auth ensure
export GH_TOKEN="$(smoke ghxd auth token)"
```

`auth ensure` leaves a still-fresh credential unchanged. Inside the safety margin it refreshes locally through GitHub's OAuth token endpoint and atomically replaces the `0600` checkpoint.

To mirror the complete current checkpoint into a repository Actions secret:

```bash
smoke ghxd auth sync \
  --repo xd-dash/huram-abi-master \
  --secret HURAM_GITHUB_DEVICE_TOKEN
```

The repository secret is an Actions checkpoint, not a readable object store: arbitrary clients can update/list secret metadata but cannot read its plaintext back. An Actions workflow receives the bundle only when GitHub injects `${{ secrets.HURAM_GITHUB_DEVICE_TOKEN }}`.

Inside Actions, import that injected value before ensuring freshness:

```bash
export GITHUB_DEVICE_TOKEN_BUNDLE='${{ secrets.HURAM_GITHUB_DEVICE_TOKEN }}'
smoke ghxd auth import --repo-secret-env GITHUB_DEVICE_TOKEN_BUNDLE
smoke ghxd auth ensure
export GH_TOKEN="$(smoke ghxd auth token)"
```

If `ensure` refreshes the pair, synchronize the new bundle back to the repository secret before the run ends. Huram's reusable GitHub credential action performs that step automatically.

The historical `github-device-auth-router` may remain deployed for browser/serverless consumers and bootstrap/recovery, but it is not part of ghxd's normal authentication path. ghxd must not require WIF, GCP function discovery or GCS access to refresh an already-established device-flow credential.
