#!/usr/bin/env sh
set -eu

checkout=${1:?checkout}
revision=${2:?exact revision}
root=${SMOKE_GHXD_STAGE_ROOT:-/workspaces}

case "$checkout" in *[!a-z0-9.-]* | "" | .* | *..*) echo "Invalid checkout" >&2; exit 2;; esac
case "$revision" in *[!0-9a-f]* | "") echo "Invalid revision" >&2; exit 2;; esac
[ "${#revision}" -eq 40 ] || exit 2

generation="$root/.source-generations/$checkout/$revision"
alias="$root/$checkout"
[ -f "$generation/.nix-xd-source-revision" ] || { echo "Missing verified source generation" >&2; exit 1; }
[ "$(cat "$generation/.nix-xd-source-revision")" = "$revision" ] || exit 1

lock="$root/.source-generations/$checkout/.activate-lock"
mkdir "$lock" 2>/dev/null || { echo "Source activation already in progress" >&2; exit 1; }
trap 'rmdir "$lock"' EXIT

if [ -e "$alias" ] || [ -L "$alias" ]; then
  previous="$(cat "$alias/.nix-xd-source-revision" 2>/dev/null || true)"
  case "$previous" in *[!0-9a-f]* | "") echo "Unrecognized preexisting source alias" >&2; exit 1;; esac
  [ "${#previous}" -eq 40 ] || exit 1
  if [ "$previous" = "$revision" ]; then
    echo "GHXD_STAGE=reused checkout=$checkout sha=$revision"
    exit 0
  fi
  if [ -L "$alias" ]; then
    alternate="$root/.source-generations/$checkout/.alias-$$"
    ln -s "$generation" "$alternate"
    mv -Tf "$alternate" "$alias"
  else
    archive="$root/.source-generations/$checkout/$previous"
    [ ! -e "$archive" ] || { echo "Previous source generation already exists; refusing overwrite" >&2; exit 1; }
    mv "$alias" "$archive"
    ln -s "$generation" "$alias"
  fi
  echo "GHXD_STAGE=reconciled checkout=$checkout previous=$previous sha=$revision retained=true"
else
  ln -s "$generation" "$alias"
  echo "GHXD_STAGE=activated checkout=$checkout sha=$revision"
fi
