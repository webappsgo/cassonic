#!/usr/bin/env bash
set -e

# =============================================================================
# Container Entrypoint Script - MINIMAL
# Only: set env, start services, start binary, handle signals
# Binary handles: directories, permissions, user/group, Tor, etc.
# =============================================================================

APP_NAME="cassonic"
APP_BIN="/usr/local/bin/${APP_NAME}"

export TZ="${TZ:-America/New_York}"
export CONFIG_DIR="${CONFIG_DIR:-/config/${APP_NAME}}"
export DATA_DIR="${DATA_DIR:-/data/${APP_NAME}}"

declare -a PIDS=()

log() { echo "[entrypoint] $(date '+%Y-%m-%d %H:%M:%S') $*"; }

cleanup() {
    log "Shutdown signal received..."
    for ((i=${#PIDS[@]}-1; i>=0; i--)); do
        kill -TERM "${PIDS[i]}" 2>/dev/null || true
    done
    wait
    exit 0
}
trap cleanup SIGTERM SIGINT SIGQUIT

# =============================================================================
# Start main application
# =============================================================================
log "Starting ${APP_NAME}..."

FLAGS="--address ${ADDRESS:-0.0.0.0} --port ${PORT:-80}"
[ "${DEBUG:-false}" = "true" ] && FLAGS="$FLAGS --debug"

exec $APP_BIN $FLAGS "$@"
