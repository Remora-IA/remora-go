# DETECTIVE: remora-go-lite - Análisis Completo

**Run ID:** `20260516202204`  
**Timestamp:** 2026-05-17T02:22:04Z  
**Code Hash:** `c19d061c8ecf5ff5`

---

## Resumen Ejecutivo

Proyecto **remora-go-lite** es un sistema multi-componente de orquestación de flujos con capacidades de IA conversacional. El sistema está diseñado como una trenza de 4 componentes principales que se comunican entre sí.

| Componente | Binaries | Archivos .go | CPG Size |
|------------|----------|--------------|----------|
| remora-cli | remora, devcli | 7 | 588K |
| remora-flujo | api_rest, flujo, agentrpc, framework_session, llmtest | 97 | 4.1M |
| channel | channel, orchestrator, vault, alfa-runner, echo-runner | 19 | 420K |
| framework-alfa | (incluido en channel) | - | 372K |

---

## Componentes Analizados

### 1. remora-cli (CLI principal)

**Propósito:** Interfaz de línea de comandos para desarrolladores que inspeccionan, prueban y debuggean flujos.

#### Entry Points
| Binary | Entry Point | Archivo |
|--------|-------------|---------|
| remora | main() | cmd/remora/main.go |
| devcli | main() | cmd/devcli/main.go |

#### Estructura de Comandos (remora)

```
remora
└── dev
    ├── inspect <framework>     → muestra manifest completo del framework
    ├── providers               → lista providers disponibles
    ├── flow
    │   ├── list                → lista flujos compilados
    │   ├── inspect <id>        → muestra definición de flujo
    │   ├── create <biz> <name> → crea nuevo flujo (WIP)
    │   ├── simulate <id>       → dry-run con timeline
    │   ├── run <id> [--live]   → ejecuta flujo
    │   └── debug <id>          → debug paso a paso
    ├── trace <run-id>          → muestra timeline de ejecución
    ├── artifacts               → lista artifacts
    └── rules                   → muestra reglas de composición
```

#### Métodos Clave (remora-cli)

| Método | Archivo | Propósito |
|--------|---------|-----------|
| handleFlowWorkbench | main.flow_workbench.go | Punto de entrada para flujos |
| delegateToCanonicalFlowWorkbench | main.flow_workbench.go | Delega a flujo CLI canónico |
| flowWorkbenchExecCommand | main.flow_workbench.go | Construye comando exec |
| handleDebugCommand | main.flow_workbench.go | Maneja subcomandos debug |
| handleDebugFrameworks | main.flow_workbench.go | Lista frameworks |
| handleDebugManifest | main.flow_workbench.go | Muestra manifest |
| handleDebugValidate | main.flow_workbench.go | Valida flujos |
| handleDebugSimulate | main.flow_workbench.go | Simula flujos |
| ensureBackendReadyWithDeps | main.backend_bootstrap.go | Bootstrap backend |
| newClient | main.main.go | Cliente HTTP |

#### Call Graph: handleFlowWorkbench

```
handleFlowWorkbench(args)
├── args[0] == "debug" → handleDebugCommand
│   ├── "frameworks" → handleDebugFrameworks
│   │   └── GET /api/v1/frameworks
│   ├── "manifest" → handleDebugManifest  
│   │   └── GET /api/v1/frameworks/{fw}/manifest
│   ├── "validate" → handleDebugValidate
│   │   └── POST /api/v1/flows/validate
│   ├── "simulate" → handleDebugSimulate
│   │   └── POST /api/v1/flows/simulate
│   └── ...
├── args[0] == "flow" → delegateToCanonicalFlowWorkbench
│   └── exec.Command("flujo", "flow", args[1:]...)
│       (Delega a remora-flujo/cmd/flujo)
└── else → delegateToCanonicalFlowWorkbench
    └── exec.Command("flujo", ...)
```

#### Sinks remora-cli

