# Axiomas Extraídos - remora flow debug CLI

## Axiomas de Símbolos

### [CLI Remora]
- **A1**: La CLI debe aceptar comandos con flags en formato `--comando` o `--comando valor`
- **A2**: La CLI debe soportar formato de salida `--format [json|yaml|table]`
- **A3**: La CLI debe presentar resultados en columnas alineadas (NOMBRE 20chars, VERSION 10chars, CAPABILITIES 40chars, GRUPOS 15chars)
- **A4**: La CLI debe mostrar errores con información accionable (comandos sugeridos, valores válidos)
- **A5**: La CLI debe tener modo interactivo con prompt `remora>`

### [CatalogoFrameworks]
- **A6**: Debe retornar lista completa de frameworks con: nombre, version, capabilities, grupos
- **A7**: Debe verificar existencia de framework antes de cargar manifest
- **A8**: Debe devolver NOT_FOUND cuando el framework no existe, incluyendo lista de disponibles

### [FrameworkLoader]
- **A9**: Debe cargar metadatos completos de cada framework
- **A10**: Debe retornar manifest estructurado con: capabilities, inputs requeridos, outputs producidos, configuracion, dependencias
- **A11**: Debe indicar tipo de cada capability: [source, transform, validate, deliver]
- **A12**: Debe mostrar flags de idempotencia por capability

### [GestordeFlujos]
- **A13**: Debe resolver y cargar definiciones de flujo por nombre
- **A14**: Debe retornar flujo con sus pasos numerados y comandos asociados
- **A15**: Debe verificar que el flujo existe antes de procesar

### [Planificador]
- **A16**: Debe calcular orden topológico de ejecución basado en dependencias
- **A17**: Debe identificar inputs requeridos vs inputs disponibles de pasos anteriores
- **A18**: Debe generar timeline estimado sin ejecutar efectos reales
- **A19**: Debe reportar inputs faltantes que el usuario debe proporcionar

### [GestorDependencias]
- **A20**: Debe resolver dependencias entre pasos en orden topológico
- **A21**: Debe construir grafo con nodos (frameworks) y aristas (datos)
- **A22**: Debe detectar ciclos en dependencias y reportar ERROR_CICLO

### [MotordeEjecucion]
- **A23**: Debe ejecutar pasos secuencialmente en orden planificado
- **A24**: Debe pasar outputs de un paso como inputs al siguiente
- **A25**: Debe capturar eventos de cada paso con timestamp

### [TrazaEjecucion]
- **A26**: Debe registrar eventos: FLOW_START, STEP_START, INPUT_ACQUIRED, STEP_COMPLETE, FLOW_COMPLETE
- **A27**: Debe incluir timestamp ISO8601 en cada evento
- **A28**: Debe mostrar payload completo de inputs y outputs
- **A29**: Debe mascarar datos sensibles (credenciales) en traza

### [ValidadorFlujo]
- **A30**: Debe verificar existencia de todos los frameworks
- **A31**: Debe verificar que inputs requeridos tienen valores o son derivables
- **A32**: Debe verificar que no hay ciclos en dependencias
- **A33**: Debe verificar permisos suficientes (lectura, escritura, red)
- **A34**: Debe retornar VALID o INVALID con lista de checks realizados

### [AnalizadorGAPs]
- **A35**: Debe analizar chain de datos paso a paso
- **A36**: Debe detectar DATA_MISMATCH cuando output anterior no coincide con input requerido
- **A37**: Debe sugerir comando para auto-corregir gap

### [GeneradorTimeline]
- **A38**: Debe calcular timestamps simulados para cada paso
- **A39**: Debe estimar duración por paso basado en tipo de framework
- **A40**: Debe mostrar tiempo total estimado del flujo

### [CatalogoFlujos]
- **A41**: Debe almacenar registro de flujos existentes
- **A42**: Debe devolver flujos disponibles cuando se solicita lista
- **A43**: Debe devolver NOT_FOUND cuando flujo no existe, incluyendo flujos disponibles

