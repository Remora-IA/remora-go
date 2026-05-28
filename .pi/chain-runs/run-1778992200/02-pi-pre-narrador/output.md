# NARRATIVA: remora-go-lite

---

## SUPERFICIE: cli-remora — Terminal con remora-cli (binario remora)

### FLUJO: flow-usage — Ver ayuda del workbench de flujos

#### ESC-1: El usuario invoca remora flow sin argumentos

El usuario abre terminal y escribe `remora flow`. El sistema detecta que no hay subcomando y muestra la ayuda del workbench.

En terminal aparece:

```
flow workbench

Uso:
  remora flow create --business <id> [--name <n>] [--description <texto>]
  remora flow draft --business <id> --name <n> --description <texto> [--create]
  remora flow compile --id <flow_id>
  remora flow inspect --id <flow_id>
  remora flow validate --id <flow_id>
  remora flow simulate --id <flow_id> [--fixtures a,b] [--input texto]
  remora flow run --id <flow_id> [--fixtures a,b] [--input texto] [--dry-run]
  remora flow install --id <flow_id> [--reconfigure]
  remora flow replay --run <run_id>
  remora flow debug --id <flow_id> [--fixtures a,b] [--input texto] [--step] [--break-on handoff,needs_input,approval]
```

TRACE:
  → handleFlowWorkbench() [remora/flow_workbench.go:237]
  → delegateToCanonicalFlowWorkbench([]) [remora/flow_workbench.go:880]
  → printFlowWorkbenchUsage() [remora/flow_workbench.go:907]
  → texto de ayuda impreso en stdout

---

### FLUJO: flow-create — Crear un flujo nuevo con wizard interactivo

#### ESC-1: El usuario inicia el wizard de creacion

El usuario escribe `remora flow create --business prospectos-demo`. El sistema detecta el subcomando `create` y delega al binario flujo via exec.Command. El flujo canonico se resuelve buscando el repo root o el binario `flujo` en PATH.

En terminal aparece el prompt interactivo pidiendo datos del flujo. El sistema pregunta nombre, descripcion, criterio de exito, modo de autonomia.

TRACE:
  → handleFlowWorkbench() [remora/flow_workbench.go:237]
  → delegateToCanonicalFlowWorkbench(["create", "--business", "prospectos-demo"]) [remora/flow_workbench.go:880]
  → canonicalFlowWorkbenchCommand() [remora/flow_workbench.go:895]
  → exec.Command("go", ["run", "./cmd/flujo", "flow", "create", ...]) [remora/flow_workbench.go:897]
  → flujo/flow_workbench.go:704 (runFlowCreate)

#### ESC-2: El wizard recopila datos y sugiere capabilities

El usuario responde las preguntas del wizard. El sistema infiere roles a partir del texto ingresado (analizar, redactar, validar, actuar) usando inferFlowCreateRoles. Construye el payload de sugerencia con intent, constraints y lifecycle.

El sistema llama a POST /api/v1/flows/suggest con el payload. El backend responde con capabilities sugeridas y nodos propuestos para el flujo.

En terminal aparece:

```
Flow create seguimiento-leads
  business: prospectos-demo
  autonomia: Preparar con aprobacion
```

TRACE:
  → runFlowCreate() [flujo/flow_workbench.go:704]
  → promptFlowCreateAnswers() [flujo/flow_workbench.go:647]
  → buildFlowCreateSuggestPayload() [flujo/flow_workbench.go:565]
  → post("/flows/suggest", payload) [flujo/flow_workbench.go:397]
  → suggestFlowCapabilities (api_rest) [api_rest/flow_store.go]
  → respuesta con manifest propuesto

#### ESC-3: El usuario confirma y el flujo se persiste

El sistema muestra el preview del flujo propuesto con printFlowCreatePreview. El usuario confirma. El sistema envia POST /api/v1/businesses/{id}/flows para persistir el flujo.

En terminal aparece la confirmacion con el flow_id asignado.

TRACE:
  → printFlowCreatePreview() [flujo/flow_workbench.go:685]
  → post("/businesses/{id}/flows", manifest) [flujo/flow_workbench.go:397]
  → handleCreateFlow() [api_rest/flow_store.go:763]
  → flujo persistido en store

---

### FLUJO: flow-draft — Crear flujo desde spec declarativa

#### ESC-1: El usuario proporciona nombre y descripcion directamente

El usuario escribe `remora flow draft --business prospectos-demo --name "cobranza-diaria" --description "Analizar cartera y enviar recordatorios" --create`. El sistema delega al flujo canonico.

El binario flujo ejecuta runFlowDraft, que construye el payload de sugerencia usando buildFlowDraftSuggestPayload e inferCLIIntentRoles. Envia POST /api/v1/flows/suggest y, si --create esta presente, persiste automaticamente sin wizard interactivo.

TRACE:
  → handleFlowWorkbench() [remora/flow_workbench.go:237]
  → delegateToCanonicalFlowWorkbench(["draft", ...]) [remora/flow_workbench.go:880]
  → runFlowDraft() [flujo/flow_workbench.go:806]
  → buildFlowDraftSuggestPayload() [flujo/flow_workbench.go:870]
  → post("/flows/suggest", payload) [flujo/flow_workbench.go:397]
  → post("/businesses/{id}/flows", manifest) si --create
  → flujo persistido

---

### FLUJO: flow-inspect — Inspeccionar un flujo existente

#### ESC-1: El usuario consulta la definicion de un flujo

El usuario escribe `remora flow inspect --id flow-abc123`. El sistema delega al flujo canonico que ejecuta runFlowInspect.

El sistema hace GET /api/v1/flows/{id} y muestra la estructura completa del flujo: nodos, capabilities, inputs, outputs, roles.

TRACE:
  → delegateToCanonicalFlowWorkbench(["inspect", "--id", "flow-abc123"]) [remora/flow_workbench.go:880]
  → runFlowInspect() [flujo/flow_workbench.go:936]
  → get("/flows/flow-abc123") [flujo/flow_workbench.go:393]
  → handleGetFlow() [api_rest/flow_store.go:796]
  → respuesta JSON con manifest del flujo

---

### FLUJO: flow-simulate — Simular ejecucion de un flujo

#### ESC-1: El usuario ejecuta dry-run de un flujo

El usuario escribe `remora flow simulate --id flow-abc123 --input "test"`. El sistema delega al flujo canonico que ejecuta runFlowSimulate.

El sistema hace POST /api/v1/flows/simulate con el manifest del flujo. El backend responde con una timeline simulada mostrando que pasaria en cada nodo.

TRACE:
  → delegateToCanonicalFlowWorkbench(["simulate", ...]) [remora/flow_workbench.go:880]
  → runFlowSimulate() [flujo/flow_workbench.go:983]
  → get("/flows/{id}") → post("/flows/simulate", {...}) [flujo/flow_workbench.go:397]
  → simulateFlow() [api_rest]
  → timeline simulada

---

### FLUJO: flow-run — Ejecutar un flujo con streaming SSE

#### ESC-1: El usuario ejecuta el flujo en vivo

El usuario escribe `remora flow run --id flow-abc123 --input "cobrar morosos"`. El sistema delega al flujo canonico que ejecuta runFlowRun.

El sistema abre conexion SSE a POST /api/v1/flows/run/stream. Los eventos van llegando en tiempo real: cada paso del flujo emite un evento con framework, capability, status, outputs.

En terminal aparece un stream de eventos conforme se ejecutan los nodos del flujo.

TRACE:
  → delegateToCanonicalFlowWorkbench(["run", ...]) [remora/flow_workbench.go:880]
  → runFlowRun() [flujo/flow_workbench.go:1015]
  → stream("/flows/run/stream", payload) [flujo/flow_workbench.go:401]
  → runFlowStream() [api_rest]
  → SSE events por cada nodo ejecutado

---

### FLUJO: flow-compile — Compilar flujo para ejecucion

#### ESC-1: El usuario compila un flujo

El usuario escribe `remora flow compile --id flow-abc123`. El sistema delega al flujo canonico que ejecuta runFlowCompile.

El sistema hace POST /api/v1/flows/workbench/compile con el flow_id. El backend compila el flujo resolviendo dependencias entre nodos y genera un compiled_id.

TRACE:
  → delegateToCanonicalFlowWorkbench(["compile", ...]) [remora/flow_workbench.go:880]
  → runFlowCompile() [flujo/flow_workbench.go:915]
  → post("/flows/workbench/compile", {...}) [flujo/flow_workbench.go:397]
  → compileFlowWorkbench() [api_rest]
  → compiled_id generado

---

### FLUJO: flow-validate — Validar coherencia de un flujo

#### ESC-1: El usuario valida las reglas de un flujo

El usuario escribe `remora flow validate --id flow-abc123`. El sistema delega al flujo canonico que ejecuta runFlowValidate.

El sistema hace POST /api/v1/flows/validate con el manifest. El backend verifica reglas: tipos de datos matching entre nodos, capabilities existentes, constraints satisfechos.

TRACE:
  → delegateToCanonicalFlowWorkbench(["validate", ...]) [remora/flow_workbench.go:880]
  → runFlowValidate() [flujo/flow_workbench.go:957]
  → post("/flows/validate", {...}) [flujo/flow_workbench.go:397]
  → validateFlow() [api_rest]
  → resultado con valid/violations/warnings

---

### FLUJO: flow-install — Instalar flujo para ejecucion recurrente

#### ESC-1: El usuario instala un flujo compilado

El usuario escribe `remora flow install --id flow-abc123`. El sistema delega al flujo canonico que ejecuta runFlowInstall.

El sistema hace POST /api/v1/flows/{id}/install. El backend marca el flujo como instalado y listo para ejecucion autonoma o programada.

TRACE:
  → delegateToCanonicalFlowWorkbench(["install", ...]) [remora/flow_workbench.go:880]
  → runFlowInstall() [flujo/flow_workbench.go:1058]
  → post("/flows/{id}/install", {...}) [flujo/flow_workbench.go:397]
  → handleInstallFlow() [api_rest/flow_store.go:811]
  → flujo marcado como instalado

---

### FLUJO: debug-usage — Ver ayuda de subcomandos debug

#### ESC-1: El usuario invoca remora debug sin argumentos

El usuario escribe `remora flow debug` o `remora debug`. El sistema muestra la ayuda de debug.

En terminal aparece:

```
debug

Uso:
  remora debug frameworks                    lista todos los frameworks disponibles
  remora debug manifest <framework>          muestra manifest completo del framework
  remora debug commands <framework>         lista comandos del framework
  remora debug capabilities <framework>      lista capabilities del framework
  remora debug trace <run-id>                muestra timeline de ejecucion
  remora debug validate <flow-id>           valida flujo
  remora debug simulate <flow-id>           dry-run con timeline
  remora debug dependencies <flow-id>        muestra grafo de dependencias
```

TRACE:
  → handleFlowWorkbench() [remora/flow_workbench.go:237]
  → handleDebugCommand([]) [remora/flow_workbench.go:274]
  → printDebugUsage() [remora/flow_workbench.go:259]
  → texto de ayuda impreso

---

### FLUJO: debug-frameworks — Listar frameworks disponibles

#### ESC-1: El usuario lista todos los frameworks

El usuario escribe `remora debug frameworks`. El sistema hace GET /frameworks al backend.

En terminal aparece:

```
Frameworks disponibles (21)

  echo (v1.0.0) [sync] fresh
  alfa (v1.0.0) [sync] fresh
  bravo (v0.1.0) [async_trigger]
  sabio (v1.0.0) [sync_chain]
  ...
```

Cada framework se muestra con su color ANSI asignado (echo en verde, alfa en azul, bravo en magenta, etc.), version, modo de ejecucion y freshness.

TRACE:
  → handleDebugFrameworks([]) [remora/flow_workbench.go:310]
  → newClient() [remora/main.go:60]
  → get("/frameworks") [remora/main.go:71]
  → listFrameworks() [api_rest]
  → fmt.Printf con colores ANSI [remora/flow_workbench.go:356]

---

### FLUJO: debug-manifest — Ver manifest de un framework

#### ESC-1: El usuario inspecciona el manifest de echo

El usuario escribe `remora debug manifest echo`. El sistema hace GET /frameworks/echo/manifest.

En terminal aparece:

```
Manifest: echo

  name:        echo
  version:     1.0.0
  command:     frameworkecho
  mode:        sync
```

TRACE:
  → handleDebugManifest(["echo"]) [remora/flow_workbench.go:373]
  → get("/frameworks/echo/manifest") [remora/main.go:71]
  → getFramework() [api_rest]
  → formatFrameworkManifest() [remora/flow_workbench.go:663]

---

### FLUJO: debug-commands — Listar comandos de un framework

#### ESC-1: El usuario lista comandos del framework echo

El usuario escribe `remora debug commands echo`. El sistema hace GET /frameworks/echo/commands.

En terminal aparece:

```
Comandos de echo

  init Inicializa un proyecto
  add-axiom sin descripcion
  add-theory sin descripcion
  add-task sin descripcion
  add-pain sin descripcion
  add-opportunity sin descripcion
  validate sin descripcion
  show-tree sin descripcion
```

TRACE:
  → handleDebugCommands(["echo"]) [remora/flow_workbench.go:401]
  → get("/frameworks/echo/commands") [remora/main.go:71]
  → datos parseados y formateados [remora/flow_workbench.go:430]

---

### FLUJO: debug-capabilities — Listar capabilities de un framework

#### ESC-1: El usuario consulta capabilities del framework auditor

El usuario escribe `remora debug capabilities auditor`. El sistema hace GET /frameworks/auditor/capabilities.

En terminal aparece:

```
Capabilities de auditor

  data.quality.audit
    inputs:  external.api.dump.v1
    outputs: auditor.findings.v1, data.gaps.v1
```

TRACE:
  → handleDebugCapabilities(["auditor"]) [remora/flow_workbench.go:444]
  → get("/frameworks/auditor/capabilities") [remora/main.go:71]
  → datos formateados con colores [remora/flow_workbench.go:493]

---

### FLUJO: debug-trace — Ver trazas de ejecucion de un run

#### ESC-1: El usuario inspecciona un run completado

El usuario escribe `remora debug trace run-abc123`. El sistema hace GET /flows/runs/run-abc123.

En terminal aparece:

```
Run Trace run-abc123
  status:      completed (finished: 2026-05-17 10:30)
  timeline:    4 steps
  execution:   echo → alfa → sabio → mensajero

Handoffs
  echo-node → alfa-node (echo.tree.v1)

Timeline detalhada

  1. echo.discovery [echo-node] completed
     → 10:28 → 10:29 (1200ms)
     produces: echo.tree.v1

  2. alfa.compile [alfa-node] completed
     → 10:29 → 10:30 (800ms)
     produces: alfa.spec.v1
```

TRACE:
  → handleDebugTrace(["run-abc123"]) [remora/flow_workbench.go:505]
  → get("/flows/runs/run-abc123") [remora/main.go:71]
  → handleGetFlowRun() [api_rest/flow_compiled_store.go:113]
  → printRunTrace(result, verbose) [remora/flow_workbench.go:684]

---

### FLUJO: debug-validate — Validar un flujo desde CLI debug

#### ESC-1: El usuario valida un flujo localmente

El usuario escribe `remora debug validate flow-abc123`. El sistema hace POST /flows/flow-abc123/validate.

En terminal aparece:

```
✓ Flow valido flow-abc123
```

O en caso de errores:

```
✗ Flow invalido flow-abc123

Violaciones (2)
  - missing_input: Nodo alfa-node requiere echo.tree.v1 at echo-node
  - orphan_node: Nodo sabio-node sin conexion

Advertencias (1)
  - unused_output: Output alfa.ideal_flow.v1 no consumido
```

TRACE:
  → handleDebugValidate(["flow-abc123"]) [remora/flow_workbench.go:539]
  → post("/flows/flow-abc123/validate", {}) [remora/main.go:93]
  → printFlowValidation() [remora/flow_workbench.go:755]

---

### FLUJO: debug-simulate — Simular flujo desde CLI debug

#### ESC-1: El usuario simula ejecucion con fixtures

El usuario escribe `remora debug simulate flow-abc123 --fixtures dataset.json --input "test"`. El sistema obtiene el flow record, luego hace POST /flows/simulate con manifest, input y fixture_artifacts.

En terminal aparece:

```
✓ Simulacion valida flow-abc123

Timeline:
  1. auditor.scan → datos procesados
  2. mecanico.propose → 3 propuestas generadas
  3. mensajero.send → email preparado (dry-run)
```

TRACE:
  → handleDebugSimulate(["flow-abc123", ...]) [remora/flow_workbench.go:572]
  → get("/flows/flow-abc123") [remora/main.go:71]
  → post("/flows/simulate", {...}) [remora/main.go:93]
  → printSimulateResult() [remora/flow_workbench.go:781]

---

### FLUJO: debug-dependencies — Ver grafo de dependencias

#### ESC-1: El usuario consulta dependencias de un flujo

El usuario escribe `remora debug dependencies flow-abc123`. El sistema obtiene el flow record completo y muestra las dependencias entre nodos.

TRACE:
  → handleDebugDependencies(["flow-abc123"]) [remora/flow_workbench.go:625]
  → get("/flows/flow-abc123") [remora/main.go:71]
  → printFlowDependencies(record) [remora/flow_workbench.go:826]

---

## SUPERFICIE: cli-devcli — Terminal con devcli (binario de desarrollo)

### FLUJO: health-check — Verificar conectividad con backend

#### ESC-1: El usuario verifica que el backend este activo

El usuario escribe `remora dev` (cualquier subcomando). Antes de ejecutar el comando, el app hook Before verifica la salud del backend via HealthCheck.

En terminal aparece:

```
[ok] backend reachable at http://localhost:8084
```

O si falla:

```
[warn] backend no reachable: connection refused
```

TRACE:
  → app.Before() [devcli/main.go:21]
  → newClient() [devcli/client.go:60]
  → HealthCheck() [devcli/client.go:608]
  → GET /health → respuesta HTTP

---

### FLUJO: inspect-frameworks — Listar frameworks con capabilities

#### ESC-1: El usuario lista todos los frameworks

El usuario escribe `remora dev inspect`. El sistema llama GetFrameworks al backend.

En terminal aparece una tabla tabulada:

```
Framework     Capabilities          Mode           Produces
-----------   -------------         ----           --------
echo          discovery             sync           echo.tree.v1
alfa          compilation           sync           alfa.spec.v1, alfa.ideal_flow.v1
auditor       data.quality.audit    sync_chain     auditor.findings.v1, data.gaps.v1
sabio         query                 sync_chain     -
...
```

TRACE:
  → inspectCmd.Action() [devcli/main.go:59]
  → GetFrameworks() [devcli/client.go:251]
  → get("/frameworks") [devcli/client.go:71]
  → listFrameworks() [api_rest]
  → tabwriter output [devcli/main.go:70-91]

---

### FLUJO: providers-capability — Ver mapeo capability a framework

#### ESC-1: El usuario consulta quien provee cada capability

El usuario escribe `remora dev providers`. El sistema llama GetProviders al backend.