| Tipo | Sink | Archivo |
|------|------|---------|
| exec | exec.Command("flujo", "flow", ...) | main.flow_workbench.go |
| network | HTTP GET/POST a REMORA_BACKEND_URL | main.main.go |

#### Variables de Entorno

| Variable | Uso |
|----------|-----|
| REMORA_BACKEND_URL | URL base del API (default: localhost:8084) |
| REMORA_ROOT | Raíz del repositorio |

#### **GAP: handleFlowDebug (local)**
El método `handleFlowDebug` NO existe en remora-cli. El debug se maneja a través de la CLI canónica `flujo flow debug`.

---

### 2. remora-flujo (Backend/API)

**Propósito:** Motor de ejecución de flujos, API REST, orquestación de sesión.

#### Entry Points
| Binary | Entry Point | Puerto | Archivo |
|--------|-------------|--------|---------|
| api_rest | main() | 8084 | main.main.go |
| flujo | main() | CLI | main.flujo.go |
| agentrpc | main() | RPC | main.agentrpc.go |

#### API REST Endpoints (api_rest)

| Ruta | Handler | Método |
|------|---------|--------|
| GET /healthz | health | Health check |
| GET /api/v1/frameworks | listFrameworks | Lista frameworks |
| GET /api/v1/frameworks/:name | getFramework | Detalle framework |
| GET /api/v1/flows | handleListFlows | Lista flujos |
| POST /api/v1/flows | handleCreateFlow | Crea flujo |
| GET /api/v1/flows/:id | handleGetFlow | Get flujo |
| PUT /api/v1/flows/:id | handleUpdateFlow | Update flujo |
| DELETE /api/v1/flows/:id | handleDeleteFlow | Delete flujo |
| POST /api/v1/flows/run | runFlow | Ejecuta flujo |
| POST /api/v1/flows/validate | validateFlow | Valida flujo |
| POST /api/v1/flows/simulate | simulateFlow | Simula flujo |
| GET /api/v1/flows/:id/compiled | handleGetCompiledFlow | Get compilado |
| GET /api/v1/flows/:id/run/:runID | handleGetFlowRun | Get run |
| GET /api/v1/traces/:runID | handleTracesLatest | Timeline run |

#### Handlers Clave

| Handler | Archivo | Propósito |
|---------|---------|-----------|
| handleListFlows | flow_store.go | Lista flujos por business |
| handleCreateFlow | flow_store.go | Crea flujo |
| runFlow | flow_run.go | Ejecuta flujo completo |
| validateFlow | flow_backend.go | Valida manifest |
| simulateFlow | flow_backend.go | Simula dry-run |
| runFlowManifest | flow_runner.go | Runner principal |
| executeFlowNode | flow_execution.go | Ejecuta nodo individual |

#### Call Graph: runFlow()

```
runFlow(req)
├── loadFlowRun(req.CompiledID)
├── compileFlowManifest(manifest)
├── runFlowManifest(compiled, req)
│   ├── init driver registry
│   ├── prepare lifecycle
│   ├── foreach node in order:
│   │   └── executeFlowNode(node, contract)
│   │       ├── findProviderForCapability()
│   │       ├── check preflight audit
│   │       ├── resolve artifacts
│   │       └── execute action
│   └── finalize cycles
└── return FlowRunResult
```

#### Sinks remora-flujo (exec)

| Sink | Archivo | Propósito |
|------|---------|-----------|
| exec.Command("channel", ...) | flow_channel.go | Inicia canal |
| exec.Command("go", "build", ...) | flow_channel.go | Build channel |
| exec.Command("frameworkecho", ...) | framework_session.go | Sesión echo |
| exec.Command("frameworkalfa", ...) | drivers.go | Driver alfa |
| exec.Command("mecanico", "resolve-gaps", ...) | flow_gap_resolution.go | Resuelve gaps |
| exec.Command("foco", "next-task", ...) | flow_provider_interaction.go | Tasks |
| ch.ExecuteCommand() | orchestrator.go | Comandos runtime |
| s.scoped().ExecuteCommand() | session_engine.go | Delegaciones |
| exec.Command("sabio", ...) | orchestrator.go | Data entity |

