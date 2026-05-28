

## Resumen del Análisis - pi-analizador

He extraído **45 axiomas** de la narrativa ideal del Narrador para la CLI `remora flow debug`:

### Axiomas Extraídos

| Categoría | Cantidad | Descripción |
|-----------|----------|-------------|
| **Símbolos** | 20 | Responsabilidades de cada componente del sistema |
| **Interacción** | 15 | Comportamiento entre componentes al interactuar |
| **Validación** | 26 | Reglas para que el flujo proceda correctamente |
| **Integridad** | 20 | Garantías de que el sistema es correcto |

### Comandos Priorizados

| Criticidad | Comandos |
|------------|----------|
| **CRITICAL** (MVP) | `list-frameworks`, `manifest`, `dry-run`, `validate`, `execute`, `trace` |
| **IMPORTANT** (v1.0) | `inspect-inputs`, `inspect-outputs`, `dependencies`, `gaps`, `frameworks-only`, `framework --commands/--capabilities` |
| **NICE-TO-HAVE** | `capabilities-map`, `history`, `diff`, `interactive`, `config init` |

### Formatos de Salida Definidos

- **Table**: Ancho fijo (NOMBRE 20, VERSION 10, CAPABILITIES 40, GRUPOS 15)
- **JSON**: Estructura con `{command, status, timestamp, data}`
- **YAML**: Parseable y válido

### Estados de Ejecución

- Pasos: `waiting → running → success/failed/skipped`
- Flujo: `pending → running → completed/failed`

**Siguiente paso**: `pi-cartografo` convertirá estos axiomas a queries de Joern (una query por axioma).

<!-- CHAIN_RUN_ID: c28cea68 -->