En terminal aparece:

```
Capability     Frameworks
-----------    ----------
discovery      echo
compilation    alfa
data.quality   auditor
query          sabio
deploy         deployer
```

Con flag `--capability data.quality` filtra a una sola capability.

TRACE:
  → providersCmd.Action() [devcli/main.go:103]
  → GetProviders() [devcli/client.go:588]
  → get("/capabilities") [devcli/client.go:71]
  → listCapabilities() [api_rest]
  → tabwriter output [devcli/main.go:116-130]

---

### FLUJO: flow-list — Listar flujos compilados

#### ESC-1: El usuario lista todos los flujos existentes

El usuario escribe `remora dev flow list`. El sistema llama ListFlows.

En terminal aparece JSON con la lista de flujos registrados.

TRACE:
  → flowListCmd.Action() [devcli/main.go:165]
  → ListFlows() [devcli/client.go:322]
  → get("/flows") [devcli/client.go:71]
  → handleListFlows() [api_rest/flow_store.go:745]
  → JSON output via printJSON [devcli/main.go:424]

---

### FLUJO: flow-inspect-devcli — Inspeccionar flujo desde devcli

#### ESC-1: El usuario inspecciona un flujo especifico

El usuario escribe `remora dev flow inspect flow-abc123`. El sistema obtiene el flujo y muestra su manifest.

En terminal aparece:

```
Flow: flow-abc123 (prospectos-demo)

  Intent: Analizar cartera y enviar recordatorios

  Nodes (3):
  ID          Framework     Capability        Role       Inputs              Outputs
  audit-n     auditor       data.quality      analizar   external.api.dump   auditor.findings.v1
  meca-n      mecanico      propose           actuar     auditor.findings    mecanico.proposals.v1
  msg-n       mensajero     send              actuar     -                   -
```

TRACE:
  → flowInspectCmd.Action() [devcli/main.go:181]
  → GetFlow(id) [devcli/client.go:343]
  → get("/flows/{id}") [devcli/client.go:71]
  → printFlowManifest(flow) [devcli/main.go:428]

---

### FLUJO: flow-simulate-devcli — Simular flujo desde devcli

#### ESC-1: El usuario simula un flujo con fixtures

El usuario escribe `remora dev flow simulate flow-abc123 --fixture dataset.json`. El sistema llama SimulateFlow.

En terminal aparece el resultado de simulacion con timeline de pasos, duraciones estimadas y outputs generados.

TRACE:
  → flowSimulateCmd.Action() [devcli/main.go:225]
  → SimulateFlow(id, fixtures) [devcli/client.go:369]
  → post("/flows/simulate", {...}) [devcli/client.go:93]
  → simulateFlow() [api_rest]
  → printFlowRunResult(result) [devcli/main.go:468]

---

### FLUJO: flow-run-devcli — Ejecutar flujo desde devcli

#### ESC-1: El usuario ejecuta un flujo en dry-run

El usuario escribe `remora dev flow run flow-abc123`. Por defecto es dry-run. El sistema llama RunFlow.

En terminal aparece la ejecucion paso a paso:

```
[1] auditor.scan (data.quality.audit)
  role: analizar
  inputs: external.api.dump.v1
  outputs: auditor.findings.v1
  status: completed
  duration: 2300ms

[2] mecanico.propose (propose)
  ...
```

Con `--live` se ejecuta realmente. Con `--recipient email@test.com` se usa test_mode.

TRACE:
  → flowRunCmd.Action() [devcli/main.go:253]
  → RunFlow(req) [devcli/client.go:356]
  → post("/flows/run", {...}) [devcli/client.go:93]
  → runFlow() [api_rest]
  → printFlowRunResult(result, true) [devcli/main.go:468]

---

### FLUJO: flow-debug-devcli — Debug paso a paso desde devcli

#### ESC-1: El usuario debuggea un flujo con breakpoints

El usuario escribe `remora dev flow debug flow-abc123 --break-on needs_input`. El sistema ejecuta en dry-run y muestra cada paso con detalle expandido.

En terminal aparece:

```
[debug] iniciando debug para flow-abc123
  break-on: needs_input

[1] auditor.scan (data.quality.audit)
  role: analizar
  inputs: external.api.dump.v1
  outputs: auditor.findings.v1
  status: completed
  duration: 2300ms
  summary: 5 findings detectados

[2] mecanico.propose (propose)
  ...

completed run_id: run-xyz789
```

TRACE:
  → flowDebugCmd.Action() [devcli/main.go:290]
  → RunFlow(req{DryRun:true}) [devcli/client.go:356]
  → post("/flows/run", {...}) [devcli/client.go:93]
  → output paso a paso [devcli/main.go:312-347]

---

### FLUJO: trace-run — Ver trazas de un run existente

#### ESC-1: El usuario consulta un run por su ID

El usuario escribe `remora dev trace run-xyz789`. El sistema llama GetFlowRun.

En terminal aparece el resultado completo del run con timeline y artefactos.

TRACE:
  → traceCmd.Action() [devcli/main.go:357]
  → GetFlowRun(runID) [devcli/client.go:381]
  → get("/flows/runs/{id}") [devcli/client.go:71]
  → handleGetFlowRun() [api_rest/flow_compiled_store.go:113]
  → printFlowRunResult(result, true) [devcli/main.go:468]

---

### FLUJO: artifacts-run — Inspeccionar artefactos de un run

#### ESC-1: El usuario lista artefactos generados

El usuario escribe `remora dev artifacts run-xyz789 --list`. El sistema obtiene el run y lista sus artefactos.

En terminal aparece:

```
Artefactos del run run-xyz789
TYPE                                     SOURCE          CREATED
----                                     ------          -------
auditor.findings.v1                      auditor         2026-05-17
mecanico.proposals.v1                    mecanico        2026-05-17
```

Con `--view auditor.findings.v1` muestra el contenido JSON del artefacto.

TRACE:
  → artifactsCmd.Action() [devcli/main.go:380]
  → GetFlowRun(runID) [devcli/client.go:381]
  → get("/flows/runs/{id}") [devcli/client.go:71]
  → output formateado [devcli/main.go:403-418]

---

### FLUJO: rules-list — Ver reglas de composicion de flujos

#### ESC-1: El usuario consulta las reglas activas

El usuario escribe `remora dev rules`. El sistema llama GetRules.

En terminal aparece el JSON de reglas de composicion vigentes.

TRACE:
  → rulesCmd.Action() [devcli/main.go:137]
  → GetRules() [devcli/client.go:394]
  → get("/rules") [devcli/client.go:71]
  → getRules() [api_rest]
  → JSON output

---

## SUPERFICIE: cli-flujo — Terminal con binario flujo (motor de ejecucion)

### FLUJO: flujo-flow-run — Ejecutar flujo via SSE streaming

#### ESC-1: El usuario ejecuta un flujo con streaming

El usuario escribe `flujo flow run --id flow-abc123 --input "procesar morosos"`. El binario flujo abre una conexion SSE a /api/v1/flows/run/stream.

Los eventos llegan en tiempo real al stdout. Cada evento representa un nodo que inicia, progresa o termina. El usuario ve el progreso conforme ocurre.

TRACE:
  → cmdFlow() [flujo/flow_workbench.go:338]
  → runFlowWorkbench() [flujo/flow_workbench.go:344]
  → runFlowRun() [flujo/flow_workbench.go:1015]
  → stream("/flows/run/stream", payload) [flujo/flow_workbench.go:401]
  → runFlowStream() [api_rest]
  → SSE events via Server-Sent Events

---

### FLUJO: flujo-flow-create — Crear flujo via wizard del motor

#### ESC-1: El usuario crea flujo directamente desde flujo

El usuario escribe `flujo flow create --business prospectos-demo`. El motor ejecuta runFlowCreate con su propio wizard interactivo que es la implementacion canonica (la misma que remora delega).

TRACE:
  → cmdFlow() [flujo/flow_workbench.go:338]
  → runFlowCreate() [flujo/flow_workbench.go:704]
  → promptFlowCreateAnswers() [flujo/flow_workbench.go:647]
  → buildFlowCreateSuggestPayload() [flujo/flow_workbench.go:565]
  → post("/flows/suggest") → post("/businesses/{id}/flows")

---

### FLUJO: flujo-flow-draft — Crear flujo desde spec

#### ESC-1: Draft sin interaccion

El usuario escribe `flujo flow draft --business X --name N --description D --create`.

TRACE:
  → cmdFlow() [flujo/flow_workbench.go:338]
  → runFlowDraft() [flujo/flow_workbench.go:806]
  → buildFlowDraftSuggestPayload() [flujo/flow_workbench.go:870]
  → post("/flows/suggest") → post("/businesses/{id}/flows")

---

### FLUJO: flujo-flow-compile — Compilar flujo

#### ESC-1: El usuario compila un flujo

El usuario escribe `flujo flow compile --id flow-abc123`.

TRACE:
  → cmdFlow() [flujo/flow_workbench.go:338]
  → runFlowCompile() [flujo/flow_workbench.go:915]
  → post("/flows/workbench/compile", {...})
  → compileFlowWorkbench() [api_rest]

---

### FLUJO: flujo-flow-inspect — Inspeccionar flujo

#### ESC-1: Consulta de flujo

El usuario escribe `flujo flow inspect --id flow-abc123`.

TRACE:
  → cmdFlow() [flujo/flow_workbench.go:338]
  → runFlowInspect() [flujo/flow_workbench.go:936]
  → get("/flows/{id}")

---

### FLUJO: flujo-flow-validate — Validar flujo

#### ESC-1: Validacion del flujo

El usuario escribe `flujo flow validate --id flow-abc123`.

TRACE:
  → cmdFlow() [flujo/flow_workbench.go:338]
  → runFlowValidate() [flujo/flow_workbench.go:957]
  → post("/flows/validate", {...})

---

### FLUJO: flujo-flow-simulate — Simular flujo

#### ESC-1: Simulacion dry-run

El usuario escribe `flujo flow simulate --id flow-abc123`.

TRACE:
  → cmdFlow() [flujo/flow_workbench.go:338]
  → runFlowSimulate() [flujo/flow_workbench.go:983]
  → post("/flows/simulate", {...})

---

### FLUJO: flujo-flow-install — Instalar flujo

#### ESC-1: Instalacion para ejecucion recurrente

El usuario escribe `flujo flow install --id flow-abc123`.

TRACE:
  → cmdFlow() [flujo/flow_workbench.go:338]
  → runFlowInstall() [flujo/flow_workbench.go:1058]
  → post("/flows/{id}/install")

---

## SUPERFICIE: api-rest — HTTP API en localhost:8084

### FLUJO: auth-register — Registrar usuario nuevo

#### ESC-1: El cliente envia POST /api/v1/auth/register

El cliente HTTP envia un POST con email y password. El sistema crea el usuario en SQLite via createUser, genera una sesion y devuelve el token.

Respuesta JSON:

```json
{
  "success": true,
  "data": {
    "token": "sess-xxx",
    "user": { "id": "u-1", "email": "user@test.com" }
  }
}
```

TRACE:
  → handleAuthRegister() [api_rest/auth_handlers.go:42]
  → createUser() [api_rest/auth.go:257]
  → createSession() [api_rest/auth.go:306]
  → JSON response

---

### FLUJO: auth-login — Iniciar sesion

#### ESC-1: El cliente envia POST /api/v1/auth/login

El cliente envia email y password. El sistema autentica contra SQLite via authenticate, genera sesion.

TRACE:
  → handleAuthLogin() [api_rest/auth_handlers.go:68]
  → authenticate() [api_rest/auth.go:291]
  → createSession() [api_rest/auth.go:306]
  → JSON response con token

---

### FLUJO: auth-logout — Cerrar sesion

#### ESC-1: El cliente envia POST /api/v1/auth/logout

El sistema invalida la sesion activa.

TRACE:
  → handleAuthLogout() [api_rest/auth_handlers.go:94]
  → sesion invalidada

---

### FLUJO: auth-me — Consultar usuario actual

#### ESC-1: El cliente envia GET /api/v1/auth/me

El sistema lee el token del header Authorization, busca la sesion y devuelve datos del usuario con sus memberships.

TRACE:
  → handleAuthMe() [api_rest/auth_handlers.go:103]
  → authTokenFromRequest() [api_rest/auth.go:701]
  → JSON con user + memberships

---

### FLUJO: businesses-list — Listar negocios del usuario

#### ESC-1: El cliente envia GET /api/v1/businesses

El sistema devuelve los negocios a los que el usuario autenticado tiene acceso.

TRACE:
  → handleBusinesses() [api_rest/auth_handlers.go:117]
  → query memberships [api_rest/auth.go:350]
  → JSON array de businesses

---

### FLUJO: business-create — Crear negocio nuevo

#### ESC-1: El cliente envia POST /api/v1/businesses

El sistema crea un nuevo negocio y asigna al usuario como owner.

TRACE:
  → handleBusinessCreate() [api_rest/auth_handlers.go:130]
  → db.Exec() insert business
  → JSON con business_id

---

### FLUJO: business-members — Ver miembros de un negocio

#### ESC-1: El cliente envia GET /api/v1/businesses/{id}/members

El sistema lista los miembros del negocio con sus roles.

TRACE:
  → handleBusinessMembers() [api_rest/auth_handlers.go:232]
  → query memberships
  → JSON array

---

### FLUJO: business-invite — Invitar usuario a negocio

#### ESC-1: El cliente envia POST /api/v1/businesses/{id}/invites

El sistema genera un token de invitacion para unirse al negocio.

TRACE:
  → handleBusinessInviteCreate() [api_rest/auth_handlers.go:245]
  → token generado
  → JSON con invite token

---

### FLUJO: invite-lookup — Consultar invitacion por token

#### ESC-1: El cliente envia GET /api/v1/invites/lookup?token=X

El sistema busca la invitacion y devuelve sus datos (negocio, rol ofrecido).

TRACE:
  → handleInviteLookup() [api_rest/auth_handlers.go:273]
  → query invite
  → JSON con datos de invitacion

---

### FLUJO: invite-accept — Aceptar invitacion

#### ESC-1: El cliente envia POST /api/v1/invites/accept

El sistema agrega al usuario al negocio con el rol de la invitacion.

TRACE:
  → handleInviteAccept() [api_rest/auth_handlers.go:283]
  → insert membership
  → JSON confirmacion

---

### FLUJO: admin-users — Listar usuarios (admin)

#### ESC-1: El admin consulta GET /api/v1/admin/users

El sistema devuelve la lista completa de usuarios registrados.

TRACE:
  → handleAdminUsers() [api_rest/auth_handlers.go:157]
  → db.Query users
  → JSON array

---

### FLUJO: admin-team — Ver equipo (admin)

#### ESC-1: El admin consulta GET /api/v1/admin/team

El sistema devuelve informacion del equipo completo.

TRACE:
  → handleAdminTeam() [api_rest/auth_handlers.go:169]
  → query team
  → JSON

---

### FLUJO: admin-remora-invite — Crear invitacion de plataforma

#### ESC-1: El admin envia POST /api/v1/admin/remora-invites

El sistema genera una invitacion a nivel plataforma (no de negocio).

TRACE:
  → handleAdminRemoraInviteCreate() [api_rest/auth_handlers.go:181]
  → token generado
  → JSON

---

### FLUJO: remora-invite-lookup — Consultar invitacion de plataforma

#### ESC-1: El cliente envia GET /api/v1/remora-invites/lookup?token=X

TRACE:
  → handleRemoraInviteLookup() [api_rest/auth_handlers.go:203]
  → query invite
  → JSON

---

### FLUJO: remora-invite-accept — Aceptar invitacion de plataforma

#### ESC-1: El cliente envia POST /api/v1/remora-invites/accept

TRACE:
  → handleRemoraInviteAccept() [api_rest/auth_handlers.go:213]
  → crear usuario/membership
  → JSON

---

### FLUJO: frameworks-list — Listar frameworks disponibles

#### ESC-1: El cliente envia GET /api/v1/frameworks

El sistema escanea los frameworks registrados y devuelve name, version, mode, commands, capabilities para cada uno.

TRACE:
  → listFrameworks() [api_rest]
  → scan framework manifests
  → JSON array con 21 frameworks

---

### FLUJO: frameworks-testable — Listar frameworks testeables

#### ESC-1: El cliente envia GET /api/v1/frameworks/testable

El sistema filtra frameworks que tienen testable=true (los que soportan sesion conversacional).

TRACE:
  → listTestableFrameworks() [api_rest]
  → filtro testable
  → JSON array

---

### FLUJO: frameworks-chainable — Listar frameworks encadenables

#### ESC-1: El cliente envia GET /api/v1/frameworks/chainable

El sistema filtra frameworks con execution_mode != async_trigger.

TRACE:
  → listChainableFrameworks() [api_rest]
  → filtro chainable
  → JSON array

---

### FLUJO: capabilities-list — Listar todas las capabilities

#### ESC-1: El cliente envia GET /api/v1/capabilities

El sistema agrega capabilities de todos los frameworks.

TRACE:
  → listCapabilities() [api_rest]
  → aggregate capabilities
  → JSON array

---

### FLUJO: capability-providers — Ver providers de una capability

#### ESC-1: El cliente envia GET /api/v1/capabilities/{id}/providers

El sistema busca que frameworks proveen la capability indicada.

TRACE:
  → listCapabilityProviders() [api_rest]
  → filtro por capability id
  → JSON array de frameworks

---

### FLUJO: flows-validate — Validar un flujo

#### ESC-1: El cliente envia POST /api/v1/flows/validate

El sistema verifica coherencia del manifest: tipos matching, capabilities existentes, nodos conectados.

TRACE:
  → validateFlow() [api_rest]
  → validacion de reglas
  → JSON con valid/violations/warnings

---

### FLUJO: flows-simulate — Simular un flujo

#### ESC-1: El cliente envia POST /api/v1/flows/simulate

El sistema ejecuta el flujo en modo dry-run sin efectos secundarios reales.

TRACE:
  → simulateFlow() [api_rest]
  → ejecucion simulada
  → JSON con timeline

---

### FLUJO: flows-run — Ejecutar un flujo

#### ESC-1: El cliente envia POST /api/v1/flows/run

El sistema ejecuta el flujo invocando cada framework via channel. Los frameworks se ejecutan secuencialmente siguiendo el orden topologico del grafo.

TRACE:
  → runFlow() [api_rest]
  → ensureChannel() [api_rest/flow_channel.go:56]
  → exec.Command(channelBin) para cada nodo
  → JSON con resultado

---

### FLUJO: flows-run-stream — Ejecutar flujo con SSE

#### ESC-1: El cliente envia POST /api/v1/flows/run/stream

El sistema ejecuta el flujo y emite Server-Sent Events conforme cada nodo progresa. El frontend consume este stream para mostrar progreso en tiempo real.

TRACE:
  → runFlowStream() [api_rest]
  → ensureChannel() [api_rest/flow_channel.go:56]
  → SSE events: node_start, node_progress, node_complete, flow_complete

---

