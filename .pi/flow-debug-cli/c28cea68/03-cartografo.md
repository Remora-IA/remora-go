# pi-cartografo - Queries de Joern para remora flow debug CLI

**CHAIN_RUN_ID:** c28cea68  
**Fecha:** 2026-05-16

---

## Mapeo de Comandos CLI → Frameworks

| Comando CLI | Source (Origen) | Sink (Destino) | Frameworks Involucrados |
|-------------|-----------------|----------------|--------------------------|
| `--list-frameworks` | Lectura de `*/framework.manifest.json` | `CatalogoFrameworks` | **quine** (índice), **todos** |
| `--manifest FRAMEWORK` | Carga de manifest específico | `FrameworkLoader` | **quine** (descubrimiento) |
| `--framework FRAMEWORK --commands` | Análisis de manifest.commands | Formateo de salida | **alfa**, **bravo**, **paladin**, etc. |
| `--framework FRAMEWORK --capabilities` | Análisis de capabilities_semantic | Formateo de salida | **alfa**, **bravo**, **paladin**, etc. |
| `--flow FLUJO --dry-run` | API REST `/api/flows/{id}/simulate` | `Planificador` | **alfa** (compilación) |
| `--flow FLUJO --validate` | API REST `/api/flows/{id}/validation` | `ValidadorFlujo` | **alfa**, **bravo** (verificación) |
| `--flow FLUJO --execute` | API REST `/api/flows/{id}/run` | `MotordeEjecucion` | **alfa**, **bravo**, **paladin** (tracing) |
| `--flow FLUJO --trace` | API REST `/api/traces/{run_id}` | `TrazaEjecucion` | **paladin** (audit/trace) |
| `--flow FLUJO --dependencies` | API REST `/api/flows/{id}/compile` | `GestorDependencias` | **bravo** (idealflow) |
| `--flow FLUJO --gaps` | API REST + análisis de derivation | `AnalizadorGAPs` | **bravo** (verifier) |
| `--step N --inspect-inputs` | API REST `/api/flows/{id}` | Inspección de metadata | **alfa** (inputs[]) |
| `--step N --inspect-outputs` | API REST `/api/traces/{run_id}/artifacts` | Inspección de artifacts | **alfa** (outputs[]) |

---

## AXIOMAS DE SÍMBOLOS → Queries Joern

### A1: La CLI debe aceptar comandos con flags en formato `--comando` o `--comando valor`

```scala
// Buscar todas las definiciones de FlagSet en la CLI
cpg.call.name(".*FlagSet.*").code.l
cpg.local.name(".*flag.*").code.l
```

### A2: La CLI debe soportar formato de salida `--format [json|yaml|table]`

```scala
// Buscar formatters de salida en main.go y flow_workbench.go
cpg.call.name(".*format.*").code.l
cpg.call.name(".*json.*").code.l
cpg.call.name(".*yaml.*").code.l
cpg.call.name(".*table.*").code.l
```

### A3: La CLI debe presentar resultados en columnas alineadas

```scala
// Buscar uso de tabwriter para formateo de columnas
cpg.call.name(".*tabwriter.*").code.l
cpg.call.name(".*NewWriter.*").code.l
cpg.call.name(".*Write.*").code.l
```

### A4: La CLI debe mostrar errores con información accionable

```scala
// Buscar manejo de errores con mensajes accionables
cpg.call.name(".*Errorf.*").code.l
cpg.call.name(".*Fatalf.*").code.l
```

### A5: La CLI debe tener modo interactivo con prompt `remora>`

```scala
// Buscar prompt interactivo en la CLI
cpg.call.name(".*ReadLine.*").code.l
cpg.call.name(".*printFlowWorkbenchUsage.*").code.l
```

### A6: Debe retornar lista completa de frameworks

```scala
// Buscar función que lista frameworks en remora-cli
cpg.method.name(".*list.*Framework.*").code.l
cpg.method.name(".*List.*").code.l
```

### A7: Debe verificar existencia de framework antes de cargar manifest

```scala
// Buscar verificación de existencia de framework
cpg.call.name(".*exist.*").code.l
cpg.call.name(".*lookup.*").code.l
cpg.call.name(".*Get.*Framework.*").code.l
```

### A8: Debe devolver NOT_FOUND cuando el framework no existe

