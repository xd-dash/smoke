#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

action="${1:?Select source-sync or auth-refresh}"
case "$action" in source-sync|auth-refresh) ;; *) echo "Unsupported native GHXD action" >&2; exit 2;; esac
[[ "$(uname -s)" == Linux && "$(id -u)" == 1000 ]] || exit 2
[[ -n "${SMOKE_GHXD_BINARY:-}" && "$SMOKE_GHXD_BINARY" == /nix/store/*-smoke && -x "$SMOKE_GHXD_BINARY" ]] || exit 2
[[ -d /workspaces && -w /workspaces ]] || exit 2

grants=/run/nix-xd/mounts
for name in ghxd-secret ghxd-home ghxd-workspaces; do
  p="$grants/$name"
  [[ -d "$p" && ! -L "$p" && -r "$p" && -w "$p" ]] || {
    echo "SMOKE_GHXD_NATIVE=blocked reason=missing-grant name=$name" >&2
    exit 1
  }
  [[ "$(stat -c %u "$p")" == 1000 ]] || {
    echo "SMOKE_GHXD_NATIVE=blocked reason=wrong-grant-owner name=$name" >&2
    exit 1
  }
done
export HOME="$grants/ghxd-home" SMOKE_GHXD_NATIVE=1
source_root="$grants/ghxd-workspaces"
operator="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/operator.sh"

if [[ "$action" == auth-refresh ]]; then
  [[ $# == 1 ]] || exit 2
  bash "$operator" auth-refresh
  echo 'SMOKE_GHXD_NATIVE=passed action=auth-refresh execution=nix-alpine'
  exit 0
fi

[[ $# == 2 ]] || { echo 'source-sync requires a staged manifest path' >&2; exit 2; }
manifest="$2"
[[ "$manifest" == /workspaces/* && "$manifest" != *..* && -f "$manifest" && ! -L "$manifest" ]] || exit 2
real="$(readlink -f -- "$manifest")"
[[ "$real" == /workspaces/* ]] || { echo 'Manifest escapes staged workspace' >&2; exit 2; }
python3 "$(dirname "$operator")/validate-source.py" "$manifest"
payload="$(cat "$manifest")"
printf '%s\n' "$payload" | bash "$operator" source-sync

rows="$(python3 -c '
import json,sys
d=json.load(sys.stdin)
for item in d["sources"].values():
    print(item["checkout"] + chr(9) + item["revision"])
' <<< "$payload")"
while IFS=$'\t' read -r checkout sha; do
  [[ -n "$checkout" && "$sha" =~ ^[0-9a-f]{40}$ ]] || exit 2
  selected="$(SMOKE_GHXD_SOURCES_ROOT="$source_root" sh "$(dirname "$operator")/select-source-path.sh" "$checkout" "$sha")"
  [[ "$(git -C "$selected" rev-parse HEAD)" == "$sha" ]] || {
    echo "SMOKE_GHXD_NATIVE=blocked reason=worktree-revision-mismatch checkout=$checkout" >&2
    exit 1
  }
  [[ -z "$(git -C "$selected" status --porcelain --untracked-files=no)" ]] || exit 1
  generation="/workspaces/.source-generations/$checkout/$sha"
  if [[ ! -e "$generation" ]]; then
    mkdir -p "/workspaces/.source-generations/$checkout"
    temporary="$(mktemp -d "/workspaces/.source-generations/$checkout/.incoming.XXXXXXXX")"
    if ! git -C "$selected" archive HEAD | tar -xf - -C "$temporary"; then
      rm -rf -- "$temporary"
      echo "SMOKE_GHXD_NATIVE=blocked reason=archive-extraction-failed checkout=$checkout" >&2
      exit 1
    fi
    printf '%s\n' "$sha" > "$temporary/.nix-xd-source-revision"
    mv -T -- "$temporary" "$generation"
    echo "GHXD_GENERATION=staged checkout=$checkout sha=$sha"
  else
    [[ "$(cat "$generation/.nix-xd-source-revision")" == "$sha" ]] || exit 1
    echo "GHXD_GENERATION=reused checkout=$checkout sha=$sha"
  fi
  SMOKE_GHXD_STAGE_ROOT=/workspaces sh "$(dirname "$operator")/activate-source-generation.sh" "$checkout" "$sha"
done <<< "$rows"
echo 'GHXD_WORKSPACE_SYNC=passed'
echo 'SMOKE_GHXD_NATIVE=passed action=source-sync execution=nix-alpine'
