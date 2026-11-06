# Native SOPS/age in minimal ghxd Smoke

This branch compiles `cmd/ghxd-minimal` with the optional
`cmd/secrets` component. The existing Go composition registry, environment
snapshots, and self-rebuild mechanism remain authoritative. Logmash is not
included in this executable.

## Cryptography and process boundary

The reusable Go package `github.com/xd-dash/smoke/secrets` uses
`filippo.io/age` for X25519 identity management and pinned
`github.com/getsops/sops/v3` APIs for real SOPS JSON encryption and
authenticated decryption. **No `sops` or `age-keygen` child processes
are invoked by normal Smoke secret operations.** The optional
`cmd/secrets` command only adapts the library to CLI calls.

```
smoke secrets seal-json output.enc.json          # plaintext JSON on stdin
smoke secrets verify-json output.enc.json        # redacted success/failure
smoke secrets inspect output.enc.json            # ciphertext metadata/digest
smoke secrets exec-env output.enc.json -- program args...
smoke secrets exec-file output.enc.json -- program --config '{}'
```

`exec-env` requires a JSON object mapping environment names to strings.
It performs one SOPS in-process decryption and Linux process replacement
(`syscall.Exec`). No separate Smoke supervisor persists. Avoid using it to
pass secrets to Nix evaluation, Nix builders, or untrusted executables.

`exec-file` uses a Linux sealed `memfd` rather than disk-backed plaintext.
The `{}` placeholder becomes `/proc/self/fd/<fd>`. The descriptor remains
open across process replacement and is available to the target. Applications
that need an ordinary persistent file or fork/close inherited descriptors
are not compatible with this mode. Non-Linux hosts fail closed.

Future native Smoke commands can consume the same reusable `secrets`
package without subprocesses or process-global `os.Setenv` mutation.
A scoped native command invocation interface is not yet implemented.

## Age source of truth

**The canonical bootstrap Nix-Alpine Podman scope owns the age identity.**
The rootless Podman Machine holds a dedicated named volume, conventionally
`nix-xd-smoke-podman-age`. Its writer mount is:

```
/run/secrets/age/keys.txt       (0600, uid:gid 1000:1000)
```

The first bootstrap generates that identity using the pinned
`nix-alpine/toolbox#security` flake (age and SOPS CLI available for
bootstrap/diagnostics only). Later bootstraps reuse the same identity.
An existing encrypted credential with a missing age identity makes
bootstrap fail closed instead of generating an incompatible key.

A second Nix-Alpine scope mounts **the same named age volume read-only**,
with its own separate Nix store and a read-only source workspace.
The private identity is not stored in macOS configuration, in `/workspaces`,
in an OCI image, or in `/nix/store`. The shared ciphertext is at
`/workspaces/.private/sops/ghxd.enc.json`, excluded by `nix.xd/.gitignore`.

Both containers are deliberately authorized to decrypt the same ciphertext.
Read-only filesystem access cannot prevent disclosure by a holder of the key;
use a different identity/volume if a scope must not decrypt secrets.

## GitHub user device authentication

```
smoke ghxd-secrets auth
smoke ghxd-secrets status
```

The first command requests a GitHub device code, accepts interactive user
approval, and **requires both an access token and refresh token**. It
encrypts the JSON credential in-process to the bootstrap-owned age
recipient, saves ciphertext with mode 0600 using an exclusive atomic
publish, and refuses to overwrite an existing credential. It never prints
either token.

The `status` command decrypts and verifies the SOPS document in-process
and reports only a redacted status, refresh-token presence and granted
scopes. Real token refresh/rotation and validation against the GitHub API
are separate follow-on work. A GitHub App user token is not an
installation token and may lack permission to update Actions secrets.

`xd-dash/nix.xd` provides the logged `make qualify-ghxd-auth` gate.
It bootstraps both Podman scopes, performs a synthetic cross-container
SOPS roundtrip, and then **pauses only for device approval**. The
verification URL and short-lived code go directly to the controlling TTY
instead of the diagnostic log. The actual token is encrypted before
the final gate verifies it can be read by both scopes. The gate does not
remove existing Apple Container resources or previous Nix store volumes.
