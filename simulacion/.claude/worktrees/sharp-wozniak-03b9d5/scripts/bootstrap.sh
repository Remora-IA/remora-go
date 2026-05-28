#!/usr/bin/env bash
# =============================================================================
# Remora Go - Bootstrap
# =============================================================================
# Setup inicial del repo. Idempotente: corerlo varias veces no rompe nada.
#
#   1. Verifica dependencias (go, gcloud opcional).
#   2. Crea .env desde .env.example si no existe.
#   3. Genera REMORA_VAULT_KEY si esta vacia.
#   4. Compila todos los binarios.
#   5. Reporta estado final.
# =============================================================================

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

green() { printf "\033[32m%s\033[0m\n" "$1"; }
yellow() { printf "\033[33m%s\033[0m\n" "$1"; }
red() { printf "\033[31m%s\033[0m\n" "$1"; }
bold() { printf "\033[1m%s\033[0m\n" "$1"; }

bold "=== Remora Go Bootstrap ==="

# --- 1. Dependencias --------------------------------------------------------
echo ""
echo "→ Verificando dependencias..."
if ! command -v go >/dev/null 2>&1; then
  red "❌ Go no esta instalado. Instalalo desde https://go.dev/dl/"
  exit 1
fi
go_version=$(go version | awk '{print $3}')
green "  ✓ Go: $go_version"

if command -v gcloud >/dev/null 2>&1; then
  green "  ✓ gcloud: $(gcloud --version 2>/dev/null | head -1)"
else
  yellow "  ⚠ gcloud no instalado (necesario solo para 'make deploy-dev')"
fi

# --- 2. .env ----------------------------------------------------------------
echo ""
echo "→ Configurando .env..."
if [ ! -f .env ]; then
  if [ ! -f .env.example ]; then
    red "❌ Falta .env.example. Repo corrupto?"
    exit 1
  fi
  cp .env.example .env
  green "  ✓ .env creado desde .env.example"
else
  green "  ✓ .env ya existe"
fi

# --- 3. Vault key -----------------------------------------------------------
echo ""
echo "→ Verificando REMORA_VAULT_KEY..."
current_key=$(grep -E "^REMORA_VAULT_KEY=" .env | cut -d= -f2- | tr -d '"' | tr -d "'")
if [ -z "$current_key" ] || [ "$current_key" = "" ]; then
  yellow "  ⚠ REMORA_VAULT_KEY vacia, generando una nueva..."
  new_key=$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p -c 64)
  if [ -z "$new_key" ]; then
    red "❌ No pude generar la clave (falta openssl o xxd)"
    exit 1
  fi
  # Reemplazar in-place (compatible BSD/GNU sed)
  if sed --version >/dev/null 2>&1; then
    sed -i "s|^REMORA_VAULT_KEY=.*|REMORA_VAULT_KEY=$new_key|" .env
    sed -i "s|^HOSTING_VAULT_KEY=.*|HOSTING_VAULT_KEY=$new_key|" .env
  else
    sed -i '' "s|^REMORA_VAULT_KEY=.*|REMORA_VAULT_KEY=$new_key|" .env
    sed -i '' "s|^HOSTING_VAULT_KEY=.*|HOSTING_VAULT_KEY=$new_key|" .env
  fi
  green "  ✓ REMORA_VAULT_KEY generada y persistida en .env"
else
  green "  ✓ REMORA_VAULT_KEY ya seteada"
fi

# --- 4. API keys (warning si faltan) ---------------------------------------
echo ""
echo "→ Verificando API keys..."
groq=$(grep -E "^GROQ_API_KEY=" .env | cut -d= -f2- | tr -d '"' | tr -d "'")
minimax=$(grep -E "^MINIMAX_API_KEY=" .env | cut -d= -f2- | tr -d '"' | tr -d "'")

if [ -z "$groq" ] && [ -z "$minimax" ]; then
  yellow "  ⚠ Ni GROQ_API_KEY ni MINIMAX_API_KEY estan seteadas."
  yellow "    Editar .env y completar al menos una antes de 'make dev'."
else
  [ -n "$groq" ] && green "  ✓ GROQ_API_KEY seteada" || yellow "  ⚠ GROQ_API_KEY no seteada"
  [ -n "$minimax" ] && green "  ✓ MINIMAX_API_KEY seteada" || yellow "  ⚠ MINIMAX_API_KEY no seteada"
fi

# --- 5. Compilacion ---------------------------------------------------------
echo ""
echo "→ Compilando binarios..."
make build-flujo 2>&1 | sed 's/^/  /'
green "  ✓ Build OK"

# --- 6. Resumen -------------------------------------------------------------
echo ""
bold "=== Bootstrap completo ==="
echo ""
echo "Proximos pasos:"
echo "  1. Editar .env y completar GROQ_API_KEY o MINIMAX_API_KEY si falta."
echo "  2. make dev      # arranca api_rest en :8084"
echo "  3. make restart-api  # reinicia recompilando el backend"
echo "  4. make test     # corre la suite"
echo ""
