#!/usr/bin/env bash
set -Eeuo pipefail
script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/select-source-path.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export SMOKE_GHXD_SOURCES_ROOT="$tmp"
sha="$(printf 'a%.0s' {1..40})"
other="$(printf 'b%.0s' {1..40})"
checkout=probot-runtime20261112
mkdir -p "$tmp/$checkout"
printf 'gitdir: /data/object-db/worktrees/test\n' > "$tmp/$checkout/.git"
[[ "$(sh "$script" "$checkout" "$sha")" == "$tmp/$checkout" ]]
mkdir -p "$tmp/generations/$checkout/$sha"
printf 'gitdir: /data/object-db/worktrees/generation\n' > "$tmp/generations/$checkout/$sha/.git"
[[ "$(sh "$script" "$checkout" "$sha")" == "$tmp/generations/$checkout/$sha" ]]
rm -f "$tmp/generations/$checkout/$sha/.git"
[[ "$(sh "$script" "$checkout" "$sha")" == "$tmp/$checkout" ]]
rm -f "$tmp/$checkout/.git"
if sh "$script" "$checkout" "$sha" >/dev/null 2>&1; then
  echo 'Missing Git metadata was accepted' >&2; exit 1
fi
if sh "$script" "$checkout" invalid >/dev/null 2>&1; then
  echo 'Invalid exact SHA was accepted' >&2; exit 1
fi
if sh "$script" '../bad' "$other" >/dev/null 2>&1; then
  echo 'Path traversal was accepted' >&2; exit 1
fi
echo SMOKE_GHXD_SOURCE_PATH=passed
