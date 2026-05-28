#!/bin/bash
# Deploy remora-flujo a Cloud Run
# Uso: ./deploy.sh
# Requisitos: gcloud CLI autenticado, Docker corriendo (o usa Cloud Build si no hay Docker)
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ── Cargar variables de entorno ───────────────────────────────────────────────
ENV_FILE="${SCRIPT_DIR}/.env"
ENV_LOCAL="${SCRIPT_DIR}/.env.local"

for f in "$ENV_FILE" "$ENV_LOCAL"; do
  if [ -f "$f" ]; then
    echo "=== Cargando variables desde $f ==="
    set -a
    # shellcheck disable=SC1090
    source "$f"
    set +a
  fi
done

# Normalizar REMORA_GROQ_API_KEY -> GROQ_API_KEY
if [ -z "$GROQ_API_KEY" ] && [ -n "$REMORA_GROQ_API_KEY" ]; then
  export GROQ_API_KEY="$REMORA_GROQ_API_KEY"
fi

# ── Configuración ─────────────────────────────────────────────────────────────
PROJECT_ID="${GCLOUD_PROJECT:-project-ceae5831-a2c9-49aa-b1c}"
SERVICE_NAME="flujo-api"
REGION="us-central1"
IMAGE="gcr.io/${PROJECT_ID}/${SERVICE_NAME}"

# Auth0
AUTH0_DOMAIN="${AUTH0_DOMAIN:-remora-ia.us.auth0.com}"
AUTH0_CLIENT_ID="${AUTH0_CLIENT_ID:-p6m3MHqoklOZkpCFGXF2owxIxtDwSJO6}"
AUTH0_AUDIENCE="${AUTH0_AUDIENCE:-https://remora-ia.us.auth0.com/api/v2/}"

# Canal
CHANNEL_API_KEY="${CHANNEL_API_KEY:-test-key-001}"

echo ""
echo "=== Configuración de deploy ==="
echo "   Proyecto:  ${PROJECT_ID}"
echo "   Servicio:  ${SERVICE_NAME}"
echo "   Region:    ${REGION}"
echo "   Image:     ${IMAGE}"
echo "   Auth0:     ${AUTH0_DOMAIN}"
echo "   GROQ:      ${GROQ_API_KEY:+set}${GROQ_API_KEY:-NO CONFIGURADO}"
echo "   MINIMAX:   ${MINIMAX_API_KEY:+set}${MINIMAX_API_KEY:-NO CONFIGURADO}"
echo "   VAULT KEY: ${REMORA_VAULT_KEY:+set}${REMORA_VAULT_KEY:-NO CONFIGURADO}"
echo ""

