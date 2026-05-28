#!/bin/bash
# Deploy api_rest + Channel + Frameworks to Google Cloud Run
# Lee automáticamente remora-flujo/.env.local para obtener API keys locales.
# Uso: ./deploy.sh
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Cargar API keys desde .env.local si existe
ENV_LOCAL="${REPO_ROOT}/remora-flujo/.env.local"
if [ -f "$ENV_LOCAL" ]; then
  echo "=== Loading API keys from .env.local ==="
  set -a
  source "$ENV_LOCAL"
  set +a
fi

# Normalizar: REMORA_GROQ_API_KEY -> GROQ_API_KEY
if [ -z "$GROQ_API_KEY" ] && [ -n "$REMORA_GROQ_API_KEY" ]; then
  export GROQ_API_KEY="$REMORA_GROQ_API_KEY"
fi

PROJECT_ID="project-ceae5831-a2c9-49aa-b1c"
SERVICE_NAME="flujo-api"
REGION="us-central1"
IMAGE="gcr.io/${PROJECT_ID}/${SERVICE_NAME}"

CHANNEL_API_KEY="${CHANNEL_API_KEY:-test-key-001}"

echo "=== Deploy config ==="
echo "Project: ${PROJECT_ID}"
echo "Service: ${SERVICE_NAME}"
echo "Region:  ${REGION}"
echo "LLM Provider: ${REMORA_LLM_PROVIDER:-groq}"

# Detectar si Docker está disponible; si no, usar Cloud Build
if command -v docker >/dev/null 2>&1; then
  echo "=== Building Docker image ==="
  docker build -t ${IMAGE} -f "${REPO_ROOT}/remora-flujo/cmd/api_rest/Dockerfile" "${REPO_ROOT}"

  echo "=== Pushing to GCR ==="
  docker push ${IMAGE}

  echo "=== Deploying to Cloud Run ==="
  gcloud run deploy ${SERVICE_NAME} \
    --image=${IMAGE} \
    --platform=managed \
    --region=${REGION} \
    --allow-unauthenticated \
    --memory=1Gi \
    --cpu=2 \
    --timeout=300s \
    --max-instances=10 \
    --set-env-vars="PORT=8080,CHANNEL_PORT=8765,CHANNEL_URL=http://localhost:8765,CHANNEL_BASE_DIR=/workspace,REMORA_ROOT=/workspace,CHANNEL_API_KEY=${CHANNEL_API_KEY},CHANNEL_API_KEYS=${CHANNEL_API_KEY},REMORA_VAULT_DIR=/workspace/channel/vault_data" \
    $([ -n "$MINIMAX_API_KEY" ] && echo "--set-env-vars=MINIMAX_API_KEY=${MINIMAX_API_KEY}") \
    $([ -n "$GROQ_API_KEY" ] && echo "--set-env-vars=GROQ_API_KEY=${GROQ_API_KEY}") \
    $([ -n "$GEMINI_API_KEY" ] && echo "--set-env-vars=GEMINI_API_KEY=${GEMINI_API_KEY}") \
    $([ -n "$REMORA_VAULT_KEY" ] && echo "--set-env-vars=REMORA_VAULT_KEY=${REMORA_VAULT_KEY}")
else
  echo "Docker not found. Using Cloud Build instead..."
  cd "${REPO_ROOT}"

  # Obtener short SHA del commit o usar timestamp si no hay git
  if command -v git >/dev/null 2>&1 && [ -d ".git" ]; then
    SHORT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "$(date +%s)")
  else
    SHORT_SHA="$(date +%s)"
  fi

  SUST="_CHANNEL_API_KEY=${CHANNEL_API_KEY},_SHORT_SHA=${SHORT_SHA}"
  [ -n "$GEMINI_API_KEY" ] && SUST="${SUST},_GEMINI_API_KEY=${GEMINI_API_KEY}"

  echo "=== Building & Deploying via Cloud Build ==="
  echo "Image tag: ${SHORT_SHA}"
  gcloud builds submit --config=cloudbuild.yaml --substitutions="${SUST}"

  # cloudbuild.yaml SOLO compila y pushea la imagen a GCR; NO hace deploy a
  # Cloud Run. Acá forzamos que el servicio apunte a la imagen recién pusheada
  # (tag :${SHORT_SHA}) y de paso seteamos/actualizamos las env vars.
  REMORA_PROFILE="${REMORA_PROFILE:-cobranza-chile}"
  CHANNEL_EXEC_TIMEOUT="${CHANNEL_EXEC_TIMEOUT:-120s}"
  EXTRA="CHANNEL_PORT=8765,CHANNEL_URL=http://localhost:8765,CHANNEL_BASE_DIR=/workspace,REMORA_ROOT=/workspace,CHANNEL_API_KEY=${CHANNEL_API_KEY},CHANNEL_API_KEYS=${CHANNEL_API_KEY},REMORA_PROFILE=${REMORA_PROFILE},REMORA_PROFILE_PATH=/workspace/profiles,CHANNEL_EXEC_TIMEOUT=${CHANNEL_EXEC_TIMEOUT},REMORA_VAULT_DIR=/workspace/channel/vault_data"
  [ -n "$MINIMAX_API_KEY" ]     && EXTRA="${EXTRA},MINIMAX_API_KEY=${MINIMAX_API_KEY}"
  [ -n "$GROQ_API_KEY" ]        && EXTRA="${EXTRA},GROQ_API_KEY=${GROQ_API_KEY}"
  [ -n "$GEMINI_API_KEY" ]      && EXTRA="${EXTRA},GEMINI_API_KEY=${GEMINI_API_KEY}"
  [ -n "$REMORA_VAULT_KEY" ]    && EXTRA="${EXTRA},REMORA_VAULT_KEY=${REMORA_VAULT_KEY}"
  [ -n "$HOSTING_VAULT_KEY" ]   && EXTRA="${EXTRA},HOSTING_VAULT_KEY=${HOSTING_VAULT_KEY}"
  [ -n "$SMTP_HOST" ]           && EXTRA="${EXTRA},SMTP_HOST=${SMTP_HOST}"
  [ -n "$SMTP_PORT" ]           && EXTRA="${EXTRA},SMTP_PORT=${SMTP_PORT}"
  [ -n "$SMTP_USER" ]           && EXTRA="${EXTRA},SMTP_USER=${SMTP_USER}"
  [ -n "$SMTP_PASS" ]           && EXTRA="${EXTRA},SMTP_PASS=${SMTP_PASS}"
  [ -n "$SMTP_FROM" ]           && EXTRA="${EXTRA},SMTP_FROM=${SMTP_FROM}"
  [ -n "$TEST_EMAIL_RECIPIENT" ] && EXTRA="${EXTRA},TEST_EMAIL_RECIPIENT=${TEST_EMAIL_RECIPIENT}"

  echo "=== Deploying new image to Cloud Run: ${IMAGE}:${SHORT_SHA} ==="
  gcloud run deploy "${SERVICE_NAME}" \
    --image="${IMAGE}:${SHORT_SHA}" \
    --region="${REGION}" \
    --platform=managed \
    --allow-unauthenticated \
    --memory=1Gi \
    --cpu=2 \
    --timeout=300s \
    --max-instances=10 \
    --set-env-vars="${EXTRA}"
fi

echo ""
echo "=== Deployed! ==="
echo "URL: https://${SERVICE_NAME}-${PROJECT_ID}.${REGION}.run.app"
