#!/bin/bash
set -e

echo "=== Remora API REST + Channel ==="
echo "Channel URL: $CHANNEL_URL"
echo "LLM Providers: Minimax=${MINIMAX_API_KEY:+set} Groq=${GROQ_API_KEY:+set}"

# Configurar perfil por defecto (puede ser sobreescrito en deploy)
export REMORA_PROFILE_PATH="${REMORA_PROFILE_PATH:-/workspace/profiles}"
export REMORA_PROFILE="${REMORA_PROFILE:-generic}"
echo "Profile: $REMORA_PROFILE (from $REMORA_PROFILE_PATH)"

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

# Seed de contactos por perfil. Si existe profiles/<perfil>/contacts.seed.csv
# y aún no hay contacts.db, lo importamos. Esto vuelve idempotente el boot:
# en la primera revisión se siembra, en las siguientes ya está hecho.
PROFILE_DIR="$REMORA_PROFILE_PATH/$REMORA_PROFILE"
CONTACTS_SEED="$PROFILE_DIR/contacts.seed.csv"
CONTACTS_DB="$PROFILE_DIR/contacts.db"
if [ -f "$CONTACTS_SEED" ] && [ ! -s "$CONTACTS_DB" ]; then
    echo "Sembrando contactos desde $CONTACTS_SEED..."
    /workspace/framework-sabio/frameworksabio contact-import-csv \
        --profile "$REMORA_PROFILE" \
        --file "$CONTACTS_SEED" || echo "WARN: no se pudo sembrar contactos (continuamos)"
fi

echo "Starting API REST on :${API_REST_PORT:-${PORT:-8080}}..."
# Iniciar API REST
exec /api_rest
