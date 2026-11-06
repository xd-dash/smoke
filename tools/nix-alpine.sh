#!/usr/bin/env bash
set -euo pipefail

# Smoke owns its own workspace/testing lifecycle; Nix-Alpine supplies Go.
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
action="${1:-test}"
shift || true

smoke_state() {
  local state="${SMOKE_STATE_DIR:-}"
  if [[ -z "$state" ]]; then
    if [[ -d /workspaces && -w /workspaces ]]; then
      state=/workspaces/.smoke-state
    elif [[ -d /exchange && -w /exchange ]]; then
      state=/exchange/smoke-state
    else
      echo "No writable shared workspace; set SMOKE_STATE_DIR" >&2
      return 2
    fi
  fi
  printf '%s\n' "$state"
}

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
  build)
    require_go
    state="$(smoke_state)"
    mkdir -p "$state/bin"
    staged="$(mktemp "$state/bin/.smoke-new-XXXXXXXX")"
    trap 'rm -f "$staged"' EXIT
    GOWORK=off CGO_ENABLED=0 go build -o "$staged" ./cmd/smoke
    chmod 0755 "$staged"
    mv -f "$staged" "$state/bin/smoke"
    trap - EXIT
    printf 'Installed Smoke binary at %s (source %s)\n' "$state/bin/smoke" "$(git rev-parse HEAD)"
    ;;
  inspect|commands|compose|env)
    require_go
    state="$(smoke_state)"
    binary="$state/bin/smoke"
    [[ -x "$binary" ]] || {
      echo "No installed Smoke binary; run tools/nix-alpine.sh build first" >&2
      exit 2
    }
    export SMOKE_ENV_DIR="${SMOKE_ENV_DIR:-$state/envs}"
    export SMOKE_COMPOSITION_MANIFEST="${SMOKE_COMPOSITION_MANIFEST:-$state/composition.json}"
    export SMOKE_COMPOSITION_DIR="${SMOKE_COMPOSITION_DIR:-$state/composition}"
    export SMOKE_LOCAL_SOURCE="${SMOKE_LOCAL_SOURCE:-$root}"
    export GOWORK=off CGO_ENABLED=0
    exec "$binary" "$action" "$@"
    ;;
  *)
    echo "Usage: tools/nix-alpine.sh {refresh|refresh-test|test|vet|build|inspect|commands|compose|env}" >&2
    exit 2
    ;;
esac
