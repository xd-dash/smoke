# ghxd local GitHub authentication

`ghxd` keeps GitHub OAuth/device-flow credential lifecycle outside Smoke core. The exact-pinned `github-device-auth` Go tool owns device login, refresh, strict bundle validation, atomic local persistence and repository-secret synchronization; Smoke provides the GitHub-specific operator surface.

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

To mirror the complete current checkpoint into repository Actions secrets:

```bash
smoke ghxd auth sync \
  --repo xd-dash/huram-abi-master \
  --secret HURAM_GITHUB_DEVICE_TOKEN \
  --recovery-secret HURAM_GITHUB_DEVICE_TOKEN_RECOVERY
```

The recovery checkpoint is written first and primary only after recovery succeeds. Each secret write uses bounded retries. The recovery secret is a transaction journal, not a second normal authentication rail.

The journal protects a failed primary cutover once GitHub accepts the newly rotated bundle. It cannot make the OAuth refresh exchange and GitHub's secret service one atomic transaction: if refresh succeeds while all repository-secret writes remain unavailable, the new local `0600` checkpoint is the only fresh copy. In that case fail closed and preserve that local state; explicit device login or the retained bootstrap/recovery path is required if the runner-local copy is lost.

Repository secrets are Actions checkpoints, not readable object storage: arbitrary clients can update/list secret metadata but cannot retrieve secret plaintext. An Actions workflow receives the bundle only when GitHub injects the corresponding `${{ secrets.* }}` value.

Inside Actions the preferred path is the reusable Huram credential action, which invokes the same Smoke/ghxd implementation rather than duplicating OAuth refresh logic. At the primitive level the flow is:

```bash
export GITHUB_DEVICE_TOKEN_BUNDLE='${{ secrets.HURAM_GITHUB_DEVICE_TOKEN }}'
smoke ghxd auth import --repo-secret-env GITHUB_DEVICE_TOKEN_BUNDLE
smoke ghxd auth ensure
export GH_TOKEN="$(smoke ghxd auth token)"
```

If `ensure` refreshes the pair, synchronize the complete new bundle before the run ends. Huram's reusable action also accepts the recovery checkpoint, falls back to it only when primary is unusable, and repairs primary through `smoke ghxd auth sync`.

The historical `github-device-auth-router` may remain deployed for browser/serverless consumers and explicit bootstrap/recovery, but it is not part of ghxd's normal authentication path. Normal ghxd authentication must not require WIF, GCP function discovery or GCS access to refresh an already-established device-flow credential.
