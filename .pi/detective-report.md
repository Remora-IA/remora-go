# DETECTIVE: remora-go-lite - Análisis Completo con Joern

**Fecha:** 2026-05-16  
**CPG:** `workspace/api_rest_final.cpg.bin/cpg.bin` (API) + `workspace/remora-cli-cpg/cpg.bin` (CLI)

---

## RESUMEN EJECUTIVO

| Componente | Métodos | Tipos | Sinks |
|------------|---------|-------|-------|
| **api_rest** | ~400+ | ~80 | 473 |
| **remora-cli** | 49 | 56 | sin exec.Command* |

*exec.Command USADO pero no detectado por Joern (variable injectada)

---

## COMPONENTES DEL PROYECTO

### 1. API REST (`cmd/api_rest/`)

#### Estructura de Handlers

```
main()
├── loadDotEnv()
├── openAuthStore() → auth.db (SQLite)
├── openFlowStore(flowDBPath) → flows.db (SQLite)
├── loadFlowRules(./flow.rules.json)
├── ensureChannel(channelURL, apiKey)
├── initDriverRegistry(rootDir) → descubre framework-*/
└── mux.Router con ~80+ rutas

Rutas principales:
├── /health, /healthz
├── /api/v1/auth/* (register, login, logout, me)
├── /api/v1/businesses/* (CRUD, members, invites)
├── /api/v1/admin/* (users, team, remora-invites)
├── /api/v1/frameworks/* (list, testable, chainable, commands/run)
├── /api/v1/capabilities/*
├── /api/v1/flows/* (validate, simulate, run, compile, templates)
├── /api/v1/conversations/* (CRUD, messages, queue)
├── /api/v1/data/tables/* (schema browser)
├── /api/v1/rules
├── /api/v1/send-email
├── /api/v1/tasks/*
├── /api/v1/simulations/autonomia-controlada/*
└── /* (static files: index.html, data.html)
```

#### Métodos Clave (API REST)

| Método | Archivo | Descripción |
|--------|---------|-------------|
| `initDriverRegistry` | server.go | Descubre y carga frameworks desde `framework-*` |
| `executeFlowNode` | flow_executor.go | Ejecuta un nodo de flow via framework |
| `handleFlowRun` | flow_handlers.go | POST /flows/run -Corre un flow completo |
| `handleCreateFlow` | flow_handlers.go | POST /businesses/{id}/flows |
| `handleGetFlow` | flow_handlers.go | GET /flows/{id} |
| `handleUpdateFlow` | flow_handlers.go | PUT /flows/{id} |
| `handleDeleteFlow` | flow_handlers.go | DELETE /flows/{id} |
| `validateFlow` | flow_handlers.go | POST /flows/validate |
| `simulateFlow` | flow_handlers.go | POST /flows/simulate |
| `suggestFlowCapabilities` | flow_handlers.go | POST /flows/suggest |
| `runFlowStream` | flow_handlers.go | POST /flows/run/stream |
| `handleBusinessArtifacts` | business_artifacts.go | GET /businesses/{id}/artifacts |
| `handleBusinessData*` | business_artifacts.go | Upload/tables/rows para datos |
| `openAuthStore` | auth.go | Abre auth.db SQLite |
| `openFlowStore` | flows.go | Abre flows.db SQLite |
| `handleSendEmail` | email_handler.go | POST /send-email |
| `handleTasksList/Create/Next/Event` | task_handlers.go | CRUD de tareas |

#### Tipos Estructurales (API REST)

**Auth/Business:**
- `authStore`, `authUser`, `authMembership`, `authBusiness`, `authSession`
- `authBusinessMember`, `authBusinessInvite`, `remoraUserOverview`, `remoraTeamInvite`
- `authRegisterRequest`, `authLoginRequest`, `createBusinessRequest`

**Flows:**
- `flowManifest`, `flowIntent`, `flowLifecycle`, `flowLifecycleEntry`
- `flowNode`, `flowEdge`, `flowValidationIssue`, `flowValidationResult`
- `flowSimulationRequest`, `flowSimulationResult`, `flowSimulationStep`
- `flowCompiledManifest`, `flowCompiledRecord`, `flowProvenance`
- `flowDerivation`, `flowDataGrounding`, `flowAmendment`, `flowExecutablePlan`

**Frameworks:**
- `FrameworkDriver`, `capabilityRegistry`, `capabilityProviderInfo`

**Datos:**
- `dataTableInfo`, `dataRowsResponse`, `contactsLookupResult`, `contactsStoreResult`
- `uploadSheet`, `odsTable`, `odsRow`, `odsCell`

---

### 2. CLI (`cmd/remora/`)

#### Call Graph Principal

