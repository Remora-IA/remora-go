# PI-PRE-NARRADOR: Narrativa Experiencial

**Run ID:** run-1778990960  
**Fecha:** 2026-05-17  
**Inspirado en:** Detective output del mismo run

---

## NARRATIVA DEL SISTEMA

### SUPERFICIE-1: remora CLI (Terminal del Usuario)

#### FLUJO-1.1: El usuario inicia remora sin argumentos

```
VISIÓN:
$ remora
→ Pantalla: Flow Workbench — Usage printed
→ Usuario ve: " remora [flow] [subcommand]", flow-autonomy-summary, canonical flow create/draft/inspect/simulate/run/install/debug

DECISIÓN:
El usuario debe elegir un subcommand. Sin argumentos, ve el menú.

ESCENA: main() → handleFlowWorkbench() → printFlowWorkbenchUsage()
TRACE:
cpg.method.name("handleFlowWorkbench").code.l
→ printFlowWorkbenchUsage()
→ flowAutonomySummary()
→ canonicalFlowWorkbenchCommand()
```

#### FLUJO-1.2: Usuario ejecuta flow create interactivo

```
VISIÓN:
$ remora flow create
→ Pregunta: "¿Qué quieres automatizar?"
→ Prompts para: Goal, Capability, Strategy
→ Usuario responde → Sistema propone preview

DECISIÓN:
El usuario revisa preview y confirma o cambia respuestas.

ESCENA: handleFlowCreate() → promptFlowCreateAnswers() → buildFlowCreateSuggestPayload() → printFlowCreatePreview()
TRACE:
cpg.method.name("handleFlowCreate").caller.code.l
→ promptFlowField("goal")
→ promptFlowField("capability")  
→ promptFlowField("strategy")
→ applyFlowCreateIntentModel()
→ buildFlowCreateLifecycle()
→ printFlowCreatePreview()
```

#### FLUJO-1.3: Usuario запускает flow simulate (dry-run)

```
VISIÓN:
$ remora flow simulate --flow mi-flow
→ Sistema muestra: nodos que se ejecutarían, acciones sin side-effects
→ Sin datos reales, sin costo real

DECISIÓN:
El usuario verifica que el flow está bien diseñado antes de ejecutar.

ESCENA: runFlowSimulate() → simulateFlowManifest() → printSimulationSummary()
TRACE:
cpg.method.name("runFlowSimulate").code.l
→ mustFetchFlowRecord()
→ simulateFlow(manifest)
→ printSimulateResult()
```

---

### SUPERFICIE-2: API REST (Backend HTTP :8084)

#### FLUJO-2.1: Cliente autentica y crea negocio

```
VISIÓN:
POST /auth/register {email, password}
→ Response: {token, user}
POST /businesses {name}
→ Response: {business_id, ...}

DECISIÓN:
El usuario crea cuenta, obtiene token, crea su primer workspace.

ESCENA: handleAuthRegister() → createUser() → createSession()
→ handleBusinessCreate() → createBusiness()
TRACE:
cpg.method.name("handleAuthRegister").code.l
cpg.method.name("handleBusinessCreate").code.l
```

#### FLUJO-2.2: Cliente sube archivo Excel de datos

```
VISIÓN:
POST /business/{id}/data/upload
→ Selecciona archivo .xlsx
→ Sistema parsea: headers, rows, detect тип данных
→ Crea tabla en SQLite local

DECISIÓN:
El usuario verifica que las columnas se detectaron correctamente.

ESCENA: handleBusinessDataUpload() → parseUploadedTables() → parseExcelLegacy() → importSheet()
TRACE:
cpg.method.name("handleBusinessDataUpload").code.l
→ parseDelimited() o parseExcelLegacy()
→ normalizeHeaders()
→ safeIdent()
→ importSheet()
```

#### FLUJO-2.3: Cliente ejecuta flow y observa gaps

```
VISIÓN:
POST /flows/{id}/run
→ Sistema ejecuta nodos secuencialmente
→ En nodo "auditor.contact", detecta: FALTADATA
→ Request: POST /input-requests {field: "contact.email", reason: "missing"}

DECISIÓN:
El usuario provee email faltante → Sistema continua.

ESCENA: runFlow() → executeFlowNode() → resolveMissingFlowArtifacts() → inputRequestsForMissingArtifacts()
TRACE:
cpg.method.name("runFlow").code.l
→ executeFlowNode() en cada nodo
→ Si gap: invokeMecanicoResolveGaps() o inputRequestsForMissingArtifacts()
→ Si approval: generateHumanAcceptance() → esperar
```

---

### SUPERFICIE-3: Channel (JSON-RPC Handler)

#### FLUJO-3.1: Alfa runner ejecuta comando en workspace

```
VISIÓN:
{ "method": "ExecuteCommand", "params": { "command": "go", "args": ["run", "./cmd/frameworkalfa", "compile", ...] } }
→ Channel valida: API key, path, command whitelist
→ Ejecuta: exec.CommandContext(ctx, cmd, args...)
→ Retorna: { "stdout": "...", "stderr": "..." }

DECISIÓN:
Si comando es peligroso (rm -rf), IsCommandAllowed() returns false.

ESCENA: Handle() → ValidateSecurity() → IsCommandAllowed() → executeCommand()
TRACE:
cpg.call.name("ExecuteCommand").code.l
cpg.method.name("ValidateSecurity").code.l
cpg.method.name("isPathSafe").code.l
```

