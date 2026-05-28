# 🔍 PI-DETECTIVE: Investigación remora-cli flow debug

**CHAIN_RUN_ID:** e61672e8  
**Fecha:** 2026-05-16  
**Proyecto:** remora-go-lite / remora-cli

---

## FASE 1: Investigación del Estado Actual

### Archivos Investigados

| Archivo | Líneas | Tipo |
|---------|--------|------|
| `remora-cli/cmd/remora/main.go` | 350 | API Client (Client, FlowRunResult, etc.) |
| `remora-cli/cmd/remora/flow_workbench.go` | 750+ | Comandos CLI de flow |
| `remora-flujo/cmd/api_rest/main.go` | 1806 | API REST endpoints |
| `remora-flujo/cmd/flujo/flow_workbench.go` | 1300+ | Implementación canonical de flow workbench |

### Endpoint de API Disponible para Debug

```
POST /api/v1/flows/run/stream
```

Eventos SSE emitidos:
- `step_start` - cuando inicia un step
- `step_complete` - cuando termina un step
- `flow_complete` - cuando termina el flow
- `needs_input` - cuando el flow necesita input
- `error` - cuando hay un error

Payload del request:
```json
{
  "compiled_id": "...",
  "input": "...",
  "dry_run": true,
  "fixture_artifacts": ["artifact1", "artifact2"]
}
```

---

## FASE 2: Análisis de GAPS

### GAP #1: `remora-cli` NO implementa `handleFlowDebug()`

**Estado IDEAL:** El comando `remora flow debug --id <flow_id>` debería existir y funcionar.

**Estado ACTUAL:**

```go
// remora-cli/cmd/remora/flow_workbench.go

// Commands implementados:
func handleFlowWorkbench()         // línea 237
func handleFlowCreate(args []string)  // línea 482
func handleFlowDraft(args []string)   // línea 603
func handleFlowInspect(args []string) // línea 680
func handleFlowSimulate(args []string) // línea 698

// MISSING: handleFlowDebug() - NO EXISTE
```

**Evidencia en usage message (línea 288):**
```
remora flow debug --id <flow_id> [--fixtures a,b] [--input texto] [--step] [--break-on handoff,needs_input,approval]
```

El usage menciona `debug` pero la función `handleFlowDebug()` NO está implementada en remora-cli.

### GAP #2: Delegación a `flujo flow debug` puede tener problemas

**Delegate implementation (líneas 252-280):**
```go
func delegateToCanonicalFlowWorkbench(args []string) error {
    if len(args) == 0 {
        printFlowWorkbenchUsage()
        return nil
    }
    cmd, err := canonicalFlowWorkbenchCommand(args)
    if err != nil {
        return err
    }
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}

func canonicalFlowWorkbenchCommand(args []string) (*exec.Cmd, error) {
    if repoRoot := findRepoRoot(); repoRoot != "" {
        cmd := flowWorkbenchExecCommand("go", append([]string{"run", "./cmd/flujo", "flow"}, args...)...)
        cmd.Dir = filepath.Join(repoRoot, "remora-flujo")
        return cmd, nil
    }
    if _, err := exec.LookPath("flujo"); err == nil {
        return flowWorkbenchExecCommand("flujo", append([]string{"flow"}, args...)...), nil
    }
    return nil, fmt.Errorf("flow workbench canónico no disponible; usá `flujo flow ...`")
}
```

**Problema potencial:** La delegación solo busca `flujo` en PATH o hace `go run ./cmd/flujo flow`. Si el usuario está dentro de `remora-cli/` y ejecuta `go run`, podría fallar porque el repo root no tiene la estructura esperada.

### GAP #3: Test de delegación menciona debug (pero no verifica su ejecución)

```go
// flow_workbench_delegate_test.go:35
// "remora flow debug --id <flow_id>",
```

El test existe para verificar que `debug` se delega correctamente, pero no hay implementación real de `handleFlowDebug()` en el CLI wrapper.

---

## FASE 3: SLICES - Archivos a Modificar

