# PI-DETECTIVE: Análisis de Módulos Core

**Fecha:** 2026-05-16  
**Proyecto:** remora-go-lite

---

## Resumen Ejecutivo

| Módulo | Métodos | LOC aprox. | Entry Points | Sinks | Complejidad |
|--------|---------|------------|--------------|-------|-------------|
| **remora-cli** | 124 | ~3,500 | 4 | 267 | Media |
| **remora-flujo** | 1,068 | ~25,000 | 12 | 577 | Alta |
| **channel** | 129 | ~4,000 | 6 | 34 | Media |

---

## MÓDULO 1: remora-cli

### Descripción
CLI principal que actúa como interface de usuario para flujos de trabajo. Delega la mayor parte del trabajo pesado a `remora-flujo` y usa comandos `flow workbench` para ejecutar flujos.

### Estructura de Archivos
```
remora-cli/
├── cmd/remora/
│   ├── main.go              # Entry point CLI
│   ├── backend_bootstrap.go # Bootstrapping del backend
│   ├── flow_workbench.go    # Dispatcher de comandos de flujo
│   ├── client.go           # Cliente HTTP al backend
│   └── *_test.go           # Tests
```

### Entry Points
| Handler | Archivo:Línea | Responsabilidad |
|---------|---------------|-----------------|
| `main` | main.go:1 | Entry principal del CLI |
| `handleFlowWorkbench` | flow_workbench.go:237 | Entry point para flujo workbench |
| `handleDebugCommand` | flow_workbench.go:274 | Dispatcher de debug |
| `delegateToCanonicalFlowWorkbench` | flow_workbench.go:~900 | Delega a CLI externo |

### Comandos Disponibles
- `debug frameworks` → Lista frameworks
- `debug manifest` → Muestra manifest
- `debug commands` → Lista comandos
- `debug capabilities` → Lista capabilities
- `debug trace` → Muestra timeline
- `debug validate` → Valida flujo
- `debug simulate` → Dry-run con timeline
- `debug dependencies` → Muestra dependencias

### Call Graph: main → handleFlowWorkbench

```
main()
└── args[0] == "debug"
      └── handleFlowWorkbench
            └── args[0] == "debug"
                  └── handleDebugCommand
                        ├── "frameworks" → handleDebugFrameworks
                        │     └── newClient → c.Get("/frameworks")
                        ├── "manifest" → handleDebugManifest
                        ├── "trace" → handleDebugTrace
                        ├── "validate" → handleDebugValidate
                        ├── "simulate" → handleDebugSimulate
                        └── "dependencies" → handleDebugDependencies
```

### Sinks Principales (Top 10)

| Tipo | Método | Riesgo |
|------|--------|--------|
| exec | `flowWorkbenchExecCommand()` | Command Injection |
| http | `client.RunFlow()` | SSRF |
| http | `client.GetFlowRun()` | SSRF |
| env | `canonicalFlowWorkbenchCommand()` | Secret Exposure |

### GAPs Detectados

| Símbolo | Status | Evidencia |
|---------|--------|-----------|
| `handleFlowRun` | ❌ NO_EXISTE | No existe handler directo |
| `handleFlowDebug` | ❌ NO_EXISTE | Delega a flujo externo |
| `remora flow debug` | ❌ NO_EXISTE | Delega a `flujo` CLI |
| `remora flow run` | ❌ NO_EXISTE | No hay handler |

---

## MÓDULO 2: remora-flujo

### Descripción
Backend principal del sistema. API REST que orquesta la ejecución de flujos, maneja autenticación, bases de datos, orquestación de frameworks y store de flujos. Es el motor de ejecución más complejo.

### Estructura de Archivos
```
remora-flujo/
├── cmd/api_rest/
│   ├── main.go              # Entry point API
│   ├── auth*.go             # Sistema de autenticación
│   ├── flow_*.go            # Lógica de flujos
│   ├── builder_*.go        # Construcción de flows
│   ├── orchestrator.go      # Orquestación principal
│   └── ...
```

