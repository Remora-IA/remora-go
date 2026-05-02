#!/usr/bin/env bash
# Demo end-to-end del flujo Auditor → Mecánico sobre el dataset real
# de timebilling. Funciona sin Channel ni flujo_api: invoca los binarios
# directamente para mostrar el flujo de detección y reparación.
#
# Para resetear el demo entre corridas: este mismo script lo hace al inicio.
#
# Requisitos: ya haber compilado:
#   cd framework-auditor  && go build -buildvcs=false -o frameworkauditor  ./cmd/frameworkauditor
#   cd framework-mecanico && go build -buildvcs=false -o frameworkmecanico ./cmd/frameworkmecanico

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AUDITOR="$ROOT/framework-auditor"
MECANICO="$ROOT/framework-mecanico"

BOLD=$'\033[1m'; CYAN=$'\033[36m'; GREEN=$'\033[32m'; YELLOW=$'\033[33m'; DIM=$'\033[2m'; RST=$'\033[0m'

step() { echo; echo "${BOLD}${CYAN}━━━ $1 ━━━${RST}"; }
narrate() { echo "${DIM}$1${RST}"; }
ask() { echo; read -p "${YELLOW}↪ $1 [Enter para continuar]${RST}" _; }

# Asegurar binarios.
if [[ ! -x "$AUDITOR/frameworkauditor" ]]; then
  step "Compilando auditor"
  (cd "$AUDITOR" && go build -buildvcs=false -o frameworkauditor ./cmd/frameworkauditor)
fi
if [[ ! -x "$MECANICO/frameworkmecanico" ]]; then
  step "Compilando mecánico"
  (cd "$MECANICO" && go build -buildvcs=false -o frameworkmecanico ./cmd/frameworkmecanico)
fi

step "1. RESET — restauramos el dataset al estado original (con errores reales)"
narrate "El auditor copia dataset.golden.json → dataset.working.json. El mecánico borra propuestas y log."
(cd "$AUDITOR"  && ./frameworkauditor  reset)
(cd "$MECANICO" && ./frameworkmecanico reset)

ask "Listo el reset. Pasamos al scan inicial"

step "2. AUDITOR SCAN — el agente revisa proactivamente todos los registros"
narrate "Sin que nadie le diga qué auditar, recorre el ERP entero y emite hallazgos."
(cd "$AUDITOR" && ./frameworkauditor scan)

ask "Veamos el listado completo de hallazgos"

step "3. AUDITOR LIST — qué encontró exactamente"
(cd "$AUDITOR" && ./frameworkauditor list) | head -60
echo "${DIM}(salida truncada a 60 líneas para legibilidad)${RST}"

ask "Vamos a ver el detalle de un hallazgo específico"

step "4. AUDITOR DETAIL F-004 — número de factura inválido (-1)"
(cd "$AUDITOR" && ./frameworkauditor detail --id F-004)

ask "Ahora interviene el mecánico"

step "5. MECANICO PROPOSE-ALL-AUTO — genera plan de fix SIN aplicar nada"
narrate "Mostramos qué cambios propone. El usuario decide si aplica."
(cd "$MECANICO" && ./frameworkmecanico propose-all-auto)

ask "${YELLOW}APROBACIÓN DEL USUARIO${RST} — ¿aplicamos los fixes propuestos?"

step "6. MECANICO APPLY-ALL — recién ahora muta el dataset real"
(cd "$MECANICO" && ./frameworkmecanico apply-all)

ask "Verificamos que el ERP quedó arreglado"

step "7. AUDITOR RESCAN — el contador de hallazgos debe bajar"
(cd "$AUDITOR" && ./frameworkauditor scan)

step "8. AUDIT TRAIL — qué cambió exactamente (compliance-ready)"
narrate "Cada fix queda en applied.jsonl con before/after y timestamp."
echo
cat "$MECANICO/data/applied.jsonl" | head -5
echo "${DIM}(...)${RST}"
echo
echo "Total fixes aplicados: $(wc -l < "$MECANICO/data/applied.jsonl")"

step "9. RESET FINAL — listo para volver a mostrar el demo"
narrate "Un solo comando devuelve el dataset al estado original con todos los errores."
(cd "$AUDITOR"  && ./frameworkauditor  reset)
(cd "$MECANICO" && ./frameworkmecanico reset)
echo
echo "${GREEN}${BOLD}Demo completa. Para volver a correr: $0${RST}"
