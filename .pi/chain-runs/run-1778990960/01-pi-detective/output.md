# DETECTIVE: remora-go-lite - 2026-05-17

## Componentes analizados

| Componente | CPG | Tamaño | Métodos |
|------------|-----|--------|---------|
| remora-cli  | .pi/remora-cli-cpg.bin  | 600K  | 124 |
| remora-flujo| .pi/remora-flujo-cpg.bin | 5.2M | 1411 |
| channel     | .pi/channel-cpg.bin    | 420K  | 129 |

---

## Componente: remora-cli

### Métodos principales (124)
| Método | Archivo | Descripción |
|--------|---------|-------------|
| main | cmd/remora/main.go | Entry point, routing HTTP |
| handleFlowWorkbench | cmd/remora/flow_workbench.go:235 | CLI flow workbench, detecta "debug" y delega |
| handleDebugCommand | flow_workbench.go:274 | Handler de debug frameworks/manifest/etc |
| delegateToCanonicalFlowWorkbench | flow_workbench.go:880 | Delega a binario flujo |
| handleFlowCreate | flow_workbench.go:1240 | `remora flow create` |
| handleFlowSimulate | flow_workbench.go:1440 | `remora flow simulate` |

### Call graph: main()
main → handleFlowWorkbench (line 235)
  ├── args[1] == "debug" → handleDebugCommand
  │     ├── "frameworks" → handleDebugFrameworks → get(/frameworks)
  │     ├── "manifest" → handleDebugManifest → get(/frameworks/{fw}/manifest)
  │     ├── "commands" → handleDebugCommands → get(/frameworks/{fw}/commands)
  │     ├── "capabilities" → handleDebugCapabilities
  │     ├── "trace" → handleDebugTrace → get(/flows/runs/{id})
  │     ├── "validate" → handleDebugValidate → post(/flows/validate)
  │     ├── "simulate" → handleDebugSimulate → post(/flows/simulate)
  │     └── "dependencies" → handleDebugDependencies → get(/flows/{id})
  └── else → delegateToCanonicalFlowWorkbench → exec.Command("flujo", args...)

### Interfaces HTTP salientes
| URL Base | Métodos | Propósito |
|----------|---------|-----------|
| {REMORA_BACKEND_URL} (default localhost:8084) | GET /frameworks | Listar frameworks |
| | POST /conversations | Crear conversación |
| | GET /flows/runs/{id} | Trace de flow |
| | POST /flows/validate | Validar flow |
| | POST /flows/simulate | Simular flow |
| | GET /businesses/{id}/data-tables | Tablas de datos |

---

## Componente: remora-flujo

### Sub-componentes internos

#### api_rest (cmd/api_rest)
- Puerto: 8084
- Handler principal: main() en main.go
- Rutas registradas (53 rutas):