```scala
// Buscar manejo de NOT_FOUND
cpg.call.name(".*NOT_FOUND.*").code.l
cpg.call.name(".*404.*").code.l
```

### A9: Debe cargar metadatos completos de cada framework

```scala
// Buscar carga de metadata de framework
cpg.call.name(".*manifest.*").code.l
cpg.call.name(".*Load.*").code.l
```

### A10: Debe retornar manifest estructurado

```scala
// Buscar struct de manifest en CLI
cpg.typeDecl.name(".*Manifest.*").code.l
```

### A11: Debe indicar tipo de cada capability

```scala
// Buscar capabilities_semantic en framework manifests
cpg.call.name(".*capabilities_semantic.*").code.l
cpg.call.name(".*capability.*").code.l
```

### A12: Debe mostrar flags de idempotencia por capability

```scala
// Buscar idempotencia en capabilities
cpg.call.name(".*idempotent.*").code.l
cpg.call.name(".*produces.*").code.l
```

---

## AXIOMAS DE INTERACCIÓN → Queries Joern

### A46: `--list-frameworks` debe consultar CatalogoFrameworks

```scala
// Query para listar todos los framework manifests
import io.shiftleft.semanticcpg.language._
val manifestFiles = os.walk("/Users/alcless_a1234_cursor/remora-go-lite")
  .filter(_.name == "framework.manifest.json")
  .map(f => scala.io.Source.fromFile(f).mkString)
manifestFiles.foreach(println)
```

### A47: `--manifest FRAMEWORK` debe cargar manifest específico

```scala
// Buscar función que carga manifest individual
cpg.call.name(".*LoadManifest.*").code.l
cpg.call.name(".*ReadFile.*").code.l
cpg.local.name(".*manifest.*").code.l
```

### A48: `--framework FRAMEWORK --commands` debe devolver lista de comandos

```scala
// Buscar extracción de commands del manifest
cpg.call.name(".*commands.*").code.l
cpg.call.name(".*args.*").code.l
```

### A49: `--framework FRAMEWORK --capabilities` debe devolver capabilities

```scala
// Buscar extracción de capabilities del manifest
cpg.call.name(".*capabilities.*").code.l
cpg.call.name(".*produces.*").code.l
```

### A50: `--flow FLUJO --dry-run` debe solicitar plan a Planificador

```scala
// Buscar endpoint de simulate/dry-run
cpg.call.name(".*simulate.*").code.l
cpg.call.name(".*dry-run.*").code.l
cpg.call.name(".*dryRun.*").code.l
```

### A51: Planificador debe resolver dependencias con GestorDependencias

```scala
// Buscar resolución de dependencias
cpg.call.name(".*dependency.*").code.l
cpg.call.name(".*topological.*").code.l
cpg.call.name(".*resolve.*").code.l
```

### A52: GestorDependencias debe devolver orden topológico

```scala
// Buscar ordenamiento topológico
cpg.call.name(".*topological.*").code.l
cpg.call.name(".*order.*").code.l
cpg.call.name(".*sort.*").code.l
```

### A53: Planificador debe generar timeline con GeneradorTimeline

```scala
// Buscar generación de timeline
cpg.call.name(".*timeline.*").code.l
cpg.call.name(".*estimate.*").code.l
```

### A54: CLI debe presentar dry-run completo

```scala
// Buscar presentación de dry-run
cpg.call.name(".*printSimulation.*").code.l
cpg.call.name(".*showPlan.*").code.l
```

### A55: `--flow FLUJO --trace` debe iniciar captura de TrazaEjecucion

```scala
// Buscar inicio de traza en paladin
cpg.call.name(".*StartSpan.*").code.l
cpg.call.name(".*BeginTrace.*").code.l
```

### A56: MotordeEjecucion debe registrar STEP_START

```scala
// Buscar registro de eventos de paso
cpg.call.name(".*STEP_START.*").code.l
cpg.call.name(".*StepStart.*").code.l
```

### A57: Registrar STEP_COMPLETE con outputs

```scala
// Buscar registro de completación de paso
cpg.call.name(".*STEP_COMPLETE.*").code.l
cpg.call.name(".*StepComplete.*").code.l
```

### A58: CLI debe presentar traza formateada

```scala
// Buscar formateo de traza
cpg.call.name(".*PrintTrace.*").code.l
cpg.call.name(".*FormatTrace.*").code.l
```

### A59: `--flow FLUJO --execute` debe validar primero

