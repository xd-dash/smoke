# Minimal ghxd Smoke with local SOPS/age

This branch builds `cmd/ghxd-minimal` instead of the normal Logmash-composed
`cmd/smoke`. It keeps the existing Smoke environment, composition and
GitHub-tool contracts, but **does not link Logmash into the executable**.
The first local composition manifest is `{"components":[]}`, and a manifest
containing Logmash is rejected rather than silently migrated.

## Relation to huram-abi

The historical `xd-dash/huram-abi` workflow invoked
`dash-xd/bootstrap-secrets` to restore or generate `SOPS_AGE_KEY`, derive
its public recipient, write `.sops.yaml`, and upload the age identity as a
GitHub Actions secret. This local bootstrap deliberately does **not** upload
private material to GitHub or expose it as a container environment variable.

The macOS host holds the private key at
`~/.config/nix.xd/age/keys.txt` (mode 0600), outside the workspace.
`nix.xd` may reuse an existing conventional SOPS identity or generate a new
identity if no existing identity and no encrypted local data are found.
An older identity retained exclusively in GitHub Actions secrets cannot be
retrieved from GitHub; new keys cannot decrypt its existing ciphertext.

Only explicitly selected Podman Quadlets receive the key mounted read-only at
`/run/secrets/age/keys.txt`; their `SOPS_AGE_KEY_FILE` environment variable
contains the *path*, never the secret. SOPS ciphertext is local under
`/workspaces/.private/sops` (gitignored). Two authorized Nix-Alpine scopes
can independently decrypt the same ciphertext while keeping separate Nix
stores. Reader source mounts are read-only. The private age key should not
be committed, included in OCI images, copied to `/nix/store`, or uploaded
as a CI artifact.

## Authentication

The minimal executable exposes:

```
smoke ghxd auth
smoke ghxd status
smoke ghxd bootstrap
```

`auth` uses GitHub's device authorization flow with
`GITHUB_CLIENT_ID` (the public OAuth/GitHub App **client ID**, not the App ID).
The user must authorize the displayed device code in a browser. On success,
the access token and any refresh token are piped from Go process memory
directly through `sops encrypt` and saved to
`/workspaces/.private/sops/ghxd.enc.json` with mode 0600.
The code refuses to overwrite an existing encrypted credential. An explicit
refresh-and-rotation command remains future work; re-running `auth` does
not silently invalidate a stored token.

`status` decrypts in process memory and prints only whether the access token
is readable, whether a refresh token exists, and its scopes. It does not
print the token bytes. This command can be run from both authorized scopes
and its redacted output can be logged.

The host's `make ghxd-auth GITHUB_CLIENT_ID=...` is intentionally
**not logged**, because the GitHub device authorization user code is
short-lived credential material. `make ghxd-token-check` is the logged,
two-container check after authorization.

GitHub App *user* device-flow tokens are permission-limited OAuth tokens;
they are not installation access tokens, and they do not automatically
provide the Actions secrets-write permissions used by older huram-abi
automation. Grant only the necessary app permissions and do not assume this
user credential can update GitHub Actions secrets.

## Qualification

`make qualify-ghxd` in `nix.xd` builds the minimal branch, establishes
persistent Go caches, bootstraps two isolated Nix-Alpine scopes, exports and
imports an exact Nix security-toolchain closure via the shared file cache,
and verifies an encrypted **synthetic** probe can be decrypted by both
containers. It then refreshes the writer and reruns the Go tests with
`GOPROXY=off` to prove reuse of the persistent module cache. It never
needs a real GitHub token for this qualification and never prints the age
identity or decrypted probe contents.

Only one scope mutates the shared workspace; the reader has an explicitly
read-only bind mount. A shared age identity means **both authorized containers
have decryption authority**. Container separation is not a security boundary
between those two holders of the same private key.