### FLUJO: flows-suggest — Sugerir capabilities para un flujo

#### ESC-1: El cliente envia POST /api/v1/flows/suggest

El sistema recibe intent, business_id, constraints y devuelve un manifest propuesto con nodos y capabilities sugeridas.

TRACE:
  → suggestFlowCapabilities() [api_rest]
  → analisis de intent + capabilities disponibles
  → JSON con manifest propuesto

---

### FLUJO: flows-compile — Compilar flujo para ejecucion

#### ESC-1: El cliente envia POST /api/v1/flows/workbench/compile

El sistema compila el flujo resolviendo dependencias y genera un compiled_id.

TRACE:
  → compileFlowWorkbench() [api_rest]
  → resolucion de dependencias
  → JSON con compiled_id

---

### FLUJO: flows-compiled-get — Obtener flujo compilado

#### ESC-1: El cliente envia GET /api/v1/flows/compiled/{compiled_id}

TRACE:
  → handleGetCompiledFlow() [api_rest/flow_compiled_store.go:98]
  → lectura de store
  → JSON con flujo compilado

---

### FLUJO: flows-run-get — Obtener resultado de un run

#### ESC-1: El cliente envia GET /api/v1/flows/runs/{id}

TRACE:
  → handleGetFlowRun() [api_rest/flow_compiled_store.go:113]
  → lectura de store
  → JSON con timeline, status, artifacts

---

### FLUJO: flow-templates-list — Listar templates de flujos

#### ESC-1: El cliente envia GET /api/v1/businesses/{id}/flow-templates

TRACE:
  → handleListFlowTemplates() [api_rest/flow_templates.go:97]
  → scan templates
  → JSON array

---

### FLUJO: flows-crud — CRUD de flujos por negocio

#### ESC-1: Listar flujos — GET /api/v1/businesses/{id}/flows

TRACE:
  → handleListFlows() [api_rest/flow_store.go:745]
  → query flows por business_id
  → JSON array

#### ESC-2: Crear flujo — POST /api/v1/businesses/{id}/flows

TRACE:
  → handleCreateFlow() [api_rest/flow_store.go:763]
  → persist flow
  → JSON con flow_id

#### ESC-3: Obtener flujo — GET /api/v1/flows/{id}

TRACE:
  → handleGetFlow() [api_rest/flow_store.go:796]
  → read flow
  → JSON

#### ESC-4: Actualizar flujo — PUT /api/v1/flows/{id}

TRACE:
  → handleUpdateFlow() [api_rest/flow_store.go:831]
  → update flow
  → JSON

#### ESC-5: Eliminar flujo — DELETE /api/v1/flows/{id}

TRACE:
  → handleDeleteFlow() [api_rest/flow_store.go:868]
  → delete flow
  → JSON confirmacion

#### ESC-6: Instalar flujo — POST /api/v1/flows/{id}/install

TRACE:
  → handleInstallFlow() [api_rest/flow_store.go:811]
  → mark installed
  → JSON

---

### FLUJO: conversations-crud — CRUD de conversaciones

#### ESC-1: Listar conversaciones — GET /api/v1/conversations

TRACE:
  → listConversations() [api_rest]
  → query conversations
  → JSON array

#### ESC-2: Crear conversacion — POST /api/v1/conversations

TRACE:
  → createConversation() [api_rest]
  → insert conversation
  → JSON con conversation_id

#### ESC-3: Obtener conversacion — GET /api/v1/conversations/{id}

TRACE:
  → getConversation() [api_rest]
  → read conversation
  → JSON

#### ESC-4: Eliminar conversacion — DELETE /api/v1/conversations/{id}

TRACE:
  → deleteConversation() [api_rest]
  → delete conversation
  → JSON

#### ESC-5: Listar mensajes — GET /api/v1/conversations/{id}/messages

TRACE:
  → getMessages() [api_rest]
  → query messages
  → JSON array

#### ESC-6: Enviar mensaje — POST /api/v1/conversations/{id}/messages

El sistema recibe el mensaje del usuario, lo persiste, invoca el framework_session correspondiente que ejecuta runAgentLoop con el LLM. La respuesta se genera via streaming y se persiste.

TRACE:
  → postMessage() [api_rest]
  → exec.Command(framework_session, "message", ...) [api_rest]
  → runMessage() [framework_session/main.go:172]
  → runAgentLoop() [framework_session/main.go:332]
  → llm.Stream() [llm/client.go:218]
  → respuesta persistida

#### ESC-7: Consultar cola de preguntas — GET /api/v1/conversations/{id}/queue

TRACE:
  → getQueue() [api_rest]
  → GetNextPending() [handoff/questions_queue.go:174]
  → JSON con pregunta pendiente o null

---

### FLUJO: conversations-single — Sesion de framework individual

#### ESC-1: Crear sesion single — POST /api/v1/conversations-single

El sistema crea una sesion de framework individual (un solo framework en modo interactivo).

TRACE:
  → createSingleConversation() [api_rest]
  → init session
  → JSON con session_id

#### ESC-2: Enviar mensaje single — POST /api/v1/conversations-single/{id}/messages

TRACE:
  → postSingleMessage() [api_rest]
  → framework_session start/message
  → JSON con respuesta

#### ESC-3: Eventos live — GET /api/v1/conversations-single/{id}/live

El sistema abre un stream SSE que emite eventos del framework en tiempo real (tool calls, respuestas parciales).

TRACE:
  → getFrameworkSessionLiveEvents() [api_rest]
  → SSE events via liveEventWriter [framework_session/main.go:274]

---

### FLUJO: framework-command-run — Ejecutar comando de framework via API

#### ESC-1: El cliente envia POST /api/v1/frameworks/{name}/commands/{command}/run

El sistema invoca el framework indicado con el comando y parametros dados. Usa framework_session para manejar la sesion del agente.

TRACE:
  → runFrameworkCommand() [api_rest]
  → exec framework_session [api_rest]
  → runStart() [framework_session/main.go:93]
  → runAgentLoop() [framework_session/main.go:332]
  → resultado

---

### FLUJO: rules-crud — Gestion de reglas de composicion

#### ESC-1: Obtener reglas — GET /api/v1/rules

TRACE:
  → getRules() [api_rest]
  → read rules
  → JSON

#### ESC-2: Actualizar reglas — PUT/POST /api/v1/rules

TRACE:
  → updateRules() [api_rest]
  → persist rules
  → JSON confirmacion

---

### FLUJO: data-browser — Explorar datos

#### ESC-1: Listar tablas — GET /api/v1/data/tables

El sistema lee las tablas disponibles en la base SQLite de datos.

TRACE:
  → handleDataTables() [api_rest/data_browser.go:44]
  → query sqlite tables
  → JSON array de tablas

#### ESC-2: Ver filas de tabla — GET /api/v1/data/tables/{table}

TRACE:
  → handleDataTableRows() [api_rest/data_browser.go:64]
  → query rows
  → JSON con filas

#### ESC-3: Tablas por negocio — GET /api/v1/businesses/{id}/data/tables

TRACE:
  → handleBusinessDataTables() [api_rest/data_browser.go:114]
  → query
  → JSON

#### ESC-4: Filas por negocio — GET /api/v1/businesses/{id}/data/tables/{table}

TRACE:
  → handleBusinessDataTableRows() [api_rest/data_browser.go:133]
  → query
  → JSON

#### ESC-5: Upload de datos — POST /api/v1/businesses/{id}/data/upload

TRACE:
  → handleBusinessDataUpload() [api_rest/data_browser.go:150]
  → procesar archivo
  → JSON confirmacion

---

### FLUJO: business-artifacts — Consultar artefactos del negocio

#### ESC-1: GET /api/v1/businesses/{id}/artifacts

El sistema devuelve artefactos disponibles para el negocio (datasets, configs, etc.).

TRACE:
  → handleBusinessArtifacts() [api_rest/business_artifacts.go:19]
  → scan artifacts
  → JSON

---

### FLUJO: api-connections — Gestion de conexiones API externas

#### ESC-1: Listar conexiones — GET /api/v1/businesses/{id}/api-connections

TRACE:
  → handleAPIConnectionsList() [api_rest]
  → query connections
  → JSON array

#### ESC-2: Crear conexion — POST /api/v1/businesses/{id}/api-connections

TRACE:
  → handleAPIConnectionCreate() [api_rest]
  → persist connection
  → JSON

#### ESC-3: Plan de conexion — POST /api/v1/businesses/{id}/api-connections/plan

TRACE:
  → handleAPIConnectionPlan() [api_rest]
  → generate plan
  → JSON

#### ESC-4: Sincronizar conexion — POST /api/v1/businesses/{id}/api-connections/{id}/sync

TRACE:
  → handleAPIConnectionSync() [api_rest]
  → execute sync
  → JSON

---

### FLUJO: smtp-operations — Operaciones SMTP

#### ESC-1: Check SMTP — GET /api/v1/businesses/{id}/smtp/check

TRACE:
  → handleSMTPCheck() [api_rest/flow_smtp.go:16]
  → vault has credentials.smtp
  → JSON con estado

#### ESC-2: Conectar hosting — POST /api/v1/businesses/{id}/hosting/connect

TRACE:
  → handleHostingConnect() [api_rest/flow_smtp.go:79]
  → exec framework-hosting
  → JSON

#### ESC-3: Importar SMTP — POST /api/v1/businesses/{id}/smtp/import

TRACE:
  → handleSMTPImport() [api_rest/flow_smtp.go:147]
  → vault set credentials.smtp
  → JSON

#### ESC-4: Eliminar SMTP — DELETE /api/v1/businesses/{id}/smtp

TRACE:
  → handleSMTPDelete() [api_rest/flow_smtp.go:224]
  → vault delete
  → JSON

---

### FLUJO: send-email — Enviar email via mensajero

#### ESC-1: El cliente envia POST /api/v1/send-email

El sistema invoca el binario mensajero externo via exec.Command(REMORA_MENSAJERO_BIN).

TRACE:
  → handleSendEmail() [api_rest/main.go:1150]
  → exec.Command(binPath, args...) [api_rest/main.go:1254]
  → framework-mensajero binary (externo)
  → resultado

---

### FLUJO: config-runtime — Consultar configuracion

#### ESC-1: GET /api/v1/config

El sistema devuelve la configuracion runtime (provider LLM, modo dev, etc.).

TRACE:
  → handleConfig() [api_rest/main.go:1141]
  → JSON con config

---

### FLUJO: runtime-info — Consultar runtime

#### ESC-1: GET /api/v1/runtime

TRACE:
  → getRuntime() [api_rest]
  → JSON con info de runtime

---

### FLUJO: models-list — Listar modelos LLM disponibles

#### ESC-1: GET /api/v1/models

TRACE:
  → listModels() [api_rest]
  → JSON array de modelos

---

### FLUJO: traces-latest — Consultar trazas recientes

#### ESC-1: GET /api/v1/traces/latest

TRACE:
  → handleTracesLatest() [api_rest/traces.go:20]
  → read latest traces
  → JSON

---

### FLUJO: tasks-crud — Gestion de tareas

#### ESC-1: Listar tareas — GET /api/v1/tasks

TRACE:
  → handleTasksList() [api_rest/tareas.go:89]
  → query tasks
  → JSON array

#### ESC-2: Siguiente tarea — GET /api/v1/tasks/next

TRACE:
  → handleTasksNext() [api_rest/tareas.go:99]
  → next pending task
  → JSON

#### ESC-3: Crear tarea — POST /api/v1/tasks

TRACE:
  → handleTasksCreate() [api_rest/tareas.go:117]
  → insert task
  → JSON

#### ESC-4: Evento en tarea — POST /api/v1/tasks/{id}/event

TRACE:
  → handleTaskEvent() [api_rest/tareas.go:144]
  → insert event
  → JSON

---

### FLUJO: autonomia-controlada — Simulacion de sesion autonoma

#### ESC-1: Bootstrap — GET /api/v1/simulations/autonomia-controlada/bootstrap

El sistema inicializa una sesion de autonomia controlada con estado inicial.

TRACE:
  → handleAutonomiaBootstrap() [api_rest/simulation_autonomia.go:15]
  → InitialState() [autonomia/session.go:75]
  → Bootstrap() [autonomia/session.go:82]
  → JSON con estado inicial

#### ESC-2: Mensaje — POST /api/v1/simulations/autonomia-controlada/message

El sistema procesa un mensaje del usuario en la sesion autonoma. Detecta saludos, reparaciones, forecasts y responde con generalResponse o generalSocialResponse.

TRACE:
  → handleAutonomiaMessage() [api_rest/simulation_autonomia.go:23]
  → HandleMessage() [autonomia/session.go:95]
  → ensureState() / ensureEntity() [autonomia/session.go:242,255]
  → generalResponse() [autonomia/session.go:179]
  → JSON con respuesta

---

### FLUJO: contactos-operations — Operaciones de contactos

#### ESC-1: Lookup de perfil

El sistema invoca un binario externo de contactos via exec.Command para buscar un perfil.

TRACE:
  → contactosLookupProfile() [api_rest/contactos.go:51]
  → exec.Command(bin, ...) [api_rest/contactos.go:51]
  → resultado

#### ESC-2: Store de perfil

TRACE:
  → contactosStoreProfile() [api_rest/contactos.go:83]
  → exec.Command(bin, ...) [api_rest/contactos.go:83]
  → resultado

---

### FLUJO: health-endpoints — Health checks

#### ESC-1: GET /health o GET /healthz

El sistema responde con status OK para que load balancers y CLIs verifiquen disponibilidad.

TRACE:
  → health handler [api_rest]
  → 200 OK

---

### FLUJO: frontend-static — Servir frontend estatico

#### ESC-1: GET / — Canvas visual

El sistema sirve el index.html del canvas de flujos en :8084.

TRACE:
  → static file handler [api_rest/main.go]
  → serve api_rest/static/index.html

#### ESC-2: GET /data — Data browser

TRACE:
  → static file handler
  → serve data browser page

#### ESC-3: GET /app — App frontend

TRACE:
  → static file handler
  → serve app page

---

## SUPERFICIE: frontend-web — Interfaz visual de flujos (localhost:8084)

### FLUJO: inicio-sesion-web — Login/Registro en la web

#### ESC-1: El usuario accede a la aplicacion web

El usuario abre http://localhost:8084 en su navegador. El sistema carga el HTML con tema light por defecto. La pagina muestra un header con logo "Remora", botones de accion y un area central con glow difuso.

El sistema verifica si hay token en localStorage (AUTH_TOKEN_KEY = 'remora_token'). Si existe, llama GET /api/v1/auth/me para validar la sesion. Si no existe, muestra el auth pill sin autenticar.

En el navegador aparece:
- Header sticky con logo Remora, indicador de modelo (minimax), y auth pill
- Area central con glow difuso y campo de input

#### ESC-2: El usuario se registra o logea

El usuario hace click en el auth pill. Aparece un modal con tabs "Login" y "Register". El usuario completa email y password.

El sistema envia POST /api/v1/auth/login o /auth/register. Al obtener el token, lo guarda en localStorage y actualiza el estado de authState.

TRACE:
  → click auth pill → modal visible
  → fetch POST /api/v1/auth/login [index.html:3343]
  → setAuthState(data) [index.html:3356]
  → localStorage.setItem(AUTH_TOKEN_KEY, token)

---

### FLUJO: seleccionar-framework — Elegir framework para sesion

#### ESC-1: El usuario ve los frameworks disponibles

El sistema carga GET /api/v1/frameworks al iniciar. Los frameworks testeables aparecen como chips en la interfaz, cada uno con su icono SVG y color asignado:

- Echo (violeta) — "Entiende tu problema, hace preguntas estrategicas"
- Alfa (cyan) — "Simula soluciones y debate hasta encontrar el mejor approach"
- Bravo (verde) — "Ejecuta el plan elegido, valida cada paso"
- Sabio (amarillo) — "Experto en tus datos: hace retrieval sobre tu informacion indexada"
- Indexa (verde) — "Ingiere data desde tus APIs"
- Auditor (rojo) — "Recorre tu ERP buscando errores enterrados"
- Mecanico (azul) — "Toma los hallazgos del Auditor, propone planes de fix"
- Foco (naranja) — "Priorizador: analiza deudores y genera lista de cobranza"

#### ESC-2: El usuario selecciona un framework

El usuario hace click en un chip de framework. Aparece un modal de confirmacion (singleFwModal). El usuario confirma. El sistema crea una sesion via POST /api/v1/conversations-single y abre el chat con ese framework.

TRACE:
  → click chip → onClick(fw, div) [index.html:3664]
  → showSingleFwModal() [index.html]
  → singleFwConfirm click [index.html:3703]
  → fetch POST /api/v1/conversations-single
  → currentConvId asignado
  → chat habilitado

---

### FLUJO: conversacion-chat — Interactuar via chat con framework

#### ESC-1: El usuario envia un mensaje

El usuario escribe en el campo de input y presiona Enter o el boton de envio. El sistema envia POST /api/v1/conversations-single/{id}/messages con el texto.

La interfaz muestra el mensaje del usuario en la burbuja de chat. Luego muestra un indicador de carga mientras espera la respuesta del framework.

#### ESC-2: El sistema responde via streaming

El sistema invoca framework_session que ejecuta runAgentLoop con el LLM. Los eventos llegan via SSE endpoint /api/v1/conversations-single/{id}/live. El frontend procesa cada evento y muestra la respuesta progresivamente.

En el navegador aparece la respuesta del framework en una burbuja con el color del framework activo. Si el framework hace tool calls (bash, read_file, etc.), el usuario puede ver indicadores de las herramientas ejecutadas.

TRACE:
  → input submit [index.html]
  → fetch POST /api/v1/conversations-single/{id}/messages
  → postSingleMessage() [api_rest]
  → exec framework_session message [api_rest]
  → runMessage() [framework_session/main.go:172]
  → runAgentLoop() [framework_session/main.go:332]
  → llm.Stream() → SSE events
  → getFrameworkSessionLiveEvents() [api_rest]
  → frontend renderiza respuesta

---

### FLUJO: flujo-builder-visual — Construir flujos desde el canvas

#### ESC-1: El usuario abre el modal de flujos

El usuario hace click en el boton "Flows" del header. Aparece el modal flowsModal con dos tabs: "Design" y "Run".

En el tab Design se muestra la biblioteca de flujos existentes (flowsLibraryView) cargada desde GET /api/v1/businesses/{id}/flows.

#### ESC-2: El usuario crea un nuevo flujo

El usuario hace click en "Nuevo flujo" (flowsNewBtn). Aparece flowsBuilderView donde puede seleccionar frameworks para agregar como nodos del flujo.

Los frameworks chainables (execution_mode != async_trigger) aparecen como opciones disponibles. El usuario selecciona los que quiere encadenar.

#### ESC-3: El usuario ejecuta un flujo

En el tab "Run" (flowsExecutionView), el usuario selecciona un flujo compilado y lo ejecuta. El sistema llama POST /api/v1/flows/run/stream y muestra los eventos en tiempo real.

En el navegador aparecen los nodos del flujo activandose secuencialmente, cada uno con su color asignado y estado (pending, running, completed, failed).