```scala
// Buscar validación antes de ejecutar
cpg.call.name(".*validate.*").code.l
cpg.call.name(".*Validate.*").code.l
cpg.call.name(".*Validation.*").code.l
```

### A60: Si validación pasa, CLI debe mostrar confirmación

```scala
// Buscar confirmación antes de ejecutar
cpg.call.name(".*confirm.*").code.l
cpg.call.name(".*AskConfirm.*").code.l
cpg.call.name(".*Approve.*").code.l
```

### A61: Usuario debe confirmar con "ejecutar"

```scala
// Buscar lectura de confirmación
cpg.call.name(".*ReadLine.*").code.l
cpg.call.name(".*Scan.*").code.l
cpg.call.name(".*readLine.*").code.l
```

### A62: MotordeEjecucion ejecuta pasos en orden

```scala
// Buscar ejecución de pasos
cpg.call.name(".*execute.*").code.l
cpg.call.name(".*Execute.*").code.l
cpg.call.name(".*RunStep.*").code.l
```

### A63: CLI presenta resultado final

```scala
// Buscar presentación de resultado final
cpg.call.name(".*PrintResult.*").code.l
cpg.call.name(".*ShowSummary.*").code.l
```

### A67: `--flow FLUJO --validate` ejecuta todos los checks

```scala
// Buscar función de validación completa
cpg.call.name(".*validateFlow.*").code.l
cpg.call.name(".*ValidateFlow.*").code.l
```

### A68: ValidadorFlujo debe verificar frameworks con CatalogoFrameworks

```scala
// Buscar verificación de frameworks
cpg.call.name(".*checkFramework.*").code.l
cpg.call.name(".*ExistsFramework.*").code.l
```

### A69: Verificar chain de datos con AnalizadorGAPs

```scala
// Buscar análisis de gaps
cpg.call.name(".*gap.*").code.l
cpg.call.name(".*missing.*").code.l
cpg.call.name(".*analyze.*").code.l
```

### A71: `--flow FLUJO --dependencies` muestra grafo y matrix

```scala
// Buscar generación de grafo de dependencias
cpg.call.name(".*dependencyGraph.*").code.l
cpg.call.name(".*BuildGraph.*").code.l
```

### A72: Grafo debe mostrar nodos (frameworks) y aristas (datos)

```scala
// Buscar construcción de grafo
cpg.call.name(".*node.*").code.l
cpg.call.name(".*edge.*").code.l
cpg.call.name(".*AddNode.*").code.l
```

### A73: Matrix debe mostrar Framework x Framework

```scala
// Buscar generación de matriz
cpg.call.name(".*matrix.*").code.l
cpg.call.name(".*DependencyMatrix.*").code.l
```

### A75: AnalizadorGAPs analiza paso a paso

```scala
// Buscar análisis de gaps en datos
cpg.call.name(".*AnalyzeGap.*").code.l
cpg.call.name(".*DetectGap.*").code.l
```

### A76: Para cada gap, mostrar tipo, ubicación, problema

```scala
// Buscar tipos de gap
cpg.call.name(".*DATA_MISMATCH.*").code.l
cpg.call.name(".*INPUT_MISSING.*").code.l
cpg.call.name(".*GAP.*").code.l
```

### A77: Gap types: DATA_MISMATCH, INPUT_MISSING, CAPABILITY_MISSING

```scala
// Buscar constantes de gap
cpg.local.name(".*MISMATCH.*").code.l
cpg.local.name(".*MISSING.*").code.l
```

### A78: `--step N --inspect-outputs` muestra metadata del artifact

```scala
// Buscar inspección de outputs
cpg.call.name(".*inspectOutput.*").code.l
cpg.call.name(".*GetArtifact.*").code.l
```

### A79: Metadata incluye tipo, tamaño, schema, muestra

```scala
// Buscar metadata de artifact
cpg.call.name(".*artifact.*").code.l
cpg.call.name(".*metadata.*").code.l
```

### A80: `--step N --inspect-inputs` muestra inputs requeridos

```scala
// Buscar inspección de inputs
cpg.call.name(".*inspectInput.*").code.l
cpg.call.name(".*GetInput.*").code.l
```

### A81: Para cada input: tipo, requerido, source, schema

```scala
// Buscar información de input
cpg.call.name(".*input.*").code.l
cpg.call.name(".*required.*").code.l
```