#### Sinks remora-flujo (file)

| Sink | Archivo | Propósito |
|------|---------|-----------|
| os.ReadFile | flow_artifacts.go | Lee artifacts |
| os.WriteFile | flow_artifacts.go | Escribe artifacts |
| os.ReadDir | store.go | Lee directorio |

#### Sinks remora-flujo (network)

| Sink | Archivo | Propósito |
|------|---------|-----------|
| http.Client.Get | main.main.go | GET requests |
| http.Client.Post | main.main.go | POST requests |
| adapter.HTTPGet | adapter.adapter.go | Adaptador HTTP |

---

### 3. channel (Orquestador/Adapter)

**Propósito:** Adapter de ejecución de comandos que actúa como proxy seguro para framework drivers.

#### Entry Points
| Binary | Entry Point | Propósito |
|--------|-------------|-----------|
| channel | main() | Servidor JSON-RPC |
| orchestrator | main() | Orquestador principal |
| vault | main() | Gestión de credenciales |
| alfa-runner | main() | Runner de alfa |
| echo-runner | main() | Runner de echo |

#### Comandos channel (JSON-RPC)

| Método | Handler | Descripción |
|--------|---------|-------------|
| ExecuteCommand | ExecuteCommand | Ejecuta comando |
| ReadFile | readFile | Lee archivo |
| WriteFile | writeFile | Escribe archivo |
| ListDir | listDir | Lista directorio |
| Grep | grep | Busca en archivos |
| Find | find | Encuentra archivos |
| EditFile | editFile | Edita archivo |
| HTTPGet | httpGet | GET HTTP |

#### Security (channel)

- **Whitelist:** IsCommandAllowed, IsDestructiveCommand
- **Path Validation:** ValidatePath, isPathSafe
- **API Keys:** ValidateSecurity

---

## Interfaces Detectadas (Trenza de Componentes)

### 1. remora-cli → remora-flujo (HTTP)

| Desde | Endpoint | Receptor |
|-------|----------|----------|
| remora cli.Client | GET /api/v1/frameworks | api_rest.listFrameworks |
| remora cli.Client | GET /api/v1/frameworks/:name/manifest | api_rest.getFramework |
| remora cli.Client | POST /api/v1/flows/validate | api_rest.validateFlow |
| remora cli.Client | POST /api/v1/flows/simulate | api_rest.simulateFlow |
| remora cli.Client | GET /api/v1/flows | api_rest.handleListFlows |
| remora cli.Client | GET /api/v1/flows/:id | api_rest.handleGetFlow |
| remora cli.Client | POST /api/v1/flows/run | api_rest.runFlow |

**Protocolo:** REST over HTTP, JSON payloads  
**Base URL:** localhost:8084 (configurable via REMORA_BACKEND_URL)

### 2. remora-cli → remora-flujo (exec.Command - Fallback)

| Desde | Código | Receptor |
|-------|--------|----------|
| handleFlowWorkbench | exec.Command("flujo", "flow", args...) | cmd/flujo.main() |
| delegateToCanonicalFlowWorkbench | exec.Command("flujo", ...) | cmd/flujo.main() |

**Protocolo:** Args passthrough, stdin/stdout/stderr heredados

### 3. remora-flujo → channel (exec.Command)

| Desde | Sink | Binario |
|-------|------|---------|
| flow_channel.go | exec.Command("channel", ...) | channel |
| orchestrator.go | ch.ExecuteCommand(...) | commands via channel |
| session_engine.go | s.scoped().ExecuteCommand() | delegations |

**Protocolo:** JSON-RPC sobre stdin/stdout

### 4. remora-flujo → framework drivers (exec.Command)

| Driver | Comandos |
|--------|----------|
| frameworkecho | init, next-question, reset |
| frameworkalfa | compile, next-question |
| frameworkmecanico | resolve-gaps |
| foco | next-task |
| sabio | contact-lookup, contact-store |