TRACE:
  → flowsCancelBtn, flowsTabDesign, flowsTabRun [index.html:3850]
  → fetch GET /api/v1/businesses/{id}/flows
  → fetch POST /api/v1/flows/run/stream [index.html:7852,9215,9403]
  → SSE events renderizados en UI

---

### FLUJO: seleccionar-modelo — Cambiar modelo LLM

#### ESC-1: El usuario cambia el modelo

El usuario hace click en el indicador de modelo (modelIndicator). Aparece el modal modelModal con la lista de modelos disponibles cargada desde GET /api/v1/models.

El usuario selecciona un modelo. El sistema actualiza currentModel y lo envia en las requests subsecuentes.

TRACE:
  → modelIndicator click [index.html:3598]
  → modelModal visible
  → option click → selectModel(model.id) [index.html:3585]
  → currentModel actualizado

---

## SUPERFICIE: channel-rpc — Servidor JSON-RPC en localhost:8080

### FLUJO: execute-command — Ejecutar comando en el sistema

#### ESC-1: Un framework envia execute_command via JSON-RPC

El framework (ej: nativeagent ejecutando una tool) envia un request JSON-RPC al channel:

```json
{
  "jsonrpc": "2.0",
  "method": "execute_command",
  "params": { "command": "ls", "args": ["-la"], "cwd": "/tmp" },
  "id": 1
}
```

El sistema valida el request con ValidateJSONRPC y IsMethodAllowed. Si pasa, ejecuta el comando via ExecuteCommandWithEnv con timeout configurable (CHANNEL_EXEC_TIMEOUT).

Respuesta:

```json
{
  "jsonrpc": "2.0",
  "result": { "stdout": "...", "stderr": "", "exit_code": 0 },
  "id": 1
}
```

TRACE:
  → Handle() [internal/handler.go:50]
  → ValidateJSONRPC() [internal/jsonrpc.go:14]
  → IsMethodAllowed() [internal/jsonrpc.go:45]
  → executeCommand() [internal/handler.go:133]
  → ExecuteCommandWithEnv() [internal/exec.go:22]
  → exec.CommandContext(ctx, cmd, args...) [internal/exec.go:26]
  → resultado

---

### FLUJO: read-file — Leer archivo

#### ESC-1: Un framework solicita leer un archivo

Request: `{"method": "read_file", "params": {"path": "/path/to/file"}}`

El sistema resuelve la ruta con resolveWithinBase para prevenir path traversal, luego lee con os.ReadFile.

TRACE:
  → Handle() [internal/handler.go:50]
  → readFile() [internal/handler.go:192]
  → resolveWithinBase() [internal/handler.go:411]
  → os.ReadFile()
  → contenido en result

---

### FLUJO: write-file — Escribir archivo

#### ESC-1: Un framework solicita escribir un archivo

Request: `{"method": "write_file", "params": {"path": "/path/to/file", "content": "..."}}`

TRACE:
  → Handle() [internal/handler.go:50]
  → writeFile() [internal/handler.go:209]
  → resolveWithinBase() [internal/handler.go:411]
  → os.WriteFile() [internal/handler.go:223]

---

### FLUJO: list-dir — Listar directorio

#### ESC-1: Un framework solicita listar un directorio

Request: `{"method": "list_dir", "params": {"path": "/path/to/dir"}}`

TRACE:
  → Handle() [internal/handler.go:50]
  → listDir() [internal/handler.go:229]
  → os.ReadDir()
  → array de entries

---

### FLUJO: grep-search — Buscar en archivos

#### ESC-1: Un framework busca texto en archivos

Request: `{"method": "grep", "params": {"pattern": "TODO", "path": "/project"}}`

El sistema ejecuta grep o ripgrep (rg) como proceso externo.

TRACE:
  → Handle() [internal/handler.go:50]
  → grep() [internal/handler.go:250]
  → exec(grep/rg)
  → resultado

---

### FLUJO: find-files — Encontrar archivos

#### ESC-1: Un framework busca archivos por patron

Request: `{"method": "find", "params": {"path": "/project", "pattern": "*.go"}}`

El sistema usa walkReadableFiles para recorrer el arbol.

TRACE:
  → Handle() [internal/handler.go:50]
  → find() [internal/handler.go:293]
  → walkReadableFiles() [internal/handler.go:464]
  → array de paths

---

### FLUJO: edit-file — Editar archivo

#### ESC-1: Un framework solicita editar un archivo

Request: `{"method": "edit_file", "params": {"path": "...", "content": "..."}}`

TRACE:
  → Handle() [internal/handler.go:50]
  → editFile() [internal/handler.go:340]
  → resolveWithinBase() [internal/handler.go:411]
  → os.WriteFile() [internal/handler.go:371]

---

### FLUJO: http-get — Hacer request HTTP externo

#### ESC-1: Un framework solicita datos via HTTP

Request: `{"method": "http_get", "params": {"url": "https://api.example.com/data"}}`

TRACE:
  → Handle() [internal/handler.go:50]
  → httpGet() [internal/handler.go:377]
  → http.Do() [internal/handler.go:393]
  → body en result

---

## SUPERFICIE: cli-orchestrator — Orquestador generico de frameworks

### FLUJO: orchestrator-list — Listar frameworks descubiertos

#### ESC-1: El usuario lista frameworks via orchestrator

El usuario escribe `orchestrator list`. El sistema descubre frameworks buscando framework.manifest.json en cada subdirectorio del base-dir.

En terminal aparece:

```
Frameworks descubiertos: 21

• echo v1.0.0 — Framework para guiar reuniones de descubrimiento de procesos
    outputs:
      - tree : echo.tree.v1 → framework-echo/frameworkecho.json
    commands: init, add-axiom, add-theory, add-task, add-pain, add-opportunity, validate, show-tree

• alfa v1.0.0 — Compilador semantico Echo→Bravo
    inputs:
      - echo_tree : echo.tree.v1 (required)
    outputs:
      - alfa_spec : alfa.spec.v1 → framework-alfa/temp/alfa_spec.json
      - ideal_flow : alfa.ideal_flow.v1 → framework-alfa/temp/ideal_flow.json
    commands: compile, inspect, export-bravo, next-question, ingest-answer
...
```

TRACE:
  → main() [orchestrator/main.go:24]
  → discover(baseDir) [orchestrator/main.go:92]
  → manifest.Load(path) para cada framework
  → cmdList(frameworks) [orchestrator/main.go:116]
  → fmt.Printf con formato detallado

---

### FLUJO: orchestrator-chains — Ver cadenas validas

#### ESC-1: El usuario consulta cadenas posibles

El usuario escribe `orchestrator chains`. El sistema calcula todas las cadenas validas comparando output.format de un framework con input.format de otro.

En terminal aparece:

```
Cadenas validas (output.format → input.format):

  echo:tree (echo.tree.v1) → alfa:echo_tree (echo.tree.v1)
  auditor:findings (auditor.findings.v1) → mecanico:findings (auditor.findings.v1)
  ...
```

TRACE:
  → cmdChains(frameworks) [orchestrator/main.go:141]
  → from.CanChain(to) [manifest]
  → fmt.Printf para cada link valido

---

### FLUJO: orchestrator-run — Ejecutar comando de framework

#### ESC-1: El usuario ejecuta un comando especifico

El usuario escribe `orchestrator run echo add-axiom title="Demo" evidence="..."`. El sistema resuelve el framework, construye los argumentos y ejecuta via Channel adapter.

TRACE:
  → cmdRun(c, frameworks, "echo", "add-axiom", kv) [orchestrator/main.go:162]
  → resolveInputs() [orchestrator/main.go:233]
  → adapter.ExecuteCommand() [adapter/adapter.go:48]
  → JSON-RPC execute_command al channel
  → resultado

---

### FLUJO: orchestrator-chain — Ejecutar cadena de comandos

#### ESC-1: El usuario ejecuta una cadena completa

El usuario escribe `orchestrator chain echo:show-tree alfa:compile alfa:inspect`. El sistema ejecuta los comandos secuencialmente, pasando outputs del paso anterior como inputs del siguiente.

TRACE:
  → cmdChain(c, frameworks, args) [orchestrator/main.go:208]
  → para cada step: resolveInputs + adapter.ExecuteCommand
  → output de cada paso alimenta el siguiente

---

## SUPERFICIE: cli-vault — Almacen de secretos (binario vault)

### FLUJO: vault-has — Verificar existencia de secreto

#### ESC-1: Un framework o api_rest verifica si existe un secreto

Se ejecuta `vault has --conv conv-123 --key credentials.smtp`. El sistema verifica si la capability existe en el vault cifrado.

En terminal aparece `true` (exit 0) o `false` (exit 2).

TRACE:
  → main() [vault/main.go:34]
  → cmdHas(args) [vault/main.go:77]
  → requireConvKey() [vault/main.go:186]
  → vault.Has("", conv, key) [vault/vault.go:210]
  → verificacion de archivo cifrado en disco

---

### FLUJO: vault-get — Obtener secreto descifrado

#### ESC-1: Un framework obtiene credenciales

Se ejecuta `vault get --conv conv-123 --key credentials.smtp`. El sistema descifra el valor con AES-GCM usando REMORA_VAULT_KEY.

En stdout aparece el JSON descifrado sin newline extra. Exit 0 si existe, exit 2 si no.

TRACE:
  → cmdGet(args) [vault/main.go:91]
  → vault.Get("", conv, key) [vault/vault.go:176]
  → masterKey() [vault/vault.go:121]
  → AES-GCM decrypt
  → os.Stdout.Write(val)

---

### FLUJO: vault-set — Guardar secreto cifrado

#### ESC-1: El sistema guarda credenciales SMTP

Se ejecuta `vault set --conv conv-123 --key credentials.smtp --stdin` con el JSON en stdin. El sistema cifra con AES-GCM y persiste en disco.

TRACE:
  → cmdSet(args) [vault/main.go:109]
  → io.ReadAll(os.Stdin) si --stdin
  → vault.Set("", conv, key, data) [vault/vault.go:145]
  → masterKey() → AES-GCM encrypt
  → os.WriteFile() [vault/vault.go:168]

---

### FLUJO: vault-delete — Eliminar secreto

#### ESC-1: Eliminacion de credencial

Se ejecuta `vault delete --conv conv-123 --key credentials.smtp`.

TRACE:
  → cmdDelete(args) [vault/main.go:164]
  → vault.Delete("", conv, key) [vault/vault.go:249]
  → os.Remove() [vault/vault.go:251]

---

### FLUJO: vault-genkey — Generar clave maestra

#### ESC-1: El admin genera una clave nueva

Se ejecuta `vault genkey`. El sistema genera 32 bytes aleatorios.

En terminal aparece: `REMORA_VAULT_KEY=<hex>`

TRACE:
  → cmdGenKey() [vault/main.go:177]
  → vault.GenerateKey() [vault/vault.go:259]
  → hex output

---

## SUPERFICIE: nativeagent — Agente LLM con herramientas bash

### FLUJO: agent-prompt — Ejecutar prompt con herramientas

#### ESC-1: El sistema invoca al agente con un prompt

Un framework_session o agentrpc invoca nativeagent.Prompt(prompt). El agente resuelve el provider LLM (MiniMax, Groq, OpenRouter), carga variables de entorno y archivos .env, y envia el request al LLM.

El LLM puede responder con texto directo o con tool calls. Si responde con un tool call, el agente lo parsea con toolCommandFromText o shellCommandFromTextResponse.

TRACE:
  → Prompt(prompt) [nativeagent/agent.go:210]
  → PromptWithImages(prompt, nil) [nativeagent/agent.go:214]
  → resolveProvider() [nativeagent/agent.go:113]
  → request() [nativeagent/agent.go:395]
  → requestMiniMax() o requestGroq() [nativeagent/agent.go:415,476]
  → respuesta del LLM

#### ESC-2: El agente ejecuta una tool bash

El LLM solicita ejecutar un comando bash. El agente valida la politica con validateBashPolicy, luego ejecuta via exec.Command("/bin/zsh", "-lc", command).

TRACE:
  → toolCommandFromText(response) [nativeagent/agent.go:564]
  → runTool("bash", input) [nativeagent/agent.go:956]
  → toolBash(command) [nativeagent/agent.go:1013]
  → validateBashPolicy(command) [nativeagent/agent.go:1031]
  → exec.Command("/bin/zsh", "-lc", command) [nativeagent/agent.go:1021]
  → output capturado

#### ESC-3: El agente completa con handoff terminal

Si el agente detecta que puede completar la tarea directamente con un comando, usa successfulTerminalHandoff que combina el resultado del bash con una respuesta final.

TRACE:
  → successfulTerminalHandoff() [nativeagent/agent.go:657]
  → bashCommandFromInput() [nativeagent/agent.go:668]
  → resultado final

---

## SUPERFICIE: framework-session — Gestor de sesiones de framework

### FLUJO: session-start — Iniciar sesion de framework

#### ESC-1: El sistema inicia una sesion nueva

api_rest invoca framework_session con el subcomando "start". El sistema crea el workspace, inicializa el toolRunner y ejecuta runAgentLoop con el prompt inicial.

El framework_session emite eventos via liveEventWriter (SSE) para que el frontend pueda mostrar progreso en tiempo real.

TRACE:
  → runStart(args) [framework_session/main.go:93]
  → ensureWorkspace() [framework_session/main.go:315]
  → newToolRunner() [framework_session/main.go:326]
  → injectFrameworkCommandContext() [framework_session/main.go:750]
  → normalizeCommandAndArgs() [framework_session/main.go:800]
  → runAgentLoop(ctx, client, runner, live, framework, spec, prompt, user) [framework_session/main.go:332]
  → llm.Stream() → tool decisions → execute → streamFinal

---

### FLUJO: session-message — Continuar sesion con mensaje

#### ESC-1: El usuario envia mensaje en sesion activa

api_rest invoca framework_session con subcomando "message" y el texto del usuario. El sistema carga la sesion existente y ejecuta runAgentLoop con el nuevo mensaje.

El agente puede hacer multiples tool calls (parseToolDecisions) antes de dar la respuesta final (streamFinal).

TRACE:
  → runMessage(args) [framework_session/main.go:172]
  → runAgentLoop(ctx, ..., user_message) [framework_session/main.go:332]
  → parseToolDecisions(out) [framework_session/main.go:424]
  → execute() para cada tool [framework_session/main.go:611]
  → streamFinal() [framework_session/main.go:393]
  → respuesta final al usuario

---

### FLUJO: session-configuration — Proponer configuracion

#### ESC-1: El framework propone una configuracion

Durante el agent loop, el framework puede proponer una configuracion (credentials, settings) via proposeConfiguration y commitConfiguration.

TRACE:
  → proposeConfiguration() [framework_session/main.go:671]
  → commitConfiguration() [framework_session/main.go:690]
  → vault set (si hay credenciales)

---

## SUPERFICIE: agentrpc — Servidor RPC para agentes externos

### FLUJO: agentrpc-handle — Procesar request de agente externo

#### ESC-1: Un agente externo envia request

Un servicio externo envia un HTTP request al servidor agentrpc. El handler procesa el request, determina las tools permitidas con allowedTools, e invoca nativeagent.Prompt.

TRACE:
  → main() [agentrpc/main.go:30]
  → handle() [agentrpc/main.go:54]
  → allowedTools() [agentrpc/main.go:84]
  → nativeagent.Prompt()
  → write(response) [agentrpc/main.go:93]

---

## SUPERFICIE: framework-alfa — Compilador semantico Echo a Bravo

### FLUJO: ejecutar-alfa-compile — Compilar arbol Echo a spec

#### ESC-1: El sistema invoca frameworkalfa compile

El sistema ejecuta `frameworkalfa compile --echo-tree frameworkecho.json --out alfa_spec.json --allow-draft=true`. El framework lee el arbol Echo (formato echo.tree.v1) que contiene AXIOM, THEORY, TASK, PAIN, OPPORTUNITY.

El framework usa el LLM (groq/llama-4-scout) para compilar el arbol en una alfa_spec.json que contiene la spec compilada con linaje OPPORTUNITY->PAIN->TASK->THEORY->AXIOM y open_questions.

Contrato:
- Input: echo_tree (echo.tree.v1) — arbol Echo validado
- Output: alfa_spec (alfa.spec.v1) — spec compilada
- Output: ideal_flow (alfa.ideal_flow.v1) — flujo ideal para Bravo

TRACE:
  → exec.Command("frameworkalfa", "compile", ...) [orchestrator o api_rest]
  → main() [framework-alfa/cmd/frameworkalfa/main.go]
  → compile command
  → alfa_spec.json escrito en framework-alfa/temp/

---

### FLUJO: ejecutar-alfa-inspect — Inspeccionar spec compilada

#### ESC-1: Inspeccion de la spec

El sistema ejecuta `frameworkalfa inspect --spec alfa_spec.json`. Muestra el contenido de la spec compilada.

TRACE:
  → exec.Command("frameworkalfa", "inspect", ...)
  → inspect command
  → output en stdout

---

### FLUJO: ejecutar-alfa-export-bravo — Exportar flujo ideal para Bravo

#### ESC-1: Generacion de flujo ideal

El sistema ejecuta `frameworkalfa export-bravo --spec alfa_spec.json --out ideal_flow.json --allow-draft=true`. Transforma la spec en un flujo ideal consumible por framework-bravo.

TRACE:
  → exec.Command("frameworkalfa", "export-bravo", ...)
  → export-bravo command
  → ideal_flow.json escrito

---

### FLUJO: ejecutar-alfa-questions — Ciclo de preguntas

#### ESC-1: Obtener siguiente pregunta

`frameworkalfa next-question --spec alfa_spec.json --echo-tree frameworkecho.json`

#### ESC-2: Ingerir respuesta

`frameworkalfa ingest-answer --spec alfa_spec.json --question-id Q1 --answer "..."`

TRACE:
  → next-question / ingest-answer commands
  → open_questions[] actualizado en spec

---

## SUPERFICIE: framework-echo — Guia de reuniones de descubrimiento

### FLUJO: ejecutar-echo-init — Inicializar proyecto de descubrimiento

#### ESC-1: El sistema inicia una sesion Echo

El sistema ejecuta `frameworkecho init --project-id proj-1 --client "Empresa X" --date 2026-05-17`. Echo inicializa el arbol de conocimiento vacio.

Contrato:
- Input: ninguno
- Output: tree (echo.tree.v1) — arbol de conocimiento progresivo

TRACE:
  → exec.Command("frameworkecho", "init", ...)
  → init command
  → frameworkecho.json creado

---

### FLUJO: ejecutar-echo-discovery — Agregar conocimiento al arbol

#### ESC-1: Agregar axioma

`frameworkecho add-axiom --title "CRM manual" --evidence "El equipo usa Excel"`

#### ESC-2: Agregar teoria

`frameworkecho add-theory --parent AX-1 --title "Perdida de leads" --evidence "No hay seguimiento"`

#### ESC-3: Agregar tarea