### Entry Points (12 principales)
| Handler | Archivo | Responsabilidad |
|---------|---------|-----------------|
| `main` | main.go | Entry API REST |
| `handleAuth*` | auth_handlers.go | Login, register, logout |
| `handleFlow*` | flow_store.go | CRUD de flujos |
| `runFlow` | flow_run.go | Ejecución de flujos |
| `runLoop` | orchestrator.go | Loop principal de chat |
| `handleTracesLatest` | traces.go | Trazas de debugging |
| `handleTasks*` | tareas.go | Sistema de tareas |

### Flujo de Ejecución Principal

```
runFlow
├── runFlowManifest
│     ├── executeFlowNode
│     │     ├── resolvePortableCommandArgs
│     │     └── ExecuteCommand (channel)
│     ├── executeSessionFollowupDetailed
│     │     └── executeDelegations
│     └── resolveFlowGapsIteratively
│           └── invokeMecanicoResolveGaps
├── simulateFlowManifest
└── validateFlowManifest
```

### Call Graph: main → runLoop

```
main()
├── health/healthz
├── listFrameworks → collectFrameworkInfos
├── listConversations → createConversation
├── postMessage → runLoop
│     ├── consumePendingQuestionsForFramework
│     ├── executeSessionFollowup
│     │     ├── executeDelegations
│     │     └── persistFollowupArtifact
│     └── persistSessionSummary
└── runFlow → executeFlowNode
```

### Sinks Principales (Top 15)

| Tipo | Archivo | Riesgo |
|------|---------|--------|
| exec | `ExecuteCommand()` | Command Injection |
| sql | `db.QueryRow()` | SQL Injection potencial |
| http | `ch.ExecuteCommand()` | SSRF |
| file | `writeFile()` | Path Traversal |
| env | `exec.Command(vaultBin)` | Credential Exposure |

### Drivers/Frameworks Invocados

| Driver | Ubicación | Capability |
|--------|-----------|------------|
| `alfa` | drivers.go | Análisis |
| `echo` | generic_driver.go | Logging/chat |
| `sabio` | contactos.go | Contactos |
| `mecanico` | flow_gap_resolution.go | Resolución de gaps |
| `foco` | tareas.go | Sistema de tareas |

### Data Flows Críticos

```
User Input → postMessage → runLoop
  → executeSessionFollowup
    → ExecuteCommand(channel)
      → Framework Binario
        → Artifacts

Flow Execution → runFlow
  → executeFlowNode
    → resolvePortableCommandArgs
      → ExecuteCommand(channel)
        → Sabio (contactos)
        → Mecanico (resolución)
```

---

## MÓDULO 3: channel

### Descripción
Middleware de ejecución que actúa como proxy seguro entre remora-flujo y los frameworks. Expone un handler JSON-RPC para ejecutar comandos de forma controlada con whitelist de operaciones.

### Estructura de Archivos
```
channel/
├── cmd/channel/main.go      # Entry point principal
├── cmd/alfa-runner/main.go  # Runner específico para alfa
├── cmd/echo-runner/main.go  # Runner para echo
├── cmd/orchestrator/main.go # Orquestación
├── cmd/vault/main.go        # Gestión de credenciales
├── internal/
│   ├── handler.go           # Handler JSON-RPC principal
│   ├── security.go          # Validación de seguridad
│   ├── whitelist.go         # Whitelist de comandos
│   ├── exec.go              # Ejecución de comandos
│   └── session.go           # Logging de sesiones
├── adapter.go               # Adapter HTTP
├── profile.go               # Gestión de perfiles
├── vault.go                 # Vault integration
├── credentials/smtp.go      # Credenciales SMTP
└── manifest/manifest.go     # Manifest de comandos
```

### Entry Points
| Handler | Archivo | Responsabilidad |
|---------|---------|-----------------|
| `main` | main.go | Entry del canal |
| `Handle` | handler.go:67 | Handler JSON-RPC |
| `ExecuteCommand` | exec.go | Ejecución de comandos |
| `ValidateSecurity` | security.go | Validación de seguridad |