---

## AXIOMAS DE VALIDACIÓN → Queries Joern

### V1: `--list-frameworks` debe funcionar siempre

```scala
// Verificar que existe función de list frameworks
cpg.method.name(".*listFrameworks.*").code.l
cpg.method.name(".*ListFrameworks.*").code.l
```

### V2: `--manifest FRAMEWORK` debe mostrar estructura completa

```scala
// Verificar que existe función de mostrar manifest
cpg.method.name(".*showManifest.*").code.l
cpg.method.name(".*PrintManifest.*").code.l
```

### V3: `--flow FLUJO --dry-run` debe simular sin efectos

```scala
// Verificar dry-run en API REST
cpg.call.name(".*simulate.*").code.l
cpg.call.name(".*dryRun.*").code.l
```

### V4: `--flow FLUJO --execute` debe validar primero, ejecutar luego

```scala
// Verificar flujo validate -> execute
cpg.call.name(".*run.*").code.l
cpg.call.name(".*execute.*").code.l
```

### V5: `--flow FLUJO --validate` debe verificar estructura

```scala
// Verificar función de validación
cpg.method.name(".*validate.*").code.l
cpg.method.name(".*Validate.*").code.l
```

### V6: `--flow FLUJO --trace` debe mostrar timeline con eventos

```scala
// Verificar trazas en API REST
cpg.call.name(".*trace.*").code.l
cpg.call.name(".*Trace.*").code.l
```

### V7: `--flow FLUJO --dependencies` debe mostrar grafo

```scala
// Verificar endpoint de dependencias
cpg.call.name(".*dependencies.*").code.l
cpg.call.name(".*dependency.*").code.l
```

### V8: `--flow FLUJO --gaps` debe identificar y sugerir

```scala
// Verificar análisis de gaps
cpg.call.name(".*gap.*").code.l
cpg.call.name(".*suggest.*").code.l
```

### V9: `--step N --inspect-inputs` debe detallar inputs

```scala
// Verificar inspección de inputs
cpg.call.name(".*inspectInput.*").code.l
cpg.call.name(".*InputDetails.*").code.l
```

### V10: `--step N --inspect-outputs` debe mostrar metadata

```scala
// Verificar inspección de outputs
cpg.call.name(".*inspectOutput.*").code.l
cpg.call.name(".*OutputDetails.*").code.l
```

### V11: `--flow FLUJO --capabilities-map` debe mostrar mapa visual

```scala
// Verificar mapa de capabilities
cpg.call.name(".*capabilityMap.*").code.l
cpg.call.name(".*Map.*Capabilities.*").code.l
```

### V12: `--flow FLUJO --timeline` debe mostrar timeline estimado

```scala
// Verificar generación de timeline
cpg.call.name(".*timeline.*").code.l
cpg.call.name(".*TimeLine.*").code.l
```

### V13: `--history` debe mostrar historial de ejecuciones

```scala
// Verificar historial
cpg.call.name(".*history.*").code.l
cpg.call.name(".*History.*").code.l
cpg.call.name(".*runHistory.*").code.l
```

### V14: `--interactive` debe permitir modo interactivo

```scala
// Verificar modo interactivo
cpg.call.name(".*interactive.*").code.l
cpg.call.name(".*Interactive.*").code.l
cpg.call.name(".*ReadLine.*").code.l
```

### V15: `--config init` debe permitir wizard de configuración

```scala
// Verificar wizard de configuración
cpg.call.name(".*config.*init.*").code.l
cpg.call.name(".*initConfig.*").code.l
```

### V16: Todos los inputs requeridos deben tener valor

```scala
// Verificar validación de inputs
cpg.call.name(".*checkInputs.*").code.l
cpg.call.name(".*Required.*").code.l
```

### V17: Si input no disponible, ofrecer opciones

```scala
// Verificar manejo de inputs faltantes
cpg.call.name(".*missingInput.*").code.l
cpg.call.name(".*askInput.*").code.l
```

### V18: Si --ask-prompts, solicitar interactivamente

```scala
// Verificar prompts interactivos
cpg.call.name(".*ask.*").code.l
cpg.call.name(".*prompt.*").code.l
```

### V19: Flujo debe tener al menos 1 paso

```scala
// Verificar validación de estructura de flujo
cpg.call.name(".*nodes.*").code.l
cpg.call.name(".*steps.*").code.l
```

