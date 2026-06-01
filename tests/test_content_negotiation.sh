#!/usr/bin/env bash
# Integration tests for content negotiation across endpoint types
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

# Verify Content-Type header contains the expected mime fragment
check_content_type() {
  local label="$1"
  local url="$2"
  local accept_header="$3"
  local expected_mime="$4"
  local ct
  ct=$(curl -sf -o /dev/null -w '%{content_type}' -H "Accept: ${accept_header}" "$url" 2>/dev/null || true)
  if echo "$ct" | grep -qi "$expected_mime"; then
    echo "  PASS: $label (Content-Type: $ct)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $label (Content-Type: $ct, wanted $expected_mime)"
    FAIL=$((FAIL + 1))
  fi
}

DATA_DIR="${TEMP_DIR}/conneg-data"
CONFIG_DIR="${TEMP_DIR}/conneg-config"
mkdir -p "$DATA_DIR" "$CONFIG_DIR"

CONTAINER_NAME="cassonic-conneg-$$-${RANDOM}"
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

# /health — frontend route
check "GET /health Accept:application/json returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' -H 'Accept: application/json' '${BASE}/health' | grep -qx '200'"

check "GET /health Accept:application/json body is JSON" \
  "curl -sf -H 'Accept: application/json' '${BASE}/health' | grep -q '\"ok\"'"

check_content_type \
  "GET /health Accept:application/json Content-Type is json" \
  "${BASE}/health" "application/json" "application/json"

check "GET /health Accept:text/plain returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' -H 'Accept: text/plain' '${BASE}/health' | grep -qx '200'"

check "GET /health Accept:text/plain body is plain text" \
  "curl -sf -H 'Accept: text/plain' '${BASE}/health' | grep -qi 'ok'"

check_content_type \
  "GET /health Accept:text/plain Content-Type is text/plain" \
  "${BASE}/health" "text/plain" "text/plain"

check "GET /health Accept:text/html returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' -H 'Accept: text/html' '${BASE}/health' | grep -qx '200'"

check_content_type \
  "GET /health Accept:text/html Content-Type is text/html" \
  "${BASE}/health" "text/html" "text/html"

# /api/v1/health — API route
check "GET /api/v1/health Accept:application/json returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' -H 'Accept: application/json' '${BASE}/api/v1/health' | grep -qx '200'"

check "GET /api/v1/health Accept:application/json body is JSON" \
  "curl -sf -H 'Accept: application/json' '${BASE}/api/v1/health' | grep -q '\"ok\"'"

check "GET /api/v1/health Accept:text/plain returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' -H 'Accept: text/plain' '${BASE}/api/v1/health' | grep -qx '200'"

check "GET /api/v1/health Accept:text/plain body is plain text" \
  "curl -sf -H 'Accept: text/plain' '${BASE}/api/v1/health' | grep -qi 'ok'"

# .txt endpoint extension
check "GET /robots.txt returns 200 as text/plain" \
  "curl -sf -o /dev/null -w '%{http_code}' '${BASE}/robots.txt' | grep -qx '200'"

check_content_type \
  "GET /robots.txt Content-Type is text/plain" \
  "${BASE}/robots.txt" "*/*" "text/plain"

echo ""
echo "Content negotiation tests: ${PASS} passed, ${FAIL} failed"

[ "$FAIL" -eq 0 ] || exit 1
