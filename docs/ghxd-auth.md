# ghxd GitHub authentication

`ghxd` owns GitHub-provider credential semantics outside Smoke core. The exact-pinned `github-device-auth` Go tool is deliberately stateless: it only requests a device code, polls GitHub for a token response, and exchanges a refresh token for a replacement token response. It does not own files, repository secrets, GCS, or deployment state.

`ghxd` owns the transport/checkpoint bundle shape used by callers:

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

The bundle is an in-memory/input-output value, not a Smoke credential database. Smoke/ghxd does not persist it to disk.

Normal device login is stateless and emits one complete bundle:

```bash
smoke ghxd bootstrap
smoke ghxd auth login <github-device-flow-client-id>
```

For an existing checkpoint supplied by the caller:

```bash
export GITHUB_DEVICE_TOKEN_BUNDLE='...'

current="$(smoke ghxd auth ensure \
  --bundle-env GITHUB_DEVICE_TOKEN_BUNDLE)"

export GH_TOKEN="$(jq -r '.access_token' <<<"$current")"
```

`auth ensure` strictly decodes the bundle. If the access token remains fresh outside the safety margin, it emits the same bundle. If stale, it invokes the stateless `github-device-auth refresh <client-id> <refresh-token>` primitive, requires a complete replacement access+refresh response with both expiry durations, constructs a new ghxd bundle, and emits it. `auth refresh` forces that transformation regardless of access-token freshness.

The caller owns persistence. In Huram Actions the durable checkpoint is `secrets.HURAM_GITHUB_DEVICE_TOKEN`; Huram injects that JSON, calls ghxd, projects the returned access token as `GH_TOKEN`, and replaces the same repository secret if the returned pair rotated. Repository-secret write policy, retries, concurrency, and permissions therefore belong to Huram rather than github-device-auth or ghxd.

The old HTTP router is a separate adapter around the same device-flow primitives. Its historical GCS cache was a convenience for a remotely hosted stateful HTTP deployment. It is not part of ghxd, and ghxd has no router, GCS, WIF, GCP, or local credential-file dependency.

Ownership is therefore:

```text
github-device-auth
    GitHub device/poll/refresh protocol primitives

ghxd
    bundle schema + freshness/refresh transformation

Huram / other caller
    durable credential persistence + GH_TOKEN projection

router
    optional HTTP adapter
```
