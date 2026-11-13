#!/bin/sh
set -eu
checkout=${1:?dated checkout}
sha=${2:?exact revision}
root=${SMOKE_GHXD_SOURCES_ROOT:-/data/ghxd}
case "$checkout" in
  ''|.*|*..*|*[!a-zA-Z0-9._-]*) echo "Invalid source checkout" >&2; exit 2 ;;
esac
case "$sha" in
  ''|*[!0-9a-f]*) echo "Invalid source revision" >&2; exit 2 ;;
esac
[ "${#sha}" -eq 40 ] || exit 2
generation="$root/generations/$checkout/$sha"
legacy="$root/$checkout"
if [ -d "$generation" ] && [ -e "$generation/.git" ]; then
  selected="$generation"
elif [ -d "$legacy" ] && [ -e "$legacy/.git" ]; then
  # The preceding Smoke operator validates the exact revision of legacy
  # worktrees and creates a generation path whenever it differs.
  selected="$legacy"
else
  echo "Missing prepared GHXD worktree checkout=$checkout sha=$sha" >&2
  exit 1
fi
[ ! -L "$selected" ] || { echo "Source worktree is a symlink" >&2; exit 1; }
printf '%s\n' "$selected"
