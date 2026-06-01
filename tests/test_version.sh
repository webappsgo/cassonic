#!/usr/bin/env bash
# Integration tests for /version and /api/version endpoints
# Requires: BINARY (path to cassonic binary), TEMP_DIR (temp working dir)

set -euo pipefail

PASS=0
FAIL=0

check() {
  if eval "$2"; then
    echo "  PASS: $1"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $1"
    FAIL=$((FAIL + 1))
  fi
}

DATA_DIR="${TEMP_DIR}/version-data"
CONFIG_DIR="${TEMP_DIR}/version-config"
mkdir -p "$DATA_DIR" "$CONFIG_DIR"

CONTAINER_NAME="cassonic-version-$$-${RANDOM}"
CONTAINER=""

cleanup() {
  if [ -n "$CONTAINER" ]; then
    docker stop "$CONTAINER" >/dev/null 2>&1 || true
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

CONTAINER=$(docker run -d \
  --name "$CONTAINER_NAME" \
  --rm \
  -v "${BINARY}:/usr/local/bin/cassonic:ro" \
  -v "${DATA_DIR}:/data" \
  -v "${CONFIG_DIR}:/config" \
  -e MODE=development \
  -p 0:80 \
  alpine:latest \
  /usr/local/bin/cassonic --port 80 --data /data --config /config)

PORT=$(docker port "$CONTAINER_NAME" 80/tcp 2>/dev/null | head -1 | cut -d: -f2)
BASE="http://localhost:${PORT}"

for i in $(seq 1 15); do
  curl -sf "${BASE}/health" >/dev/null 2>&1 && break
  sleep 1
done

check "GET /version returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' '${BASE}/version' | grep -qx '200'"

check "GET /version JSON body contains version field" \
  "curl -sf '${BASE}/version' | grep -q '\"version\"'"

check "GET /version JSON body has ok:true" \
  "curl -sf '${BASE}/version' | grep -q '\"ok\"[[:space:]]*:[[:space:]]*true'"

check "GET /api/version returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' '${BASE}/api/version' | grep -qx '200'"

check "GET /api/version JSON body contains version field" \
  "curl -sf '${BASE}/api/version' | grep -q '\"version\"'"

check "GET /api/v1/version returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' '${BASE}/api/v1/version' | grep -qx '200'"

check "GET /api/v1/version JSON body contains version field" \
  "curl -sf '${BASE}/api/v1/version' | grep -q '\"version\"'"

echo ""
echo "Version tests: ${PASS} passed, ${FAIL} failed"

[ "$FAIL" -eq 0 ] || exit 1