`frameworkecho add-task --parent TH-1 --title "Enviar recordatorio" --evidence "Tarea diaria manual"`

#### ESC-4: Agregar dolor

`frameworkecho add-pain --parent TK-1 --title "Se olvidan leads" --evidence "3 de 10 no se contactan"`

#### ESC-5: Agregar oportunidad

`frameworkecho add-opportunity --parent PN-1 --title "Automatizar seguimiento" --evidence "ROI estimado 40%"`

TRACE:
  → exec.Command("frameworkecho", "add-{type}", ...)
  → arbol actualizado en frameworkecho.json

---

### FLUJO: ejecutar-echo-validate — Validar arbol

#### ESC-1: Validacion del arbol

`frameworkecho validate` — verifica integridad del arbol.

TRACE:
  → validate command
  → resultado de validacion

---

### FLUJO: ejecutar-echo-show-tree — Mostrar arbol completo

#### ESC-1: Visualizacion del arbol

`frameworkecho show-tree` — imprime el arbol con jerarquia AXIOM->THEORY->TASK->PAIN->OPPORTUNITY.

TRACE:
  → show-tree command
  → arbol formateado en stdout

---

## SUPERFICIE: framework-auditor — Auditor proactivo de ERP

### FLUJO: ejecutar-auditor-scan — Escanear datos buscando errores

#### ESC-1: El sistema ejecuta un scan completo

El sistema ejecuta `frameworkauditor scan --source data/dataset.working.json --json`. El auditor lee el dataset (formato external.api.dump.v1), corre checks deterministicos de calidad de datos y emite findings.

Contrato:
- Input: dataset (external.api.dump.v1) — dump canonico del negocio
- Output: findings (auditor.findings.v1) — lista de hallazgos
- Output: data_gaps (data.gaps.v1) — brechas de datos

Los checks incluyen: integridad referencial, datos inconsistentes, valores invalidos, contactos faltantes.

TRACE:
  → exec.Command("frameworkauditor", "scan", ...)
  → scan command
  → data/findings.json escrito

---

### FLUJO: ejecutar-auditor-list — Listar findings

#### ESC-1: Ver lista de hallazgos

`frameworkauditor list` — imprime la lista de findings en texto plano.

TRACE:
  → list command
  → findings listados

---

### FLUJO: ejecutar-auditor-detail — Ver detalle de finding

#### ESC-1: Consultar un finding especifico

`frameworkauditor detail --id F-001`

TRACE:
  → detail command
  → detalle del finding

---

### FLUJO: ejecutar-auditor-reset — Resetear datos

#### ESC-1: Restaurar dataset golden

`frameworkauditor reset` — restaura dataset.working.json desde dataset.golden.json y limpia findings.

TRACE:
  → reset command
  → dataset restaurado, findings limpiados

---

### FLUJO: ejecutar-auditor-questions — Ciclo de preguntas

#### ESC-1: Obtener siguiente pregunta del auditor

`frameworkauditor next-question` — el auditor habla primero reportando hallazgos.

#### ESC-2: Ingerir respuesta del usuario

`frameworkauditor ingest-answer --question-id Q1 --answer "..."`

TRACE:
  → next-question / ingest-answer
  → flujo conversacional avanzado

---

## SUPERFICIE: framework-sabio — Experto en datos indexados

### FLUJO: ejecutar-sabio-query — Consultar datos via SQLite

#### ESC-1: El sistema consulta datos indexados

El sistema ejecuta `frameworksabio query --query "cuantos morosos hay"`. El framework traduce la pregunta en lenguaje natural a SQL contra la base SQLite de datos.

Contrato:
- Input: data_sqlite_db (data.sqlite_db.v1) — base SQLite
- Output: respuesta con citas a fuentes reales

TRACE:
  → exec.Command("frameworksabio", "query", ...)
  → query command
  → SQL generado y ejecutado
  → respuesta con datos

---

### FLUJO: ejecutar-sabio-explain — Explicar capacidades

#### ESC-1: El usuario pregunta que puede hacer Sabio

`frameworksabio explain-capabilities`

TRACE:
  → explain-capabilities command
  → listado de capacidades

---

### FLUJO: ejecutar-sabio-inspect-source — Inspeccionar fuente

#### ESC-1: Inspeccion de fuente de datos

`frameworksabio inspect-source`

TRACE:
  → inspect-source command
  → info de la fuente

---

### FLUJO: ejecutar-sabio-validate-business — Validar config de negocio

#### ESC-1: Validacion de configuracion

`frameworksabio validate-business-config`

TRACE:
  → validate-business-config command
  → resultado de validacion

---

### FLUJO: ejecutar-sabio-contact-lookup — Buscar contacto

#### ESC-1: Busqueda de contacto en datos indexados

`frameworksabio contact-lookup --query "Juan Perez"`

TRACE:
  → contact-lookup command
  → resultados de busqueda

---

### FLUJO: ejecutar-sabio-reset — Reset de estado

#### ESC-1: Reset

`frameworksabio reset`

TRACE:
  → reset command
  → estado reseteado

---

### FLUJO: ejecutar-sabio-questions — Ciclo conversacional

#### ESC-1: Siguiente pregunta

`frameworksabio next-question`

#### ESC-2: Ingerir respuesta

`frameworksabio ingest-answer --question-id Q1 --answer "..."`

TRACE:
  → next-question / ingest-answer commands

---

## SUPERFICIE: framework-mecanico — Reparador de datos

### FLUJO: ejecutar-mecanico-propose — Proponer remediaciones

#### ESC-1: El sistema propone fixes basado en findings

El sistema ejecuta `frameworkmecanico propose`. El framework lee los findings del auditor y el dataset, genera propuestas de remediacion.

Contrato:
- Input: findings (auditor.findings.v1)
- Input: dataset (external.api.dump.v1)
- Output: proposals (mecanico.proposals.v1)
- Output: applied_log (mecanico.applied.v1)

TRACE:
  → exec.Command("frameworkmecanico", "propose", ...)
  → propose command
  → proposals generadas

---

### FLUJO: ejecutar-mecanico-propose-all-auto — Proponer todo automatico

#### ESC-1: Propuestas automaticas para todos los findings

`frameworkmecanico propose-all-auto`

TRACE:
  → propose-all-auto command
  → todas las propuestas generadas

---

### FLUJO: ejecutar-mecanico-apply — Aplicar remediacion

#### ESC-1: El usuario confirma y aplica un fix

`frameworkmecanico apply --proposal-id P-001`

TRACE:
  → apply command
  → fix aplicado al dataset
  → applied_log actualizado

---

### FLUJO: ejecutar-mecanico-apply-all — Aplicar todos los fixes

#### ESC-1: Aplicacion masiva

`frameworkmecanico apply-all`

TRACE:
  → apply-all command
  → todos los fixes aplicados

---

### FLUJO: ejecutar-mecanico-list-proposals — Listar propuestas

#### ESC-1: Ver propuestas pendientes

`frameworkmecanico list-proposals`

TRACE:
  → list-proposals command
  → propuestas listadas

---

### FLUJO: ejecutar-mecanico-reset — Resetear propuestas

#### ESC-1: Limpiar propuestas

`frameworkmecanico reset`

TRACE:
  → reset command
  → propuestas limpiadas

---

### FLUJO: ejecutar-mecanico-questions — Ciclo conversacional

#### ESC-1: Siguiente pregunta

`frameworkmecanico next-question`

#### ESC-2: Ingerir respuesta

`frameworkmecanico ingest-answer --question-id Q1 --answer "..."`

TRACE:
  → next-question / ingest-answer

---

## SUPERFICIE: framework-bravo — Verificador de flujos ideales

### FLUJO: ejecutar-bravo-verify — Verificar flujo ideal vs traza real

#### ESC-1: El sistema compara flujo ideal contra traza

El sistema ejecuta framework-bravo para comparar un IdealFlow declarado (alfa.ideal_flow.v1) contra un Trace real. Detecta desvios, pasos faltantes o reordenados.

Contrato:
- Input: ideal_flow + trace
- Output: verification_report

TRACE:
  → exec.Command("frameworkbravo", ...)
  → comparacion ideal vs real
  → reporte de verificacion

---

## SUPERFICIE: framework-charlie — Gestion de ciclo de vida Git

### FLUJO: ejecutar-charlie-doctor — Diagnostico de salud del repo

#### ESC-1: El sistema ejecuta diagnostico

`charlie doctor` — verifica estado del repo: commits pendientes, ramas huerfanas, archivos sin trackear.

TRACE:
  → exec.Command("charlie", "doctor")
  → diagnostico ejecutado
  → reporte en stdout

---

### FLUJO: ejecutar-charlie-plan — Generar plan de release

#### ESC-1: Plan de release

`charlie plan` — genera plan de release con cambios desde ultimo tag.

TRACE:
  → exec.Command("charlie", "plan")
  → plan generado

---

### FLUJO: ejecutar-charlie-preflight — Checks pre-publicacion

#### ESC-1: Preflight checks

`charlie preflight` — verifica que todo este listo para publicar.

TRACE:
  → exec.Command("charlie", "preflight")
  → checks ejecutados

---

### FLUJO: ejecutar-charlie-status — Estado del repo

#### ESC-1: Estado actual

`charlie status`

TRACE:
  → exec.Command("charlie", "status")
  → estado reportado

---

### FLUJO: ejecutar-charlie-changelog — Generar changelog

#### ESC-1: Changelog

`charlie changelog`

TRACE:
  → exec.Command("charlie", "changelog")
  → changelog generado

---

### FLUJO: ejecutar-charlie-backup — Backup del estado

#### ESC-1: Backup

`charlie backup`

TRACE:
  → exec.Command("charlie", "backup")
  → backup creado

---

## SUPERFICIE: framework-critico — Evaluador adversarial

### FLUJO: ejecutar-critico-evaluate — Evaluar propuesta de cambio

#### ESC-1: El sistema evalua una propuesta

El sistema ejecuta `frameworkcritico evaluate --proposal "refactorizar auth" --context repo_model.json --severity normal`. El framework adversarial analiza riesgos, contradicciones y asunciones no verificadas.

Contrato:
- Input: repo_model (arquitecto.model.v1) — opcional
- Output: evaluation (critico.eval.v1) — riesgos, contradicciones, asunciones

TRACE:
  → exec.Command("frameworkcritico", "evaluate", ...)
  → evaluacion adversarial
  → evaluation.json escrito

---

### FLUJO: ejecutar-critico-challenge — Cuestionar asuncion

#### ESC-1: Challenge especifico

`frameworkcritico challenge --assumption "esto es seguro" --evidence "no hay tests"`

TRACE:
  → challenge command
  → cuestionamiento generado

---

### FLUJO: ejecutar-critico-init — Inicializar sesion

#### ESC-1: Init sesion de evaluacion

`frameworkcritico init --session-id sess-1`

TRACE:
  → init command
  → sesion creada

---

### FLUJO: ejecutar-critico-questions — Ciclo conversacional

#### ESC-1: Preguntas del critico

`frameworkcritico next-question` / `ingest-answer`

TRACE:
  → next-question / ingest-answer

---

## SUPERFICIE: framework-arquitecto — Indexador de codebases Go

### FLUJO: ejecutar-arquitecto-init — Inicializar analisis

#### ESC-1: El sistema inicia analisis del repo

`frameworkarquitecto init --session-id sess-1 --repo-path /path/to/repo`

Contrato:
- Output: repo_model (arquitecto.model.v1) — modelo indexado del repo

TRACE:
  → exec.Command("frameworkarquitecto", "init", ...)
  → sesion de analisis creada

---

### FLUJO: ejecutar-arquitecto-index-repo — Indexar repo

#### ESC-1: Indexacion completa o delta

`frameworkarquitecto index-repo --scope delta`

TRACE:
  → index-repo command
  → repo_model.json actualizado

---

### FLUJO: ejecutar-arquitecto-query-structure — Consultar estructura

#### ESC-1: Query al modelo indexado

`frameworkarquitecto query-structure --query "interfaces de auth" --format json`

TRACE:
  → query-structure command
  → resultado de consulta

---

### FLUJO: ejecutar-arquitecto-trace-flow — Trazar flujo de ejecucion

#### ESC-1: Traza desde entrypoint

`frameworkarquitecto trace-flow --entrypoint handleFlowCreate --depth 5`

TRACE:
  → trace-flow command
  → trace report

---

### FLUJO: ejecutar-arquitecto-questions — Ciclo conversacional

#### ESC-1: Preguntas del arquitecto

`frameworkarquitecto next-question` / `ingest-answer`

TRACE:
  → next-question / ingest-answer

---

## SUPERFICIE: framework-deployer — Deploy a Cloud Run dev

### FLUJO: ejecutar-deployer-plan — Ver plan de deploy

#### ESC-1: El sistema muestra el plan sin ejecutar

`deployer plan` — muestra que se deployaria a flujo-api-dev sin ejecutar.

Contrato:
- Guards: forbidden_targets=[prod, flujo-api], no_git_writes=true
- Side effects: gcloud builds submit, gcloud run deploy (solo en apply)

TRACE:
  → exec.Command("deployer", "plan")
  → plan mostrado sin side effects

---

### FLUJO: ejecutar-deployer-apply — Deployar a dev

#### ESC-1: Deploy real a Cloud Run dev

`deployer --apply` — ejecuta gcloud builds submit + gcloud run deploy a flujo-api-dev.

TRACE:
  → exec.Command("deployer", "--apply")
  → gcloud builds submit
  → gcloud run deploy flujo-api-dev
  → deploy completado

---

## SUPERFICIE: framework-foco — Priorizador operativo

### FLUJO: ejecutar-foco-priorities — Consultar prioridades del dia

#### ESC-1: El sistema genera prioridades

`frameworkfoco priorities` — analiza deudores y genera lista de cobranza con scores.

TRACE:
  → exec.Command("frameworkfoco", "priorities")
  → prioridades generadas

---

### FLUJO: ejecutar-foco-next-task — Siguiente tarea

#### ESC-1: Obtener siguiente tarea

`frameworkfoco next-task`

TRACE:
  → next-task command
  → tarea siguiente

---

### FLUJO: ejecutar-foco-complete-cycle — Completar ciclo

#### ESC-1: Marcar ciclo completado

`frameworkfoco complete-cycle`

TRACE:
  → complete-cycle command
  → ciclo cerrado

---

### FLUJO: ejecutar-foco-session-start — Iniciar sesion Foco

#### ESC-1: Inicio de sesion diaria

`frameworkfoco session-start`

TRACE:
  → session-start command
  → sesion iniciada

---

### FLUJO: ejecutar-foco-questions — Ciclo conversacional

#### ESC-1: Preguntas / respuestas

`frameworkfoco next-question` / `ingest-answer` / `query`

TRACE:
  → next-question / ingest-answer / query

---

## SUPERFICIE: framework-mensajero — Envio de mensajes salientes

### FLUJO: ejecutar-mensajero-send — Enviar mensaje

#### ESC-1: El sistema envia un email/sms/whatsapp

El sistema ejecuta `frameworkmensajero send --type email --to user@example.com --subject "Recordatorio"`. El framework es agnostico al negocio; solo entrega el mensaje.

TRACE:
  → exec.Command(REMORA_MENSAJERO_BIN, "send", ...)
  → handleSendEmail() [api_rest/main.go:1254]
  → mensaje enviado

---

### FLUJO: ejecutar-mensajero-can-send — Verificar capacidad de envio

#### ESC-1: Check previo

`frameworkmensajero can-send --type email`

TRACE:
  → can-send command
  → true/false

---

### FLUJO: ejecutar-mensajero-questions — Ciclo conversacional

#### ESC-1: Preguntas / respuestas

`frameworkmensajero next-question` / `ingest-answer`

TRACE:
  → next-question / ingest-answer

---

## SUPERFICIE: framework-gmail — Gestor de emails Gmail

### FLUJO: ejecutar-gmail-send-email — Enviar email via Gmail

#### ESC-1: Envio de email

`frameworkgmail send-email` — prepara y envia email.

TRACE:
  → send-email command
  → email enviado

---

### FLUJO: ejecutar-gmail-get-unread — Obtener emails no leidos

#### ESC-1: Lectura de inbox

`frameworkgmail get-unread-emails`

TRACE:
  → get-unread-emails command
  → lista de emails

---

### FLUJO: ejecutar-gmail-search — Buscar emails

#### ESC-1: Busqueda

`frameworkgmail search-emails`

TRACE:
  → search-emails command
  → resultados

---

### FLUJO: ejecutar-gmail-labels — Listar labels

#### ESC-1: Labels

`frameworkgmail list-labels`

TRACE:
  → list-labels command
  → labels listados

---

### FLUJO: ejecutar-gmail-drafts — Listar borradores

#### ESC-1: Borradores

`frameworkgmail list-drafts`

TRACE:
  → list-drafts command
  → borradores listados

---

### FLUJO: ejecutar-gmail-create-draft — Crear borrador

#### ESC-1: Nuevo borrador

`frameworkgmail create-draft`

TRACE:
  → create-draft command
  → borrador creado

---

## SUPERFICIE: framework-hosting — Conector de panel hosting cPanel

### FLUJO: ejecutar-hosting-connect — Conectar panel hosting

#### ESC-1: Conexion a cPanel

`frameworkhosting connect` — conecta al panel UAPI de cPanel.

TRACE:
  → connect command
  → conexion establecida

---

### FLUJO: ejecutar-hosting-list-emails — Listar emails del hosting

#### ESC-1: Emails del panel

`frameworkhosting list-emails`

TRACE:
  → list-emails command
  → emails listados

---

### FLUJO: ejecutar-hosting-provision-smtp — Provisionar SMTP

#### ESC-1: Setup SMTP desde hosting

`frameworkhosting provision-smtp`

TRACE:
  → provision-smtp command
  → SMTP provisionado

---

### FLUJO: ejecutar-hosting-import-smtp — Importar config SMTP

#### ESC-1: Importar SMTP al vault

`frameworkhosting import-smtp`

TRACE:
  → import-smtp command
  → config importada al vault

---

### FLUJO: ejecutar-hosting-verify-smtp — Verificar SMTP

#### ESC-1: Verificacion

`frameworkhosting verify-smtp`

TRACE:
  → verify-smtp command
  → verificacion completada

---

### FLUJO: ejecutar-hosting-questions — Ciclo conversacional

#### ESC-1: Preguntas / respuestas

`frameworkhosting next-question` / `ingest-answer`

TRACE:
  → next-question / ingest-answer

---

## SUPERFICIE: framework-indexa — Ingesta de datos y embeddings

### FLUJO: ejecutar-indexa-init — Inicializar fuente de datos

#### ESC-1: Init de fuente

`frameworkindexa init` — inicializa la configuracion de fuente.

Contrato:
- Input: source_json (external.api.dump.v1)
- Output: vector_store (indexa.store.v1)

TRACE:
  → init command
  → fuente configurada

---

### FLUJO: ejecutar-indexa-index — Indexar datos

#### ESC-1: Ingesta y generacion de embeddings

`frameworkindexa index` — ingiere data desde API externa, genera embeddings y persiste en vector store.