### Slice Principal: `handleFlowDebug()` en remora-cli

```
SOURCE:  printFlowWorkbenchUsage() (menciona "remora flow debug")
         ↓
SINK:    Command doesn't exist → error o silencio
         
MIDDLE:  No hay handler que capture los argumentos de "debug"
```

### Archivos a Modificar

| # | Archivo | Acción | Líneas Involucradas |
|---|---------|--------|---------------------|
| 1 | `remora-cli/cmd/remora/flow_workbench.go` | AGREGAR: `handleFlowDebug()` | ~700-800 líneas de nuevo código |
| 2 | `remora-cli/cmd/remora/main.go` | AGREGAR: Tipos para eventos SSE debug | ~50 líneas |
| 3 | `remora-cli/cmd/remora/flow_workbench.go` | MODIFICAR: Routing en `handleFlowWorkbench()` para capturar "debug" | ~5 líneas |

### Código Similar de Referencia (en flujo, línea 1132):

```go
func runFlowDebug(out io.Writer, client *flowWorkbenchClient, args []string) error {
    fs := newFlowFlagSet("flow debug")
    flowID := fs.String("id", "", "flow id")
    input := fs.String("input", "", "texto de prueba")
    fixtures := fs.String("fixtures", "", "artifacts separados por coma")
    stepMode := fs.Bool("step", false, "pausar en cada step_complete")
    breakOn := fs.String("break-on", "", "handoff,needs_input,approval")
    dryRun := fs.Bool("dry-run", true, "usar dry-run por defecto")
    // ... stream handling ...
}
```

---

## FASE 4: Resumen de Dependencies

### API Endpoints Disponibles (del análisis de main.go)

| Endpoint | Método | Uso |
|----------|--------|-----|
| `/flows/run/stream` | POST | **PRIMARIO** para debug - streaming SSE |
| `/flows/runs/{id}` | GET | Obtener run completo después |
| `/flows/{id}` | GET | Obtener flow record |
| `/flows/compiled/{compiled_id}` | GET | Obtener flow compilado |
| `/businesses/{business_id}/flows` | POST | Listar flows de un negocio |

### Tipos Existentes en remora-cli/main.go

```go
// Ya existen en Client:
func (c *Client) get(path string) (map[string]interface{}, error)
func (c *Client) post(path string, body interface{}) (map[string]interface{}, error)

// Ya existen tipos:
FlowRunResult       // Para parsear resultado final
FlowRunStep         // Para steps individuales
FlowRunArtifact     // Para artifacts
FlowHandoff         // Para handoffs
FlowValidation      // Para validación
FlowIssue           // Para issues/warnings
```

---

## 🎯 CONCLUSIONES

### Lo que SÍ existe:
- ✅ API endpoint `/flows/run/stream` con soporte SSE completo
- ✅ Implementación canonical `runFlowDebug()` en `remora-flujo/cmd/flujo/flow_workbench.go`
- ✅ Tipos Go necesarios en `remora-cli/main.go`
- ✅ Usage message mencionando `remora flow debug`
- ✅ Test de delegación esperando `debug`

### Lo que FALTA:
- ❌ Función `handleFlowDebug()` en `remora-cli/cmd/remora/flow_workbench.go`
- ❌ Routing en `handleFlowWorkbench()` para capturar subcomando "debug"

### Workaround actual:
El comando `remora flow debug` delega a `flujo flow debug` si:
1. Se ejecuta desde el repo root con `go run ./cmd/remora flow debug`
2. O si `flujo` está en PATH

### % Código a Agregar: ~2.5%
```
Archivos en remora-cli: ~15 archivos .go
handleFlowDebug() agregar: ~300 líneas
Total: ~300 / ~12000 líneas = ~2.5% nuevo código
```

---

**Siguiente paso (pi-cirujano):** Implementar `handleFlowDebug()` en `remora-cli/cmd/remora/flow_workbench.go` basándose en el patrón de `runFlowDebug()` de flujo.

<!-- CHAIN_RUN_ID: e61672e8 -->
