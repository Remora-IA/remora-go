#!/usr/bin/env bash
# =============================================================================
# Setup de Secret Manager para Cloud Run dev
# =============================================================================
# Lee secretos del .env local y los sube a GCP Secret Manager. Despues monta
# cada secreto como env var en el servicio Cloud Run dev.
#
# Idempotente: si el secreto ya existe, le agrega una nueva version en lugar
# de fallar.
#
# Pre-requisitos:
#   - gcloud autenticado: gcloud auth login
#   - APIs habilitadas: secretmanager.googleapis.com, run.googleapis.com
#   - .env presente con los valores reales (no .env.example)
# =============================================================================

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PROJECT_ID="${PROJECT_ID:-project-ceae5831-a2c9-49aa-b1c}"
REGION="${REGION:-us-central1}"
SERVICE="${SERVICE:-flujo-api-dev}"

green() { printf "\033[32m%s\033[0m\n" "$1"; }
yellow() { printf "\033[33m%s\033[0m\n" "$1"; }
red() { printf "\033[31m%s\033[0m\n" "$1"; }

# Secrets que se suben a Secret Manager (sensitive).
# Las env vars puramente configuracionales (REMORA_PROFILE, REMORA_DEV_MODE)
# se setean directo en Cloud Run con --set-env-vars, no via Secret Manager.
SENSITIVE_KEYS=(
  GROQ_API_KEY
  MINIMAX_API_KEY
  REMORA_VAULT_KEY
  HOSTING_VAULT_KEY
  SMTP_USER
  SMTP_PASS
)

if [ ! -f .env ]; then
  red "❌ Falta .env. Corre 'make bootstrap' primero."
  exit 1
fi

if ! command -v gcloud >/dev/null 2>&1; then
  red "❌ gcloud no instalado."
  exit 1
fi

echo "=== Setup secrets para $SERVICE en $PROJECT_ID ==="
echo ""

# --- 1. Habilitar APIs (idempotente) ---------------------------------------
echo "→ Verificando APIs..."
gcloud services enable secretmanager.googleapis.com --project="$PROJECT_ID" >/dev/null 2>&1 || true
green "  ✓ secretmanager API habilitada"

# --- 2. Subir cada secret ---------------------------------------------------
echo ""
echo "→ Subiendo secrets..."
SECRETS_TO_BIND=()
for key in "${SENSITIVE_KEYS[@]}"; do
  value=$(grep -E "^${key}=" .env | cut -d= -f2- | tr -d '"' | tr -d "'" | head -1)
  if [ -z "$value" ]; then
    yellow "  ⚠ $key vacia en .env, skip"
    continue
  fi

  # Crear secret si no existe.
  if gcloud secrets describe "$key" --project="$PROJECT_ID" >/dev/null 2>&1; then
    echo "  · $key existe, agregando nueva version..."
  else
    gcloud secrets create "$key" \
      --replication-policy="automatic" \
      --project="$PROJECT_ID" >/dev/null
    echo "  · $key creado"
  fi

  # Subir nueva version.
  printf "%s" "$value" | gcloud secrets versions add "$key" \
    --data-file=- \
    --project="$PROJECT_ID" >/dev/null
  green "    ✓ version subida"

  SECRETS_TO_BIND+=("${key}=${key}:latest")
done

# --- 3. Bind secrets al servicio Cloud Run ---------------------------------
echo ""
echo "→ Bindeando secrets a $SERVICE..."
if [ ${#SECRETS_TO_BIND[@]} -eq 0 ]; then
  yellow "  ⚠ No hay secrets para bindear"
  exit 0
fi

# --update-secrets reemplaza los bindings existentes con esta lista.
SECRETS_ARG=$(IFS=,; echo "${SECRETS_TO_BIND[*]}")
gcloud run services update "$SERVICE" \
  --region="$REGION" \
  --project="$PROJECT_ID" \
  --update-secrets="$SECRETS_ARG" \
  >/dev/null
green "  ✓ Secrets bindeados a $SERVICE"

# --- 4. Env vars no-sensibles ---------------------------------------------
echo ""
echo "→ Seteando env vars no-sensibles..."
gcloud run services update "$SERVICE" \
  --region="$REGION" \
  --project="$PROJECT_ID" \
  --update-env-vars="REMORA_PROFILE=cobranza-chile,REMORA_DEV_MODE=true,TEST_EMAIL_RECIPIENT=tom3bs@gmail.com,CHANNEL_BASE_DIR=/workspace,REMORA_LLM_PROVIDER=groq" \
  >/dev/null
green "  ✓ Env vars seteadas"

echo ""
green "=== Listo ==="
echo "Verificar:"
echo "  gcloud run services describe $SERVICE --region=$REGION --project=$PROJECT_ID"
echo "  curl https://${SERVICE}-760602975866.${REGION}.run.app/healthz"
