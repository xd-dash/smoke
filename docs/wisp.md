# Wisp SSH transport: first qualification

Wisp is an optional compiled-in Smoke command (`cmd/wisp`). The example composition
`cmd/smoke-wisp` links Wisp and Logmash into one Smoke executable. The default Smoke
composition is unchanged. No external SSH process is used by these commands.

Build with Go 1.26 or newer:

```sh
go build -o smoke-wisp ./cmd/smoke-wisp
go test -race ./wisp ./cmd/wisp
```

Use an already provisioned endpoint and its existing trusted OpenSSH known_hosts
file. Unknown or changed host keys fail closed. This first slice supports an
unencrypted private key file; agent and interactive encrypted-key unlocking are
not implemented. There is no implicit SSH config, ProxyJump or trust-on-first-use.

```sh
./smoke-wisp wisp qualify --address 127.0.0.1:2222 --user wisp \
  --identity "$HOME/.ssh/starwisp-test" --known-hosts /absolute/path/known_hosts
```

The gate verifies rootless guest execution, the user systemd bus and Podman,
then distinct success, exit-1 and RuntimeMaxSec timeout results. Its transient
units have unique UUID names and cleanup targets only those names. It does not
provision or reboot the guest, test Redis, release roots, or qualify full Fatline.
An interrupted SSH connection alone is not proof that remote work stopped.

Remote argv is individually shell-quoted (SSH exec itself transports a command
string). Stdin/stdout/stderr are streamed and the package returns SSH exit errors.
The current Smoke dispatcher reports errors; it does not promise the same numeric
process exit code as the remote command. Interactive PTYs are outside this slice.

```sh
./smoke-wisp wisp exec --identity /key --known-hosts /known_hosts -- /usr/bin/id -u
mkdir -m 700 /private/parent
./smoke-wisp wisp forward --identity /key --known-hosts /known_hosts \
  --listen /private/parent/vm1.sock --target-network tcp \
  --target 127.0.0.1:16379 --timeout 10m
```

The Unix listener is mode 0600 inside a private existing directory. An existing
path is never unlinked to take ownership. The listener is removed when closed.
`WISP_FORWARD_READY=true` means the local listener and authenticated SSH connection
are ready; the destination service is checked when each client connects. Forwarding
TCP to nginx preserves nginx -> Redis Unix-socket ingress. `--target-network unix`
can dial a guest socket directly if the SSH user has access, bypassing nginx.
Byte forwarding does not carry original Unix peer credentials or file descriptors.

Cancellation/deadlines close owned connections and wait for forwarding workers.
Runtime supervision stays with guest systemd. This is not a resident controller,
recovery engine or new workload declaration format. Make remains the host interface;
Agni and nix.xd continue owning deployment declarations and retained build outputs.

Tests use a local in-process SSH server for authentication, streams, exit errors,
host rejection, cancellation and TCP/streamlocal channels. Restricted runners may
skip AF_UNIX listener tests explicitly; run those tests on the target Mac before
claiming socket permissions and cleanup are qualified.