# ── Build + Push + Deploy ─────────────────────────────────────────────────────
if command -v docker >/dev/null 2>&1; then
  echo "=== Compilando binarios Go ==="
  cd "${REPO_ROOT}"

  go_build() {
    local mod_dir="$1" bin_name="$2" main_pkg="$3"
    echo "  -> $bin_name"
    (cd "$mod_dir" && go build -o "$bin_name" "$main_pkg")
  }

  go_build remora-flujo          cmd/api_rest/api_rest          ./cmd/api_rest
  go_build channel               remora-flujo/cmd/api_rest/channel ./cmd/channel
  go_build channel               channel/bin/vault              ./cmd/vault
  go_build framework-echo        framework-echo/frameworkecho   ./cmd/frameworkecho
  go_build framework-alfa        framework-alfa/frameworkalfa   ./cmd/frameworkalfa
  go_build framework-arquitecto  framework-arquitecto/frameworkarquitecto ./cmd/frameworkarquitecto
  go_build framework-auditor     framework-auditor/frameworkauditor ./cmd/frameworkauditor
  go_build framework-indexa      framework-indexa/frameworkindexa ./cmd/frameworkindexa
  go_build framework-sabio       framework-sabio/frameworksabio ./cmd/frameworksabio
  go_build framework-foco        framework-foco/foco            ./cmd/foco
  go_build framework-hosting     framework-hosting/frameworkhosting ./cmd/frameworkhosting
  go_build framework-radar       framework-radar/frameworkradar ./cmd/frameworkradar
  go_build framework-mensajero   framework-mensajero/frameworkmensajero ./cmd/frameworkmensajero
  go_build framework-critico     framework-critico/frameworkcritico ./cmd/frameworkcritico
  go_build framework-deployer    framework-deployer/deployer    ./cmd/deployer
  go_build framework-mecanico    framework-mecanico/frameworkmecanico ./cmd/frameworkmecanico
  go_build framework-paladin     framework-paladin/frameworkpaladin ./cmd/paladin
  go_build framework-tareas      framework-tareas/frameworktareas ./cmd/frameworktareas

  echo "=== Construyendo imagen Docker ==="
  docker build \
    -t "${IMAGE}:latest" \
    -f "${REPO_ROOT}/remora-flujo/cmd/api_rest/Dockerfile" \
    "${REPO_ROOT}"

  echo "=== Subiendo imagen a GCR ==="
  docker push "${IMAGE}:latest"

  echo "=== Desplegando en Cloud Run ==="
  ENV_VARS="PORT=8080"
  ENV_VARS="${ENV_VARS},CHANNEL_PORT=8765"
  ENV_VARS="${ENV_VARS},CHANNEL_URL=http://localhost:8765"
  ENV_VARS="${ENV_VARS},CHANNEL_BASE_DIR=/workspace"
  ENV_VARS="${ENV_VARS},REMORA_ROOT=/workspace"
  ENV_VARS="${ENV_VARS},REMORA_VAULT_DIR=/workspace/channel/vault_data"
  ENV_VARS="${ENV_VARS},CHANNEL_API_KEY=${CHANNEL_API_KEY}"
  ENV_VARS="${ENV_VARS},CHANNEL_API_KEYS=${CHANNEL_API_KEY}"
  ENV_VARS="${ENV_VARS},AUTH0_DOMAIN=${AUTH0_DOMAIN}"
  ENV_VARS="${ENV_VARS},AUTH0_CLIENT_ID=${AUTH0_CLIENT_ID}"
  ENV_VARS="${ENV_VARS},AUTH0_AUDIENCE=${AUTH0_AUDIENCE}"
  [ -n "$GROQ_API_KEY" ]          && ENV_VARS="${ENV_VARS},GROQ_API_KEY=${GROQ_API_KEY}"
  [ -n "$MINIMAX_API_KEY" ]       && ENV_VARS="${ENV_VARS},MINIMAX_API_KEY=${MINIMAX_API_KEY}"
  [ -n "$OPENROUTER_API_KEY" ]    && ENV_VARS="${ENV_VARS},OPENROUTER_API_KEY=${OPENROUTER_API_KEY}"
  [ -n "$REMORA_VAULT_KEY" ]      && ENV_VARS="${ENV_VARS},REMORA_VAULT_KEY=${REMORA_VAULT_KEY}"
  [ -n "$SMTP_HOST" ]             && ENV_VARS="${ENV_VARS},SMTP_HOST=${SMTP_HOST}"
  [ -n "$SMTP_PORT" ]             && ENV_VARS="${ENV_VARS},SMTP_PORT=${SMTP_PORT}"
  [ -n "$SMTP_USER" ]             && ENV_VARS="${ENV_VARS},SMTP_USER=${SMTP_USER}"
  [ -n "$SMTP_PASS" ]             && ENV_VARS="${ENV_VARS},SMTP_PASS=${SMTP_PASS}"
  [ -n "$SMTP_FROM" ]             && ENV_VARS="${ENV_VARS},SMTP_FROM=${SMTP_FROM}"
  [ -n "$TEST_EMAIL_RECIPIENT" ]  && ENV_VARS="${ENV_VARS},TEST_EMAIL_RECIPIENT=${TEST_EMAIL_RECIPIENT}"

  gcloud run deploy "${SERVICE_NAME}" \
    --image="${IMAGE}:latest" \
    --platform=managed \
    --region="${REGION}" \
    --allow-unauthenticated \
    --port=8080 \
    --memory=1Gi \
    --cpu=2 \
    --timeout=300s \
    --max-instances=10 \
    --set-env-vars="${ENV_VARS}"

else
  # ── Fallback: Cloud Build (sin Docker local) ──────────────────────────────
  echo "Docker no encontrado. Usando Cloud Build..."
  cd "${REPO_ROOT}"

  SHORT_SHA=$(git rev-parse --short HEAD 2>/dev/null || date +%s)

  SUBS="_SHORT_SHA=${SHORT_SHA}"
  SUBS="${SUBS},_CHANNEL_API_KEY=${CHANNEL_API_KEY}"
  SUBS="${SUBS},_AUTH0_DOMAIN=${AUTH0_DOMAIN}"
  SUBS="${SUBS},_AUTH0_CLIENT_ID=${AUTH0_CLIENT_ID}"
  SUBS="${SUBS},_AUTH0_AUDIENCE=${AUTH0_AUDIENCE}"
  [ -n "$GEMINI_API_KEY" ] && SUBS="${SUBS},_GEMINI_API_KEY=${GEMINI_API_KEY}"

  echo "=== Enviando a Cloud Build (tag: ${SHORT_SHA}) ==="
  gcloud builds submit \
    --config=remora-flujo/cloudbuild.yaml \
    --substitutions="${SUBS}" \
    --project="${PROJECT_ID}"
fi

# ── URL final ─────────────────────────────────────────────────────────────────
echo ""
echo "=== Deploy completado ==="
SERVICE_URL=$(gcloud run services describe "${SERVICE_NAME}" \
  --region="${REGION}" \
  --project="${PROJECT_ID}" \
  --format='value(status.url)' 2>/dev/null || echo "https://${SERVICE_NAME}-$(echo ${PROJECT_ID} | tr -d '-').${REGION}.run.app")

echo "   URL: ${SERVICE_URL}"
echo ""
echo "Siguiente paso obligatorio:"
echo "   Actualiza Auth0 Allowed Callback URLs con:"
echo "   ${SERVICE_URL}/callback"
echo "   Y Allowed Logout URLs con:"
echo "   ${SERVICE_URL}"
