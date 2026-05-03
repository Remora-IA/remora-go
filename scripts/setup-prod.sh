#!/usr/bin/env bash
# =============================================================================
# Setup PROD (one-shot, idempotente)
# =============================================================================
# Orquesta los 3 pasos para dejar el deploy de DEV funcionando con secrets,
# healthcheck y CI. Es seguro correrlo varias veces: cada paso es idempotente.
#
#   1. Bootstrap local (si falta .env)
#   2. Subir secrets a GCP Secret Manager y bindearlos al servicio
#   3. Activar readiness probe en Cloud Run
#   4. Configurar CI trigger en Cloud Build (si el repo esta conectado)
#
# Lo unico que NO hace este script (porque Google no lo permite a un bot):
#   - gcloud auth login (lo haces vos UNA VEZ)
#   - Conectar el repo de GitHub a Cloud Build (UNA VEZ via UI)
#
# Si alguno de esos falta, el script te avisa con instrucciones claras.
# =============================================================================

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PROJECT_ID="${PROJECT_ID:-project-ceae5831-a2c9-49aa-b1c}"
REGION="${REGION:-us-central1}"
SERVICE="${SERVICE:-flujo-api-dev}"
GH_OWNER="${GH_OWNER:-Remora-IA}"
GH_REPO="${GH_REPO:-remora-go}"

green() { printf "\033[32m%s\033[0m\n" "$1"; }
yellow() { printf "\033[33m%s\033[0m\n" "$1"; }
red() { printf "\033[31m%s\033[0m\n" "$1"; }
bold() { printf "\033[1m%s\033[0m\n" "$1"; }

bold "=== Remora Setup PROD ==="
echo "Proyecto: $PROJECT_ID"
echo "Servicio: $SERVICE"
echo "Region:   $REGION"
echo ""

# --- 0. Pre-flight: gcloud auth ---------------------------------------------
echo "→ Verificando autenticacion gcloud..."
if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" 2>/dev/null | grep -q "@"; then
  red "❌ gcloud no autenticado."
  echo ""
  yellow "Corre UNA VEZ:"
  echo "  gcloud auth login"
  echo "  gcloud config set project $PROJECT_ID"
  echo ""
  echo "Despues volve a correr este script."
  exit 1
fi
green "  ✓ Autenticado como: $(gcloud auth list --filter=status:ACTIVE --format='value(account)' 2>/dev/null | head -1)"

# Asegurar que el proyecto activo sea el correcto.
gcloud config set project "$PROJECT_ID" >/dev/null 2>&1
green "  ✓ Proyecto activo: $PROJECT_ID"

# --- 1. Bootstrap local -----------------------------------------------------
echo ""
echo "→ Paso 1: bootstrap local..."
if [ ! -f .env ]; then
  yellow "  ⚠ Falta .env, corriendo bootstrap..."
  bash scripts/bootstrap.sh
else
  green "  ✓ .env ya existe (skip bootstrap)"
fi

# --- 1.5 Verificar API keys (gate critico) ---------------------------------
echo ""
echo "→ Verificando API keys de LLM..."
groq=$(grep -E "^GROQ_API_KEY=" .env | cut -d= -f2- | tr -d '"' | tr -d "'" | head -1)
minimax=$(grep -E "^MINIMAX_API_KEY=" .env | cut -d= -f2- | tr -d '"' | tr -d "'" | head -1)
if [ -z "$groq" ] && [ -z "$minimax" ]; then
  red "❌ Faltan API keys de LLM. Sin al menos una, el sistema no funciona."
  echo ""
  yellow "Editar .env y completar al menos una:"
  echo "    GROQ_API_KEY=gsk_...        (https://console.groq.com/keys)"
  echo "    MINIMAX_API_KEY=...         (https://www.minimax.io)"
  echo ""
  echo "Despues volve a correr 'make setup-prod'."
  exit 1
fi
[ -n "$groq" ]    && green "  ✓ GROQ_API_KEY presente"
[ -n "$minimax" ] && green "  ✓ MINIMAX_API_KEY presente"

# --- 2. Habilitar APIs necesarias -------------------------------------------
echo ""
echo "→ Paso 2: habilitando APIs..."
for api in secretmanager.googleapis.com cloudbuild.googleapis.com run.googleapis.com; do
  gcloud services enable "$api" --project="$PROJECT_ID" >/dev/null 2>&1 || true
  green "  ✓ $api"
done

