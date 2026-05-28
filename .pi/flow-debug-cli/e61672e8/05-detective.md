

## 🔍 PI-DETECTIVE: Reporte Completo

He investigado el estado actual de `remora-cli` y comparado contra el IDEAL. Aquí están los hallazgos:

---

### 📊 **Estado Actual vs IDEAL**

| Componente | Estado | Detalle |
|------------|--------|---------|
| `remora flow debug` usage | ✅ Existe | Mencionado en línea 288 de `flow_workbench.go` |
| `handleFlowDebug()` | ❌ **FALTA** | No existe función en `remora-cli` |
| API endpoint `/flows/run/stream` | ✅ Existe | Soporte SSE completo en `remora-flujo` |
| Implementación canonical | ✅ Existe | `runFlowDebug()` en `flujo/cmd/flujo/flow_workbench.go` |
| Tipos Go necesarios | ✅ Existen | `FlowRunResult`, `FlowRunStep`, etc. |

---

### 🚨 **GAP #1: `handleFlowDebug()` NO implementada**

```go
// Commands que SÍ existen en remora-cli:
func handleFlowWorkbench()       // línea 237
func handleFlowCreate()          // línea 482
func handleFlowDraft()           // línea 603
func handleFlowInspect()         // línea 680
func handleFlowSimulate()        // línea 698

// Command que FALTA:
func handleFlowDebug(args []string)  // ❌ NO EXISTE
```

**Impacto:** El usuario ejecuta `remora flow debug` → no pasa nada o error.

---

### 🚨 **GAP #2: Routing no captura "debug"**

El `handleFlowWorkbench()` no tiene lógica para detectar y manejar el subcomando "debug".

---

### 🗂️ **SLICE a Modificar**

| Archivo | Acción | Magnitud |
|---------|--------|----------|
| `remora-cli/cmd/remora/flow_workbench.go` | **AGREGAR** `handleFlowDebug()` | ~300 líneas |
| `remora-cli/cmd/remora/main.go` | Verificar/adicionar tipos SSE | ~50 líneas |

---

### 🔗 **Dependencies: API Endpoints Disponibles**

| Endpoint | Método | Para qué sirve |
|----------|--------|---------------|
| `/flows/run/stream` | **POST** | Streaming SSE de debug (PRIMARIO) |
| `/flows/runs/{id}` | GET | Obtener run completo |
| `/flows/{id}` | GET | Obtener flow record |
| `/flows/compiled/{compiled_id}` | GET | Obtener flow compilado |

---

### 💡 **Workaround Actual**

El comando `remora flow debug` delega a `flujo flow debug` si:
1. Se ejecuta desde repo root con `go run ./cmd/remora`
2. O si `flujo` está en PATH

---

**Reporte guardado en:** `.pi/flow-debug-cli/e61672e8/05-detective/output.md`