# PRE-NARRATIVA: remora-go-lite - "implementar remora flow debug CLI"

## Fuente: Grafo Joern + Análisis Manual

| Campo | Valor |
|-------|-------|
| **Grafo JSON** | `.pi/remora-cli-graph.json` |
| **CPG** | No disponible (análisis manual por workspace issue) |
| **Detective Output** | `.pi/chain-runs/20260516202204/01-pi-detective/output.md` |
| **Run ID** | `20260516202204` |
| **Proyecto** | remora-go-lite |

---

## Grafos de Llamadas

### GRAFO PRINCIPAL: Entry → Router → Handlers

```
main() [main.go:1]
    │
    └─→ handleFlowWorkbench() [flow_workbench.go:237]
            │
            ├─→ args[0] == "debug"
            │       │
            │       └─→ handleDebugCommand(args[1:]) [274]
            │               │
            │               ├─→ handleDebugFrameworks(args) [310]
            │               ├─→ handleDebugManifest(args) [373]
            │               ├─→ handleDebugCommands(args) [401]
            │               ├─→ handleDebugCapabilities(args) [444]
            │               ├─→ handleDebugTrace(args) [505]
            │               ├─→ handleDebugValidate(args) [539]
            │               ├─→ handleDebugSimulate(args) [572]
            │               └─→ handleDebugDependencies(args) [625]
            │
            └─→ args[0] != "debug"
                    │
                    └─→ delegateToCanonicalFlowWorkbench(args) [880]
                            │
                            ├─→ canonicalFlowWorkbenchCommand(args) [895]
                            │       │
                            │       ├─→ "go run ./cmd/flujo flow ..." (si repoRoot existe)
                            │       └─→ "flujo flow ..." (si flujo está en PATH)
                            │
                            └─→ exec.Command("flujo", ...) → remora-flujo [SINK]
```

### GRAFO SECUNDARIO: Flow Workbench Local

```
handleFlowWorkbench(args) [237]
    │
    ├─→ args[0] == "debug" → handleDebugCommand() [274]
    │       └─→ [8 handlers de debug]
    │
    ├─→ args[0] == "create" → handleFlowCreate() [1114]
    │       ├─→ newClient()
    │       ├─→ Client.post("/businesses/{id}/flows")
    │       └─→ fetchBusinessArtifacts()
    │
    ├─→ args[0] == "draft" → handleFlowDraft() [1235]
    │       ├─→ newClient()
    │       └─→ Client.post("/flows/suggest")
    │
    ├─→ args[0] == "inspect" → handleFlowInspect() [1312]
    │       └─→ mustFetchFlowRecord()
    │
    └─→ args[0] == "simulate" → handleFlowSimulate() [1330]
            ├─→ mustFetchFlowRecord()
            ├─→ newClient()
            └─→ Client.post("/flows/simulate")
```

### GRAFO DELEGACIÓN: remora-cli → remora-flujo

```
remora flow [create|draft|inspect|simulate|...] 
    ↓
handleFlowWorkbench(args) [237]
    │
    └─→ delegateToCanonicalFlowWorkbench(args) [880]
            │
            └─→ exec.Command("flujo", "flow", args...)
                    │
                    └─→ remora-flujo (proceso externo)
                            │
                            └─→ flujo/cmd/flujo/main.go
```

---

## Flujo Paso a Paso: "remora flow debug --id X"

### ESCENARIO: Usuario ejecuta `remora flow debug --id mi-flow`

```
INPUT: os.Args = ["remora", "flow", "debug", "--id", "mi-flow"]

[1] main() [main.go]
    ├─ os.Args = ["remora", "flow", "debug", "--id", "mi-flow"]
    └─ Llama → handleFlowWorkbench()

[2] handleFlowWorkbench() [flow_workbench.go:237]
    ├─ args = ["flow", "debug", "--id", "mi-flow"]
    │
    ├─❌ IF: len(args) > 0 && args[0] == "debug"
    │       └─ args[0] = "flow" (NO "debug")
    │       └─ NO entra al if
    │
    └─→ delegateToCanonicalFlowWorkbench(args)

[3] delegateToCanonicalFlowWorkbench() [880]
    ├─ args = ["flow", "debug", "--id", "mi-flow"]
    └─→ canonicalFlowWorkbenchCommand(args)

[4] canonicalFlowWorkbenchCommand() [895]
    ├─ Busca repoRoot con findRepoRoot()
    │   └─ SI existe: cmd = exec.Command("go", "run", "./cmd/flujo", "flow", "flow", "debug", "--id", "mi-flow")
    │   └─ NO existe: cmd = exec.Command("flujo", "flow", "debug", "--id", "mi-flow")
    └─→ exec.Command("flujo", ...) [SINK]

[5] exec.Command("flujo", ...) [895]
    └─ Delegado a remora-flujo (proceso externo)
            │
            └─→ flujo/cmd/flujo/main.go
                    │
                    └─→ runFlowDebug() en remora-flujo ✅ EXISTE PERO NO LOCAL
```

