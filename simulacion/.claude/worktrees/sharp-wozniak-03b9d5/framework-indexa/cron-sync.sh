#!/bin/bash
# cron-sync.sh: wrapper para ejecutar sync.sh desde cron
# Carga .env, redirige output a logs con rotación automática.
#
# Uso manual:  ./cron-sync.sh
# Cron:        0 */6 * * * /path/to/framework-indexa/cron-sync.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# --- Cargar credenciales desde .env ---
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
else
  echo "ERROR: .env no encontrado en $SCRIPT_DIR" >&2
  exit 1
fi

# --- Configurar logging ---
LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/sync.log"
MAX_LOGS=10
MAX_LOG_SIZE=$((5 * 1024 * 1024))  # 5 MB

# Rotar log si excede tamaño
if [ -f "$LOG_FILE" ] && [ "$(stat -f%z "$LOG_FILE" 2>/dev/null || stat -c%s "$LOG_FILE" 2>/dev/null)" -gt "$MAX_LOG_SIZE" ]; then
  for i in $(seq $((MAX_LOGS - 1)) -1 1); do
    [ -f "$LOG_FILE.$i" ] && mv "$LOG_FILE.$i" "$LOG_FILE.$((i + 1))"
  done
  mv "$LOG_FILE" "$LOG_FILE.1"
  # Borrar logs viejos
  find "$LOG_DIR" -name "sync.log.*" -type f | sort -t. -k3 -n -r | tail -n +$((MAX_LOGS + 1)) | xargs rm -f 2>/dev/null
fi

# --- Lockfile para evitar ejecuciones simultáneas ---
LOCKFILE="$SCRIPT_DIR/.sync.lock"
if [ -f "$LOCKFILE" ]; then
  LOCK_PID=$(cat "$LOCKFILE" 2>/dev/null || echo "")
  if [ -n "$LOCK_PID" ] && kill -0 "$LOCK_PID" 2>/dev/null; then
    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) SKIP: sync ya corriendo (PID $LOCK_PID)" >> "$LOG_FILE"
    exit 0
  fi
  rm -f "$LOCKFILE"
fi

echo $$ > "$LOCKFILE"
trap 'rm -f "$LOCKFILE"' EXIT

# --- Ejecutar sync ---
{
  echo ""
  echo "========================================"
  echo "CRON SYNC START: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "========================================"

  if ./sync.sh; then
    echo "CRON SYNC OK: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  else
    EXIT_CODE=$?
    echo "CRON SYNC FAILED (exit=$EXIT_CODE): $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    exit $EXIT_CODE
  fi

  echo "========================================"
} >> "$LOG_FILE" 2>&1