**Protocolo:** Args con parámetros, salida JSON

### 5. remora-flujo → vault (exec.Command)

| Sink | Propósito |
|------|-----------|
| exec.Command("vault", "set", ...) | Guardar credenciales |
| exec.Command("vault", "has", ...) | Verificar clave |

---

## Flujo de Ejecución Completo

```
┌─────────────────────────────────────────────────────────────────┐
│ USUARIO                                                         │
│   $ remora dev flow debug mi-flujo --step                       │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ remora-cli                                                      │
│   handleFlowWorkbench(args)                                     │
│   ├── "debug" → handleDebugCommand                              │
│   └── "flow" → delegateToCanonicalFlowWorkbench                 │
│       └── exec.Command("flujo", "flow", "debug", ...)          │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼ (exec.Command)
┌─────────────────────────────────────────────────────────────────┐
│ remora-flujo                                                    │
│   cmd/flujo/main.go: main()                                     │
│   flowWorkbenchCommand → compileFlowWorkbench                   │
│   → exec.HTTP → POST /api/v1/flows/run (dry-run)                │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼ (HTTP POST)
┌─────────────────────────────────────────────────────────────────┐
│ remora-flujo (api_rest)                                         │
│   runFlow() → runFlowManifest() → executeFlowNode()              │
│   ├── findProviderForCapability()                               │
│   └── ch.ExecuteCommand(driver, args)                            │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼ (ExecuteCommand)
┌─────────────────────────────────────────────────────────────────┐
│ channel                                                         │
│   ExecuteCommand(cmd, args)                                      │
│   ├── ValidateSecurity()                                        │
│   ├── IsCommandAllowed(cmd)                                     │
│   └── exec.CommandContext(ctx, cmd, args...)                    │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼ (exec.CommandContext)
┌─────────────────────────────────────────────────────────────────┐
│ framework-echo/alfa/mecanico                                   │
│   Driver.ExecuteCommand(args)                                    │
└─────────────────────────────────────────────────────────────────┘
```

---

## GAPs Detectados

| Símbolo/Búsqueda | Componente | Status | Notas |
|------------------|------------|--------|-------|
| handleFlowDebug | remora-cli | ❌ NO_EXISTE | Debug local no existe |
| handleFlowRun (local) | remora-cli | DELEGADO | Existe en remora-flujo |
| cmd/flujo/main() entry point | remora-flujo | ⚠️ PARCIAL | Depende de compileFlowWorkbench |
| flowDebugCmd | remora-cli | ❌ NO_EXISTE | No hay comando debug local |
| handleFlowDebug | remora-flujo | ✅ EXISTE | En api_rest flow handlers |

---

## Inventario de Tests

| Archivo | Tests |
|---------|-------|
| flow_workbench_test.go | 4 tests |
| flow_backend_test.go | 28 tests |
| flow_run_test.go | 35+ tests |
| auth_test.go | 4 tests |
| session_engine_test.go | 10+ tests |

---

## Artefactos Generados

| Archivo | Tamaño | Descripción |
|---------|--------|-------------|
| .pi/remora-cli-cpg.bin | 588K | CPG remora-cli |
| .pi/remora-flujo-cpg.bin | 4.1M | CPG remora-flujo |
| .pi/channel-cpg.bin | 420K | CPG channel |
| .pi/framework-alfa-cpg.bin | 372K | CPG framework-alfa |

---

## Notas de Joern

- **Call graph vacía:** Las queries de call graph con `methodName="main"` retornaron 0 resultados debido a que Joern no resolvió correctamente los entry points de Go para este CPG.
- **Queries con `.dump`:** Retornan null para algunos métodos — esto es esperado cuando Joern no tiene acceso al código fuente o el método está en un paquete externo.
- **Listado de métodos:** Extraído exitosamente usando `cpg.method.name.l`.

---

**CHAIN_RUN_ID:** 20260516202204
