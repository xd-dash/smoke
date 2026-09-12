#!/usr/bin/env bash
set -euo pipefail

: "${MARAI_DIR:?MARAI_DIR is required}"
: "${PRAJAPATI_DIR:?PRAJAPATI_DIR is required}"
: "${LOGMA_DIR:?LOGMA_DIR is required}"
: "${RATELIMITER_DIR:?RATELIMITER_DIR is required}"
: "${AGNI_DIR:?AGNI_DIR is required}"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
work="${SMOKE_SECURITY_CELL_WORK:-${RUNNER_TEMP:-/tmp}/smoke-fatline-security-cell}"
artifacts="$work/artifacts"
helpers="$work/helpers"
runtime="$work/runtime"
cell="$work/cell"
cell_id="${AGNI_CELL_ID:-smoke-fatline}"
prajapati_port="${PRAJAPATI_PORT:-18081}"
logma_redis_port="${LOGMA_REDIS_PORT:-16379}"
acl_password='smoke-logma-local-acl-password'
secret_plaintext='smoke-axiom-token-value'

cleanup() {
  AGNI_CELL_ID="$cell_id" AGNI_CELL_ROOT="$cell" \
    bash "$AGNI_DIR/local/security-cell.sh" stop >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup
rm -rf "$work"
mkdir -p "$artifacts" "$helpers" "$runtime"

cat >"$helpers/go.mod" <<EOF
module smoke-fatline-security-cell

go 1.26

require (
  github.com/dash-xd/ratelimiter v0.0.0
  github.com/xd-dash/logma v0.0.0
  github.com/xd-dash/prajapati v0.0.0
)

replace github.com/dash-xd/ratelimiter => $RATELIMITER_DIR
replace github.com/xd-dash/logma => $LOGMA_DIR
replace github.com/xd-dash/prajapati => $PRAJAPATI_DIR
EOF
cp "$script_dir/prepare.go.tmpl" "$helpers/prepare.go"
cp "$script_dir/resolve.go.tmpl" "$helpers/resolve.go"

(
  cd "$helpers"
  go run ./prepare.go "$artifacts" "$acl_password" >"$artifacts/prepare.out"
)
binding_digest="$(cat "$artifacts/binding.digest")"
[[ "$binding_digest" == sha256:* ]]

# Agni owns disposable placement. Smoke owns the exact behavior recipe and
# supplies already-compiled identity/policy artifacts.
AGNI_CELL_ID="$cell_id" \
AGNI_CELL_ROOT="$cell" \
MARAI_DIR="$MARAI_DIR" \
PRAJAPATI_DIR="$PRAJAPATI_DIR" \
PRAJAPATI_TENANT_REGISTRY_FILE="$artifacts/registry.json" \
PRAJAPATI_ED25519_KEYS_FILE="$artifacts/keys.json" \
PRAJAPATI_PORT="$prajapati_port" \
LOGMA_REDIS_PORT="$logma_redis_port" \
  bash "$AGNI_DIR/local/security-cell.sh" start >"$artifacts/agni-start.out"

agni() {
  AGNI_CELL_ID="$cell_id" AGNI_CELL_ROOT="$cell" \
    bash "$AGNI_DIR/local/security-cell.sh" "$@"
}

# Lifecycle authority is explicit and separate from Prajapati. KMS.CREATE
# returns the initial primary key version, not a generic OK reply.
agni lifecycle KMS.CREATE logma-secret | grep -qx 1
agni lifecycle KMS.CREATE gateway-key | grep -qx 1

ready=0
for _ in $(seq 1 100); do
  status="$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${prajapati_port}/healthz" || true)"
  if [[ "$status" == 200 ]]; then
    ready=1
    break
  fi
  sleep 0.1
done
[[ "$ready" == 1 ]]

logma_token="$(cat "$artifacts/logma.token")"
callback_token="$(cat "$artifacts/callback.token")"
gateway_token="$(cat "$artifacts/gateway.token")"
echo "::add-mask::$logma_token" 2>/dev/null || true
echo "::add-mask::$callback_token" 2>/dev/null || true
echo "::add-mask::$gateway_token" 2>/dev/null || true

# The callback identity is allowed to encrypt this exact secret resource.
plaintext_b64="$(printf '%s' "$secret_plaintext" | base64 -w0)"
encrypt_response="$(curl -fsS \
  -H "Authorization: Bearer $callback_token" \
  -H 'Content-Type: application/json' \
  --data "{\"data\":\"$plaintext_b64\"}" \
  "http://127.0.0.1:${prajapati_port}/v1/keys/logma-secret/encrypt")"
ciphertext_b64="$(printf '%s' "$encrypt_response" | jq -er '.data')"