#### FLUJO-3.2: Usuario pide ReadFile en path no autorizado

```
VISIÓN:
{ "method": "ReadFile", "params": { "path": "../../etc/passwd" } }
→ ValidatePath() detecta: path traversal attempt
→ Retorna error: { "code": -32001, "message": "path not safe" }

DECISIÓN:
El sistema protege el filesystem de lecturas fuera del workspace.

ESCENA: ValidatePath() → isPathSafe()
TRACE:
cpg.method.name("ValidatePath").code.l
→ Check: !strings.Contains(path, "..")
→ Check: startsWith(baseDir)
```

---

### SUPERFICIE-4: Orchestrator (LLM Loop)

#### FLUJO-4.1: Orchestrator atiende pregunta de usuario

```
VISIÓN:
User message → runLoop() → classifyIntent() → driverNames()
→ pollQuestion() → generar respuesta
→ ejecutar acciones (tools) → responder

DECISIÓN:
Si es delegable: executeDelegations() → driver.Delegate()
Si es conversacional: generalResponse()

ESCENA: runLoop() → classifySegmentIntent() → executeSessionFollowup()
TRACE:
cpg.method.name("runLoop").code.l
cpg.method.name("classifySegmentIntent").code.l
→ Si intent == "operational": consumePendingQuestionsForFramework()
→ Si delegable: executeDelegations()
→ else: generalResponse()
```

---

## GAPS EN LA NARRATIVA

### GAP-N1: El usuario no ve qué pasa dentro de runFlow

```
El flujo se ejecuta silenciosamente.
No hay streaming de nodos, no hay progreso visual.
El usuario espera y recibe resultado final.
```

### GAP-N2: Channel no registra comandos ejecutados

```
ExecuteCommand() retorna stdout/stderr
Pero no hay log persistente de qué ejecutó cada session.
Post-mortem debugging es difícil.
```

### GAP-N3: Auth token expira sin refresh

```
El token es estático.
No hay /auth/refresh endpoint.
Si el token leak, el atacante tiene acceso hasta que el usuario cambie password.
```

---

## CABLES EN LA NARRATIVA

### CABLE-N1: remora CLI solo funciona si api_rest está corriendo

```
Si backend :8084 no responde:
$ remora flow create
→ Error: "connection refused"
→ El usuario debe hacer: make restart-api
```

### CABLE-N2: Channel timeout es fijo (30s)

```
Si comando tarda más de 30s, Channel mata el proceso.
No hay way de aumentar timeout por comando.
```

### CABLE-N3: Orchestrator usa Groq como provider único

```
Si Groq está down, el sistema no puede generar respuestas.
No hay fallback a OpenAI u otro provider.
```

---

## RESUMEN EXPERiencial

| Lo que el usuario ve | Lo que pasa internamente |
|----------------------|---------------------------|
| `$ remora` → usage | main() → handleFlowWorkbench() → printFlowWorkbenchUsage() |
| `$ remora flow create` → prompts | handleFlowCreate() → buildFlowCreateSuggestPayload() → promptFlowField() |
| `POST /flows/{id}/run` → resultado | runFlow() → executeFlowNode() → gap resolution → persistFlowArtifact() |
| Channel: ExecuteCommand → output | ValidateSecurity() → IsCommandAllowed() → executeCommand() → return stdout |
| Orchestrator: "hola" → respuesta | runLoop() → classifyIntent() → PollQuestion() → tools() → response |

---

## TRACE TECH

### Trace main flow create:
```
handleFlowCreate
├── promptFlowCreateAnswers
│   └── promptFlowField("goal")
│   └── promptFlowField("capability")
│   └── promptFlowField("strategy")
├── buildFlowCreateSuggestPayload
│   └── inferCLIIntentRoles()
│   └── applyFlowCreateIntentHints()
├── buildFlowCreateLifecycle
│   └── cliLifecycleBindingLabel()
│   └── emptyCLILifecycle()
├── printFlowCreatePreview
└── runFlowCreate
    └── doJSON("createFlow", payload)
        └── handleCreateFlow (server side)
```

### Trace flow execution:
```
runFlow
├── loadFlowRun
├── runFlowManifest
│   ├── executeFlowNode (entry)
│   │   └── applyCapabilityParamDefaults()
│   │   └── flowNodeUsesBusinessVault()
│   │   └── resolveMissingFlowArtifacts
│   │       └── invokeMecanicoResolveGaps
│   │           └── requestMecanicoProposalApproval
│   │           └── applyApprovedMecanicoProposals
│   ├── if nodeRequiresRuntimeApproval
│   │   └── generateHumanAcceptance
│   │   └── pause until approved
│   └── executeFlowNode (next node)...
├── recordFlowArtifact
└── recordTaskLedgerCycleCompleted
```

---

**Fin de narrativa experiencial.**

<!-- CHAIN_RUN_ID: run-1778990960 -->
<!-- STEP: 02-pi-pre-narrador -->