```
main()
├── os.Args[1:] → args
├── args[0] == "debug" → handleDebugCommand(args[1:])
│   ├── "frameworks"  → handleDebugFrameworks()
│   ├── "manifest"    → handleDebugManifest()
│   ├── "commands"    → handleDebugCommands()
│   ├── "capabilities" → handleDebugCapabilities()
│   ├── "trace"       → handleDebugTrace()
│   ├── "validate"    → handleDebugValidate()
│   ├── "simulate"    → handleDebugSimulate()
│   └── "dependencies" → handleDebugDependencies()
│
└── else → delegateToCanonicalFlowWorkbench(args)
    ├── printFlowWorkbenchUsage() [si args vacío]
    ├── canonicalFlowWorkbenchCommand(args)
    │   ├── findRepoRoot()
    │   ├── exec.LookPath("flujo") → ¿existe?
    │   │   ├── SÍ → flowWorkbenchExecCommand("flujo", "flow", args...)
    │   │   └── NO → flowWorkbenchExecCommand("go", "run", "./cmd/flujo", "flow", args...)
    │   └── cmd.Dir = filepath.Join(repoRoot, "remora-flujo")
    └── cmd.Run() [stdin/stdout/stderr pipeados]
```

#### Métodos (CLI)

| Método | Archivo | Descripción |
|--------|---------|-------------|
| `handleFlowWorkbench` | flow_workbench.go:237 | Entry point: routing debug vs flow |
| `handleDebugCommand` | flow_workbench.go:274 | Routing subcommands debug |
| `handleDebugFrameworks` | flow_workbench.go:310 | `remora debug frameworks` |
| `handleDebugManifest` | flow_workbench.go:373 | `remora debug manifest` |
| `handleDebugCommands` | flow_workbench.go:401 | `remora debug commands` |
| `handleDebugCapabilities` | flow_workbench.go:444 | `remora debug capabilities` |
| `handleDebugTrace` | flow_workbench.go:505 | `remora debug trace` |
| `handleDebugValidate` | flow_workbench.go:539 | `remora debug validate` |
| `handleDebugSimulate` | flow_workbench.go:572 | `remora debug simulate` |
| `handleDebugDependencies` | flow_workbench.go:625 | `remora debug dependencies` |
| `delegateToCanonicalFlowWorkbench` | flow_workbench.go:897 | Delega a flujo CLI |
| `canonicalFlowWorkbenchCommand` | flow_workbench.go:850 | Construye comando exec |
| `newClient` | main.go | Crea HTTP client |
| `Client.get` | main.go:71 | GET al backend :8084 |
| `Client.post` | main.go:93 | POST al backend :8084 |

#### Tipos (CLI)

- `Client` - HTTP client para API
- `FlowManifest`, `FlowNode`, `FlowEdge`, `FlowRunResult`, `FlowRunStep`
- `FlowHandoff`, `FlowValidation`, `FlowIssue`, `FlowInput`
- `FrameworkInfo`, `CapabilityInfo`, `cliFlow*` (manifest, lifecycle, derivation, etc.)

---

## GAPS DETECTADOS

### API REST

| Símbolo | Tipo | Status | Notas |
|---------|------|--------|-------|
| `handleFlowDebug` | method | ❌ NO_EXISTE | No hay endpoint para "debug" de flows |
| `streaming` | feature | ⚠️ PARCIAL | `/flows/run/stream` existe pero retorna error "streaming not supported" |
| `Client.stream` | method | ❌ NO_EXISTE | Sin WebSocket en CLI |
| `websocket` | feature | ❌ NO_EXISTE | No hay upgrade a WebSocket |

### CLI

| Símbolo | Tipo | Status | Notas |
|---------|------|--------|-------|
| `handleFlowDebug` | method | ❌ NO_EXISTE | `remora flow debug` delega a flujo CLI |
| `handleFlowRun` | method | ❌ NO_EXISTE | Delegado a flujo CLI |
| `handleFlowCompile` | method | ❌ NO_EXISTE | Delegado a flujo CLI |
| `handleFlowReplay` | method | ❌ NO_EXISTE | Delegado a flujo CLI |
| `printFlowDebugUsage` | method | ❌ NO_EXISTE | Solo existe `printDebugUsage` para debug local |

---

## SINKS ENCONTRADOS

### API REST (473 sinks encontrados)

#### DB Sinks (SQLite)

| Sink | Frecuencia | Descripción |
|------|-------------|-------------|
| `s.db.QueryRow(...)` | ~20+ | Auth queries |
| `fs.db.QueryRow(...)` | ~10+ | Flow queries |
| `openAuthStore()` | singleton | Auth SQLite |
| `openFlowStore()` | singleton | Flow SQLite |
| `s.openDataDB(businessID)` | por request | Data SQLite por negocio |
| `s.openWritableDataDB(businessID)` | por request | Data write |

