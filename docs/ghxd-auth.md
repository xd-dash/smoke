# ghxd local GitHub authentication

`ghxd` keeps GitHub OAuth/device-flow credential lifecycle outside Smoke core. The exact-pinned `github-device-auth` Go tool owns device login, refresh, strict bundle validation, atomic local persistence and repository-secret synchronization; Smoke provides the GitHub-specific operator surface.

The local checkpoint defaults to:

```text
~/.local/share/smoke/ghxd/credentials/github-device.json
```

`SMOKE_DATA_HOME` overrides the Smoke data root. Otherwise `XDG_DATA_HOME` is honored before the conventional `~/.local/share` fallback.

The checkpoint is one atomic JSON document:

```json
{
  "version": 1,
  "client_id": "...",
  "access_token": "...",
  "refresh_token": "...",
  "access_token_expires_at": "...",
  "refresh_token_expires_at": "..."
}
```

Access and refresh tokens are never persisted separately because a refresh rotates the pair together.

Normal use:

```bash
smoke ghxd bootstrap
smoke ghxd auth login <github-device-flow-client-id>
smoke ghxd auth status
smoke ghxd auth ensure
export GH_TOKEN="$(smoke ghxd auth token)"
```

`auth ensure` leaves a still-fresh credential unchanged. Inside the safety margin it refreshes locally through GitHub's OAuth token endpoint and atomically replaces the `0600` local checkpoint before any repository-secret writeback is attempted.

To mirror the complete current checkpoint into the repository Actions secret:

```bash
smoke ghxd auth sync \
  --repo xd-dash/huram-abi-master \
  --secret HURAM_GITHUB_DEVICE_TOKEN
```

There is deliberately one remote credential checkpoint. `auth sync` writes the complete JSON bundle to that one repository secret with bounded retries.

The local file is the transient durability boundary for the running process. If OAuth refresh succeeds but repository-secret synchronization fails, the freshly rotated pair remains in the local `0600` checkpoint. The operation fails closed and may retry synchronization from that same local bundle while the runner remains alive. It must not overwrite the local bundle with the stale remote value and must not create a second repository-secret recovery rail merely to model this narrow failure window.

If the runner-local copy is lost before remote synchronization succeeds, normal recovery is a new device login or an explicit external import of a complete credential bundle. The source of an imported bundle is outside ghxd's architecture.

Repository secrets are Actions checkpoints, not readable object storage: arbitrary clients can update/list secret metadata but cannot retrieve secret plaintext. An Actions workflow receives the bundle only when GitHub injects the corresponding `${{ secrets.* }}` value.

Inside Actions the primitive flow is:

```bash
export GITHUB_DEVICE_TOKEN_BUNDLE='${{ secrets.HURAM_GITHUB_DEVICE_TOKEN }}'
smoke ghxd auth import --repo-secret-env GITHUB_DEVICE_TOKEN_BUNDLE
smoke ghxd auth ensure
export GH_TOKEN="$(smoke ghxd auth token)"
```

If `ensure` refreshes the pair, synchronize that complete new bundle back to `HURAM_GITHUB_DEVICE_TOKEN` before the run ends.

`ghxd` has no router, GCS, WIF or GCP dependency for normal authentication. `github-device-auth` may have other optional deployment adapters elsewhere, but those are not part of Smoke's credential model.