### V20: Cada paso debe referenciar framework existente

```scala
// Verificar referencia a framework en paso
cpg.call.name(".*framework.*").code.l
```

### V21: Dependencias no deben formar ciclos

```scala
// Verificar detección de ciclos
cpg.call.name(".*cycle.*").code.l
cpg.call.name(".*Cycle.*").code.l
cpg.call.name(".*hasCycle.*").code.l
```

### V22: Outputs deben coincidir con inputs del siguiente paso

```scala
// Verificar match de outputs->inputs
cpg.call.name(".*match.*").code.l
cpg.call.name(".*compatible.*").code.l
```

### V23: `--help` o `-h` debe funcionar en todos los comandos

```scala
// Verificar flags de help
cpg.call.name(".*help.*").code.l
cpg.call.name(".*Usage.*").code.l
cpg.call.name(".*PrintHelp.*").code.l
```

### V24: `--format [json|yaml|table]` debe estar disponible

```scala
// Verificar formato de salida
cpg.call.name(".*format.*").code.l
cpg.call.name(".*Format.*").code.l
```

### V25: `--output FILE` debe redirigir salida

```scala
// Verificar redirección de output
cpg.call.name(".*output.*").code.l
cpg.call.name(".*redirect.*").code.l
```

### V26: `--verbose` o `-v` debe mostrar detalles adicionales

```scala
// Verificar verbose flag
cpg.call.name(".*verbose.*").code.l
cpg.call.name(".*Debug.*").code.l
```

---

## AXIOMAS DE INTEGRIDAD → Queries Joern

### I1: Outputs de un paso deben fluir como inputs al siguiente

```scala
// Verificar flujo de datos entre pasos
cpg.call.name(".*passOutput.*").code.l
cpg.call.name(".*forward.*").code.l
```

### I2: Chain de datos debe mantenerse

```scala
// Verificar linaje de datos
cpg.call.name(".*lineage.*").code.l
cpg.call.name(".*chain.*").code.l
```

### I3: DATA_MISMATCH debe ser detectado

```scala
// Verificar detección de mismatch
cpg.call.name(".*mismatch.*").code.l
cpg.call.name(".*Mismatch.*").code.l
```

### I4: Cada ejecución debe generar run_id único

```scala
// Verificar generación de run_id
cpg.call.name(".*run_id.*").code.l
cpg.call.name(".*generateId.*").code.l
```

### I5: Cada ejecución debe ser grabada en historial

```scala
// Verificar almacenamiento de historial
cpg.call.name(".*saveHistory.*").code.l
cpg.call.name(".*Record.*").code.l
```

### I6: Traza debe ser recuperable por run_id

```scala
// Verificar recuperación de traza
cpg.call.name(".*GetTrace.*").code.l
cpg.call.name(".*RetrieveTrace.*").code.l
```

### I7: Timestamp de eventos debe ser ISO8601 UTC

```scala
// Verificar formato de timestamp
cpg.call.name(".*RFC3339.*").code.l
cpg.call.name(".*timestamp.*").code.l
cpg.call.name(".*UTC.*").code.l
```

### I8: Datos sensibles deben ser mascarados en traza

```scala
// Verificar masking de datos sensibles
cpg.call.name(".*mask.*").code.l
cpg.call.name(".*sensitive.*").code.l
cpg.call.name(".*redact.*").code.l
```

### I9: Credenciales por nombre, no valor

```scala
// Verificar referencia a credenciales
cpg.call.name(".*credential.*").code.l
cpg.call.name(".*env.*key.*").code.l
```

### I10: Output table con ancho fijo

```scala
// Verificar formateo de tabla
cpg.call.name(".*tabwriter.*").code.l
cpg.call.name(".*formatTable.*").code.l
```

### I11: Output JSON con estructura específica

```scala
// Verificar estructura JSON
cpg.call.name(".*json.*marshal.*").code.l
cpg.call.name(".*json.*encode.*").code.l
```

### I12: Output YAML debe ser parseable

```scala
// Verificar YAML encoding
cpg.call.name(".*yaml.*").code.l
```

### I13: Estados de paso: waiting, running, success, failed, skipped

```scala
// Verificar estados de paso
cpg.call.name(".*waiting.*").code.l
cpg.call.name(".*running.*").code.l
cpg.call.name(".*success.*").code.l
cpg.call.name(".*failed.*").code.l
cpg.call.name(".*skipped.*").code.l
```