TRACE:
  → index command
  → datos indexados en vector store

---

### FLUJO: ejecutar-indexa-status — Ver estado de indexacion

#### ESC-1: Estado

`frameworkindexa status`

TRACE:
  → status command
  → estado reportado

---

### FLUJO: ejecutar-indexa-api-plan — Plan de conexion API

#### ESC-1: Planificar conexion

`frameworkindexa api-plan`

TRACE:
  → api-plan command
  → plan generado

---

## SUPERFICIE: framework-excel — Conector de archivos Excel

### FLUJO: ejecutar-excel — Operar archivos xlsx

#### ESC-1: El sistema opera un archivo Excel

El framework lee, escribe y manipula archivos .xlsx. Se ejecuta en modo async_trigger.

TRACE:
  → exec.Command("frameworkexcel", ...)
  → operacion sobre .xlsx

---

## SUPERFICIE: framework-pingpong — Tutor 80/20 de codigo

### FLUJO: ejecutar-pingpong-init — Inicializar ejercicio

#### ESC-1: El sistema inicia un ejercicio

`frameworkpingpong init` — inicializa un nuevo ejercicio de codigo.

TRACE:
  → init command
  → ejercicio creado

---

### FLUJO: ejecutar-pingpong-next — Siguiente paso del ejercicio

#### ESC-1: Avanzar al siguiente paso

`frameworkpingpong next`

TRACE:
  → next command
  → siguiente paso presentado

---

### FLUJO: ejecutar-pingpong-review — Revisar solucion

#### ESC-1: El tutor revisa la solucion

`frameworkpingpong review`

TRACE:
  → review command
  → feedback generado

---

### FLUJO: ejecutar-pingpong-check — Verificar completitud

#### ESC-1: Check de avance

`frameworkpingpong check`

TRACE:
  → check command
  → estado verificado

---

### FLUJO: ejecutar-pingpong-accept — Aceptar solucion

#### ESC-1: Aceptar y avanzar

`frameworkpingpong accept`

TRACE:
  → accept command
  → paso completado

---

### FLUJO: ejecutar-pingpong-verify — Verificar ejercicio

#### ESC-1: Verificacion final

`frameworkpingpong verify`

TRACE:
  → verify command
  → verificacion

---

### FLUJO: ejecutar-pingpong-peek — Ver pista

#### ESC-1: Pista sin resolver

`frameworkpingpong peek`

TRACE:
  → peek command
  → pista mostrada

---

### FLUJO: ejecutar-pingpong-clean — Limpiar ejercicio

#### ESC-1: Reset

`frameworkpingpong clean`

TRACE:
  → clean command
  → ejercicio limpiado

---

## SUPERFICIE: framework-quine — Generador de frameworks

### FLUJO: ejecutar-quine-create — Crear nuevo framework

#### ESC-1: El sistema genera un framework nuevo

`frameworkquine create` — genera un framework completo desde spec.

TRACE:
  → create command
  → framework generado

---

### FLUJO: ejecutar-quine-review — Revisar framework existente

#### ESC-1: Review de framework

`frameworkquine review`

TRACE:
  → review command
  → feedback generado

---

### FLUJO: ejecutar-quine-list — Listar frameworks generados

#### ESC-1: Listado

`frameworkquine list`

TRACE:
  → list command
  → frameworks listados

---

### FLUJO: ejecutar-quine-spec — Ver spec

#### ESC-1: Spec del framework

`frameworkquine spec`

TRACE:
  → spec command
  → spec mostrada

---

### FLUJO: ejecutar-quine-fix — Corregir framework

#### ESC-1: Fix automatico

`frameworkquine fix`

TRACE:
  → fix command
  → correcciones aplicadas

---

## SUPERFICIE: framework-radar — Radar analitico data-aware

### FLUJO: ejecutar-radar-prioritize — Priorizar entidades

#### ESC-1: El sistema prioriza entidades con scoring

`frameworkradar prioritize` — genera scoring y prioridad de analisis.

TRACE:
  → exec.Command("frameworkradar", "prioritize")
  → scoring generado

---

### FLUJO: ejecutar-radar-configure-analysis — Configurar analisis

#### ESC-1: Configuracion del esquema

`frameworkradar configure-analysis`

TRACE:
  → configure-analysis command
  → esquema configurado

---

### FLUJO: ejecutar-radar-deep-dive — Analisis profundo

#### ESC-1: Deep dive en entidad

`frameworkradar deep-dive`

TRACE:
  → deep-dive command
  → analisis detallado

---

### FLUJO: ejecutar-radar-analyze-followup — Followup de analisis

#### ESC-1: Continuacion de analisis

`frameworkradar analyze-followup`

TRACE:
  → analyze-followup command
  → seguimiento

---

## SUPERFICIE: framework-paladin — Tracing semantico para Go

### FLUJO: ejecutar-paladin-audit — Auditar repo

#### ESC-1: El sistema audita el repo

`frameworkpaladin audit` — tracing semantico del repo Go.

Contrato:
- Output: audit_report (paladin.audit.v1)

TRACE:
  → exec.Command("frameworkpaladin", "audit")
  → audit_report generado

---

### FLUJO: ejecutar-paladin-explain — Explicar trace

#### ESC-1: Explicacion de un trace

`frameworkpaladin explain`

TRACE:
  → explain command
  → explicacion generada

---

### FLUJO: ejecutar-paladin-status — Estado

#### ESC-1: Estado del audit

`frameworkpaladin status`

TRACE:
  → status command

---

### FLUJO: ejecutar-paladin-readiness — Readiness check

#### ESC-1: Check de readiness

`frameworkpaladin readiness`

TRACE:
  → readiness command
  → readiness verificado

---

## SUPERFICIE: framework-tareas — Task ledger por perfil

### FLUJO: ejecutar-tareas-list — Listar tareas

#### ESC-1: Listado de tareas activas

`frameworktareas list`

TRACE:
  → list command
  → tareas listadas

---

### FLUJO: ejecutar-tareas-next — Siguiente tarea

#### ESC-1: Tarea pendiente mas prioritaria

`frameworktareas next`

TRACE:
  → next command
  → siguiente tarea

---

### FLUJO: ejecutar-tareas-create — Crear tarea

#### ESC-1: Nueva tarea

`frameworktareas create`

TRACE:
  → create command
  → tarea creada

---

### FLUJO: ejecutar-tareas-complete — Completar tarea

#### ESC-1: Marcar completada

`frameworktareas complete`

TRACE:
  → complete command
  → tarea completada

---

### FLUJO: ejecutar-tareas-event — Registrar evento

#### ESC-1: Evento en tarea

`frameworktareas event`

TRACE:
  → event command
  → evento registrado

---

### FLUJO: ejecutar-tareas-seed-from-foco — Seed desde Foco

#### ESC-1: Importar tareas desde Foco

`frameworktareas seed-from-foco`

TRACE:
  → seed-from-foco command
  → tareas importadas

---

### FLUJO: ejecutar-tareas-questions — Ciclo conversacional

#### ESC-1: Preguntas / respuestas

`frameworktareas next-question` / `ingest-answer`

TRACE:
  → next-question / ingest-answer

---

## MECANISMO: run-flow-manifest — Orquesta la ejecucion completa de un flujo declarativo multi-nodo con ciclos

Disparado por: FLUJO: flow-run, SUPERFICIE: api-rest

### PIPELINE: flow-execution-loop — Ejecuta nodos en orden topologico con resoluciones JIT

#### PASO-1: Carga y compilacion del manifest

El motor recibe un flowRunRequest con el manifest del flujo o un compiled_id previo. Si existe compiled_id, carga el record compilado y extrae el manifest; si no existe, invoca compileAndPersistFlowManifest para generar la compilacion desde cero. El motor asigna un runID unico y resuelve los businessArtifacts del negocio asociado.

INVARIANTES:
  - El flujo SIEMPRE se compila antes de ejecutar; si no existe compiled previo, se genera uno nuevo
  - Si compiled_id proporcionado, DEBE pertenecer al mismo business_id del request

TRACE:
  -> runFlowManifest() [cmd/api_rest/flow_runner.go:12]
  -> loadCompiledRecord() [cmd/api_rest/flow_runner.go:20]
  -> compileAndPersistFlowManifest() [cmd/api_rest/flow_runner.go:40]

#### PASO-2: Validacion y calculo del orden de ejecucion

El motor valida el manifest contra los artefactos disponibles invocando validateFlowManifestWithArtifacts. Si la validacion falla (Valid=false), el flujo se marca como "invalid" y se persiste inmediatamente sin ejecutar ningun nodo. Si la validacion pasa, el motor calcula el orden topologico de nodos con flowExecutionOrder.

INVARIANTES:
  - Validacion invalida aborta inmediatamente (status=invalid, sin ejecucion de nodos)
  - El orden topologico respeta las dependencias declaradas entre nodos

TRACE:
  -> validateFlowManifestWithArtifacts() [cmd/api_rest/flow_runner.go:49]
  -> flowExecutionOrder() [cmd/api_rest/flow_runner.go:50]

#### PASO-3: Inicializacion de artefactos y segmento semantico

El motor inicializa el mapa de artefactos disponibles con: artefactos de sistema (systemFlowArtifacts), artefactos del negocio, provided artifacts del manifest, e initial artifacts del request. Si el flujo declara un intent, se inyecta flow.intent.v1. Finalmente, se inicializa el segmento semantico y se inyecta el nodo owner si aplica.

INVARIANTES:
  - Los artefactos de sistema se inyectan ANTES de iterar nodos; ningun nodo puede alterar este conjunto base
  - flow.intent.v1 se genera SOLO si el manifest declara un intent explicito

TRACE:
  -> systemFlowArtifacts() [cmd/api_rest/flow_runner.go:80]
  -> ensureSemanticSegmentInitialized() [cmd/api_rest/flow_runner.go:105]
  -> injectSemanticSegmentOwnerNode() [cmd/api_rest/flow_runner.go:107]

#### PASO-4: Iteracion de nodos con resoluciones just-in-time

El motor itera cada nodo en el orden calculado. Para cada nodo: resuelve el contrato (resolveFlowNodeContract), verifica artefactos faltantes, y si faltan, intenta resolverlos via ensureDataPipeline o resolveMissingFlowArtifacts. Si el nodo requiere preflight audit (por politica o por tener side-effects), ejecuta ensureFlowPreflightAudit antes de continuar. Si hay artefactos que no se pudieron resolver, el flujo se detiene con status needs_input.

INVARIANTES:
  - Un nodo NUNCA se ejecuta si tiene artefactos requeridos faltantes sin resolucion posible
  - La iteracion es breakable: cualquier fallo, needs_input o needs_approval DETIENE la iteracion
  - Bootstrap nodes se saltan en ciclos subsiguientes (cyclesDone > 0)

TRACE:
  -> resolveFlowNodeContract() [cmd/api_rest/flow_runner.go:191]
  -> ensureDataPipeline() [cmd/api_rest/flow_runner.go:251]
  -> resolveMissingFlowArtifacts() [cmd/api_rest/flow_runner.go:269]
  -> ensureFlowPreflightAudit() [cmd/api_rest/flow_runner.go:222]

#### PASO-5: Ejecucion del nodo y registro de artefactos

Si el nodo pasa todas las validaciones, el motor verifica si requiere aprobacion runtime (nodeRequiresRuntimeApproval). Si la requiere, se detiene con status needs_approval. Si no, ejecuta el nodo via executeFlowNode y parsea la respuesta. Si la ejecucion falla (exit code != 0), se marca failed y se detiene la iteracion. Si completa, se registran artefactos producidos con recordNodeArtifacts y se gestionan delegaciones y retornos semanticos.

INVARIANTES:
  - La aprobacion runtime es un gate que SIEMPRE detiene la ejecucion antes de side-effects no autorizados
  - Exit code != 0 SIEMPRE produce status=failed y rompe la iteracion

TRACE:
  -> nodeRequiresRuntimeApproval() [cmd/api_rest/flow_runner.go:304]
  -> executeFlowNode() [cmd/api_rest/flow_runner.go:314]
  -> recordNodeArtifacts() [cmd/api_rest/flow_runner.go:345]

#### PASO-6: Gestion de ciclos y finalizacion

Si un nodo es cycle-terminal (isCycleTerminalStep), el motor registra el ciclo completado e incrementa cyclesDone. Si maxCycles permite mas ciclos, resetea artefactos de ciclo con resetCycleArtifacts y salta a cycleStart via goto. Al finalizar todos los nodos: normaliza la timeline, persiste el run, y registra readiness si el flujo completo con exito.

INVARIANTES:
  - Ciclos se resetean con resetCycleArtifacts; artefactos de bootstrap/infra persisten entre ciclos
  - El flujo SIEMPRE se persiste al finalizar (persistFlowRun) independientemente del status final
  - maxCycles <= 0 se normaliza a 1 (siempre al menos un ciclo)

TRACE:
  -> isCycleTerminalStep() [cmd/api_rest/flow_runner.go:426]
  -> resetCycleArtifacts() [cmd/api_rest/flow_runner.go:503]
  -> normalizeFlowTimelineSegments() [cmd/api_rest/flow_runner.go:507]
  -> persistFlowRun() [cmd/api_rest/flow_runner.go:515]

---

## MECANISMO: run-loop — Orquesta el ciclo de pregunta-respuesta multi-framework conversacional

Disparado por: FLUJO: conversacion-chat, SUPERFICIE: api-rest

### PIPELINE: conversational-routing — Decide que framework habla y entrega respuestas del usuario

#### PASO-1: Deteccion de sesion analitica activa

El orquestador carga la cola de preguntas (loadQueue) y los drivers activos (driversFor). Si existe un segmento de sesion analitica activo para el negocio de la conversacion, clasifica la intencion del usuario con classifySegmentIntent. Si la intencion es "continue", ejecuta executeSessionFollowup y retorna directamente sin pasar por el pipeline normal. Si es "exit", concluye la sesion. Si es "operational", genera un handoff artifact para Foco.

INVARIANTES:
  - Las sesiones activas tienen prioridad ABSOLUTA sobre el pipeline normal de drivers
  - El claim de sesion para una conversacion es atomico (una vez asignada, no se reasigna)
  - Un solo framework queda elegido como proximo speaker por ciclo (nunca responden dos simultaneamente)

TRACE:
  -> loadQueue() [cmd/api_rest/orchestrator.go:56]
  -> driversFor() [cmd/api_rest/orchestrator.go:65]
  -> loadActiveSessionFromDisk() [cmd/api_rest/orchestrator.go:82]
  -> classifySegmentIntent() [cmd/api_rest/orchestrator.go:96]
  -> executeSessionFollowup() [cmd/api_rest/orchestrator.go:145]

#### PASO-2: Clasificacion de intent y evaluacion de reglas

El orquestador clasifica la intencion del usuario contra los intent_examples de todos los manifests activos (classifyIntent). Si hay match, reordena los drivers para que el framework matcheado hable primero. Luego evalua las reglas de composicion declarativas de flow.rules.json que pueden reordenar drivers adicionales, activar preprocesamiento vision, o delegar a un provider por capability.

INVARIANTES:
  - La clasificacion de intent (capability-based) PRECEDE a las reglas declarativas
  - Vision preprocessing se aplica SOLO si la regla lo pide Y hay imagenes en resources
  - La whitelist de allowed_delegates del session owner NUNCA se puede saltar

TRACE:
  -> classifyIntent() [cmd/api_rest/orchestrator.go:165]
  -> reorderDrivers() [cmd/api_rest/orchestrator.go:169]
  -> rules.Match() [cmd/api_rest/orchestrator.go:190]

#### PASO-3: Entrega de respuesta e ingestion

Si hay una respuesta del usuario (userAnswer no vacio), el orquestador enriquece la respuesta con contexto operacional y de tarea activa. Si hay una pregunta pendiente en la cola, la marca como respondida y entrega la respuesta al driver dueno via IngestAnswer. Si no hay pregunta pendiente, entrega al primer driver del orden actual.

INVARIANTES:
  - El enriquecimiento con contexto de tarea NUNCA contiene newlines (Channel rechaza unsafe args)
  - Si vision preprocessing falla, se continua con texto plano sin abortar

TRACE:
  -> queue.GetNextPending() [cmd/api_rest/orchestrator.go:270]
  -> queue.MarkAnswered() [cmd/api_rest/orchestrator.go:271]
  -> d.IngestAnswer() [cmd/api_rest/orchestrator.go:286]

#### PASO-4: Polling de la proxima pregunta

El orquestador pollea cada driver en orden por la proxima pregunta usando PollQuestionFull (o PollQuestion como fallback). El primer driver que devuelve una pregunta queda elegido como speaker. Se registra la pregunta en la cola y se retorna al cliente.

INVARIANTES:
  - Solo el primer driver con pregunta lista es elegido; los demas se ignoran en este ciclo
  - La pregunta queda registrada en queue con speaker asignado antes de retornar

TRACE:
  -> PollQuestionFull() [cmd/api_rest/orchestrator.go:331]
  -> queue.AddQuestionWithReasoning() [cmd/api_rest/orchestrator.go:349]
  -> saveQueue() [cmd/api_rest/orchestrator.go:313]

---

## MECANISMO: resolve-flow-gaps-iteratively — Resuelve brechas de datos iterativamente post-auditoria

Disparado por: FLUJO: flow-run, SUPERFICIE: api-rest

### PIPELINE: gap-resolution-loop — Itera passes de resolucion con fallback escalonado

#### PASO-1: Carga de scope y calculo de requisitos

El motor carga las tablas de scope del negocio (loadBusinessScopeTables) y el semantic pack path. Calcula los requisitos terminales del flujo (flowTerminalRequirementsForArtifacts) y los campos requeridos (flowRequiredDataFields). Estos determinan que gaps son relevantes para este flujo.

INVARIANTES:
  - Si no hay requisitos terminales NI campos requeridos, gaps se anulan (gaps = nil)
  - El scope del negocio filtra gaps que no pertenecen a las tablas del negocio actual

TRACE:
  -> loadBusinessScopeTables() [cmd/api_rest/flow_gap_resolution.go:15]
  -> flowTerminalRequirementsForArtifacts() [cmd/api_rest/flow_gap_resolution.go:17]
  -> flowRequiredDataFields() [cmd/api_rest/flow_gap_resolution.go:18]

#### PASO-2: Filtrado y clasificacion de gaps por pass

En cada pass (maximo 2), el motor parsea gaps del artefacto data.gaps.v1, los filtra por scope (filterGapsByScope), por proposito del flow (filterGapsByFlowPurpose), y por datos existentes de la entidad (filterGapsByExistingEntityData). Separa gaps bulk_migration (solo observabilidad) de gaps user-completable. En pass 0, genera artefactos de observabilidad para bulk gaps.

INVARIANTES:
  - Maximo 2 passes de resolucion (maxResolutionPasses=2); NUNCA se hace loop infinito
  - Gaps de tipo bulk_migration NUNCA se envian a resolucion conversacional; son solo observabilidad
  - El breakout por no-progress es obligatorio: si ningun gap se resolvio en un pass, se termina

