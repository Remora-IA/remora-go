#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REMORA_FLUJO_DIR="$ROOT/remora-flujo"
LOG_PATH="$ROOT/temp/api_rest.log"
PID_PATH="$ROOT/temp/api_rest.pid"
PORT="${API_REST_PORT:-${FLUJO_API_PORT:-8084}}"

mkdir -p "$ROOT/temp"

echo "→ Reiniciando api_rest en :$PORT"

kill_port() {
  local port="$1"
  local pids
  pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
  if [[ -n "${pids:-}" ]]; then
    echo "  · cerrando proceso(s) previos en :$port ($pids)"
    kill $pids 2>/dev/null || true
    sleep 1
    pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
    if [[ -n "${pids:-}" ]]; then
      kill -9 $pids 2>/dev/null || true
      sleep 1
    fi
  fi
}

kill_port "$PORT"

if [[ -f "$ROOT/.env" ]]; then
  set -a
  source "$ROOT/.env"
  set +a
fi

export REMORA_DEV_STATIC="${REMORA_DEV_STATIC:-1}"
export REMORA_ROOT="${REMORA_ROOT:-$ROOT}"

cd "$REMORA_FLUJO_DIR"

echo "  · recompilando api_rest"
go build -o api_rest ./cmd/api_rest

echo "  · levantando backend con REMORA_DEV_STATIC=$REMORA_DEV_STATIC"
nohup ./api_rest > "$LOG_PATH" 2>&1 < /dev/null &
PID=$!
echo "$PID" > "$PID_PATH"

sleep 3

if ! ps -p "$PID" >/dev/null 2>&1; then
  echo "❌ api_rest no quedó corriendo. Últimas líneas del log:"
  tail -n 40 "$LOG_PATH" || true
  exit 1
fi

if ! curl -fsS "http://localhost:$PORT/health" >/dev/null 2>&1; then
  echo "❌ api_rest arrancó pero /health no respondió OK. Últimas líneas del log:"
  tail -n 40 "$LOG_PATH" || true
  exit 1
fi

echo "✅ api_rest reiniciado"
echo "  PID:  $PID"
echo "  URL:  http://localhost:$PORT"
echo "  LOG:  $LOG_PATH"