### [HistorialEjecuciones]
- **A44**: Debe almacenar run_id, flujo, fecha, duracion, estado de cada ejecución
- **A45**: Debe permitir recuperar traza por run_id

---

## Axiomas de Interacción

### Exploración de Frameworks
- **A46**: Cuando Desarrollador ejecuta `--list-frameworks`, CLI debe consultar CatalogoFrameworks y formatear tabla
- **A47**: Cuando Desarrollador ejecuta `--manifest FRAMEWORK`, CLI debe cargar manifest y presentar YAML-like legible
- **A48**: Cuando Desarrollador ejecuta `--framework FRAMEWORK --commands`, CLI debe devolver lista de comandos con flags globales
- **A49**: Cuando Desarrollador ejecuta `--framework FRAMEWORK --capabilities`, CLI debe devolver capabilities con tipo, inputs, outputs, idempotencia

### Simulación de Flujo
- **A50**: Cuando Desarrollador ejecuta `--flow FLUJO --dry-run`, CLI debe solicitar plan a Planificador
- **A51**: Cuando Planificador recibe flujo, debe resolver dependencias con GestorDependencias
- **A52**: Cuando GestorDependencias resuelve, debe devolver orden topológico con inputs disponibles
- **A53**: Cuando Planificador recibe orden, debe generar timeline con GeneradorTimeline
- **A54**: CLI debe presentar dry-run completo con: secuencia de pasos, dependencias, inputs requeridos, outputs producidos, warnings

### Trazado de Ejecución
- **A55**: Cuando Desarrollador ejecuta `--flow FLUJO --trace`, CLI debe iniciar captura de TrazaEjecucion
- **A56**: Cuando MotordeEjecucion ejecuta paso, debe registrar evento STEP_START en TrazaEjecucion
- **A57**: Cuando paso completa, debe registrar STEP_COMPLETE con outputs
- **A58**: CLI debe presentar traza formateada con timestamps, eventos, payloads

### Ejecución de Flujo
- **A59**: Cuando Desarrollador ejecuta `--flow FLUJO --execute`, CLI debe validar con ValidadorFlujo primero
- **A60**: Si validación pasa, CLI debe mostrar confirmación con efectos laterales
- **A61**: Si usuario confirma (escribe "ejecutar"), MotordeEjecucion inicia
- **A62**: MotordeEjecucion ejecuta pasos en orden, pasando outputs entre pasos
- **A63**: CLI presenta resultado final con estado SUCCESS/FAILED, duración, artifacts generados

### Ejecución de Paso Individual
- **A64**: Cuando Desarrollador ejecuta `--step N`, CLI debe verificar disponibilidad de inputs
- **A65**: Si inputs no disponibles, CLI debe mostrar opciones: ejecutar desde paso 1, mock data, generar mock
- **A66**: Si inputs disponibles, MotordeEjecucion ejecuta solo ese paso

### Validación de Flujo
- **A67**: Cuando Desarrollador ejecuta `--flow FLUJO --validate`, ValidadorFlujo ejecuta todos los checks
- **A68**: ValidadorFlujo debe verificar frameworks con CatalogoFrameworks
- **A69**: ValidadorFlujo debe verificar chain de datos con AnalizadorGAPs
- **A70**: CLI presenta resultado VALID/INVALID con checks realizados

### Análisis de Dependencias
- **A71**: Cuando Desarrollador ejecuta `--flow FLUJO --dependencies`, CLI muestra grafo y matrix
- **A72**: Grafo debe mostrar nodos (frameworks) y aristas (datos)
- **A73**: Matrix debe mostrar grid Framework x Framework con datos que fluyen
- **A74**: Stats debe mostrar: frameworks únicos, pasos totales, reuso, depth max

### Detección de Gaps
- **A75**: Cuando Desarrollador ejecuta `--flow FLUJO --gaps`, AnalizadorGAPs analiza paso a paso
- **A76**: Para cada gap, debe mostrar: tipo, ubicación, problema, sugerencia, comando sugerido
- **A77**: Gap types: DATA_MISMATCH, INPUT_MISSING, CAPABILITY_MISSING