### ESCENARIO: Usuario ejecuta `remora debug trace <run-id>`

```
INPUT: os.Args = ["remora", "debug", "trace", "abc123"]

[1] main()
    └─→ handleFlowWorkbench()

[2] handleFlowWorkbench() [237]
    ├─ args = ["debug", "trace", "abc123"]
    ├─ args[0] == "debug" ✅
    └─→ handleDebugCommand(args[1:])

[3] handleDebugCommand() [274]
    ├─ args = ["trace", "abc123"]
    ├─ cmd = "trace"
    └─ switch: "trace" → handleDebugTrace(args[1:])

[4] handleDebugTrace() [505]
    ├─ Parsea flags (--json)
    ├─ Llama → c.get("/flows/runs/abc123")
    └─ Imprime timeline
```

---

## Cables Cruzados (Dependencies Ocultas)

### CABLE #1: Delegación Automática a flujo canónico
```
┌─────────────────────────────────────────────────────────────────────────┐
│ handleFlowWorkbench() [237]                                           │
│                                                                          │
│  if args[0] == "debug" {                                               │
│      handleDebugCommand()  ←─ "remora debug ..."                        │
│  } else {                                                               │
│      delegateToCanonicalFlowWorkbench()  ←─ "remora flow ..."           │
│  }                                                                      │
└─────────────────────────────────────────────────────────────────────────┘
                                                                          │
                    ⚠️ PROBLEMA: args[0] = "flow", NO "debug"            │
                    Cuando usuario ejecuta "remora flow debug":          │
                    - args[0] = "flow"                                   │
                    - args[1] = "debug"                                  │
                    - El if NO captura este caso                          │
                    - Se delega a "flujo flow debug"                      │
                    - remora-cli nunca ejecuta handleFlowDebug()         │
                    - El comando "debug" se interpreta como SUBCOMANDO   │
                      de "flow", no como COMANDO PRINCIPAL               │
```

**Impacto:** `remora flow debug` se convierte en `flujo flow debug`, perdiendo la capacidad de ejecutar debug localmente en remora-cli.

---

### CABLE #2: Client sin Streaming (remora-cli vs remora-flujo)

```
┌─────────────────────────────────────────────────────────────────────────┐
│ remora-cli/Client [main.go:55]                                           │
│                                                                          │
│  type Client struct {                                                   │
│      BaseURL string                                                      │
│      Token string                                                        │
│  }                                                                      │
│                                                                          │
│  func (c *Client) get(path) {...}     ✅ EXISTE                          │
│  func (c *Client) post(path, body) {...} ✅ EXISTE                       │
│  func (c *Client) stream(path, body) {...} ❌ NO EXISTE                  │
└─────────────────────────────────────────────────────────────────────────┘
                                                                          │
┌─────────────────────────────────────────────────────────────────────────┐
│ remora-flujo/flowWorkbenchClient [cmd/api_rest/client.go]                │
│                                                                          │
│  type flowWorkbenchClient struct {                                       │
│      BaseURL string                                                      │
│      Token string                                                        │
│  }                                                                      │
│                                                                          │
│  func (c *flowWorkbenchClient) get(path) {...}     ✅ EXISTE             │
│  func (c *flowWorkbenchClient) post(path, body) {...} ✅ EXISTE           │
│  func (c *flowWorkbenchClient) stream(path, body) {...} ✅ EXISTE (SSE)  │
└─────────────────────────────────────────────────────────────────────────┘
```

**Impacto:** remora-cli no puede hacer streaming SSE para debug en tiempo real. El streaming existe solo en remora-flujo para `/flows/run/stream`.

---

### CABLE #3: Tipos Compartidos

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Tipos en remora-cli/main.go                                              │
│                                                                          │
│  FlowRunResult { RunID, Status, Timeline, Artifacts }   ✅              │
│  FlowRunStep { Node, Framework, Status, ... }         ✅              │
│  FlowHandoff { FromNode, ToNode, Artifacts }           ✅              │
└─────────────────────────────────────────────────────────────────────────┘
                                                                          │
                    ┌─────────────────────────────────────┐
                    │ Usado por:                          │
                    │ - handleDebugTrace()                │
                    │ - handleFlowSimulate()              │
                    │ - handleFlowInspect()               │
                    └─────────────────────────────────────┘