### Comandos Permitidos (Whitelist)

| Comando | Descripción |
|---------|-------------|
| `ReadFile` | Lee archivos del base-dir |
| `WriteFile` | Escribe archivos |
| `Grep` | Búsqueda en archivos |
| `Find` | Finding de archivos |
| `EditFile` | Edición de archivos |
| `HTTPGet` | GET requests controlados |
| `mkdir` | Crear directorios |
| `go run` | Ejecutar binarios Go |

### Call Graph: Handle → ExecuteCommand

```
main()
├── net/http.Serve
│     └── Handle
│           ├── ValidateJSONRPC
│           ├── ValidateSecurity
│           │     ├── ValidatePath (path traversal check)
│           │     └── IsCommandAllowed (whitelist)
│           └── executeCommand
│                 ├── ExecuteCommandWithEnv
│                 │     └── exec.CommandContext
│                 └── readFile / writeFile
```

### Sinks Principales

| Tipo | Método | Riesgo |
|------|--------|--------|
| exec | `ExecuteCommandWithEnv()` | Command Injection |
| file | `readFile()` | Path Traversal |
| file | `writeFile()` | Path Traversal |
| http | `httpGet()` | SSRF |

### Validaciones de Seguridad

```go
// security.go
ValidateSecurity(req) 
  ├── ValidateJSONRPC()      // JSON-RPC 2.0 format
  ├── IsCommandAllowed(cmd)  // Whitelist check
  ├── ValidatePath(base, target)  // Path traversal
  └── isPathSafe(path, baseDir)    // CWD escape prevention
```

---

## Análisis de Vulnerabilidades

### Alta Prioridad

| Módulo | Issue | CVSS |
|--------|-------|------|
| remora-cli | `delegateToCanonicalFlowWorkbench` - exec.Command injection | 7.5 |
| remora-flujo | `executeFlowNode` - command args from untrusted input | 8.1 |
| channel | `ValidatePath` - path traversal potential | 7.8 |
| remora-flujo | `resolvePortableCommandArgs` - template injection | 6.9 |

### Media Prioridad

| Módulo | Issue | CVSS |
|--------|-------|------|
| remora-flujo | SQL queries con string concatenation | 5.3 |
| channel | HTTP GET sin validación de URL | 4.8 |
| remora-cli | Env vars en comandos | 4.2 |

---

## Métricas de Acoplamiento

| Relación | Tipo | Intensidad |
|----------|------|------------|
| remora-cli → remora-flujo | HTTP Client | Alta (delega a API) |
| remora-flujo → channel | exec.Command | Alta (invoca binario) |
| channel → frameworks | exec.Command | Alta (ejecuta binarios) |

### Ciclo de Dependencias
```
User → remora-cli → remora-flujo (API) → channel (exec) → frameworks
                                                    ↓
                                              sabio, alfa, echo
```

---

## Recomendaciones

### Inmediatas
1. **Sanitizar comandos** en `delegateToCanonicalFlowWorkbench` y `resolvePortableCommandArgs`
2. **Validar paths** en channel antes de cualquier operación de archivo
3. **Usar prepared statements** para todas las queries SQL en remora-flujo

### Corto Plazo
1. **Centralizar logging** de seguridad en channel
2. **Agregar rate limiting** en el handler JSON-RPC
3. **Implementar audit trail** para exec.Command

### Largo Plazo
1. **Migrar a gRPC** para comunicación remora-flujo → channel
2. **Separar concerns** de channel en servicios dedicados
3. **Implementar policy engine** para autorización de comandos

---

## Archivos Generados por Joern

| Módulo | CPG | Graph JSON |
|--------|-----|------------|
| remora-cli | `remora-cli-cpg.bin` | `remora-cli-graph.json` |
| remora-flujo | `remora-flujo-cpg.bin` | `remora-flujo-graph.json` |
| channel | `channel-cpg.bin` | `channel-graph.json` |

---

*Reporte generado con pi-detective (Joern + análisis estático)*