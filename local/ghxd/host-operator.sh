#!/usr/bin/env bash
set -Eeuo pipefail

action="${1:?Select Smoke GHXD action}"
case "$action" in bootstrap|auth-start|auth-finish|auth-refresh|source-sync) ;; *) exit 2 ;; esac
machine="${NIX_XD_MACHINE:-coreos-arm64}"
scope="${NIX_XD_SCOPE:-nix-xd-arm}"
system="${NIX_XD_SYSTEM:-aarch64-linux}"
image="${NIX_XD_IMAGE:-localhost/nix-xd:arm64}"
flake="${NIX_XD_SELECTED_PROJECT_FLAKE:?Select exact Smoke project checkout}"
build_app="#source-build"
package="#credential-runtime-adapter"
program="smoke-credential-runtime-adapter"
manifest="${SMOKE_GHXD_SOURCE_MANIFEST:-}"
[[ "$machine" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || exit 2
[[ "$scope" =~ ^[a-z][a-z0-9-]{0,39}$ ]] || exit 2
[[ "$system" == aarch64-linux || "$system" == x86_64-linux ]] || exit 2
[[ "$image" =~ ^[A-Za-z0-9][A-Za-z0-9._/:@-]*$ ]] || exit 2
[[ "$flake" =~ ^/workspaces/[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*$ && "$flake" != *..* ]] || exit 2
[[ "${NIX_XD_SELECTED_PROJECT_REV:-}" =~ ^[0-9a-f]{40}$ ]] || exit 2
[[ "$build_app" =~ ^#[A-Za-z0-9_-]+$ && "$package" =~ ^#[A-Za-z0-9_-]+$ ]] || exit 2
[[ "$program" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || exit 2
[[ "$action" != source-sync || -n "$manifest" ]] || {
  echo 'Smoke source-sync requires SMOKE_GHXD_SOURCE_MANIFEST' >&2; exit 2;
}
p=(podman --connection "$machine")
checkout="${flake#/workspaces/}"
checkout="${checkout%%/*}"
[[ "$("${p[@]}" exec "$scope" cat "/workspaces/$checkout/.nix-xd-source-revision")" == "$NIX_XD_SELECTED_PROJECT_REV" ]] || { echo 'Staged Smoke source revision mismatch' >&2; exit 1; }
[[ "$(podman machine inspect "$machine" --format '{{.State}}')" == running ]] || exit 1
[[ "$("${p[@]}" inspect "$scope" --format '{{.State.Running}}')" == true ]] || exit 1
# The selected Nix-Alpine owner defines its own writable store domain.
# Neither the guest OS nor the source controller chooses its store volume.
store="$("${p[@]}" inspect "$scope" --format '{{range .Mounts}}{{if eq .Destination "/nix"}}{{.Name}}{{end}}{{end}}')"
[[ "$store" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || {
  echo 'Selected Nix-Alpine owner has no named /nix store volume' >&2; exit 1;
}
"${p[@]}" image exists "$image" || exit 1

# Existing labeled volumes are retained across the migration.
credential=nix-xd-state-secret-ghxd
home=nix-xd-state-control-ghxd-home
sources=nix-xd-state-control-ghxd-workspaces
for entry in "$credential" "$home" "$sources"; do
  kind=control
  [[ "$entry" != "$credential" ]] || kind=secret
  name="${entry#nix-xd-state-$kind-}"
  labels="$("${p[@]}" volume inspect "$entry" --format '{{json .Labels}}')" || {
    echo "Missing GHXD volume: $entry (use make state-volume-provision)" >&2
    exit 1
  }
  python3 - "$labels" "$kind" "$name" <<'PY'
import json,sys
data=json.loads(sys.argv[1])
for k,v in (("io.nix-xd.state.version","1"),("io.nix-xd.state.kind",sys.argv[2]),("io.nix-xd.state.name",sys.argv[3])):
    if data.get(k)!=v: raise SystemExit("GHXD volume labels mismatch")
PY
  # Initialize only a completely empty directory. No recursive ownership changes.
  "${p[@]}" run --rm --pull=never --network none --user 0:0 \
    --volume "$entry:/data:z" --entrypoint /bin/sh "$image" -ec '
      if test -z "$(ls -A /data)"; then chown 1000:1000 /data; chmod 0700 /data; fi
      test "$(stat -c %u /data)" = 1000
    '
done

# No other running container may hold the credential volume.
for id in $("${p[@]}" ps -aq); do
  [[ "$id" == "$scope" ]] && continue
  mounted="$("${p[@]}" inspect "$id" --format '{{range .Mounts}}{{if eq .Name "nix-xd-state-secret-ghxd"}}{{println .Destination}}{{end}}{{end}}')"
  [[ -z "$mounted" ]] || { echo "GHXD secret volume attached to container $id" >&2; exit 1; }
done

build_output="$("${p[@]}" exec "$scope" nix --option system "$system" run --no-write-lock-file \
  "$flake$build_app")"
printf '%s\n' "$build_output"
binary="$(printf '%s\n' "$build_output" | sed -n 's/^SMOKE_GHXD_NATIVE_ARTIFACT=//p' | tail -n 1)"
[[ "$binary" =~ ^/nix/store/[a-z0-9]{32}-smoke$ ]] || exit 1
output="$("${p[@]}" exec "$scope" nix --option system "$system" build --no-write-lock-file \
  --no-link --print-out-paths "$flake$package")"
[[ "$output" =~ ^/nix/store/[a-z0-9]{32}-[A-Za-z0-9._+-]+$ ]] || exit 1
"${p[@]}" exec "$scope" test -x "$binary"
"${p[@]}" exec "$scope" test -x "$output/bin/$program"

credential_access=z
[[ "$action" != source-sync ]] || credential_access=ro,z
common=(--rm --pull=never --read-only --user 1000:1000 --cap-drop all --security-opt no-new-privileges
  --tmpfs /tmp:rw,nosuid,nodev,mode=1777
  --mount "type=volume,src=$store,dst=/nix,ro"
  --volume "$credential:/run/smoke/ghxd:$credential_access"
  --volume "$home:/home/ghxd:z"
  --volume "$sources:/data/ghxd:z"
  --env HOME=/home/ghxd
  --env SMOKE_GHXD_BINARY="$binary"
  --entrypoint "$output/bin/$program")

if [[ "$action" == auth-start ]]; then
  [[ -n "${SMOKE_GHXD_CLIENT_ID:-}" ]] || {
    echo 'SMOKE_GHXD_CLIENT_ID (public GitHub App/OAuth client ID) is required' >&2; exit 2;
  }
  common+=(--env SMOKE_GHXD_CLIENT_ID="${SMOKE_GHXD_CLIENT_ID}")
fi

if [[ "$action" == source-sync ]]; then
  # The source selector is non-secret; it may be on macOS or in the Nix-Alpine owner.
  if [[ "$manifest" == /workspaces/* ]]; then
    [[ "$manifest" =~ ^/workspaces/[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*[.]json$ && "$manifest" != *..* ]] || exit 2
    manifest_payload="$("${p[@]}" exec "$scope" cat "$manifest")"
  else
    [[ -r "$manifest" ]] || { echo 'Unreadable source manifest' >&2; exit 2; }
    manifest_payload="$(cat "$manifest")"
  fi
  printf '%s\n' "$manifest_payload" | python3 "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/validate-source.py" -
  printf '%s\n' "$manifest_payload" | "${p[@]}" run -i "${common[@]}" "$image" "$action"
  rows="$(python3 -c '
import json,re,sys
d=json.load(sys.stdin)
if d.get("schema") != 1 or not isinstance(d.get("sources"),dict): raise SystemExit("Invalid source manifest")
for item in d["sources"].values():
    checkout=item["checkout"]; sha=item["revision"]
    if not re.fullmatch(r"[a-z][a-z0-9-]{1,63}",checkout): raise SystemExit("Unsafe checkout")
    if not re.fullmatch(r"[0-9a-f]{40}",sha): raise SystemExit("Invalid SHA")
    print(checkout+chr(9)+sha)
' <<< "$manifest_payload")"
  for pass in stage activate; do
    while IFS=$'\t' read -r checkout sha; do
      [[ -n "$checkout" ]] || continue
      generation="/workspaces/.source-generations/$checkout/$sha"
      if [[ "$pass" == stage ]]; then
        existing="$("${p[@]}" exec "$scope" sh -ec "if test -e '$generation'; then cat '$generation/.nix-xd-source-revision' 2>/dev/null || echo invalid; else echo absent; fi")"
        if [[ "$existing" == "$sha" ]]; then
          echo "GHXD_GENERATION=reused checkout=$checkout sha=$sha"
          continue
        fi
        [[ "$existing" == absent ]] || { echo "Existing generation identity mismatch checkout=$checkout" >&2; exit 1; }
        source_dir="/data/ghxd/generations/$checkout/$sha"
        if "${p[@]}" run --rm --pull=never --network none --user 1000:1000 \
          --volume "$sources:/data/ghxd:ro,z" --entrypoint /bin/sh "$image" \
          -ec "test -d '/data/ghxd/$checkout/.git' && test \"\$(git -C '/data/ghxd/$checkout' rev-parse HEAD)\" = '$sha'" 2>/dev/null; then
          source_dir="/data/ghxd/$checkout"
        fi
        "${p[@]}" exec "$scope" mkdir -p "/workspaces/.source-generations/$checkout"
        temporary="$("${p[@]}" exec "$scope" mktemp -d "/workspaces/.source-generations/$checkout/.incoming.XXXXXXXX")"
        if ! "${p[@]}" run --rm --pull=never --network none --user 1000:1000 \
          --volume "$sources:/data/ghxd:ro,z" --entrypoint /bin/tar "$image" \
          -C "$source_dir" --exclude=.git -cf - . |
          "${p[@]}" exec -i "$scope" tar -C "$temporary" -xf -; then
          "${p[@]}" exec "$scope" rm -rf -- "$temporary" || true
          echo "GHXD_STAGE=failed checkout=$checkout" >&2
          exit 1
        fi
        "${p[@]}" exec "$scope" sh -ec \
          "printf '%s\\n' '$sha' > '$temporary/.nix-xd-source-revision' && mv '$temporary' '$generation'"
        echo "GHXD_GENERATION=staged checkout=$checkout sha=$sha"
      else
        "${p[@]}" exec -i "$scope" sh -s -- "$checkout" "$sha" < \
          "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/activate-source-generation.sh"
      fi
    done <<< "$rows"
  done
  echo 'GHXD_WORKSPACE_SYNC=passed'
else
  "${p[@]}" run "${common[@]}" "$image" "$action"
fi
