# Guía de Commits — Remora Go

Este documento explica qué archivos son **código fuente** (hay que commitear) y cuáles son
**artefactos generados/temporales** (no commitear). Está pensado para que cualquier agente
de IA (Charlie, Devin, etc.) pueda decidir correctamente qué incluir en un commit.

---

## Regla general

> **Solo se versiona código fuente y configuración.**
> Binarios, estado de runtime, logs, secrets y datos de cliente **nunca** van al repo.

Si dudás: ¿se puede regenerar haciendo `go build`, ejecutando el framework, o corriendo un
script? → **No commitear.**

---

## Archivos temporales / generados

### 1. Binarios compilados (`go build`)

Cada framework y servicio compila a un binario ejecutable. Estos se regeneran con `go build`
y **nunca** se versionan.

| Patrón | Ejemplo | Cómo se regenera |
|---|---|---|
| `framework-*/framework*` | `framework-auditor/frameworkauditor` | `cd framework-auditor && go build ./cmd/...` |
| `framework-*/nombre_alterno` | `framework-mecanico/mecanico` | `go build ./cmd/mecanico` |
| `framework-pingpong/pingpong` | — | `go build ./cmd/...` |
| `framework-pingpong/two_sum` | — | `go build` de ejemplo |
| `remora-flujo/flujo` | — | `cd remora-flujo && go build ./cmd/flujo` |
| `remora-flujo/flujo_api` | — | `go build ./cmd/api_rest` |
| `remora-flujo/flujo_test` | — | `go test -c` |
| `remora-flujo/framework_session` | — | `go build ./cmd/framework_session` |
| `remora-cli/remora` | — | `go build .` |
| `channel/channel`, `channel/orchestrator` | — | `go build ./cmd/...` |

**Cómo identificar un binario:** `file <archivo>` muestra "Mach-O executable" o "ELF executable".

### 2. Directorios `temp/` — Estado de runtime

Cada framework escribe estado, cache y traces en su directorio `temp/`. Se crea
automáticamente al ejecutar y cada deploy parte limpio.

| Patrón | Contenido | Impacto de no commitear |
|---|---|---|
| `**/temp/state.json` | Estado de sesión activa del framework | Ninguno. El framework crea uno nuevo al iniciar. |
| `**/temp/paladin/trace_pal_*.json` | Traces de Paladin (debugging) | Ninguno. Son logs de ejecución. |
| `**/temp/paladin/trace_gf_*.json` | Traces de framework (debugging) | Ninguno. Son logs de ejecución. |
| `framework-foco/temp/*/go.mod` | Proyectos Go generados por Foco | Ninguno. Foco los genera en cada sesión. |
| `framework-foco/temp/QUINE_PROMPT_*.md` | Prompts auto-generados | Ninguno. Se regeneran. |
| `framework-bravo/examples/*/temp/` | Datos de ejemplo procesados | Ninguno. Se regeneran con el ejemplo. |

### 3. Archivos de estado de sesión (fuera de `temp/`)

Algunos frameworks guardan estado de sesión en archivos que parecen config pero son data:

| Archivo | Qué contiene | Impacto de no commitear |
|---|---|---|
| `framework-echo/frameworkecho.json` | Sesión activa de Echo (nodos axioma/theory/task del cliente actual) | Ninguno. Echo crea una sesión nueva al iniciar con un cliente. |
| `framework-echo/frameworkecho_backup_*.json` | Backups automáticos de sesiones anteriores | Ninguno. |
| `framework-auditor/data/working.json` | Datos de auditoría en progreso | Ninguno. Se regenera al analizar. |
| `framework-auditor/data/findings.json` | Hallazgos de auditoría | Ninguno. Se regenera al analizar. |
| `framework-mecanico/data/applied.jsonl` | Registro de cambios aplicados | Ninguno. Cada instancia lleva su propio registro. |
| `framework-mecanico/data/proposals.json` | Propuestas pendientes | Ninguno. Se regenera al analizar. |
| `framework-foco/foco_state.json` | Estado de sesión de Foco | Ninguno. Se crea al iniciar sesión. |

### 4. Logs

| Patrón | Descripción |
|---|---|
| `*.log` | Logs de cualquier servicio |
| `trace_pal_*.json` | Traces de Paladin |
| `trace_gf_*.json` | Traces de framework |
| `sessions/` | Logs de conversación del orquestador |

### 5. Secrets y datos de cliente

| Patrón | Descripción |
|---|---|
| `.env`, `.env.local` | Variables de entorno con API keys |
| `*.enc`, `creds_*.enc` | Credenciales encriptadas |
| `**/vault_data/` | Vault local |
| `*.db` | SQLite con datos reales de clientes |

---

## Qué SÍ commitear

| Tipo | Extensiones / patrones | Ejemplo |
|---|---|---|
| Código Go | `*.go` | `framework-pingpong/internal/paladin/client.go` |
| Manifiestos de framework | `framework.manifest.json` | `framework-sabio/framework.manifest.json` |
| Configuración de build | `go.mod`, `go.sum`, `Dockerfile`, `cloudbuild.yaml` | `remora-flujo/go.mod` |
| Scripts de infraestructura | `*.sh` en `scripts/` | `scripts/bootstrap.sh` |
| Frontend | `*.html`, `*.css`, `*.js` en `static/` | `remora-flujo/cmd/api_rest/static/index.html` |
| Documentación | `*.md` | `README.md`, este archivo |
| Tests | `*_test.go` | `framework-pingpong/internal/paladin/audit_test.go` |
| `.gitignore` | — | Siempre commitear cambios aquí |

---

## Checklist rápido para un commit

Antes de hacer `git add`, verificá cada archivo con estas preguntas:

1. **¿Es un binario ejecutable?** → No commitear. Verificar con `file <archivo>`.
2. **¿Está en un directorio `temp/`?** → No commitear.
3. **¿Es un `state.json` o similar?** → No commitear (ver tabla arriba).
4. **¿Es un trace (`trace_pal_*`, `trace_gf_*`)?** → No commitear.
5. **¿Es un `.env` o credencial?** → NUNCA commitear.
6. **¿Es código fuente (`.go`, `.html`, `.md`, `.sh`)?** → Sí commitear.
7. **¿Es configuración (`go.mod`, `manifest.json`, `.gitignore`)?** → Sí commitear.
8. **¿No está en esta lista?** → Actualizar este documento y decidir.

---

## Nota sobre archivos "tracked" que no deberían estarlo

Algunos binarios y archivos temporales fueron commiteados históricamente antes de que
se agregaran al `.gitignore`. Git los sigue trackeando aunque estén en el `.gitignore`.
Esto es normal — `.gitignore` solo previene que archivos **nuevos** se agreguen, no
destrackea los existentes.

Si `git status` muestra cambios en un binario que ya está en `.gitignore`, simplemente
**no hacer `git add`** de ese archivo. No hay que hacer nada especial.

Si en algún momento se quiere limpiar definitivamente:
```bash
git rm --cached <archivo_binario>   # destrackea sin borrar el archivo local
```
Esto se puede hacer en un commit de limpieza dedicado cuando el equipo lo decida.