| Ruta | Handler | Método HTTP |
|------|---------|-------------|
| /health | health | GET |
| /api/v1/auth/* | handleAuth* | POST |
| /api/v1/businesses* | handleBusinesses | GET/POST |
| /api/v1/frameworks | listFrameworks | GET |
| /api/v1/capabilities | listCapabilities | GET |
| /api/v1/flows/validate | validateFlow | POST |
| /api/v1/flows/simulate | simulateFlow | POST |
| /api/v1/flows/run | runFlow | POST |
| /api/v1/flows/run/stream | runFlowStream | POST (SSE) |
| /api/v1/flows/suggest | suggestFlowCapabilities | POST |
| /api/v1/flows/workbench/compile | compileFlowWorkbench | POST |

#### flujo CLI (cmd/flujo)
- Entry: main() en main.go, routing a cmdFlow()
- Commands: flow, start, done, ask, reset, run, chat
- flow workbench: runFlowCreate, runFlowDraft, runFlowInspect, runFlowValidate, runFlowSimulate, runFlowRun, runFlowInstall, runFlowReplay, runFlowDebug

### Call graph: cmdFlow() [flujo CLI]
cmdFlow → runFlowWorkbench
  ├── "create" → runFlowCreate → post(/flows/suggest) → post(/businesses/{id}/flows)
  ├── "draft" → runFlowDraft → post(/flows/suggest)
  ├── "compile" → runFlowCompile → post(/flows/workbench/compile)
  ├── "inspect" → runFlowInspect → get(/flows/compiled/{id})
  ├── "validate" → runFlowValidate → post(/flows/validate)
  ├── "simulate" → runFlowSimulate → post(/flows/simulate)
  ├── "run" → runFlowRun → post(/flows/run)
  ├── "install" → runFlowInstall → post(/flows/{id}/install)
  ├── "replay" → runFlowReplay → get(/flows/runs/{id})
  └── "debug" → runFlowDebug → post(/flows/run) con step/break flags

### Interfaces exec.Command detectadas

| Archivo | Línea | Comando | Destino |
|---------|-------|---------|---------|
| flow_channel.go | 56 | exec.Command(channelBin, ...) | channel binary |
| flow_channel.go | 134 | exec.Command("go", "build", "-o", outBin, "./cmd/channel") | Compila channel |
| contactos.go | 51 | exec.Command(bin, ...) | sabio binary |
| contactos.go | 83 | exec.Command(bin, ...) | sabio binary |
| main.go | 1111 | exec.Command(vaultBin, "set", ...) | vault binary |
| main.go | 1254 | exec.Command(binPath, args...) | driver binario |
| main.go | 1334 | exec.Command(vaultBin, "has", ...) | vault binary |

---

## Componente: channel

### Estructura
- 4 binaries: channel (main), orchestrator, alfa-runner, echo-runner, vault
- Protocolo: JSON-RPC 2.0 sobre stdin/stdout
- Puerto default: 2222 (alfa-runner), 2223 (echo-runner)

### Métodos del adapter (channel/adapter.adapter.go)
| Método | Tipo |
|--------|------|
| New | constructor |
| ExecuteCommand | exec |
| ReadFile | file |
| WriteFile | file |
| Grep | file search |
| Find | file search |
| EditFile | file |
| HTTPGet | network |
| call | dispatcher |

### CLI surfaces de channel
- **orchestrator**: `orchestrator run <framework> <command>` / `orchestrator chain <fw1>:<cmd1> <fw2>:<cmd2>`
- **vault**: `vault has|get|set|list|delete <key>`
- **alfa-runner**: main loop, JSON-RPC interface
- **echo-runner**: main loop, JSON-RPC interface

---

## Interfaces Cross-Componente

### remora-cli → remora-flujo (api_rest) via HTTP
- **Desde:** remora-cli/cmd/remora/main.go:73,101
- **Base URL:** localhost:8084 (REMORA_BACKEND_URL)
- **Endpoints:** /frameworks, /conversations, /flows/*
- **Protocolo:** REST/JSON

### remora-cli → remora-flujo (flujo CLI) via exec.Command
- **Desde:** remora-cli/cmd/remora/flow_workbench.go:235
- **Código:** exec.Command("flujo", args...)
- **Receptor:** cmd/flujo/main.go:31 → cmdFlow()
- **Protocolo:** args passthrough, stdout/stderr heredados

### remora-flujo (api_rest) → channel via exec.Command
- **Desde:** remora-flujo/cmd/api_rest/flow_channel.go:56
- **Binario:** {channelBin} (resolve via findChannelBinary)
- **Args:** ["channel", "run", "<framework>", "<command>", ...]
- **Protocolo:** JSON-RPC 2.0 stdin/stdout

### remora-flujo (api_rest) → sabio via exec.Command
- **Desde:** remora-flujo/cmd/api_rest/contactos.go:51,83
- **Binario:** sabio (resolve desde PATH)
- **Args:** ["sabio", "contacts", "lookup"|"store-profile", ...]
- **Protocolo:** CLI args + stdin/stdout

### remora-flujo (flujo CLI) → remora-flujo (api_rest) via HTTP
- **Desde:** remora-flujo/cmd/flujo/flow_workbench.go
- **Base URL:** http://localhost:8084
- **Endpoints:** /api/v1/flows/*, /businesses/{id}/flows
- **Protocolo:** REST/JSON

---

## Superficies de Usuario

### SUPERFICIE: cli-remora
- **Tipo:** CLI
- **Entry:** remora-cli binary (cmd/remora/main.go)
- **Comandos disponibles:**
  - `remora flow create --business <id> [--name <n>] [--description <texto>]`
  - `remora flow draft --business <id> --name <nombre>`
  - `remora flow inspect --id <flow_id>`
  - `remora flow simulate --id <flow_id> [--fixtures a,b] [--input texto]`
  - `remora flow run --id <flow_id> [--fixtures a,b] [--input texto] [--dry-run]`
  - `remora flow install --id <flow_id>`
  - `remora flow replay --run <run_id>`
  - `remora flow debug --id <flow_id> [--step] [--break-on ...]`
  - `remora debug frameworks [fw]`
  - `remora debug manifest <framework>`
  - `remora debug commands <framework>`
  - `remora debug capabilities [fw]`
  - `remora debug trace <run-id>`
  - `remora debug validate <flow-id>`
  - `remora debug simulate <flow-id>`
  - `remora debug dependencies <flow-id>`
- **Output:** stdout (JSON para --json, texto formateado para debug)
- **Backend:** HTTP API en localhost:8084

### SUPERFICIE: cli-flujo
- **Tipo:** CLI (canonical,取代 remora-cli para flow)
- **Entry:** remora-flujo binary (cmd/flujo/main.go)
- **Comandos disponibles:**
  - `flujo flow create --business <business_id> [--name <nombre>] [--description <texto>]`
  - `flujo flow draft --business <business_id> --name <nombre> --description <texto> [--create]`
  - `flujo flow compile --id <flow_id>`
  - `flujo flow inspect --id <flow_id>`
  - `flujo flow validate --id <flow_id>`
  - `flujo flow simulate --id <flow_id> [--fixtures a,b] [--input texto]`
  - `flujo flow run --id <flow_id> [--fixtures a,b] [--input texto] [--dry-run]`
  - `flujo flow install --id <flow_id> [--reconfigure]`
  - `flujo flow replay --run <run_id>`
  - `flujo flow debug --id <flow_id> [--step] [--break-on ...]`
  - `flujo start` — iniciar sesión interactiva con Alfa
  - `flujo done` — marcar tarea completada
  - `flujo ask <texto>` — preguntar a Echo
  - `flujo reset` — resetear sesión
  - `flujo run <framework> <command> [k=v...]` — ejecutar framework
  - `flujo chat` — chat interactivo
- **Output:** stdout (colores ANSI, JSON para APIs)
- **Backend:** HTTP API en localhost:8084

### SUPERFICIE: frontend-static
- **Tipo:** Web SPA
- **Archivo:** remora-flujo/cmd/api_rest/static/index.html
- **Se sirve en:** localhost:8084 (dev mode con REMORA_DEV_STATIC=1)
- **Pantallas:** Flow builder UI, run execution, simulation
- **Endpoints consumidos:**
  - GET /config
  - POST /flows/run/stream (SSE)
- **Interacciones:** Canvas de flow, wizard de creación, ejecución paso a paso

### SUPERFICIE: frontend-chat
- **Tipo:** Web SPA
- **Archivo:** remora-flujo/frontends/frontend-chat/index.html
- **Se sirve en:** localhost:8084/chat (via router)
- **Pantallas:** Chat con Alfa/Echo, framework selector, command runner
- **Endpoints consumidos:**
  - GET /models
  - GET /runtime
  - GET /frameworks
  - POST /conversations
  - POST /conversations/{id}/messages
  - GET /frameworks/{name}
  - POST /frameworks/{name}/commands/{cmd}/run
  - POST /send-email
- **Interacciones:** Chat input, framework dropdown, command buttons, send email form

---

## GAPs detectados

| Símbolo | Componente | Query | Status |
|---------|-----------|-------|--------|
| handleFlowDebug (local) | remora-cli | cpg.method.name("handleFlowDebug") | ❌ NO_EXISTE (delegate a flujo) |
| channel HTTP client | channel | cpg.call.name("http.Get") | ❌ NO_EXISTE (solo exec + JSON-RPC) |

---

## Notas de inferencia

1. **remora-cli delega a flujo**: La función `delegateToCanonicalFlowWorkbench` en flow_workbench.go:880 construye `exec.Command("flujo", args...)`. El argumento `flowWorkbenchExecCommand` en línea 235 indica que esta es la ruta canonical para operaciones de flow.

2. **channel usa JSON-RPC stdin/stdout**: El adapter en channel/adapter.adapter.go implementa ExecuteCommand que envía JSON-RPC requests a stdin del proceso hijo y lee responses de stdout. No usa HTTP para la comunicación interna.

3. **api_rest descubre channel binario**: flow_channel.go usa `findChannelBinary()` que busca en PATH o en locations conocidos. El binario puede ser channel compilado o built-in.

<!-- CHAIN_RUN_ID: 1778990960 -->