```

---

### CABLE #4: Routing Dual de Debug

```
┌─────────────────────────────────────────────────────────────────────────┐
│ "remora debug ..."                                                      │
│                                                                          │
│  handleFlowWorkbench()                                                  │
│      └─ args[0] == "debug" → handleDebugCommand()                       │
│              ├─ "frameworks"    → handleDebugFrameworks()              │
│              ├─ "manifest"      → handleDebugManifest()                │
│              ├─ "commands"      → handleDebugCommands()                │
│              ├─ "capabilities"  → handleDebugCapabilities()            │
│              ├─ "trace"        → handleDebugTrace()                    │
│              ├─ "validate"     → handleDebugValidate()                │
│              ├─ "simulate"     → handleDebugSimulate()                 │
│              └─ "dependencies"  → handleDebugDependencies()             │
└─────────────────────────────────────────────────────────────────────────┘
                                                                          │
┌─────────────────────────────────────────────────────────────────────────┐
│ "remora flow ..."                                                       │
│                                                                          │
│  handleFlowWorkbench()                                                  │
│      └─ args[0] != "debug" → delegateToCanonicalFlowWorkbench()         │
│              └─→ "flujo flow ..."                                       │
│                                                                          │
│  ⚠️ NO HAY case para "flow debug"                                      │
│  El caso args[0]="flow" && args[1]="debug" NO EXISTE                   │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Puntos de Intervención

### PUNTO #1: Routing en handleFlowWorkbench() [237]

**Ubicación:** `remora-cli/cmd/remora/flow_workbench.go:237`

**DEL GRAFO:**
```go
func handleFlowWorkbench() {
    args := []string{}
    if len(os.Args) > 2 {
        args = os.Args[2:]
    }

    // Handle 'remora debug ...' subcommands
    if len(args) > 0 && args[0] == "debug" {
        if len(args) == 1 {
            printDebugUsage()
            return
        }
        handleDebugCommand(args[1:])
        return
    }

    if err := delegateToCanonicalFlowWorkbench(args); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

**FALTA:**
```go
// Handle 'remora flow debug ...' - NUEVO CASO
if len(args) > 1 && args[0] == "flow" && args[1] == "debug" {
    handleFlowDebug(args[2:])
    return
}
```

**INTERVENCIÓN REQUERIDA:** Agregar un nuevo IF después del existente para capturar `remora flow debug`.

---

### PUNTO #2: handleFlowDebug() - GAP CRÍTICO

**Ubicación:** `remora-cli/cmd/remora/flow_workbench.go` (NO EXISTE)

**DEL GRAFO:**
- Status: `NO_EXISTE`
- Handler relacionado que EXISTE: `handleDebugTrace()` [505]
- Handler relacionado que EXISTE: `handleFlowSimulate()` [1330]

**INTERVENCIÓN REQUERIDA:** Crear `handleFlowDebug(args []string)` que:
1. Parsee flags: `--id`, `--fixtures`, `--input`, `--step`, `--break-on`
2. Llame a API `/flows/run` o `/flows/run/stream`
3. Implemente streaming local o polling
4. Muestre timeline con breakpoints

---

### PUNTO #3: printFlowDebugUsage() - GAP MEDIUM

**Ubicación:** `remora-cli/cmd/remora/flow_workbench.go` (NO EXISTE)

**Similar a:**
- `printDebugUsage()` [260]
- `printFlowWorkbenchUsage()` [908]

**Uso esperado (del usage en línea 920):**
```
remora flow debug --id <flow_id> [--fixtures a,b] [--input texto] [--step] [--break-on handoff,needs_input,approval]
```

---

### PUNTO #4: Client.stream() - GAP HIGH

**Ubicación:** `remora-cli/cmd/remora/main.go` (NO EXISTE en Client)

**DEL GRAFO:**
- Client.get() ✅ EXISTE [71]
- Client.post() ✅ EXISTE [93]
- Client.stream() ❌ NO EXISTE

**EXISTE en remora-flujo:**
- `flowWorkbenchClient.stream()` para `/flows/run/stream` (SSE)

**INTERVENCIÓN REQUERIDA:** Implementar `Client.stream()` para soportar streaming SSE de debug.

---

## Comportamientos No Documentados

### COMPORTAMIENTO #1: Delegación Automática sin Validación

**Observado en:** `delegateToCanonicalFlowWorkbench()` [880]

```
Cuando args[0] != "debug", remora-cli delega TODO a "flujo" CLI.
No hay validación de que "flujo" exista o esté disponible.
Si "flujo" no está en PATH → error del OS, no de remora-cli.
```

**Código relevante:**
```go
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

