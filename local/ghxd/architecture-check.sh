#!/usr/bin/env bash
set -Eeuo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
test -s "$root/docs/composition-contract-v20261115.md"
grep -Fq 'transitional adapters' "$root/docs/composition-contract-v20261115.md"
grep -Fq 'exec ${pkgs.bash}/bin/bash ${./local/ghxd/operator.sh}' "$root/flake.nix"
grep -Fq 'exec bash' "$root/local/ghxd/host-actions/source-sync.sh"
grep -Fq 'SMOKE_GHXD_SOURCE_MANIFEST' "$root/local/ghxd/host-operator.sh"
grep -Fq 'GHXD_WORKSPACE_SYNC=passed' "$root/local/ghxd/host-operator.sh"
grep -Fq 'GIT_TERMINAL_PROMPT=0' "$root/local/ghxd/operator.sh"
! grep -Fq -- '--privileged' "$root/local/ghxd/host-operator.sh"
echo 'SMOKE_ARCHITECTURE_CHECK=passed core=go ghxd=smoke-owner host_adapter=transitional'
