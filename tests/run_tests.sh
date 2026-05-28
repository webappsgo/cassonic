#!/usr/bin/env bash
# Integration test runner for cassonic
# All test data goes to /tmp/local/cassonic-XXXXXX — never the project directory

set -euo pipefail

PROJECT_ORG="local"
PROJECT_NAME="cassonic"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

PASS=0
FAIL=0

log_pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
log_fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

echo "cassonic integration tests"
echo "=========================="

# Create temp dir
mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX")
trap 'rm -rf "$TEMP_DIR"' EXIT

# Build binary for testing
echo ""
echo "Building test binary..."
docker run --rm \
  -v "${PROJECT_DIR}:/src:ro" \
  -v "${TEMP_DIR}:/out" \
  -e CGO_ENABLED=0 \
  golang:alpine \
  sh -c "cd /src && go build -o /out/cassonic ./src/main.go" 2>&1

BINARY="${TEMP_DIR}/cassonic"
if [ ! -f "$BINARY" ]; then
  echo "FATAL: binary not built"
  exit 1
fi

echo "Binary built: $BINARY"

# Run each test script
for test_script in "$SCRIPT_DIR"/test_*.sh; do
  if [ -f "$test_script" ]; then
    script_name=$(basename "$test_script")
    echo ""
    echo "Running $script_name..."
    if BINARY="$BINARY" TEMP_DIR="$TEMP_DIR" bash "$test_script"; then
      log_pass "$script_name"
    else
      log_fail "$script_name"
    fi
  fi
done

echo ""
echo "=========================="
echo "Results: ${PASS} passed, ${FAIL} failed"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