TRACE:
  -> parseDataGaps() [cmd/api_rest/flow_gap_resolution.go:21]
  -> filterGapsByScope() [cmd/api_rest/flow_gap_resolution.go:21]
  -> filterGapsByExistingEntityData() [cmd/api_rest/flow_gap_resolution.go:28]

#### PASO-3: Resolucion escalonada de gaps

Para gaps de contacto: intenta lookup Sabio (lookupSabioContactDestination); si resuelve, persiste el artefacto y registra el paso. Si no, delega a Mecanico conversacional (invokeMecanicoResolveGaps) que genera preguntas para el usuario. Para gaps hybrid data-quality: busca provider con resolutionMode=hybrid, ejecuta resolucion via executeFlowNode. Si Mecanico genera proposals, solicita aprobacion humana. Si hubo resolucion en pass 0, re-ejecuta Auditor para validacion post-resolucion.

INVARIANTES:
  - El re-audit SOLO se ejecuta en pass 0 tras resolucion exitosa
  - Si Mecanico genera proposals, se solicita aprobacion humana (requestMecanicoProposalApproval)
  - Si no hubo progreso en un pass (anyResolved=false), se rompe el loop inmediatamente

TRACE:
  -> lookupSabioContactDestination() [cmd/api_rest/flow_gap_resolution.go:72]
  -> invokeMecanicoResolveGaps() [cmd/api_rest/flow_gap_resolution.go:97]
  -> executeFlowNode() [cmd/api_rest/flow_gap_resolution.go:193]
  -> requestMecanicoProposalApproval() [cmd/api_rest/flow_gap_resolution.go:216]

---

## MECANISMO: execute-flow-node — Ejecuta un nodo individual del flujo contra Channel

Disparado por: FLUJO: flow-run, SUPERFICIE: api-rest

### PIPELINE: node-execution — Prepara params, resuelve args y ejecuta contra Channel

#### PASO-1: Resolucion de command y parametros

El motor busca el command declarado en el contrato dentro del manifest del framework del nodo. Si no existe, retorna error inmediatamente. Resuelve cada parametro del nodo aplicando resolveFlowParamTemplate contra los artefactos disponibles (para interpolar valores dinamicos). Inyecta parametros estandar: flow_run_id, capability, node_id, input, business_id.

INVARIANTES:
  - Un command no encontrado en el manifest SIEMPRE produce error sin ejecucion
  - Los parametros se resuelven con templates contra artefactos; un template invalido aborta la ejecucion

TRACE:
  -> executeFlowNode() [cmd/api_rest/flow_execution.go:15]
  -> resolveFlowParamTemplate() [cmd/api_rest/flow_execution.go:24]
  -> setParamIfDeclared() [cmd/api_rest/flow_execution.go:30]

#### PASO-2: Configuracion de contexto y scoping

El motor determina el conv_id a usar: si el nodo tiene politica "flow_state_scoped", usa un ID deterministico basado en business+flow; si tiene "vault_scoped", usa el vault conv_id del negocio. Agrega parametros contextuales como db (ruta SQLite del negocio), semantic_pack, context_b64, y history. Aplica defaults de artefactos y overrides de test mode.

INVARIANTES:
  - El scoping de conv_id es deterministico y reproducible para el mismo business+flow
  - La ruta SQLite del negocio se inyecta SOLO si el command declara el parametro "db"

TRACE:
  -> focoFlowStateConvID() [cmd/api_rest/flow_execution.go:41]
  -> businessVaultConvID() [cmd/api_rest/flow_execution.go:43]
  -> applyArtifactParamDefaults() [cmd/api_rest/flow_execution.go:64]
  -> materializePortableArtifactParams() [cmd/api_rest/flow_execution.go:67]

#### PASO-3: Ejecucion contra Channel con timeout

El motor resuelve los args finales (cmd.ResolveArgs), determina el runtime del framework, construye los argumentos completos con runtime.FullArgs, y ejecuta contra Channel via ExecuteCommand con un timeout de 300s (o 600s si la politica es "long_running"). Si Channel reporta unavailable, retorna error con URL de diagnostico.

INVARIANTES:
  - Timeout default es 300s; con politica "long_running" se extiende a 600s
  - Si Channel esta unavailable, se retorna un error especifico con la URL del channel (no un timeout generico)
  - La ejecucion es atomica: un nodo se ejecuta una sola vez por iteracion

TRACE:
  -> cmd.ResolveArgs() [cmd/api_rest/flow_execution.go:68]
  -> runtime.FullArgs() [cmd/api_rest/flow_execution.go:73]
  -> s.scoped(runID).ExecuteCommand() [cmd/api_rest/flow_execution.go:80]

---

## MECANISMO: ensure-flow-preflight-audit — Valida calidad de datos antes de ejecutar nodos con side-effect

Disparado por: FLUJO: flow-run, SUPERFICIE: api-rest

### PIPELINE: preflight-validation — Ejecuta Auditor, detecta gaps, y resuelve iterativamente

#### PASO-1: Mediacion de datos y deteccion de provider

El motor verifica si hay datos para auditar. Si no hay external.api.dump ni dataset.raw pero si hay sqlite_db, media datos via Sabio (ensureSabioDataMediation). Si no hay ninguna fuente de datos, retorna true inmediatamente (no hay que auditar). Busca el provider de capability "data.quality.audit" y registra un nodo dinamico preflight_audit_{target_id}.

INVARIANTES:
  - Si no existe dataset NI dump: retorna true INMEDIATAMENTE (asume no hay datos que auditar)
  - Sabio mediation se dispara SOLO si hay sqlite_db pero no hay dump/dataset

TRACE:
  -> ensureFlowPreflightAudit() [cmd/api_rest/flow_preflight.go:92]
  -> ensureSabioDataMediation() [cmd/api_rest/flow_preflight.go:94]
  -> findProviderForCapability() [cmd/api_rest/flow_preflight.go:99]

#### PASO-2: Ejecucion del Auditor

El motor resuelve el contrato del auditor y verifica que sus inputs esten disponibles. Si faltan inputs, skip con human summary explicativo. Si todo esta listo, ejecuta el auditor via executeFlowNode. Parsea el output extrayendo el human summary y los gap summaries. Si el auditor falla, marca el resultado como failed.

INVARIANTES:
  - Si faltan inputs del auditor: skip sin bloquear (retorna true con advertencia)
  - Si Auditor falla: el flujo se DETIENE; no se intenta resolucion de gaps con datos corruptos
  - El preflight SIEMPRE ejecuta Auditor antes de intentar resolucion de gaps

TRACE:
  -> resolveFlowNodeContract() [cmd/api_rest/flow_preflight.go:105]
  -> executeFlowNode() [cmd/api_rest/flow_preflight.go:141]
  -> extractHumanSummary() [cmd/api_rest/flow_preflight.go:149]
  -> summarizeAuditorGaps() [cmd/api_rest/flow_preflight.go:165]

#### PASO-3: Resolucion de gaps y determinacion de readiness

Si el auditor completa exitosamente, el motor invoca resolveFlowGapsIteratively para resolver los gaps detectados. Determina readiness final: ready solo si status != needs_input Y status != failed. Persiste el artefacto flow.preflight.v1 con detalle de readiness y blockers.

INVARIANTES:
  - El resultado de readiness se persiste SIEMPRE (flow.preflight.v1), tanto si ready como si no
  - La readiness se determina DESPUES de intentar resolucion de gaps (no antes)

TRACE:
  -> resolveFlowGapsIteratively() [cmd/api_rest/flow_preflight.go:180]
  -> recordFlowPreflight() [cmd/api_rest/flow_preflight.go:182]

---

## MECANISMO: install-flow-analysis — Instala la configuracion de analisis de un flujo (Radar)

Disparado por: FLUJO: flow-install, SUPERFICIE: api-rest

### PIPELINE: radar-installation — Compila, ejecuta nodo instalable, y persiste schema+plan

#### PASO-1: Validacion y compilacion

El motor valida que el flow tenga manifest. Si se proporciona compiled_id, lo carga y verifica que pertenezca al mismo business_id (cross-business esta prohibido). Si no hay compiled_id, compila el manifest. Verifica si Radar ya esta instalado (radarAnalysisInstalled): si lo esta y no es reconfigure, retorna early con status "installed" y reutiliza el plan existente.

INVARIANTES:
  - Si el flow ya esta instalado Y no es reconfigure, SIEMPRE retorna early sin re-ejecutar
  - El compiled_id DEBE pertenecer al mismo business_id; cross-business esta prohibido
  - Si falta semantic_pack para el business, retorna error (prerequisito obligatorio)

TRACE:
  -> installFlowAnalysis() [cmd/api_rest/flow_store.go:537]
  -> loadCompiledRecord() [cmd/api_rest/flow_store.go:543]
  -> compileAndPersistFlowManifest() [cmd/api_rest/flow_store.go:560]
  -> radarAnalysisInstalled() [cmd/api_rest/flow_store.go:563]

#### PASO-2: Ejecucion del nodo instalable

El motor busca el nodo instalable en el flow compilado (findInstallableFlowNode). Resuelve el contrato del nodo, prepara params (business_id, semantic_pack, db), resuelve los args, y ejecuta el command contra Channel con timeout de 120s. Si Channel esta unavailable, retorna error con URL de diagnostico.

INVARIANTES:
  - El timeout es de 120s hard-coded; no configurable por manifest
  - Si channel esta unavailable: retorna error especifico de channel-unavailable con URL

TRACE:
  -> findInstallableFlowNode() [cmd/api_rest/flow_store.go:574]
  -> resolveFlowNodeContract() [cmd/api_rest/flow_store.go:582]
  -> ExecuteCommand() [cmd/api_rest/flow_store.go:608]

#### PASO-3: Persistencia de artefactos y registro de instalacion

El motor parsea el resultado: espera artifact_type=analysis.schema.v1 en stdout. Si el tipo no coincide, retorna error. Persiste los artefactos analysis.schema.v1 y analysis.plan.v1. Registra el run, upsert installation snapshot con schema_id y weights, y actualiza status del flow a "installed".

INVARIANTES:
  - El nodo instalable DEBE devolver artifact_type=analysis.schema.v1; otro tipo es error
  - La instalacion se registra con upsertInstallation que es idempotente

TRACE:
  -> parseArtifactPayload() [cmd/api_rest/flow_store.go:625]
  -> persistFlowArtifact() [cmd/api_rest/flow_store.go:630]
  -> flows.upsertInstallation() [cmd/api_rest/flow_store.go:658]
  -> flows.updateFlowStatus() [cmd/api_rest/flow_store.go:667]

---

## MECANISMO: chat-echo — Ejecuta el loop interactivo CLI de discovery conversacional Echo-Alfa

Disparado por: FLUJO: flujo-flow-run, SUPERFICIE: cli-flujo

### PIPELINE: discovery-loop — Echo descubre, Alfa compila cuando hay readiness

#### PASO-1: Inicializacion y primera pregunta de Echo

El motor carga el estado del handoff (mustLoad), inicia la fase Echo con el reason proporcionado, y persiste el estado. Genera el prompt inicial para Echo invocando buildPrompt y ejecuta promptRole contra el LLM. Imprime la primera pregunta de Echo al usuario.

INVARIANTES:
  - Echo SIEMPRE habla primero; el usuario nunca ve una pregunta de Alfa antes de que Echo declare readiness
  - El estado se persiste a disco (mustSave) despues de cada transicion de fase

TRACE:
  -> chatEcho() [cmd/flujo/main.go:408]
  -> mustLoad() [cmd/flujo/main.go:413]
  -> buildPrompt() [cmd/flujo/main.go:423]
  -> promptRole() [cmd/flujo/main.go:427]

#### PASO-2: Loop interactivo con handoff condicional

El motor entra en un loop leyendo stdin con bufio.Scanner. En cada iteracion: verifica si Echo declaro readiness (echoReadyToHandOff) consultando el ultimo evento del estado; si lo declaro, hace handoff a Alfa via runRole. Si hay pregunta pendiente de otro role, rutea la respuesta al role correcto. Cuenta respuestas reales del usuario (countUserResponses); si >= 2, auto-handoff a Alfa.

INVARIANTES:
  - Alfa se activa SOLO si Echo declara readiness O hay 2+ respuestas reales de usuario
  - El handoff de Echo a Alfa es irreversible dentro de una sesion chatEcho
  - /salir, /exit, /quit SIEMPRE terminan el loop sin handoff

TRACE:
  -> echoReadyToHandOff() [cmd/flujo/main.go:435]
  -> runRole() [cmd/flujo/main.go:439]
  -> countUserResponses() [cmd/flujo/main.go:482]

#### PASO-3: Procesamiento de imagenes y continuacion

Si el input del usuario no activo ningun handoff, el motor parsea posibles imagenes con parseImageInput y ejecuta promptRoleWithImages para obtener la siguiente respuesta de Echo. Imprime la respuesta y vuelve al inicio del loop.

INVARIANTES:
  - Las imagenes se procesan SOLO si el input del usuario contiene markup de imagen
  - Cada respuesta de Echo se imprime con printEchoQuestionIfMissing que garantiza visibilidad

TRACE:
  -> parseImageInput() [cmd/flujo/main.go:501]
  -> promptRoleWithImages() [cmd/flujo/main.go:507]
  -> printEchoQuestionIfMissing() [cmd/flujo/main.go:511]

---

## MECANISMO: execute-session-followup-detailed — Ejecuta un turno analitico con delegaciones y sintesis

Disparado por: FLUJO: conversacion-chat, SUPERFICIE: api-rest

### PIPELINE: followup-with-delegation — Plan, delega, sintetiza, y produce QueuedQuestion

#### PASO-1: Preparacion de params y ejecucion del followup command

El motor resuelve el manifest y command del session owner (session.Framework + session.FollowupCmd). Incrementa el turn count en disco (incrementSessionOnDisk). Prepara parametros incluyendo input, business_id, turn_count, semantic_pack, history, context_b64, y artefactos previos (previous_analysis, priority_list). Genera un draft LLM para la fase "plan" si el command lo declara. Ejecuta el followup command contra Channel.

INVARIANTES:
  - El turn count SIEMPRE se incrementa antes de ejecutar; nunca se repite un turno
  - El manifest del session owner DEBE existir; si no existe, retorna error inmediatamente

TRACE:
  -> executeSessionFollowupDetailed() [cmd/api_rest/orchestrator.go:508]
  -> incrementSessionOnDisk() [cmd/api_rest/orchestrator.go:528]
  -> generateOwnerFollowupWithLLM() [cmd/api_rest/orchestrator.go:558]
  -> resolvePortableCommandArgs() [cmd/api_rest/orchestrator.go:564]
  -> ch.ExecuteCommand() [cmd/api_rest/orchestrator.go:570]

#### PASO-2: Delegacion y sintesis

El motor parsea la respuesta extrayendo texto y delegation_requests (extractFollowupTextAndDelegations). Si hay delegations, ejecuta cada una via executeDelegations respetando allowed_delegates. Con los resultados de delegacion: genera un draft LLM fresco para fase "synthesis" (borrando el draft del plan), re-ejecuta el followup command con los resultados. Si la sintesis falla o no llega a phase=synthesis, genera un artefacto de failure explicito.

INVARIANTES:
  - Las delegaciones SOLO se ejecutan si estan en allowed_delegates del session owner
  - La sintesis (second pass) SIEMPRE borra el draft del plan; nunca reutiliza el draft pre-delegacion
  - Si la sintesis falla o no llega a phase=synthesis: se genera un artefacto de failure explicito

TRACE:
  -> extractFollowupTextAndDelegations() [cmd/api_rest/orchestrator.go:589]
  -> executeDelegations() [cmd/api_rest/orchestrator.go:595]
  -> generateOwnerFollowupWithLLM(ctx, "synthesis") [cmd/api_rest/orchestrator.go:612]
  -> synthesizeFollowupRuntimeFailureArtifact() [cmd/api_rest/orchestrator.go:606]

#### PASO-3: Persistencia y construccion de respuesta

El motor persiste el artefacto analysis.followup.v1 con metadata de sintesis, delegaciones, y fase. Construye una QueuedQuestion con el texto del followup, chips de continue/exit signals de la sesion, y reasoning explicativo del turno. La pregunta se registra en la cola con speaker = session.Framework.

INVARIANTES:
  - El session followup SIEMPRE produce un QueuedQuestion con speaker = session.Framework
  - Si text es vacio tras toda la pipeline, retorna nil sin persistir (no se acepta respuesta nula)

TRACE:
  -> persistFollowupArtifact() [cmd/api_rest/orchestrator.go:652]
  -> queue.AddQuestionWithReasoning() [cmd/api_rest/orchestrator.go:668]
  -> saveQueue() [cmd/api_rest/orchestrator.go:676]

---

## MECANISMO: resolve-missing-flow-artifacts — Resuelve artefactos faltantes por tipo con estrategias escalonadas

Disparado por: FLUJO: flow-run, SUPERFICIE: api-rest

### PIPELINE: artifact-resolution — Cadena tipo-especifica de resolucion

#### PASO-1: Resolucion de contact.destination.v1

El motor itera cada artefacto faltante. Para contact.destination.v1: primero intenta extraer de artefactos existentes (contactDestinationFromArtifacts). Si no encuentra, ejecuta lookupSabioContactDestination. Si Sabio no resuelve, invoca Mecanico conversacional (invokeMecanicoResolveGaps con gap de contacto faltante). Si Mecanico genera preguntas, las retorna como needs. Si nada funciona, genera input request directo tradicional.

INVARIANTES:
  - El orden de intento es siempre: artefactos existentes > lookup externo > resolucion conversacional > input directo
  - Un artefacto resuelto se marca en available[] Y se persiste inmediatamente (persistFlowArtifact)

TRACE:
  -> resolveMissingFlowArtifacts() [cmd/api_rest/flow_gap_inputs.go:11]
  -> contactDestinationFromArtifacts() [cmd/api_rest/flow_gap_inputs.go:17]
  -> lookupSabioContactDestination() [cmd/api_rest/flow_gap_inputs.go:25]
  -> invokeMecanicoResolveGaps() [cmd/api_rest/flow_gap_inputs.go:33]

#### PASO-2: Resolucion de credentials.smtp

Para credentials.smtp: verifica si el credential ya esta disponible en artefactos (credentialAvailableFromArtifacts). Si no, consulta vault directamente via provider de credentials.smtp.check ejecutando command "has-smtp" con timeout de 10s. Si el vault confirma disponibilidad, lo marca como resuelto. Si no, invoca el wizard de Hosting (invokeProviderNextQuestion) para conectar cPanel o configurar SMTP.

INVARIANTES:
  - El vault check de credentials.smtp tiene timeout de 10s hard-coded
  - Cada tipo de artefacto tiene su propia cadena de resolucion; NUNCA se aplica una estrategia generica a contact.destination
  - Si Mecanico genera preguntas, esas se retornan como needs SIN detener el loop (el caller decide si breaks)

