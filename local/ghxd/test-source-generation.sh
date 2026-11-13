#!/usr/bin/env bash
set -Eeuo pipefail

script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/activate-source-generation.sh"
root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
export SMOKE_GHXD_STAGE_ROOT="$root"
first="$(printf 'a%.0s' {1..40})"
second="$(printf 'b%.0s' {1..40})"
third="$(printf 'c%.0s' {1..40})"
checkout=example20261112
mkdir -p "$root/.source-generations/$checkout/$first" "$root/.source-generations/$checkout/$second"
printf '%s\n' "$first" > "$root/.source-generations/$checkout/$first/.nix-xd-source-revision"
printf '%s\n' "$second" > "$root/.source-generations/$checkout/$second/.nix-xd-source-revision"
sh "$script" "$checkout" "$first"
[[ -L "$root/$checkout" && "$(cat "$root/$checkout/.nix-xd-source-revision")" == "$first" ]]
sh "$script" "$checkout" "$second"
[[ -L "$root/$checkout" && "$(cat "$root/$checkout/.nix-xd-source-revision")" == "$second" ]]
[[ -f "$root/.source-generations/$checkout/$first/.nix-xd-source-revision" ]]
sh "$script" "$checkout" "$second"

legacy=legacy20261112
mkdir -p "$root/$legacy" "$root/.source-generations/$legacy/$second"
printf '%s\n' "$third" > "$root/$legacy/.nix-xd-source-revision"
printf 'retain\n' > "$root/$legacy/legacy-data"
printf '%s\n' "$second" > "$root/.source-generations/$legacy/$second/.nix-xd-source-revision"
sh "$script" "$legacy" "$second"
[[ -L "$root/$legacy" ]]
[[ "$(cat "$root/.source-generations/$legacy/$third/legacy-data")" == retain ]]
[[ "$(cat "$root/$legacy/.nix-xd-source-revision")" == "$second" ]]
if sh "$script" "$legacy" badref >/dev/null 2>&1; then
  echo 'Unpinned source generation accepted' >&2
  exit 1
fi
echo SMOKE_SOURCE_GENERATION=passed