### I14: Estados de flujo: pending, running, completed, failed

```scala
// Verificar estados de flujo
cpg.call.name(".*pending.*").code.l
cpg.call.name(".*completed.*").code.l
```

### I15: Progreso debe mostrar [N/M] paso

```scala
// Verificar display de progreso
cpg.call.name(".*progress.*").code.l
cpg.call.name(".*percentage.*").code.l
cpg.call.name(".*step.*").code.l
```

### I16: Error con mensaje descriptivo

```scala
// Verificar mensajes de error descriptivos
cpg.call.name(".*error.*").code.l
cpg.call.name(".*Errorf.*").code.l
```

### I17: Error con acciones disponibles

```scala
// Verificar opciones de recuperación de error
cpg.call.name(".*action.*").code.l
cpg.call.name(".*retry.*").code.l
```

### I18: Si framework no existe, incluir lista de disponibles

```scala
// Verificar mensaje de framework no encontrado
cpg.call.name(".*available.*").code.l
cpg.call.name(".*notFound.*").code.l
```

### I19: Si flujo no existe, incluir flujos disponibles

```scala
// Verificar mensaje de flujo no encontrado
cpg.call.name(".*availableFlows.*").code.l
```

### I20: Si ciclo detectado, indicar path del ciclo

```scala
// Verificar reporte de ciclo
cpg.call.name(".*cyclePath.*").code.l
cpg.call.name(".*cycle.*").code.l
```

### I21: Antes de efectos externos, CLI debe confirmar

```scala
// Verificar confirmación de efectos
cpg.call.name(".*confirmSideEffect.*").code.l
cpg.call.name(".*warnExternal.*").code.l
```

### I22: Confirmación debe pedir escribir "ejecutar"

```scala
// Verificar confirmación con palabra específica
cpg.call.name(".*executar.*").code.l
cpg.call.name(".*confirmWord.*").code.l
```

---

## Mapeo de Comandos CLI → Funciones Joern

| Comando CLI | Query Joern Principal |
|------------|----------------------|
| `--list-frameworks` | `cpg.call.name(".*list.*Framework.*")` |
| `--manifest FRAMEWORK` | `cpg.call.name(".*LoadManifest.*")` |
| `--framework FRAMEWORK --commands` | `cpg.call.name(".*commands.*")` |
| `--framework FRAMEWORK --capabilities` | `cpg.call.name(".*capabilities.*")` |
| `--flow FLUJO --dry-run` | `cpg.call.name(".*simulate.*")` |
| `--flow FLUJO --validate` | `cpg.call.name(".*validate.*")` |
| `--flow FLUJO --execute` | `cpg.call.name(".*execute.*")` |
| `--flow FLUJO --trace` | `cpg.call.name(".*trace.*")` |
| `--flow FLUJO --dependencies` | `cpg.call.name(".*dependency.*")` |
| `--flow FLUJO --gaps` | `cpg.call.name(".*gap.*")` |
| `--step N --inspect-inputs` | `cpg.call.name(".*inspectInput.*")` |
| `--step N --inspect-outputs` | `cpg.call.name(".*inspectOutput.*")` |

---

## Mapeo de Frameworks → Comandos de la CLI

| Framework | Comandos CLI que lo usan | Capabilities | Inputs | Outputs |
|-----------|-------------------------|--------------|--------|---------|
| **alfa** | compile, inspect, export-bravo | compilation | echo_tree | alfa_spec, ideal_flow |
| **bravo** | verification, trace comparison | verification, trace | ideal_flow, trace | verification_report |
| **paladin** | audit, explain, status | tracing, audit | audit_path, trace_path | audit_report |
| **quine** | (índice de frameworks) | discovery, catalog | - | framework_list |
| **echo** | next-question, ingest-answer | conversation | tree | tree |
| **sabio** | (QA semántico) | sql-qa | business_data | answers |
| **foco** | (priorización) | prioritization | tasks, contacts | priority_list |
| **mecanico** | (fixers) | remediation | issues | fixes |

---

## Resumen de Queries

| Categoría | Cantidad de Queries |
|-----------|---------------------|
| Símbolos | 12 |
| Interacción | 25 |
| Validación | 16 |
| Integridad | 22 |
| **TOTAL** | **75** |

<!-- CHAIN_RUN_ID: c28cea68 -->