TRACE:
  -> credentialAvailableFromArtifacts() [cmd/api_rest/flow_gap_inputs.go:52]
  -> ExecuteCommand("has-smtp") [cmd/api_rest/flow_gap_inputs.go:68]
  -> invokeProviderNextQuestion() [cmd/api_rest/flow_gap_inputs.go:87]

---

## MECANISMO: execute-delegations — Ejecuta delegaciones a otros frameworks con whitelist y envelope

Disparado por: FLUJO: conversacion-chat, SUPERFICIE: api-rest

### PIPELINE: delegation-execution — Valida whitelist, resuelve target, ejecuta, parsea resultado

#### PASO-1: Construccion de whitelist y envelope

El motor construye un mapa lowercase de allowed_delegates para O(1) lookup. Inicializa el envelope con arrays de requests[] y results[], y summary counters (success_count, failure_count, partial_count). Para cada delegation request: resuelve el target ejecutable con resolveDelegationExecutionTarget que mapea capabilities semanticas a frameworks concretos (ej: evidence.case_360 -> sabio/data.entity_360).

INVARIANTES:
  - La whitelist de allowed_delegates es OBLIGATORIA; una capability no listada NUNCA se ejecuta
  - El envelope SIEMPRE contiene requests[] y results[] del mismo largo (uno por cada request)
  - Capabilities bloqueadas se reportan como failure silenciosa (no se propagan como error al caller)

TRACE:
  -> executeDelegations() [cmd/api_rest/orchestrator.go:737]
  -> resolveDelegationExecutionTarget() [cmd/api_rest/orchestrator.go:762]

#### PASO-2: Resolucion de command y ejecucion

Para cada delegation que pasa la whitelist: busca el manifest del framework resuelto; si no se encuentra, intenta fallback via providerOfProducedCapability. Busca el command para la capability en el manifest (findManifestCapability); si no tiene uno explicito, intenta fallback a "query", "entity-360", "audit", "analyze". Prepara params con business_id, capability, entity_ref, semantic_pack, db, context_b64. Ejecuta contra Channel.

INVARIANTES:
  - Si el manifest no tiene command para la capability: se intenta fallback a "query", "entity-360", "audit", "analyze"
  - Los resultados parciales se contabilizan separadamente de exitos y fallos

TRACE:
  -> findManifestCapability() [cmd/api_rest/orchestrator.go:804]
  -> delegateCmd.ResolveArgs() [cmd/api_rest/orchestrator.go:862]
  -> ch.ExecuteCommand() [cmd/api_rest/orchestrator.go:870]

#### PASO-3: Parseo de output y acumulacion en envelope

El motor parsea el output de cada delegacion. Si es JSON valido, extrae artifact_type, text, structured, trace. Clasifica el resultado como verified, error, o partial_success. Si no es JSON, lo trata como text/plain marcado como verified. Acumula todos los resultados en el envelope con sus contadores.

INVARIANTES:
  - JSON invalido se trata como text/plain y se marca verified=true
  - Cada resultado se acumula con su request_id correlacionado

TRACE:
  -> delegationOutputVerified() [cmd/api_rest/orchestrator.go:900]
  -> delegationOutputError() [cmd/api_rest/orchestrator.go:901]
  -> delegationOutputPartial() [cmd/api_rest/orchestrator.go:902]
  -> incrementDelegationSummary() [cmd/api_rest/orchestrator.go:904]

---

## MECANISMO: ensure-contact-destination-pipeline — Pipeline JIT para resolver destino de contacto

Disparado por: FLUJO: flow-run, SUPERFICIE: api-rest

### PIPELINE: contact-resolution-jit — Resuelve email de contacto con cadena de fuentes priorizadas

#### PASO-1: Busqueda en fuentes locales

El motor intenta resolver el contacto en orden de prioridad: primero extrae de artefactos existentes (contactDestinationFromArtifacts); luego busca una interaction answer que matchee contact.destination.v1 con campo email; finalmente verifica si el input del request es un email valido (isLikelyEmail). Si cualquier fuente local resuelve, se cortocircuita y retorna true inmediatamente.

INVARIANTES:
  - El pipeline se cortocircuita en cuanto CUALQUIER fuente resuelve el contacto (early return true)
  - Un email valido en req.Input se acepta DIRECTAMENTE sin verificacion adicional
  - El orden de prioridad es: artefactos > interaction answer > input directo > Sabio > Mecanico > input request

TRACE:
  -> ensureContactDestinationPipeline() [cmd/api_rest/flow_data_pipeline.go:39]
  -> contactDestinationFromArtifacts() [cmd/api_rest/flow_data_pipeline.go:40]
  -> flowInteractionAnswerFromArtifacts() [cmd/api_rest/flow_data_pipeline.go:45]
  -> isLikelyEmail() [cmd/api_rest/flow_data_pipeline.go:66]

#### PASO-2: Lookup externo via Sabio

Si ninguna fuente local resuelve, el motor ejecuta lookupSabioContactDestination que consulta a Sabio por el email de la entidad actual. Si Sabio encuentra: registra el contacto, emite step de persistencia (emitSabioPersistContactStep), y retorna true. Si no encuentra, emite step de infra indicando que Sabio no tiene el dato.

INVARIANTES:
  - Si se resuelve via Sabio: se emite step de persistencia contra Sabio
  - La pregunta al usuario SIEMPRE incluye el entity display name para contexto

TRACE:
  -> lookupSabioContactDestination() [cmd/api_rest/flow_data_pipeline.go:99]
  -> recordContactDestination() [cmd/api_rest/flow_data_pipeline.go:100]
  -> emitSabioPersistContactStep() [cmd/api_rest/flow_data_pipeline.go:42]

#### PASO-3: Resolucion conversacional y needs_input

Si Sabio no encuentra, el motor invoca Mecanico conversacional (invokeMecanicoResolveGaps con gap missing_contact_destination). Si genera preguntas, las agrega a result.NeedsInput con contexto de entidad. Si no genera preguntas, construye un NeedsInput directo con pregunta contextual que incluye el entity display name. Marca result.Status = needs_input y registra readiness.

INVARIANTES:
  - El pipeline SIEMPRE termina con needs_input si ninguna fuente resolvio (nunca retorna true sin contacto)
  - La pregunta contextual incluye el nombre del deudor/entidad para que el usuario sepa de quien se habla

TRACE:
  -> invokeMecanicoResolveGaps() [cmd/api_rest/flow_data_pipeline.go:116]
  -> inputRequestForContactDestination() [cmd/api_rest/flow_data_pipeline.go:118]
  -> recordFlowReadiness() [cmd/api_rest/flow_data_pipeline.go:151]

---

## MECANISMO: run-message — Ejecuta un turno completo de agent loop para un framework individual

Disparado por: FLUJO: session-message, SUPERFICIE: framework-session

### PIPELINE: single-framework-agent — Configura LLM, ejecuta agent loop, emite respuesta

#### PASO-1: Parseo de flags y carga de manifest

El motor parsea los flags de linea de comando: --framework (obligatorio), --conv-id, --message (o --message-b64), --history, --context-b64. Resuelve el root del proyecto, carga el manifest del framework (loadManifest), lee el initial prompt (readInitialPrompt), y resuelve el spec del modelo (specFor) que determina provider y model name.

INVARIANTES:
  - --framework es OBLIGATORIO; sin el la funcion retorna error inmediatamente
  - Si --message-b64 se proporciona, se decodifica con base64.RawURLEncoding; invalido produce error

TRACE:
  -> runMessage() [cmd/framework_session/main.go:199]
  -> loadManifest() [cmd/framework_session/main.go:226]
  -> readInitialPrompt() [cmd/framework_session/main.go:230]
  -> specFor() [cmd/framework_session/main.go:234]

#### PASO-2: Configuracion del workspace y agent loop

El motor crea el cliente LLM (llm.New), asegura el workspace para el framework+conv (ensureWorkspace), y abre el writer de eventos live (newLiveEventWriter). Construye el mensaje del usuario combinando history, session context, y mensaje actual. Emite eventos iniciales y ejecuta el agent loop completo (agentloop.Run) con tool runner, max turns del manifest (default 30), y max tokens 1200.

INVARIANTES:
  - El timeout del context es 120s hard-coded para todo el agent loop
  - maxTurns viene del manifest (Agent.MaxTurns) con default 30 si no esta declarado
  - MaxTokens es 1200 hard-coded para respuestas concisas
  - El workspace se crea o reutiliza por (framework, conv_id); es idempotente

TRACE:
  -> llm.New() [cmd/framework_session/main.go:238]
  -> ensureWorkspace() [cmd/framework_session/main.go:242]
  -> agentloop.Run() [cmd/framework_session/main.go:272]

#### PASO-3: Emision de respuesta

El motor extrae el texto de la respuesta del agente. Si esta vacio, retorna error (no se acepta respuesta nula). Ejecuta assisted setup completion si aplica. Construye la respuesta JSON con ID, texto, reasoning, chips, y todos los eventos acumulados del run. Escribe a stdout.

INVARIANTES:
  - Si el LLM devuelve respuesta vacia: retorna error (no se acepta respuesta nula)
  - La respuesta SIEMPRE incluye reasoning explicativo del procesamiento interno

TRACE:
  -> agentloop.Run() result [cmd/framework_session/main.go:283]
  -> assistedSetupCompletionEvents() [cmd/framework_session/main.go:293]
  -> writeJSON() [cmd/framework_session/main.go:313]

---

## GAPS

### GAP-1: Framework-mensajero es binario externo no presente en repo

SUPERFICIE:   api-rest
EXPERIENCIA:  Cuando el sistema intenta enviar un email via POST /api/v1/send-email, ejecuta exec.Command(REMORA_MENSAJERO_BIN). Si la variable no esta configurada o el binario no existe, el usuario recibe un error de ejecucion sin mensaje claro.
CAUSA:        El binario framework-mensajero no vive en los 3 modulos analizados. La referencia en api_rest/main.go:1254 apunta a un path externo configurable via env var REMORA_MENSAJERO_BIN.
REFERENCIA:   FLUJO: send-email, ESC-1

### GAP-2: Binario contactos es externo y no esta en repo

SUPERFICIE:   api-rest
EXPERIENCIA:  Las operaciones de contactos (lookup/store) dependen de un binario externo invocado via exec.Command en api_rest/contactos.go:51,83. Si el binario no esta disponible, las operaciones fallan silenciosamente.
CAUSA:        Binario no encontrado en el repositorio remora-go-lite.
REFERENCIA:   FLUJO: contactos-operations, ESC-1

### GAP-3: Framework-echo tiene path hardcodeado fuera del repo

SUPERFICIE:   cli-flujo
EXPERIENCIA:  El binario flujo ejecuta echoReadyForAlfa en flujo/main.go:319 con un path absoluto hardcodeado a /Users/alcless_a1234_cursor/remora-go/framework-echo. Si el usuario no tiene ese path exacto, la verificacion de readiness falla.
CAUSA:        Path absoluto en flujo/main.go:319: exec.Command("/bin/zsh", "-lc", "cd ... && ./frameworkecho readiness")
REFERENCIA:   FLUJO: flujo-flow-run, ESC-1

### GAP-4: Framework-radar no tiene modelo LLM configurado

SUPERFICIE:   framework-radar
EXPERIENCIA:  El framework-radar en framework-catalog.json tiene el campo model vacio. Si se intenta ejecutar con LLM, no hay provider resolvible.
CAUSA:        Campo model ausente en catalog.json para framework-radar.
REFERENCIA:   FLUJO: ejecutar-radar-prioritize, ESC-1

### GAP-5: Flujo main() no expuesto en grafo CPG

SUPERFICIE:   cli-flujo
EXPERIENCIA:  El entry point main() del binario flujo no aparece en el grafo CPG. El Detective reporta que solo cmdFlow() es visible. Esto dificulta trazar la cadena completa desde el entry point real.
CAUSA:        Limitacion del grafo CPG que no resuelve main() de flujo.
REFERENCIA:   FLUJO: flujo-flow-run, ESC-1

### GAP-6: Devcli flow create no implementado

SUPERFICIE:   cli-devcli
EXPERIENCIA:  Cuando el usuario ejecuta `remora dev flow create <business_id> <name>`, el sistema muestra un mensaje "[info] creacion de flujo via API REST (implementacion completa pendiente)" y no hace nada. El flujo esta marcado como TODO.
CAUSA:        devcli/main.go:207-208: fmt.Fprintf(os.Stderr, "[info] creacion de flujo via API REST (implementacion completa pendiente)")
REFERENCIA:   FLUJO: flow-inspect-devcli, ESC-1

### GAP-7: No hay streaming SSE en remora-cli debug

SUPERFICIE:   cli-remora
EXPERIENCIA:  Los comandos debug de remora-cli (trace, simulate, validate) hacen requests sincronos. No hay opcion de ver ejecucion en streaming como si tiene el binario flujo. El usuario de remora debug no ve progreso en tiempo real.
CAUSA:        remora/flow_workbench.go solo usa get() y post(), nunca stream().
REFERENCIA:   FLUJO: debug-trace, ESC-1

---

## CABLES

### CABLE-1: Delegacion remora → flujo via exec.Command

ENTRE:        cli-remora y cli-flujo (binario flujo)
EXPERIENCIA:  Cuando el usuario usa `remora flow <subcommand>`, el CLI remora no ejecuta la logica directamente. Delega al binario flujo via exec.Command("go", "run", "./cmd/flujo", "flow", args...). Si el repo root no existe o flujo no esta en PATH, el usuario ve "flow workbench canonico no disponible".
MECANISMO:    exec.Command con passthrough de stdin/stdout/stderr [remora/flow_workbench.go:880-893]
REFERENCIA:   FLUJO: flow-create, ESC-1

### CABLE-2: api_rest → channel via exec.Command y compilacion on-demand

ENTRE:        api_rest y channel (binario)
EXPERIENCIA:  Cuando se ejecuta un flujo, api_rest necesita channel corriendo. Si el binario no existe, lo compila on-demand con `go build -buildvcs=false -o outBin ./cmd/channel`. Luego lo inicia como proceso hijo y espera pingChannel(). Si la compilacion falla, el flujo entero falla.
MECANISMO:    buildChannelBinary() [api_rest/flow_channel.go:134] + ensureChannel() [api_rest/flow_channel.go:56]
REFERENCIA:   FLUJO: flows-run, ESC-1

### CABLE-3: api_rest → vault via exec.Command para secretos

ENTRE:        api_rest y vault (binario)
EXPERIENCIA:  Las operaciones SMTP (check, import, delete) y el bootstrap de credenciales usan vault via exec.Command. Si REMORA_VAULT_KEY no esta configurado, todas las operaciones de secretos fallan con error de clave faltante.
MECANISMO:    exec.Command(resolveVaultBin(), "set"/"has"/"get", ...) [api_rest/main.go:1111,1334]
REFERENCIA:   FLUJO: smtp-operations, ESC-1

### CABLE-4: framework_session → LLM providers via HTTP

ENTRE:        framework_session y servicios LLM externos (MiniMax/Groq/OpenRouter)
EXPERIENCIA:  Cada sesion de framework depende de un proveedor LLM externo. Si el API key (GROQ_API_KEY, etc.) no esta configurado o el servicio esta caido, el agente no puede responder. El usuario ve timeout o error generico.
MECANISMO:    llm.Stream()/Complete() → requestMiniMax()/requestGroq() → http.Do [llm/client.go:218, nativeagent/agent.go:435,499]
REFERENCIA:   FLUJO: session-message, ESC-1

### CABLE-5: Frontend → api_rest via fetch + SSE

ENTRE:        frontend-web (index.html) y api_rest (HTTP :8084)
EXPERIENCIA:  Todo lo que el usuario ve en el navegador depende de requests fetch() al backend en localhost:8084. El frontend usa apiFetch() con Bearer token en Authorization header. Si el backend no esta corriendo, la interfaz queda vacia sin feedback visual claro.
MECANISMO:    apiFetch(url, options) [index.html:3340] → fetch con credentials:'include'
REFERENCIA:   FLUJO: conversacion-chat, ESC-2

### CABLE-6: Orchestrator → Channel adapter via JSON-RPC

ENTRE:        orchestrator y channel (JSON-RPC)
EXPERIENCIA:  El orchestrator ejecuta comandos de frameworks via adapter.ExecuteCommand() que envia JSON-RPC al channel en localhost:8765. Si channel no esta corriendo o CHANNEL_API_KEYS no incluye la key usada, el orchestrator falla con error de conexion.
MECANISMO:    adapter.New(channelURL, apiKey) → call() → JSON-RPC POST [adapter/adapter.go:108]
REFERENCIA:   FLUJO: orchestrator-run, ESC-1

### CABLE-7: nativeagent → bash con validacion de politica

ENTRE:        nativeagent y sistema operativo
EXPERIENCIA:  El agente nativo puede ejecutar comandos bash arbitrarios via toolBash. La unica barrera es validateBashPolicy que puede rechazar comandos peligrosos. Si la politica es permisiva, un prompt malicioso podria ejecutar comandos destructivos en el sistema del usuario.
MECANISMO:    exec.Command("/bin/zsh", "-lc", command) [nativeagent/agent.go:1021] con validateBashPolicy [nativeagent/agent.go:1031]
REFERENCIA:   FLUJO: agent-prompt, ESC-2

### CABLE-8: Cadena Echo → Alfa → Bravo (formato de datos)

ENTRE:        framework-echo, framework-alfa, framework-bravo
EXPERIENCIA:  El pipeline de descubrimiento sigue la cadena: Echo produce echo.tree.v1, Alfa lo consume y produce alfa.spec.v1 + alfa.ideal_flow.v1, Bravo consume el ideal_flow para verificar. Si el formato de echo.tree.v1 cambia sin actualizar Alfa, la compilacion falla.
MECANISMO:    Matching de formatos en manifest: echo.outputs.tree.format == alfa.inputs.echo_tree.format
REFERENCIA:   FLUJO: ejecutar-alfa-compile, ESC-1

### CABLE-9: Cadena Auditor → Mecanico (findings)

ENTRE:        framework-auditor y framework-mecanico
EXPERIENCIA:  El Mecanico depende de los findings del Auditor (auditor.findings.v1) como input. Si el Auditor no ha corrido scan previamente, Mecanico no tiene datos para proponer remediaciones.
MECANISMO:    Mecanico inputs.findings.format == auditor.outputs.findings.format (auditor.findings.v1)
REFERENCIA:   FLUJO: ejecutar-mecanico-propose, ESC-1

### CABLE-10: Profile → SystemPrompt personalizado

ENTRE:        profile (libreria) y channel/internal handler
EXPERIENCIA:  El perfil activo (REMORA_PROFILE) modifica el system prompt de las sesiones de framework. Si un perfil tiene overlay, cambia el comportamiento del agente. El usuario no ve diferencia en la interfaz pero las respuestas varian segun el perfil cargado.
MECANISMO:    profile.Load() / SystemPromptWithOverlay() [profile/profile.go:131] usado en handler.go
REFERENCIA:   FLUJO: execute-command, ESC-1

---

<!-- CHAIN_RUN_ID: run-1778992200 -->