### Inspección de Outputs/Inputs
- **A78**: Cuando Desarrollador ejecuta `--step N --inspect-outputs`, CLI muestra metadata del artifact
- **A79**: Metadata incluye: tipo, tamaño, schema (si Array), muestra de primeros 3 registros
- **A80**: Cuando Desarrollador ejecuta `--step N --inspect-inputs`, CLI muestra inputs requeridos
- **A81**: Para cada input: tipo, requerido, source esperada, schema esperado, faltante o disponible

---

## Axiomas de Validación

### Comandos Obligatorios (CRITICAL)
- **V1**: `--list-frameworks` debe funcionar siempre y listar todos los frameworks disponibles
- **V2**: `--manifest FRAMEWORK` debe mostrar estructura completa del framework
- **V3**: `--flow FLUJO --dry-run` debe simular sin ejecutar efectos reales
- **V4**: `--flow FLUJO --execute` debe validar primero, ejecutar luego, reportar resultado
- **V5**: `--flow FLUJO --validate` debe verificar estructura y consistencia

### Comandos Importantes (IMPORTANT)
- **V6**: `--flow FLUJO --trace` debe mostrar timeline completo con eventos
- **V7**: `--flow FLUJO --dependencies` debe mostrar grafo de dependencias
- **V8**: `--flow FLUJO --gaps` debe identificar y sugerir correcciones
- **V9**: `--flow FLUJO --step N --inspect-inputs` debe detallar inputs requeridos
- **V10**: `--flow FLUJO --step N --inspect-outputs` debe mostrar metadata de artifact

### Comandos Nice-to-Have
- **V11**: `--flow FLUJO --capabilities-map` debe mostrar mapa visual de capabilities
- **V12**: `--flow FLUJO --timeline` debe mostrar timeline estimado (puede integrarse con dry-run)
- **V13**: `--history` debe mostrar historial de ejecuciones
- **V14**: `--interactive` debe permitir modo interactivo
- **V15**: `--config init` debe permitir wizard de configuración

### Validación de Inputs
- **V16**: Antes de ejecutar cualquier flujo, todos los inputs requeridos deben tener valor o ser derivables
- **V17**: Si input requerido no disponible, CLI debe ofrecer opciones: --input, --mock-input, --generate-mock
- **V18**: Si --ask-prompts, CLI debe solicitar interactivamente cada input faltante

### Validación de Estructura
- **V19**: Flujo debe tener al menos 1 paso
- **V20**: Cada paso debe referenciar framework existente
- **V21**: Dependencias no deben formar ciclos
- **V22**: Outputs de paso N deben coincidir con inputs de paso N+1 (o gap debe ser detectado)

### Flags Globales
- **V23**: `--help` o `-h` debe funcionar en todos los comandos
- **V24**: `--format [json|yaml|table]` debe estar disponible en todos los comandos
- **V25**: `--output FILE` debe redirigir salida a archivo
- **V26**: `--verbose` o `-v` debe mostrar detalles adicionales

---

## Axiomas de Integridad

### Consistencia de Datos
- **I1**: Outputs producidos por un paso deben fluir como inputs al siguiente paso sin pérdida
- **I2**: El chain de datos debe mantenerse: raw_data → normalized_data → notification_batch → delivery_receipts
- **I3**: Si hay DATA_MISMATCH, debe ser detectado y reportado antes de ejecución

### Trazabilidad
- **I4**: Cada ejecución debe generar un run_id único
- **I5**: Cada ejecución debe ser grabada en historial con run_id, flujo, fecha, duración, estado
- **I6**: Traza debe ser recuperable por run_id
- **I7**: Timestamp de eventos debe ser ISO8601 UTC

### Seguridad
- **I8**: Datos sensibles (credenciales, passwords) deben ser mascarados en traza
- **I9**: Credenciales deben referenciarse por nombre, no incluir valor en traza

