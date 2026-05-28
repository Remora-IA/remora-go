#!/bin/bash
# Levanta Channel + api_rest en local (Mac/Linux)
# Uso: ./scripts/dev-local.sh
# Requiere: estar en /Users/alcless_a1234_cursor/remora-go
#
# Auto-mata instancias previas corriendo en :8765 / :8084 para evitar
# "bind: address already in use" entre sesiones.

set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHANNEL_URL="http://localhost:8765"
API_KEY="test-key-001"
export REMORA_ROOT="$ROOT"

echo "=== Remora Local Dev ==="
echo "ROOT: $ROOT"

# ── Matar procesos previos ────────────────────────────────────────────────
# Usamos lsof porque es lo más confiable en macOS. || true para no fallar
# si el puerto está libre.
kill_port() {
  local port="$1"
  local pids
  pids="$(lsof -ti:"$port" 2>/dev/null || true)"
  if [ -n "$pids" ]; then
    echo "Matando procesos previos en :$port (pids: $pids)"
    echo "$pids" | xargs kill -9 2>/dev/null || true
    sleep 1
  fi
}
kill_port 8765
kill_port 8084

# Terminal 1: Channel (desde el directorio del módulo)
osascript -e "tell application \"Terminal\" to do script \"cd $ROOT/channel && go run ./cmd/channel --base-dir $ROOT --api-keys $API_KEY --addr :8765\""

echo "Channel levantandose en :8765..."
sleep 2

# Terminal 2: api_rest (desde el directorio del módulo)
osascript -e "tell application \"Terminal\" to do script \"cd $ROOT/remora-flujo && REMORA_ROOT=$ROOT CHANNEL_URL=$CHANNEL_URL CHANNEL_API_KEY=$API_KEY REMORA_DEV_STATIC=1 go run ./cmd/api_rest\""

echo "api_rest levantandose en :8084..."
echo ""
echo "Esperá ~5s y luego:"
echo "  remora                 → modo pair-programming"
echo "  remora frameworks      → ver frameworks disponibles"
echo ""
echo "Para matar todo luego:"
echo "  lsof -ti:8765 -ti:8084 | xargs kill -9"
