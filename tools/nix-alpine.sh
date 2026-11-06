#!/usr/bin/env bash
set -euo pipefail

# Smoke owns its own workspace/testing lifecycle; Nix-Alpine supplies Go.
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
action="${1:-test}"
shift || true

require_go() {
  command -v go >/dev/null || {
    echo "Go toolchain missing; enter the Nix-Alpine Go devShell first" >&2
    exit 2
  }
}
refresh() {
  local branch
  branch="$(git symbolic-ref -q --short HEAD)" || {
    echo "Refusing to refresh detached Smoke checkout" >&2; exit 2;
  }
  [[ -z "$(git status --porcelain --untracked-files=no)" ]] || {
    echo "Refusing to overwrite tracked Smoke changes" >&2; exit 2;
  }
  git fetch origin "$branch"
  git merge --ff-only "origin/$branch"
  printf 'Smoke source: %s\n' "$(git rev-parse HEAD)"
}

case "$action" in
  refresh)
    refresh
    ;;
  refresh-test)
    refresh
    exec bash "$root/tools/nix-alpine.sh" test
    ;;
  test)
    require_go
    export GOWORK=off CGO_ENABLED=0
    go test -count=1 ./...
    ;;
  vet)
    require_go
    export GOWORK=off CGO_ENABLED=0
    go vet ./...
    ;;
  inspect|commands|compose|env)
    require_go
    export GOWORK=off CGO_ENABLED=0
    exec go run ./cmd/smoke "$action" "$@"
    ;;
  *)
    echo "Usage: tools/nix-alpine.sh {refresh|refresh-test|test|vet|inspect|commands|compose|env}" >&2
    exit 2
    ;;
esac
