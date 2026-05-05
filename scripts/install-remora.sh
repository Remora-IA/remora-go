#!/usr/bin/env bash
# =============================================================================
# install-remora.sh — Instala `remora` como comando global.
#
# Compila remora-cli y lo linkea en ~/.local/bin/remora (o el primer directorio
# en PATH que el usuario pueda escribir). Asegura que ~/.local/bin esté en PATH
# vía ~/.zshrc o ~/.bashrc si falta.
#
# Uso:
#   bash scripts/install-remora.sh
#
# Flags (env vars):
#   INSTALL_DIR=/path        fuerza destino (default: ~/.local/bin)
#   REMORA_API_URL=<url>     URL del flujo_api a embebir en el wrapper
# =============================================================================

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CLI_DIR="$REPO_ROOT/remora-cli"
BIN_NAME="remora"

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
API_URL_DEFAULT="${REMORA_API_URL:-http://localhost:8084/api/v1}"

echo "=== remora install ==="
echo "repo:      $REPO_ROOT"
echo "bin name:  $BIN_NAME"
echo "install:   $INSTALL_DIR"
echo "api url:   $API_URL_DEFAULT"
echo ""

# ── 1) Compilar ────────────────────────────────────────────────────────────
echo "── 1) Compilando remora-cli ──"
cd "$CLI_DIR"
go build -o "$CLI_DIR/remora" ./cmd/remora
echo "   ✓ $CLI_DIR/remora"
echo ""

# ── 2) Crear wrapper ───────────────────────────────────────────────────────
# El wrapper setea REMORA_API_URL si el usuario no la exportó y después
# exec-ea el binario real. Así `remora` funciona desde cualquier cwd.
mkdir -p "$INSTALL_DIR"
WRAPPER="$INSTALL_DIR/$BIN_NAME"

cat > "$WRAPPER" <<EOF
#!/usr/bin/env bash
# Wrapper auto-generado por scripts/install-remora.sh
: "\${REMORA_API_URL:=$API_URL_DEFAULT}"
export REMORA_API_URL
exec "$CLI_DIR/remora" "\$@"
EOF
chmod +x "$WRAPPER"
echo "── 2) Wrapper instalado ──"
echo "   ✓ $WRAPPER"
echo ""

# ── 3) Verificar PATH ──────────────────────────────────────────────────────
echo "── 3) Chequeando PATH ──"
case ":$PATH:" in
  *":$INSTALL_DIR:"*)
    echo "   ✓ $INSTALL_DIR ya está en PATH"
    ;;
  *)
    echo "   ⚠  $INSTALL_DIR NO está en PATH"
    # Intentar agregarlo al rc file correspondiente.
    SHELL_RC=""
    if [ -n "${ZSH_VERSION:-}" ] || [ "$(basename "${SHELL:-}")" = "zsh" ]; then
      SHELL_RC="$HOME/.zshrc"
    elif [ -n "${BASH_VERSION:-}" ] || [ "$(basename "${SHELL:-}")" = "bash" ]; then
      SHELL_RC="$HOME/.bashrc"
    fi
    if [ -n "$SHELL_RC" ]; then
      EXPORT_LINE="export PATH=\"$INSTALL_DIR:\$PATH\""
      if [ -f "$SHELL_RC" ] && grep -q "$INSTALL_DIR" "$SHELL_RC"; then
        echo "   ✓ $SHELL_RC ya referencia $INSTALL_DIR"
      else
        echo "" >> "$SHELL_RC"
        echo "# agregado por remora-go/scripts/install-remora.sh" >> "$SHELL_RC"
        echo "$EXPORT_LINE" >> "$SHELL_RC"
        echo "   ✓ agregado a $SHELL_RC — reabrí la terminal o:"
        echo "       source $SHELL_RC"
      fi
    else
      echo "   Agregá a tu shell rc:"
      echo "       export PATH=\"$INSTALL_DIR:\$PATH\""
    fi
    ;;
esac
echo ""

# ── 4) Verificación ────────────────────────────────────────────────────────
echo "── 4) Verificación ──"
if command -v "$BIN_NAME" >/dev/null 2>&1; then
  WHICH_PATH="$(command -v "$BIN_NAME")"
  echo "   ✓ which $BIN_NAME → $WHICH_PATH"
else
  echo "   ⚠  '$BIN_NAME' no resuelve en PATH todavía. Ejecutá el wrapper directo:"
  echo "       $WRAPPER"
fi
echo ""
echo "Listo."
echo ""
echo "Uso:"
echo "   remora                → modo pair-programming (arquitecto)"
echo "   remora code           → idem"
echo "   remora c              → idem (corto)"
echo "   remora chat --frameworks alfa,echo"
echo "   remora frameworks"
echo ""
echo "Pre-requisito: flujo_api corriendo. Levantar con:"
echo "   bash $REPO_ROOT/scripts/dev-local.sh"
