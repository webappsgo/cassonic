#!/usr/bin/env bash
# Docker integration tests for cassonic
# Builds the Docker image and verifies container startup

set -euo pipefail

PROJECT_ORG="local"
PROJECT_NAME="cassonic"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

PASS=0
FAIL=0

log_pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
log_fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX")
trap 'docker rm -f "cassonic-test-$$" 2>/dev/null; rm -rf "$TEMP_DIR"' EXIT

IMAGE_TAG="cassonic-test:$$"

echo "Building Docker image..."
docker build -t "$IMAGE_TAG" -f "${PROJECT_DIR}/docker/Dockerfile" "${PROJECT_DIR}"

# Test: container starts and healthcheck passes
CONTAINER_ID=$(docker run -d \
  --name "cassonic-test-$$" \
  -e MODE=development \
  -e ADDRESS=0.0.0.0 \
  -e PORT=8080 \
  -v "${TEMP_DIR}/config:/config:z" \
  -v "${TEMP_DIR}/data:/data:z" \
  -p 18080:8080 \
  "$IMAGE_TAG")

echo "Container started: $CONTAINER_ID"
sleep 3

# Test: /health endpoint
if curl -sf http://127.0.0.1:18080/health > /dev/null; then
  log_pass "/health endpoint responds"
else
  log_fail "/health endpoint unreachable"
fi

# Test: /version endpoint
if curl -sf http://127.0.0.1:18080/version > /dev/null; then
  log_pass "/version endpoint responds"
else
  log_fail "/version endpoint unreachable"
fi

docker rm -f "cassonic-test-$$" 2>/dev/null
docker rmi "$IMAGE_TAG" 2>/dev/null

echo ""
echo "Docker test results: ${PASS} passed, ${FAIL} failed"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