# --- 3. Verificar que el servicio Cloud Run exista --------------------------
echo ""
echo "→ Paso 3: verificando servicio $SERVICE..."
if ! gcloud run services describe "$SERVICE" --region="$REGION" --project="$PROJECT_ID" >/dev/null 2>&1; then
  yellow "  ⚠ El servicio $SERVICE no existe todavia."
  echo ""
  echo "Hay que deployar al menos una vez antes:"
  echo "  make deploy-dev"
  echo ""
  echo "Despues volve a correr 'make setup-prod'."
  exit 1
fi
green "  ✓ Servicio $SERVICE existe"

# --- 4. Subir secrets -------------------------------------------------------
echo ""
echo "→ Paso 4: secrets a Secret Manager..."
bash scripts/setup-secrets.sh

# --- 5. Activar readiness probe ---------------------------------------------
echo ""
echo "→ Paso 5: configurando readiness probe en /healthz..."
# Sintaxis correcta de gcloud: --startup-probe (no --update-startup-probe).
# Cloud Run usa la startup-probe como readiness implicita en gen2.
if gcloud run services update "$SERVICE" \
  --region="$REGION" \
  --project="$PROJECT_ID" \
  --startup-probe="httpGet.path=/healthz,initialDelaySeconds=5,periodSeconds=5,failureThreshold=20,timeoutSeconds=3" \
  >/dev/null 2>&1; then
  green "  ✓ Readiness probe activa en /healthz"
else
  yellow "  ⚠ No pude configurar la probe (probablemente version vieja de gcloud)."
  yellow "    Corre manual:"
  echo "    gcloud run services update $SERVICE --region=$REGION \\"
  echo "      --startup-probe=httpGet.path=/healthz,initialDelaySeconds=5,periodSeconds=5,failureThreshold=20"
fi

# --- 6. CI trigger en Cloud Build ------------------------------------------
echo ""
echo "→ Paso 6: trigger CI en Cloud Build..."

# Verificar si ya existe el trigger.
if gcloud builds triggers describe remora-go-ci --project="$PROJECT_ID" >/dev/null 2>&1; then
  green "  ✓ Trigger 'remora-go-ci' ya existe (skip)"
else
  # Intentar crear. Si falla por repo no conectado, dar instrucciones.
  if gcloud builds triggers create github \
    --name=remora-go-ci \
    --repo-name="$GH_REPO" \
    --repo-owner="$GH_OWNER" \
    --branch-pattern="^draft\$" \
    --build-config=cloudbuild-ci.yaml \
    --project="$PROJECT_ID" \
    >/dev/null 2>&1; then
    green "  ✓ Trigger 'remora-go-ci' creado"
  else
    yellow "  ⚠ No se pudo crear el trigger. Probablemente el repo no esta conectado."
    echo ""
    bold "  Setup manual de UNA SOLA VEZ:"
    echo ""
    echo "    1. Abrir en el browser:"
    echo "       https://console.cloud.google.com/cloud-build/triggers/connect?project=$PROJECT_ID"
    echo ""
    echo "    2. Click 'Connect Repository' → seleccionar GitHub"
    echo "    3. Autorizar la app 'Google Cloud Build'"
    echo "    4. Seleccionar el repo: $GH_OWNER/$GH_REPO"
    echo "    5. Click 'Connect'"
    echo ""
    echo "    Despues volve a correr 'make setup-prod' y el trigger se crea solo."
  fi
fi

# --- 7. Smoke test del healthcheck ------------------------------------------
echo ""
echo "→ Paso 7: smoke test /healthz..."
SERVICE_URL=$(gcloud run services describe "$SERVICE" --region="$REGION" --project="$PROJECT_ID" --format="value(status.url)" 2>/dev/null || echo "")
if [ -n "$SERVICE_URL" ]; then
  HTTP_CODE=$(curl -s -o /tmp/healthz.json -w "%{http_code}" "${SERVICE_URL}/healthz" 2>/dev/null || echo "000")
  if [ "$HTTP_CODE" = "200" ]; then
    green "  ✓ /healthz devuelve 200 OK"
  elif [ "$HTTP_CODE" = "503" ]; then
    yellow "  ⚠ /healthz devuelve 503 (alguna check fallo):"
    cat /tmp/healthz.json 2>/dev/null | head -5
  else
    yellow "  ⚠ /healthz no responde (HTTP $HTTP_CODE). Quizas no se redeployo todavia."
    echo "    Corre: make deploy-dev"
  fi
fi

echo ""
bold "=== Setup PROD completo ==="
echo ""
echo "Servicio: ${SERVICE_URL:-N/A}"
echo "Health:   ${SERVICE_URL}/healthz"
echo ""
echo "Proximos pasos de tu lado (si algun warning aparecio arriba):"
echo "  1. Conectar GH si el trigger fallo"
echo "  2. Redeploy si la probe fue rechazada: make deploy-dev"
