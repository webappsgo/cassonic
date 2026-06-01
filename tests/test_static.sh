#!/usr/bin/env bash
# Integration tests for static file serving
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

# Accepts HTTP status codes from a space-separated list
check_status() {
  local label="$1"
  local url="$2"
  shift 2
  local allowed=("$@")
  local code
  code=$(curl -sf -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)
  for s in "${allowed[@]}"; do
    if [ "$code" = "$s" ]; then
      echo "  PASS: $label (got $code)"
      PASS=$((PASS + 1))
      return
    fi
  done
  echo "  FAIL: $label (got $code, wanted ${allowed[*]})"
  FAIL=$((FAIL + 1))
}

DATA_DIR="${TEMP_DIR}/static-data"
CONFIG_DIR="${TEMP_DIR}/static-config"
mkdir -p "$DATA_DIR" "$CONFIG_DIR"

CONTAINER_NAME="cassonic-static-$$-${RANDOM}"
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

check "GET /robots.txt returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' '${BASE}/robots.txt' | grep -qx '200'"

check "GET /robots.txt body contains User-agent" \
  "curl -sf '${BASE}/robots.txt' | grep -qi 'User-agent'"

# favicon.ico may redirect to an SVG; accept 200, 301, or 302
check_status "GET /favicon.ico returns 200 or redirect" "${BASE}/favicon.ico" "200" "301" "302"

check "GET /.well-known/security.txt returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' '${BASE}/.well-known/security.txt' | grep -qx '200'"

check "GET /.well-known/security.txt body contains Contact:" \
  "curl -sf '${BASE}/.well-known/security.txt' | grep -qi 'Contact:'"

check "GET /sitemap.xml returns 200" \
  "curl -sf -o /dev/null -w '%{http_code}' '${BASE}/sitemap.xml' | grep -qx '200'"

check "GET /sitemap.xml body contains urlset" \
  "curl -sf '${BASE}/sitemap.xml' | grep -qi 'urlset'"

echo ""
echo "Static file tests: ${PASS} passed, ${FAIL} failed"

[ "$FAIL" -eq 0 ] || exit 1
