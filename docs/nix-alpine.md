# Nix-Alpine as a Smoke tool-workspace component

Smoke environments are Go workspaces; they do not themselves execute Nix flakes.
The Nix-Alpine command is an optional Go component imported into the Smoke
composition. Its generated rootless Podman Quadlets own the Nix runtime.

## Prerequisites

Go 1.26+, Podman with Quadlet, rootless user systemd, Git.

```sh
go install github.com/xd-dash/smoke/cmd/smoke@latest
smoke env create nix-tools
smoke env use nix-tools /absolute/path/to/nix-alpine
smoke compose add github.com/dash-xd/nix-alpine/smoke
smoke commands
```

Until the component is promoted to nix-alpine main, pin the module's
`codex/smoke-single-user-component` commit in the local Go composition;
`smoke compose add` alone resolves published module versions.

Build the image from the Nix-Alpine repository:

```sh
podman build -t localhost/nix-toolbox:latest /absolute/path/to/nix-alpine
```

Install an idle multi-flake workspace service:

```sh
smoke nix-alpine install --name dev --workspace "$HOME/workspaces" --idle
smoke nix-alpine start dev
podman exec nix-alpine-dev nix --version
podman exec nix-alpine-dev nix build --no-link /workspaces/example
```

Each scope has its own named volume at /nix. Do not run multiple writers
against the same scope. For reader handoff, stop the writer and mount its
volume read-only; verify that the reader does not require store mutation.
