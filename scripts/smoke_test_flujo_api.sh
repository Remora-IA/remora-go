#!/usr/bin/env bash
# Smoke test end-to-end de la API REST flujo_api conectada a Channel.
#
# Levanta Channel + flujo_api, crea una conversación con frameworks=[echo,alfa],
# manda varios mensajes simulando al usuario y verifica que:
#   - los mensajes del usuario quedan persistidos
#   - los frameworks devuelven preguntas vía la cola compartida
#   - la sesión se persiste automáticamente en sessions/<conv_id>.jsonl
#
# Uso:
#   bash scripts/smoke_test_flujo_api.sh
#
# Requiere: jq, curl, go.

set -euo pipefail

REPO_ROOT="/Users/alcless_a1234_cursor/remora-go"
CHANNEL_PORT=8765
API_PORT=8084
API_KEY="test-key-001"

cleanup() {
  echo
  echo "── Cleanup ──"
  if [[ -n "${API_PID:-}" ]]; then kill "$API_PID" 2>/dev/null || true; fi
  if [[ -n "${CHANNEL_PID:-}" ]]; then kill "$CHANNEL_PID" 2>/dev/null || true; fi
  wait 2>/dev/null || true
}
trap cleanup EXIT

require() { command -v "$1" >/dev/null || { echo "falta $1"; exit 1; }; }
require jq
require curl
require go

cd "$REPO_ROOT"

echo "── 1) Levantando Channel en :$CHANNEL_PORT ──"
cd "$REPO_ROOT/channel"
CHANNEL_API_KEYS="$API_KEY" CHANNEL_BASE_DIR="$REPO_ROOT" \
  go run ./cmd/channel --addr ":$CHANNEL_PORT" &
CHANNEL_PID=$!
sleep 4

echo "── 2) Levantando flujo_api en :$API_PORT ──"
cd "$REPO_ROOT/remora-flujo"
CHANNEL_URL="http://localhost:$CHANNEL_PORT" CHANNEL_API_KEY="$API_KEY" FLUJO_API_PORT="$API_PORT" \
  go run ./cmd/flujo_api &
API_PID=$!
sleep 6

echo "── 3) Health checks ──"
curl -sf "http://localhost:$CHANNEL_PORT/health" >/dev/null || { echo "Channel no responde"; exit 1; }
curl -sf "http://localhost:$API_PORT/health" | jq .

echo "── 4) Frameworks disponibles ──"
curl -s "http://localhost:$API_PORT/api/v1/frameworks" | jq .

echo "── 5) Crear conversación con [echo, alfa] ──"
CREATE_RESP=$(curl -s -X POST "http://localhost:$API_PORT/api/v1/conversations" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Smoke test","frameworks":["echo","alfa"]}')
echo "$CREATE_RESP" | jq .
CONV_ID=$(echo "$CREATE_RESP" | jq -r '.data.conversation.id')
echo "conv_id=$CONV_ID"

send() {
  local msg="$1"
  echo
  echo "── Usuario: $msg ──"
  curl -s -X POST "http://localhost:$API_PORT/api/v1/conversations/$CONV_ID/messages" \
    -H 'Content-Type: application/json' \
    -d "$(jq -n --arg c "$msg" '{content:$c}')" | jq '.data | {framework: .framework_message.framework, question_id: .framework_message.question_id, content: .framework_message.content, idle: .idle}'
}

# Simulamos un flujo discovery → pain → opportunity. Echo va a ir creando layers
# y validando contra la respuesta. Si Alfa puede compilar un draft, también empezará
# a meter sus open_questions en la cola.
send "Tengo un proceso de tesorería: paso transferencias de WhatsApp a un Excel a mano todos los días."
send "Sí, el flujo es ese: voy copiando una por una desde el grupo."
send "Lo hago como 3 veces al día y me toma 1 hora completa."
send "Sí, lo más doloroso es justo eso, perder esa hora."
send "Sí, un parser automático sería la solución ideal."

echo
echo "── 6) Cola final de la conversación ──"
curl -s "http://localhost:$API_PORT/api/v1/conversations/$CONV_ID/queue" | jq '.data | {frameworks, current_speaker, total: (.questions | length), pending: ([.questions[] | select(.status=="pending")] | length), answered: ([.questions[] | select(.status=="answered")] | length), questions: [.questions[] | {id, framework, external_id, status, text: .text[0:60]}]}'

echo
echo "── 7) Mensajes persistidos ──"
curl -s "http://localhost:$API_PORT/api/v1/conversations/$CONV_ID/messages" | jq '.data | map({role, framework, content: .content[0:80]})'

echo
echo "── 8) Sesión Channel JSONL ──"
SESSION_FILE="$REPO_ROOT/sessions/$CONV_ID.jsonl"
if [[ -f "$SESSION_FILE" ]]; then
  echo "Archivo: $SESSION_FILE"
  echo "Líneas: $(wc -l < "$SESSION_FILE")"
  echo "Primeros 3 métodos llamados:"
  head -n 3 "$SESSION_FILE" | jq -r '.method'
else
  echo "ATENCION: no existe $SESSION_FILE (el header X-Session-ID no llegó a Channel)"
fi

echo
echo "✓ Smoke test terminado. conv_id=$CONV_ID"
