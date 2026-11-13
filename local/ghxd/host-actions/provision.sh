#!/usr/bin/env bash
set -Eeuo pipefail
: "${NIX_XD_RUNTIME_REPO:?Select the generic host infrastructure checkout}"
machine="${NIX_XD_MACHINE:?Select existing Podman Machine}"
scope="${NIX_XD_SCOPE:?Select the authoritative Nix-Alpine owner}"
for entry in secret:ghxd control:ghxd-home control:ghxd-workspaces; do
  kind="${entry%%:*}"
  name="${entry#*:}"
  make -s -C "$NIX_XD_RUNTIME_REPO" state-volume-provision \
    PROVIDER=podman MACHINE="$machine" SCOPE="$scope" \
    STATE_KIND="$kind" STATE_NAME="$name"
done
echo 'SMOKE_GHXD_VOLUMES=passed retained=true'
