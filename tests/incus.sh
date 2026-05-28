#!/usr/bin/env bash
# Incus/LXD integration tests for cassonic
# Used for testing systemd service installation and OS-level integration

set -euo pipefail

PROJECT_ORG="local"
PROJECT_NAME="cassonic"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

if ! command -v incus &>/dev/null && ! command -v lxc &>/dev/null; then
  echo "SKIP: incus/lxc not available on this host"
  echo "Incus tests are for full OS integration (systemd, service install)."
  echo "Run these in a CI environment with incus available."
  exit 0
fi

INCUS_CMD="incus"
command -v incus &>/dev/null || INCUS_CMD="lxc"

CONTAINER="cassonic-test-$$"
PASS=0
FAIL=0

log_pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
log_fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

trap '$INCUS_CMD delete --force "$CONTAINER" 2>/dev/null' EXIT

echo "Launching Incus container: $CONTAINER"
$INCUS_CMD launch images:debian/12 "$CONTAINER"
sleep 5

# Copy binary into container
mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX")
trap '$INCUS_CMD delete --force "$CONTAINER" 2>/dev/null; rm -rf "$TEMP_DIR"' EXIT

docker run --rm \
  -v "${PROJECT_DIR}:/src:ro" \
  -v "${TEMP_DIR}:/out" \
  -e CGO_ENABLED=0 \
  golang:alpine \
  sh -c "cd /src && GOOS=linux GOARCH=amd64 go build -o /out/cassonic ./src/main.go"

$INCUS_CMD file push "${TEMP_DIR}/cassonic" "${CONTAINER}/usr/local/bin/cassonic"
$INCUS_CMD exec "$CONTAINER" -- chmod 755 /usr/local/bin/cassonic

# Test: binary runs inside container
if $INCUS_CMD exec "$CONTAINER" -- /usr/local/bin/cassonic --version; then
  log_pass "binary runs in container"
else
  log_fail "binary failed in container"
fi

# Test: service install
if $INCUS_CMD exec "$CONTAINER" -- /usr/local/bin/cassonic --service --install; then
  log_pass "service install"
else
  log_fail "service install failed"
fi

$INCUS_CMD delete --force "$CONTAINER" 2>/dev/null

echo ""
echo "Incus test results: ${PASS} passed, ${FAIL} failed"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
