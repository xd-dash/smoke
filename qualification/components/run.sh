#!/usr/bin/env bash
# Smoke owns the disposable systemd-user lifecycle; Agni owns the Quadlet.
set -Eeuo pipefail
component='' agni='' builder='' evidence='' image=''
while (($#)); do
  case "$1" in
    --component) component="$2"; shift 2 ;;
    --agni) agni="$2"; shift 2 ;;
    --builder) builder="$2"; shift 2 ;;
    --evidence) evidence="$2"; shift 2 ;;
    --image) image="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
case "$component" in redis|logma|skymill|nginx) ;; *) echo "unsupported component: $component" >&2; exit 2 ;; esac
[[ -d "$agni/.git" ]] || { echo "Agni checkout required" >&2; exit 2; }
[[ -n "$builder" ]] || { echo "Nix-Alpine builder required" >&2; exit 2; }
[[ "$(podman info --format '{{.Host.Security.Rootless}}')" == true ]] || { echo "rootless Podman required" >&2; exit 1; }
if [[ "$component" == nginx ]]; then
  exec bash "$(dirname "$0")/nginx.sh" --agni "$agni" --builder "$builder" --image "${image:-localhost/fatline-nginx:candidate}" --evidence "$evidence"
fi
if [[ "$component" != redis ]]; then
  echo "Smoke $component runtime probe not implemented" >&2; exit 1
fi
if [[ -z "$image" ]]; then image=localhost/fatline-redis:candidate; fi
image_id="$(podman image inspect "$image" --format '{{.Id}}')"
image_id="sha256:${image_id#sha256:}"
[[ "$image_id" =~ ^sha256:[a-f0-9]{64}$ ]] || exit 1
entry="$(podman image inspect "$image" --format '{{index .Config.Entrypoint 0}}')"
[[ "$entry" == */bin/redis-server ]] || { echo "unexpected Redis entrypoint" >&2; exit 1; }
redis_cli="${entry%redis-server}redis-cli"
scope="redis-$$"
name="fatline-qualify-$scope"
unit="$name.service"
quadlet_dir="$HOME/.config/containers/systemd"
config_dir="$HOME/.config/fatline-qualify/$scope"
runtime_dir="${XDG_RUNTIME_DIR:?}/fatline-qualify/$scope"
quadlet="$quadlet_dir/$name.container"
work="$(mktemp -d)"
started=0
cleanup() {
  if [[ "$started" == 1 ]]; then systemctl --user stop "$unit" >/dev/null 2>&1 || true; fi
  rm -f -- "$quadlet"
  systemctl --user daemon-reload >/dev/null 2>&1 || true
  podman rm -f "$name" >/dev/null 2>&1 || true
  rm -rf -- "$config_dir" "$runtime_dir" "$work"
}
trap cleanup EXIT
mkdir -p "$quadlet_dir" "$config_dir" "$runtime_dir"
chmod 700 "$config_dir" "$runtime_dir"
out="$(podman exec "$builder" nix build --no-write-lock-file --no-link --print-out-paths /workspaces/agni/local/fatline#redis-qualification)"
podman cp "$builder:$out/quadlets/fatline-qualify-redis.container" "$work/template.container"
podman cp "$builder:$out/config/redis.conf" "$config_dir/redis.conf"
password="$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')"
printf 'user default off\nuser smoke on >%s ~smoke:* +ping +get +set\n' "$password" > "$config_dir/users.acl"
chmod 600 "$config_dir/users.acl" "$config_dir/redis.conf"
sed -e "s|__CANDIDATE_IMAGE_ID__|$image_id|g" \
    -e "s|fatline-qualify-redis|$name|g" \
    -e "s|fatline-qualify/redis|fatline-qualify/$scope|g" \
    "$work/template.container" > "$quadlet"
grep -Fxq "Image=$image_id" "$quadlet"
grep -Fxq "Network=none" "$quadlet"
systemctl --user daemon-reload
systemctl --user cat "$unit" >/dev/null
started=1
systemctl --user start "$unit"
for attempt in $(seq 1 40); do
  if podman exec "$name" "$redis_cli" -s /run/redis/redis.sock --user smoke -a "$password" --no-auth-warning PING 2>/dev/null | grep -qx PONG; then break; fi
  sleep 0.25
done
cli() { podman exec "$name" "$redis_cli" -s /run/redis/redis.sock --user smoke -a "$password" --no-auth-warning "$@"; }
[[ "$(cli PING)" == PONG ]]
[[ "$(cli SET smoke:allowed value)" == OK ]]
[[ "$(cli GET smoke:allowed)" == value ]]
[[ "$(cli SET foreign:denied value)" == *NOPERM* ]]
[[ "$(cli ACL LIST)" == *NOPERM* ]]
[[ "$(cli CONFIG GET maxmemory-policy)" == *NOPERM* ]]
# Inspect runtime settings using the actual mounted config and socket.
[[ -S "$runtime_dir/redis.sock" ]]
grep -Fxq 'maxmemory 40mb' "$config_dir/redis.conf"
grep -Fxq 'maxmemory-policy noeviction' "$config_dir/redis.conf"
grep -Fxq 'save ""' "$config_dir/redis.conf"
grep -Fxq 'appendonly no' "$config_dir/redis.conf"
running="$(podman inspect "$name" --format '{{.Image}}')"
running="sha256:${running#sha256:}"
[[ "$running" == "$image_id" ]]
systemctl --user restart "$unit"
for attempt in $(seq 1 40); do
  if cli PING 2>/dev/null | grep -qx PONG; then break; fi
  sleep 0.25
done
[[ "$(cli PING)" == PONG ]]
[[ "$(cli GET smoke:allowed)" == "" ]] || { echo "RAM-only restart retained data" >&2; exit 1; }
[[ "$(cli SET foreign:denied value)" == *NOPERM* ]]
[[ "$(podman inspect "$name" --format '{{.Image}}' | sed 's/^sha256://')" == "${image_id#sha256:}" ]]
if [[ -n "$evidence" ]]; then
  mkdir -p "$(dirname "$evidence")"
  jq -cn --arg image_id "$image_id" --arg unit "$unit" --arg container "$name" \
    --arg quadlet_sha256 "$(sha256sum "$quadlet" | cut -d' ' -f1)" \
    --arg config_sha256 "$(sha256sum "$config_dir/redis.conf" | cut -d' ' -f1)" \
    '{schema:1,image_id:$image_id,running_image_id:$image_id,unit:$unit,container:$container,quadlet_sha256:$quadlet_sha256,config_sha256:$config_sha256,complete:true}' > "$evidence"
fi
echo "Smoke Redis service gate passed: isolated Quadlet, ACL, socket, RAM-only restart"