**SQL Queries detectados:**
```sql
-- Auth
SELECT id, email, password_hash, name, role FROM users WHERE lower(email)=lower(?)
SELECT id, token, expires_at FROM sessions WHERE token_hash=?
SELECT business_id, role, scope_json FROM business_memberships WHERE user_id=? AND business_id=?

-- Flows
SELECT id, name, manifest_json, status FROM flow_templates WHERE business_id=?
SELECT id, business_id, name, manifest_json FROM flows WHERE id=?
SELECT installed, analysis_plan_path FROM flow_installations WHERE flow_id=?

-- Data
SELECT name FROM sqlite_master WHERE type='table'
SELECT COUNT(*) FROM {table}
```

#### HTTP Sinks

| Sink | Descripción |
|------|-------------|
| `channel.Send(...)` | Channel adapter para enviar mensajes |
| `adapter.New(channelURL, apiKey)` | Crear canal |

#### File System Sinks

| Sink | Descripción |
|------|-------------|
| `os.ReadFile(...)` | Leer archivos estáticos |
| `os.WriteFile(...)` | Guardar rules |
| `os.MkdirAll(...)` | Crear directorios |
| `filepath.Join(...)` | Manipulación de paths |

#### Framework Execution Sinks

| Sink | Descripción |
|------|-------------|
| `s.executeFlowNode(...)` | Ejecuta comando de framework |
| `m.Commands[cmd].Driver.Run(...)` | Driver execution |

### CLI (remora-cli)

#### exec.Command Sinks

| Sink | Archivo | Descripción |
|------|---------|-------------|
| `flowWorkbenchExecCommand(...)` | flow_workbench.go:235 | **Variable injectada** para testing |
| `exec.Command("go", "run", "./cmd/flujo", "flow", ...)` | flow_workbench.go:895 | Delega a flujo CLI |
| `exec.LookPath("flujo")` | flow_workbench.go | Busca flujo en PATH |
| `exec.Command("go", "run", ...)` | flow_workbench.go:895 | Fallback si no encuentra flujo |

**Comando construido:**
```bash
# Opción 1: si existe 'flujo' en PATH
flujo flow {args...}

# Opción 2: fallback
go run ./cmd/flujo flow {args...}
# Dir: {repoRoot}/remora-flujo
```

#### HTTP Sinks

| Sink | Descripción |
|------|-------------|
| `Client.get(path)` | GET http://localhost:8084{path} |
| `Client.post(path, body)` | POST http://localhost:8084{path} |

---

## FLUJO DE DATOS

### API REST - Inicialización

```
main()
├── loadDotEnv()
├── openAuthStore() → authStore (SQLite)
├── openFlowStore() → flowStore (SQLite)
├── resolveRemoraRoot() → rootDir
├── loadFlowRules() → rules
├── ensureChannel() → channel adapter
├── initDriverRegistry() → loadedManifests
│   └── Descubre framework-*/framework.manifest.json
└── mux.NewRouter() + register all handlers
```

### API REST - Flow Execution

```
POST /flows/run
├── parseFlowRunRequest
├── load flow manifest from DB
├── validateFlowManifest
├── executeFlowNode() para cada nodo
│   ├── lookup framework manifest
│   ├── resolve node params (templates)
│   ├── call framework driver
│   │   └── m.Commands[cmd].Driver.Run(ctx, params)
│   └── collect artifacts
└── return FlowRunResult
```

### CLI - Flow Workbench

```
remora [flow subcommand]
├── handleFlowWorkbench(args)
│   ├── args[0] == "debug" → handleDebugCommand()
│   └── else → delegateToCanonicalFlowWorkbench()
│       ├── canonicalFlowWorkbenchCommand()
│       │   ├── findRepoRoot()
│       │   ├── LookPath("flujo") → ¿existe?
│       │   └── build exec.Command
│       └── exec.Command.Run()
```

---

## FRAMEWORKS DESCUBIERTOS

Del CPG, los frameworks cargados por `initDriverRegistry`:

1. `framework-alfa`
2. `framework-auditor`
3. `framework-charlie`
4. `framework-deployer`
5. `framework-echo`
6. `framework-foco`
7. `framework-gmail`
8. `framework-hosting`
9. `framework-indexa`
10. `framework-mecanico`
11. `framework-mensajero`
12. `framework-quine`
13. `framework-sabio`
14. `framework-tareas`

---

## OBSERVACIONES

1. **Delegation Pattern:** remora-cli NO implementa lógica de flows, solo delega a `flujo CLI`
2. **No exec.Command en API:** La API no usa `exec.Command` - solo drivers de frameworks
3. **SQLite por negocio:** Cada negocio tiene su propia DB en data/
4. **Static files:** API servea frontend desde `cmd/api_rest/static/` o embedded
5. **Channel adapter:** La API se comunica con un canal externo (localhost:8765) para eventos

---

*Generado por pi-detective usando Joern CPG analysis*