# Logma resolves the ciphertext through Prajapati under the exact frozen
# binding digest and materializes plaintext only into the disposable runtime.
(
  cd "$helpers"
  go run ./resolve.go \
    "http://127.0.0.1:${prajapati_port}" \
    "$artifacts/logma.token" \
    "$ciphertext_b64" \
    "$binding_digest" \
    "$binding_digest" \
    "$runtime/axiom-token" \
    logma-secret
)
[[ "$(cat "$runtime/axiom-token")" == "$secret_plaintext" ]]
[[ "$(stat -c '%a' "$runtime/axiom-token")" == 600 ]]

# A changed binding digest is rejected by Logma before treating the artifact as
# valid runtime secret state.
wrong_digest="sha256:$(printf '0%.0s' $(seq 1 64))"
if (
  cd "$helpers"
  go run ./resolve.go \
    "http://127.0.0.1:${prajapati_port}" \
    "$artifacts/logma.token" \
    "$ciphertext_b64" \
    "$binding_digest" \
    "$wrong_digest" \
    "$runtime/should-not-exist" \
    logma-secret
) >/dev/null 2>&1; then
  echo "mismatched binding digest unexpectedly resolved" >&2
  exit 1
fi
[[ ! -e "$runtime/should-not-exist" ]]

# Semantic authz negatives: same authenticated principal, wrong action/resource.
status="$(curl -sS -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer $logma_token" \
  -H 'Content-Type: application/json' \
  --data "{\"data\":\"$plaintext_b64\"}" \
  "http://127.0.0.1:${prajapati_port}/v1/keys/logma-secret/encrypt" || true)"
[[ "$status" == 403 ]]

status="$(curl -sS -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer $logma_token" \
  -H 'Content-Type: application/json' \
  --data "{\"data\":\"$ciphertext_b64\"}" \
  "http://127.0.0.1:${prajapati_port}/v1/keys/gateway-key/decrypt" || true)"
[[ "$status" == 403 ]]

# Gateway identity owns only data-key generation for gateway-key.
curl -fsS \
  -H "Authorization: Bearer $gateway_token" \
  -X POST \
  "http://127.0.0.1:${prajapati_port}/v1/keys/gateway-key/generate-data-key" \
  | jq -e '.plaintext | type == "string" and length > 0' >/dev/null

# Apply Logma's compiled local Redis execution identity to a real Redis 7.2.5
# instance. This identity is intentionally not the ed25519 Fatline principal.
logma_redis_container="agni-${cell_id}-logma-redis"
redis_username="$(jq -er '.Username' "$artifacts/redis-execution.json")"
mapfile -t redis_rules < <(jq -er '.Rules[]' "$artifacts/redis-execution.json")
[[ "$redis_username" != ed25519:* ]]
docker exec "$logma_redis_container" redis-cli ACL SETUSER "$redis_username" "${redis_rules[@]}" | grep -qx OK
docker exec "$logma_redis_container" redis-cli ACL DRYRUN "$redis_username" PUBLISH tenant:world-17:test payload | grep -qx OK
if docker exec "$logma_redis_container" redis-cli ACL DRYRUN "$redis_username" SUBSCRIBE tenant:world-17:test 2>&1 | grep -qx OK; then
  echo "publisher Redis identity unexpectedly allowed SUBSCRIBE" >&2
  exit 1
fi
if docker exec "$logma_redis_container" redis-cli ACL DRYRUN "$redis_username" ACL LIST 2>&1 | grep -qx OK; then
  echo "publisher Redis identity unexpectedly allowed ACL administration" >&2
  exit 1
fi

# Direct application Redis identity cannot own Marai lifecycle.
denied="$(agni app KMS.QUIESCE 2>&1 || true)"
[[ "$denied" == *NOPERM* ]]

# Prajapati deliberately has no lifecycle HTTP route.
status="$(curl -sS -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer $logma_token" \
  -X POST "http://127.0.0.1:${prajapati_port}/v1/lifecycle/quiesce" || true)"
[[ "$status" == 404 ]]

# Only the explicit Agni/Huram lifecycle rail can transition authority.
agni lifecycle KMS.QUIESCE | grep -qx OK
not_ready=0
for _ in $(seq 1 50); do
  status="$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${prajapati_port}/healthz" || true)"
  if [[ "$status" == 503 ]]; then
    not_ready=1
    break
  fi
  sleep 0.1
done
[[ "$not_ready" == 1 ]]
agni lifecycle KMS.ZEROIZE | grep -qx OK
agni lifecycle --raw KMS.STATUS | head -n1 | grep -qx dead

printf 'qualified binding=%s\n' "$binding_digest"
printf 'qualified marai=%s prajapati=%s logma=%s ratelimiter=%s agni=%s\n' \
  "${MARAI_REF:-unknown}" "${PRAJAPATI_REF:-unknown}" "${LOGMA_REF:-unknown}" \
  "${RATELIMITER_REF:-unknown}" "${AGNI_REF:-unknown}"
