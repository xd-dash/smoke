#!/usr/bin/env bash
set -Eeuo pipefail

component=''
agni=''
builder=''
evidence=''
while (($#)); do
  case "$1" in
    --component) component="$2"; shift 2 ;;
    --agni) agni="$2"; shift 2 ;;
    --builder) builder="$2"; shift 2 ;;
    --evidence) evidence="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

case "$component" in
  redis|logma|skymill|nginx) ;;
  *) echo "unsupported component: $component" >&2; exit 2 ;;
esac
[[ -d "$agni/.git" ]] || { echo "Agni checkout required" >&2; exit 2; }
[[ -n "$builder" ]] || { echo "Nix-Alpine builder required" >&2; exit 2; }

if [[ -n "$evidence" ]]; then
  mkdir -p "$(dirname "$evidence")"
  printf '{"schema":1,"component":"%s","complete":false}\n' "$component" > "$evidence"
fi

echo "No verified Smoke runtime qualification for $component yet; refusing promotion" >&2
exit 1