### Formato de Salida
- **I10**: Output table debe usar ancho fijo: NOMBRE(20), VERSION(10), CAPABILITIES(40), GRUPOS(15)
- **I11**: Output JSON debe tener estructura: {command, status, timestamp, data}
- **I12**: Output YAML debe ser parseable y válido

### Estado de Ejecución
- **I13**: Cada paso puede estar en: waiting, running, success, failed, skipped
- **I14**: Flujo completo puede estar en: pending, running, completed, failed
- **I15**: Progreso debe mostrar: [N/M] paso, estado, tiempo transcurrido/estimado

### Manejo de Errores
- **I16**: Error debe incluir mensaje descriptivo, no solo código
- **I17**: Error debe incluir acciones disponibles
- **I18**: Si framework no existe, incluir lista de frameworks disponibles
- **I19**: Si flujo no existe, incluir lista de flujos disponibles
- **I20**: Si ciclo detectado, indicar path del ciclo

### Confirmación de Efectos
- **I21**: Antes de ejecutar que escriba archivos, envíe emails, o acceda red, CLI debe confirmar
- **I22**: Confirmación debe pedir escribir "ejecutar" explícitamente

---

## Comandos Priorizados por Criticidad

### CRITICAL (Deben existir en MVP)
```
remora flow debug --list-frameworks
remora flow debug --manifest FRAMEWORK
remora flow debug --flow FLUJO --dry-run
remora flow debug --flow FLUJO --validate
remora flow debug --flow FLUJO --execute
remora flow debug --flow FLUJO --trace
```

### IMPORTANT (Deben existir en v1.0)
```
remora flow debug --flow FLUJO --step N --inspect-inputs
remora flow debug --flow FLUJO --step N --inspect-outputs
remora flow debug --flow FLUJO --dependencies
remora flow debug --flow FLUJO --gaps
remora flow debug --flow FLUJO --frameworks-only
remora flow debug --framework FRAMEWORK --commands
remora flow debug --framework FRAMEWORK --capabilities
```

### NICE-TO-HAVE (Post v1.0)
```
remora flow debug --flow FLUJO --capabilities-map
remora flow debug --history
remora flow debug --run RUN-ID --trace
remora flow debug --diff RUN-ID RUN-ID
remora flow debug --interactive
remora flow debug --config init
remora flow debug --cache inspect
```

---

## Formato de Metadata de Outputs

### Tabla (default)
```
===========================
TITULO: Descripcion
===========================

COLUMNA1      COLUMNA2      COLUMNA3
---------     ---------     ---------
valor         valor         valor

===========================
STATUS: SUCCESS/ERROR
===========================
```

### JSON
```json
{
  "command": "list-frameworks",
  "status": "success",
  "timestamp": "2026-05-16T14:30:00Z",
  "data": {
    "frameworks": [
      {
        "name": "alfa",
        "version": "1.2.0",
        "capabilities": ["ingestion", "validation"],
        "groups": ["data-ingestion"]
      }
    ]
  }
}
```

### YAML
```yaml
command: list-frameworks
status: success
timestamp: 2026-05-16T14:30:00Z
data:
  frameworks:
    - name: alfa
      version: "1.2.0"
      capabilities: [ingestion, validation]
```

---

## Estados de Progreso Durante Ejecución

### En progreso
```
  [1/5] alfa.ingest ................... running
  [2/5] beta.normalize ................. waiting
  Tiempo: 0:02 / 0:05 estimado
```

### Éxito
```
  [1/5] alfa.ingest ................... SUCCESS (2.0s)
  COMPLETADO: 5/5 pasos en 5.2s
```

### Error con acciones
```
  [3/5] beta.calculate ................. FAILED
  ERROR: Invalid fee calculation
  
  ACCIONES DISPONIBLES:
    [R] Retry paso 3
    [S] Skip paso 3 y continuar
    [A] Abortar flujo
    [D] Debug paso 3
```

<!-- CHAIN_RUN_ID: c28cea68 -->