---

### COMPORTAMIENTO #2: printFlowWorkbenchUsage() Mentiona "flow debug"

**Observado en:** Línea 920 - El usage menciona `remora flow debug` pero NO EXISTE el handler.

```go
func printFlowWorkbenchUsage() {
    fmt.Print(`flow workbench

Uso:
  remora flow create --business <id> [--name <n>] [--description <texto>]
  remora flow draft --business <id> --name <n> --description <texto> [--create]
  ...
  remora flow debug --id <flow_id> [--fixtures a,b] [--input texto] [--step] [--break-on handoff,needs_input,approval]
`)
}
```

**⚠️ Inconsistencia:** El usage promete `remora flow debug` pero:
1. `handleFlowDebug()` NO EXISTE
2. El routing en `handleFlowWorkbench()` NO captura este caso
3. Se delega a `flujo flow debug` automáticamente

---

### COMPORTAMIENTO #3: Streaming Existe en API, No en CLI

**Observado en:** remora-flujo/cmd/api_rest/streaming.go

```
remora-flujo tiene:
- POST /flows/run → ejecución simple
- POST /flows/run/stream → SSE streaming ✅

remora-cli NO tiene:
- Client.stream() ❌
- Llamadas a /flows/run/stream ❌
```

**Implicación:** Para implementar `remora flow debug` con streaming real:
1. Necesita `Client.stream()` en remora-cli
2. O necesita delegar a `flujo flow debug` (que ya tiene streaming)

---

## Flujo de Datos: Input → Output

### INPUT: `remora flow debug --id mi-flow --break-on handoff`

```
os.Args = ["remora", "flow", "debug", "--id", "mi-flow", "--break-on", "handoff"]
    │
    ├─→ handleFlowWorkbench()
    │       │
    │       ├─ args = ["flow", "debug", "--id", "mi-flow", "--break-on", "handoff"]
    │       ├─ args[0] = "flow" (NO "debug")
    │       └─ delegateToCanonicalFlowWorkbench(args)
    │           │
    │           └─→ exec.Command("flujo", "flow", "debug", "--id", "mi-flow", "--break-on", "handoff")
    │                   │
    │                   └─→ remora-flujo (proceso externo)
    │                           │
    │                           ├─ loadFlowWorkbenchRecord("mi-flow")
    │                           ├─ client.stream("/flows/run/stream", body, onEvent)
    │                           │       ├─ HTTP POST con SSE Accept
    │                           │       └─ events: step_start, step_complete, flow_complete, breakpoint
    │                           │
    │                           └─→ print timeline con breakpoints pausados
    │
    └─→ remora-flujo (output)
            │
            └─→ SSE stream de eventos
```

### OUTPUT ESPERADO (cuando handleFlowDebug exista localmente):

```
═══════════════════════════════════════════════════════════════
  FLOW DEBUG: mi-flow
═══════════════════════════════════════════════════════════════

[1/5] ⏸️  BREAKPOINT: handoff (esperando confirmación)
       De: architecto/draft_proposal
       A:   bravo/review_proposal
       
       Artifacts: propuesta.pdf (2.3KB)
       
       > continuar? (s/n) 
```

---

## Resumen de GAPs Confirmados

| Símbolo | Status | Confirmado Por |
|---------|--------|----------------|
| `handleFlowDebug()` en remora-cli | ❌ NO_EXISTE | Grep en archivo, no encontrado |
| `flowDebugCmd` FlagSet | ❌ NO_EXISTE | Grep, no encontrado |
| `printFlowDebugUsage()` | ❌ NO_EXISTE | Grep, no encontrado |
| `Client.stream()` | ❌ NO_EXISTE | Solo get() y post() en Client |
| Routing para "flow debug" | ❌ NO EXISTE | args[0]="flow" entra a delegación |
| `flujo flow debug` en remora-flujo | ✅ EXISTE | Delegado a proceso externo |

---

## Archivos Involucrados

| Archivo | Rol | Acción |
|---------|-----|--------|
| `remora-cli/cmd/remora/flow_workbench.go` | Handlers CLI | MODIFICAR: routing + AGREGAR handleFlowDebug |
| `remora-cli/cmd/remora/main.go` | Client + Tipos | AGREGAR: Client.stream() |
| `remora-flujo/cmd/api_rest/streaming.go` | Streaming SSE | REFERENCIA: streaming existente |

---

<!-- CHAIN_RUN_ID: 20260516202204 -->
