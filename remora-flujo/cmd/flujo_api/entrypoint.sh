#!/bin/bash
set -e

echo "=== Remora Flujo API + Channel ==="
echo "Channel URL: $CHANNEL_URL"
echo "LLM Providers: Minimax=${MINIMAX_API_KEY:+set} Groq=${GROQ_API_KEY:+set}"

# Validar API keys
if [ -z "$CHANNEL_API_KEYS" ] && [ -z "$CHANNEL_API_KEY" ]; then
    echo "ERROR: CHANNEL_API_KEYS or CHANNEL_API_KEY required"
    exit 1
fi

# Asegurar API keys en CHANNEL_API_KEYS (formato esperado por Channel)
if [ -z "$CHANNEL_API_KEYS" ]; then
    export CHANNEL_API_KEYS="$CHANNEL_API_KEY"
fi

echo "Starting Channel on :${CHANNEL_PORT:-8765}..."
# Iniciar Channel en background
/channel \
    -addr ":${CHANNEL_PORT:-8765}" \
    -base-dir "$CHANNEL_BASE_DIR" \
    -api-keys "$CHANNEL_API_KEYS" \
    &
CHANNEL_PID=$!

echo "Channel started (PID: $CHANNEL_PID)"

# Esperar a que Channel esté listo
for i in $(seq 1 30); do
    if curl -s "http://localhost:${CHANNEL_PORT:-8765}/health" > /dev/null 2>&1; then
        echo "Channel is ready!"
        break
    fi
    if ! kill -0 $CHANNEL_PID 2>/dev/null; then
        echo "ERROR: Channel died unexpectedly"
        exit 1
    fi
    sleep 0.2
done

echo "Starting Flujo API on :${FLUJO_API_PORT_OUTER:-8080}..."
# Iniciar Flujo API
exec /flujo_api
