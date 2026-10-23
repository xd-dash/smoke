#!/usr/bin/env bash
# Isolated Nginx host-side Quadlet qualification. Never activates Fatline.
set -Eeuo pipefail
agni='' builder='' image='' evidence=''
while (($#)); do
  case "$1" in
    --agni) agni="$2"; shift 2 ;;
    --builder) builder="$2"; shift 2 ;;
    --image) image="$2"; shift 2 ;;
    --evidence) evidence="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
[[ -n "$agni" && -d "$agni/.git" && -n "$builder" && -n "$image" ]] || { echo "Agni, builder and image required" >&2; exit 2; }
[[ "$(podman info --format '{{.Host.Security.Rootless}}')" == true ]] || { echo "rootless Podman required" >&2; exit 1; }
image_id="$(podman image inspect "$image" --format '{{.Id}}')"
image_id="sha256:${image_id#sha256:}"
[[ "$image_id" =~ ^sha256:[a-f0-9]{64}$ ]] || exit 1
scope="nginx-$$"
name="fatline-qualify-$scope"
unit="$name.service"
quadlet_dir="$HOME/.config/containers/systemd"
config_dir="$HOME/.config/fatline-qualify/$scope"
quadlet="$quadlet_dir/$name.container"
work="$(mktemp -d)"
started=0
cleanup() {
  if [[ "$started" == 1 ]]; then systemctl --user stop "$unit" >/dev/null 2>&1 || true; fi
  rm -f -- "$quadlet"
  systemctl --user daemon-reload >/dev/null 2>&1 || true
  podman rm -f "$name" >/dev/null 2>&1 || true
  rm -rf -- "$config_dir" "$work"
}
trap cleanup EXIT
mkdir -p "$quadlet_dir" "$config_dir"
chmod 755 "$config_dir"
out="$(podman exec "$builder" nix build --no-write-lock-file --no-link --print-out-paths /workspaces/agni/local/fatline#nginx-qualification)"
podman cp "$builder:$out/quadlets/fatline-qualify-nginx.container" "$work/template.container"
podman cp "$builder:$out/config/nginx.conf" "$config_dir/nginx.conf"
chmod 644 "$config_dir/nginx.conf"
sed -e "s|__CANDIDATE_IMAGE_ID__|$image_id|g" \
    -e "s|fatline-qualify-nginx|$name|g" \
    -e "s|fatline-qualify/nginx|fatline-qualify/$scope|g" \
    "$work/template.container" > "$quadlet"
grep -Fxq "Image=$image_id" "$quadlet"
grep -Fxq "Network=none" "$quadlet"
grep -Fxq "ReadOnly=true" "$quadlet"
grep -Fxq "Pull=never" "$quadlet"
systemctl --user daemon-reload
systemctl --user cat "$unit" >/dev/null
started=1
systemctl --user start "$unit"
for attempt in $(seq 1 40); do
  if systemctl --user is-active --quiet "$unit" && [[ "$(podman inspect "$name" --format '{{.State.Running}}' 2>/dev/null)" == true ]]; then break; fi
  sleep 0.25
done
systemctl --user is-active --quiet "$unit"
[[ "$(podman inspect "$name" --format '{{.State.Running}}')" == true ]]
running="$(podman inspect "$name" --format '{{.Image}}')"
running="sha256:${running#sha256:}"
[[ "$running" == "$image_id" ]]
# Verify the exact packaged Nginx binary can parse the active configuration.
entry="$(podman image inspect "$image" --format '{{index .Config.Entrypoint 0}}')"
[[ "$entry" == */bin/nginx ]] || { echo "Unexpected Nginx entrypoint" >&2; exit 1; }
podman exec "$name" "$entry" -t -c /etc/nginx/nginx.conf
systemctl --user restart "$unit"
for attempt in $(seq 1 40); do
  if systemctl --user is-active --quiet "$unit" && [[ "$(podman inspect "$name" --format '{{.State.Running}}' 2>/dev/null)" == true ]]; then break; fi
  sleep 0.25
done
systemctl --user is-active --quiet "$unit"
restarted_image="$(podman inspect "$name" --format '{{.Image}}')"
restarted_image="sha256:${restarted_image#sha256:}"
[[ "$restarted_image" == "$image_id" ]] || { echo "Restarted container image identity mismatch: $restarted_image != $image_id" >&2; exit 1; }
if [[ -n "$evidence" ]]; then
  mkdir -p "$(dirname "$evidence")"
  jq -cn --arg image_id "$image_id" --arg unit "$unit" --arg container "$name" \
    --arg quadlet_sha256 "$(sha256sum "$quadlet" | cut -d' ' -f1)" \
    --arg config_sha256 "$(sha256sum "$config_dir/nginx.conf" | cut -d' ' -f1)" \
    '{schema:1,image_id:$image_id,running_image_id:$image_id,unit:$unit,container:$container,quadlet_sha256:$quadlet_sha256,config_sha256:$config_sha256,complete:true}' > "$evidence"
fi
echo "Smoke Nginx service gate passed"
