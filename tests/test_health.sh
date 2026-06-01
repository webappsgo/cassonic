#!/usr/bin/env bash
# Integration tests for /health endpoint
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

DATA_DIR="${TEMP_DIR}/health-data"
CONFIG_DIR="${TEMP_DIR}/health-config"
mkdir -p "$DATA_DIR" "$CONFIG_DIR"

CONTAINER_NAME="cassonic-health-$$-${RANDOM}"
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

check "GET /health returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' '${BASE}/health' | grep -qx '200'"

check "GET /health JSON body contains ok:true" \
  "curl -sf '${BASE}/health' | grep -q '\"ok\"[[:space:]]*:[[:space:]]*true'"

check "GET /health Accept:text/plain returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' -H 'Accept: text/plain' '${BASE}/health' | grep -qx '200'"

check "GET /health Accept:text/plain body is plain ok" \
  "curl -sf -H 'Accept: text/plain' '${BASE}/health' | grep -qi 'ok'"

check "GET /api/v1/health returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' '${BASE}/api/v1/health' | grep -qx '200'"

check "GET /api/v1/health JSON body contains ok:true" \
  "curl -sf '${BASE}/api/v1/health' | grep -q '\"ok\"[[:space:]]*:[[:space:]]*true'"

echo ""
echo "Health tests: ${PASS} passed, ${FAIL} failed"

[ "$FAIL" -eq 0 ] || exit 1
