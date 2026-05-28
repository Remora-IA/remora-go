# Historia Ideal - Flujo Documentativo

## remora flow debug: CLI de Control y Visibilidad Total

Esta narrativa describe el comportamiento IDEAL de una interfaz de línea de comandos para inspeccionar, probar y depurar flujos de Remora. El sistema permite a los desarrolladores tener visibilidad completa sobre qué ejecuta su flujo, qué necesita, qué produce, y validar su configuración antes de la ejecución real.

---

## 1. Símbolos del Sistema

### Actores (principales)
- **[Desarrollador]**: Usuario que opera la CLI
- **[CLI Remora]**: Interfaz de comandos que recibe instrucciones
- **[Gestor de Flujos]**: Componente que resuelve y carga definiciones de flujo

### Catálogos del Sistema
- **[CatalogoFrameworks]**: Registro de todos los frameworks disponibles
- **[CatalogoFlujos]**: Registro de definiciones de flujo existentes
- **[GestorDependencias]**: Resolvedor de dependencias entre frameworks

### Componentes de Ejecución
- **[MotordeEjecucion]**: Motor que ejecuta pasos de flujo
- **[Planificador]**: Calcula el orden de ejecución y dependencias
- **[TrazaEjecucion]**: Captura eventos de ejecución en tiempo real

### Entidades de Datos
- **[Flujo]**: Definición completa de un flujo
- **[Paso]**: Un nodo individual dentro de un flujo
- **[Framework]**: Componente reusable que provee capabilities
- **[Capability]**: Habilidad específica que un framework expone
- **[Manifest]**: Definición de inputs, outputs y configuración de un framework
- **[Dependencia]**: Relación entre frameworks que indica orden y datos requeridos
- **[Artifact]**: Resultado producido por la ejecución de un paso

### Utilidades de Inspección
- **[ValidadorFlujo]**: Verifica completitud y consistencia de un flujo
- **[AnalizadorGAPs]**: Identifica missing links entre pasos
- **[GeneradorTimeline]**: Calcula timestamps y secuencia de ejecución

---

## 2. Narrativa por Operación

### 2.1 EXPLORAR: Listar Frameworks Disponibles

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --list-frameworks"

[CLI Remora] -> [CatalogoFrameworks]: solicita lista completa de frameworks

[CatalogoFrameworks] -> [FrameworkLoader]: pide carga de metadatos de cada framework

[FrameworkLoader] -> [CatalogoFrameworks]: devuelve lista de frameworks con:
  - nombre: identificador del framework
  - version: versión semver
  - capabilities: lista de capacidades que provee
  - grupos: categorías a las que pertenece

[CatalogoFrameworks] -> [CLI Remora]: devuelve tabla formateada:

  NOMBRE          VERSION   CAPABILITIES                    GRUPOS
  alfa            1.2.0     ingestion,validation            [data-ingestion]
  beta            0.9.1     transformation,enrichment        [data-processing]
  foco           2.0.0     execution,orchestration          [core]
  delta          1.0.0     output,notification             [delivery]
  ...

[CLI Remora] -> [Desarrollador]: presenta lista con columnas:
  -NOMBRE (20 chars)
  -VERSION (10 chars)
  -CAPABILITIES (40 chars)
  -GRUPOS (15 chars)
```

### 2.2 EXPLORAR: Ver Manifest de un Framework

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --manifest alfa"

[CLI Remora] -> [CatalogoFrameworks]: solicita manifest del framework "alfa"

[CatalogoFrameworks] -> [FrameworkLoader]: pide carga del manifest

[FrameworkLoader] -> [CatalogoFrameworks]: devuelve manifest estructurado:

  Framework: alfa
  Version: 1.2.0
  
  Capabilities:
    - ingestion: datos externos -> formato interno
    - validation: validar estructura y contenido
  
  Inputs requeridos:
    - source_url (string): URL del origen de datos
    - source_type (enum): [api, file, stream, database]
    - credentials_ref (string): referencia a credenciales en vault
  
  Outputs producidos:
    - raw_data (buffer): datos en formato raw
    - schema (object): esquema inferido de los datos
  
  Configuracion:
    - timeout_ms: 30000
    - retry_attempts: 3
    - batch_size: 1000
  
  Dependencias externas:
    - requiere: [delta] para logging
    -冲突a con: [gamma] (mutuamente excluyentes)

[CLI Remora] -> [Desarrollador]: presenta manifest en formato YAML-like legible
```

### 2.3 EXPLORAR: Ver Comandos de un Framework

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --framework alfa --commands"

[CLI Remora] -> [CatalogoFrameworks]: solicita comandos disponibles del framework

[CatalogoFrameworks] -> [FrameworkLoader]: pide lista de comandos

[FrameworkLoader] -> [CatalogoFrameworks]: devuelve:

  Comandos disponibles para 'alfa':
  
  ingest           Ejecuta ingestion completa
  validate         Valida sin ingest (dry-validate)
  schema-infer     Infiere esquema sin procesar datos
  batch-process    Procesa por lotes
  
  Flags globales:
    --dry-run      Simula sin ejecutar efectos secundarios
    --verbose      Muestra logs detallados
    --format       [json|yaml|table]

[CLI Remora] -> [Desarrollador]: presenta lista de comandos y flags
```

### 2.4 EXPLORAR: Ver Capabilities de un Framework

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --framework alfa --capabilities"

[CLI Remora] -> [CatalogoFrameworks]: solicita capabilities del framework

[CatalogoFrameworks] -> [FrameworkLoader]: pide detalle de capabilities

[FrameworkLoader] -> [CatalogoFrameworks]: devuelve:

  Capabilities de 'alfa':
  
  [ingestion]
    descripcion: "Ingesta datos de fuentes externas"
    tipo: source
    inputs: [source_url, source_type, credentials_ref]
    outputs: [raw_data, schema]
    idempotente: false
  
  [validation]
    descripcion: "Valida estructura y contenido de datos"
    tipo: transform
    inputs: [raw_data, validation_rules_ref]
    outputs: [validation_report, is_valid]
    idempotente: true

[CLI Remora] -> [Desarrollador]: presenta capabilities con:
  -Nombre
  -Descripcion textual
  -Tipo (source/transform/validate/deliver)
  -Inputs esperados
  -Outputs producidos
  -Flags de idempotencia
```

### 2.5 SIMULAR: Dry-Run de un Flujo Completo

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza-mensual --dry-run"

[CLI Remora] -> [GestordeFlujos]: solicita definicion del flujo "cobranza-mensual"

[GestordeFlujos] -> [CatalogoFlujos]: pide registro del flujo

[CatalogoFlujos] -> [GestordeFlujos]: devuelve definicion:

  Flujo: cobranza-mensual
  Pasos:
    1. alfa (ingest from api)
    2. beta (transform: normalize dates)
    3. beta (transform: calculate fees)
    4. foco (execute: send notifications)
    5. delta (deliver: generate report)

[GestordeFlujos] -> [Planificador]: solicita plan de ejecucion

[Planificador] -> [GestorDependencias]: resuelve dependencias entre pasos

[GestorDependencias] -> [Planificador]: devuelve orden topologico:

  Orden de ejecucion calculado:
  
  [Step 1: alfa.ingest]
    dependencias: ninguna
    inputs_requeridos:
      source_url: "https://api.deudores.example/v2/deudas"
      source_type: "api"
    inputs_disponibles: ninguno (debe proporcionar)
  
  [Step 2: beta.normalize]
    dependencias: [Step 1]
    inputs_requeridos:
      raw_data: desde Step 1.raw_data
    inputs_disponibles: raw_data ( Step 1 )
  
  [Step 3: beta.calculate]
    dependencias: [Step 2]
    inputs_requeridos:
      normalized_data: desde Step 2.output
    inputs_disponibles: normalized_data ( Step 2 )
  
  [Step 4: foco.execute]
    dependencias: [Step 3]
    inputs_requeridos:
      notification_batch: desde Step 3.output
    inputs_disponibles: notification_batch ( Step 3 )
  
  [Step 5: delta.report]
    dependencias: [Step 4]
    inputs_requeridos:
      delivery_receipts: desde Step 4.output
    inputs_disponibles: delivery_receipts ( Step 4 )

[Planificador] -> [GeneradorTimeline]: genera timestamps simulados

[GeneradorTimeline] -> [Planificador]: devuelve timeline:

  Timeline simulado (sin ejecucion real):
  
  T+0.000s  [INIT] Inicializando flujo
  T+0.050s  [STEP 1] alfa.ingest inicia
  T+2.100s  [STEP 1] alfa.ingest completa (2.050s)
  T+2.150s  [STEP 2] beta.normalize inicia
  T+2.350s  [STEP 2] beta.normalize completa (0.200s)
  T+2.400s  [STEP 3] beta.calculate inicia
  T+3.800s  [STEP 3] beta.calculate completa (1.400s)
  T+3.850s  [STEP 4] foco.execute inicia
  T+5.200s  [STEP 4] foco.execute completa (1.350s)
  T+5.250s  [STEP 5] delta.report inicia
  T+5.550s  [STEP 5] delta.report completa (0.300s)
  T+5.550s  [DONE] Flujo completo: 5.550s estimado

[Planificador] -> [CLI Remora]: devuelve dry-run completo

[CLI Remora] -> [Desarrollador]: presenta:

  ===========================
  DRY-RUN: cobranza-mensual
  ===========================
  
  Flujo completo: 5 pasos
  Duracion estimada: 5.550s
  Frameworks usados: alfa, beta, beta, foco, delta
  
  SECUENCIA DE EJECUCION:
  
  [1/5] alfa.ingest (2.050s estimado)
        Inputs requeridos por usuario:
          source_url = ?
          source_type = ?
        Outputs producir:
          raw_data: Buffer
          schema: Object
  
  [2/5] beta.normalize (0.200s estimado)
        Depende de: [1/5]
        Inputs desde paso anterior:
          raw_data <- alfa.raw_data
        Outputs producir:
          normalized_data: Array
  
  [3/5] beta.calculate (1.400s estimado)
        Depende de: [2/5]
        Inputs desde paso anterior:
          normalized_data <- beta.normalized_data
        Outputs producir:
          notification_batch: Array
  
  [4/5] foco.execute (1.350s estimado)
        Depende de: [3/5]
        Inputs desde paso anterior:
          notification_batch <- beta.notification_batch
        Outputs producir:
          delivery_receipts: Array
  
  [5/5] delta.report (0.300s estimado)
        Depende de: [4/5]
        Inputs desde paso anterior:
          delivery_receipts <- foco.delivery_receipts
        Outputs producir:
          report_file: File
          summary: Object
  
  ===========================
  ADVERTENCIAS DE INPUT:
  ===========================
  Faltan 2 inputs requeridos por el usuario:
    - source_url (Step 1)
    - source_type (Step 1)
  
  Use --fill-inputs para proporcionar valores o
  --ask-prompts para solicitar interactivamente.
```

### 2.6 TRAZAR: Timeline Completo con Detalle de Inputs/Outputs

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza-mensual --trace"

[CLI Remora] -> [GestordeFlujos]: solicita flujo y pide traza completa

[GestordeFlujos] -> [TrazaEjecucion]: inicia captura de eventos

[TrazaEjecucion]: registra cada evento con timestamp, tipo, payload

[TrazaEjecucion] -> [CLI Remora]: devuelve traza formateada:

  ===========================
  TRACE: cobranza-mensual
  ===========================
  Inicio: 2026-05-16T14:30:00Z
  Fin: 2026-05-16T14:30:05.550Z
  Duracion: 5.550s
  Estado final: COMPLETED
  
  EVENTOS:
  
  [14:30:00.000] ◉ FLOW_START
    flujo: cobranza-mensual
    version: v2.1.0
    run_id: run-abc123
  
  [14:30:00.050] → STEP_START (step=1, framework=alfa)
    comando: ingest
    inputs:
      source_url: "https://api.deudores.example/v2/deudas"
      source_type: "api"
  
  [14:30:00.100] INPUT_ACQUIRED
    recurso: source_url
    resolved_from: config.env
    valor: "[MASCARADO]"
  
  [14:30:02.150] STEP_COMPLETE (step=1)
    outputs:
      raw_data:
        tipo: Buffer
        tamano: 2.4MB
        formato: json-array
        muestra: "[{id:1,monto:1500...}]"
      schema:
        campos: 12
        tipos_inferidos: {id:int, monto:decimal, ...}
  
  [14:30:02.200] → STEP_START (step=2, framework=beta)
    comando: normalize
    inputs:
      raw_data: (referencia a step=1.output)
  
  [14:30:02.350] STEP_COMPLETE (step=2)
    outputs:
      normalized_data:
        registros: 1,247
        formato: "{deudas_normalized}"
        transformaciones_aplicadas:
          - formato_fecha: DD/MM/YYYY -> ISO8601
          - monto_decimal: separador_coma -> punto
          - telefono: normalizado E164
  
  [14:30:02.400] → STEP_START (step=3, framework=beta)
    comando: calculate
    inputs:
      normalized_data: (referencia a step=2.output)
  
  [14:30:03.800] STEP_COMPLETE (step=3)
    outputs:
      notification_batch:
        total_notificaciones: 1,247
        por_tipo:
          email: 892
          sms: 355
        por_estado:
          activos: 1,198
          inactivos: 49
  
  [14:30:03.850] → STEP_START (step=4, framework=foco)
    comando: execute
    inputs:
      notification_batch: (referencia a step=3.output)
  
  [14:30:05.200] STEP_COMPLETE (step=4)
    outputs:
      delivery_receipts:
        exitosos: 1,195
        fallidos: 52
        detalles:
          - tipo: email
            enviados: 892
            fallidos: 15
          - tipo: sms
            enviados: 303
            fallidos: 37
  
  [14:30:05.250] → STEP_START (step=5, framework=delta)
    comando: report
    inputs:
      delivery_receipts: (referencia a step=4.output)
  
  [14:30:05.550] STEP_COMPLETE (step=5)
    outputs:
      report_file:
        ruta: "/reports/cobranza-2026-05-16.json"
        tamano: 45KB
      summary:
        total_deudas: 1,247
        total_monto: "$15,234,567.89"
        notificacion_exito: "96.2%"
  
  [14:30:05.550] ◉ FLOW_COMPLETE
    duracion_total: 5.550s
    pasos_completados: 5/5
    estado: SUCCESS

[CLI Remora] -> [Desarrollador]: presenta timeline interactivo:
  - Presiona ENTER para navegar eventos
  - --filter "step=2" para ver solo paso 2
  - --output json para formato machine-readable
```

### 2.7 PROBAR: Ejecutar Paso Individual

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza-mensual --step 3"

[CLI Remora] -> [GestordeFlujos]: solicita definicion del flujo

[GestordeFlujos] -> [CatalogoFlujos]: obtiene flujo "cobranza-mensual"

[CatalogoFlujos] -> [GestordeFlujos]: devuelve definicion con 5 pasos

[GestordeFlujos] -> [GestorDependencias]: resuelve inputs del paso 3

[GestorDependencias] -> [GestordeFlujos]: verifica disponibilidad de inputs:

  Paso 3: beta.calculate
  Inputs requeridos:
    - normalized_data: Array
    - fee_rules: reference
  
  Inputs disponibles:
    - normalized_data: NO DISPONIBLE ( Step 2 no ha sido ejecutado )
  
  Estado: NO PUEDE EJECUTAR
  
  Solucion requerida:
    Opcion A: Ejecutar pasos previos primero (--from-step 1)
    Opcion B: Proporcionar normalized_data manualmente (--mock-input)
    Opcion C: Generar mock data (--generate-mock)

[GestordeFlujos] -> [CLI Remora]: devuelve estado con opciones

[CLI Remora] -> [Desarrollador]: presenta:

  ===========================
  STEP EXECUTION: paso 3
  ===========================
  
  Framework: beta
  Comando: calculate
  
  BLOQUEO: Faltan inputs requeridos
  ---------------------------------
  normalized_data: no disponible
    - Paso anterior (step 2) no ha sido ejecutado
    - Sin mock data proporcionado
  
  ACCIONES DISPONIBLES:
  
  [A] Ejecutar desde paso 1
      "remora flow debug --flow cobranza-mensual --from-step 1 --to-step 3"
      Ejecutara: paso 1, paso 2, paso 3
  
  [B] Proporcionar mock data
      "remora flow debug --flow cobranza-mensual --step 3 --mock-input normalized_data=@data/test-sample.json"
      Usara datos de archivo como input
  
  [C] Generar mock automaticamente
      "remora flow debug --flow cobranza-mensual --step 3 --generate-mock"
      Generara datos de prueba basados en schema
  
  [D] Ver inputs requeridos en detalle
      "remora flow debug --flow cobranza-mensual --step 3 --inspect-inputs"
  
  Seleccione opcion o presione 'A' para ejecutar desde inicio:
```

### 2.8 PROBAR: Ejecutar Flujo Completo

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza-mensual --execute"

[CLI Remora] -> [ValidadorFlujo]: valida flujo antes de ejecutar

[ValidadorFlujo] -> [GestordeFlujos]: pide flujo completo

[GestordeFlujos] -> [CatalogoFlujos]: obtiene definicion

[CatalogoFlujos] -> [ValidadorFlujo]: devuelve flujo

[ValidadorFlujo]: ejecuta validaciones:
  - Todos los frameworks existen
  - Inputs requeridos tienen valores o son derivables
  - Dependencias no forman ciclos
  - Permisos suficientes

[ValidadorFlujo] -> [CLI Remora]: resultado de validacion:

  VALIDACION: PASSED
  - 5 pasos validados
  - 2 inputs proporcionados por usuario
  - 3 inputs derivados de pasos anteriores
  - 0 errores, 1 advertencia

[ValidadorFlujo] -> [CLI Remora]: advertencia:

  ADVERTENCIA:
  - Paso 4 (foco.execute) accedera a red externa
    Considere usar --dry-run si no desea ejecutar efectos reales

[CLI Remora] -> [Desarrollador]: presenta confirmacion:

  ===========================
  EXECUTE: cobranza-mensual
  ===========================
  
  Flujo: cobranza-mensual
  Pasos: 5
  
  VALIDACION: PASSED (con 1 advertencia)
  
  SECUENCIA:
    1. alfa.ingest
    2. beta.normalize
    3. beta.calculate
    4. foco.execute
    5. delta.report
  
  EFECTOS LATERALES:
    - Paso 1: Lectura de API externa
    - Paso 4: Envio de emails y SMS
    - Paso 5: Escritura de archivo
  
  CONFIRMAR EJECUCION:
  Escriba "ejecutar" para continuar o Ctrl+C para cancelar.
```

Si el usuario confirma:

```
[Desarrollador] -> [CLI Remora]: "ejecutar"

[MotordeEjecucion] -> [TrazaEjecucion]: inicia captura

[MotordeEjecucion] -> [Paso1 alfa.ingest]: ejecuta paso 1

[Paso1 alfa.ingest] -> [MotordeEjecucion]: completada, outputs disponibles

[MotordeEjecucion] -> [Paso2 beta.normalize]: ejecuta paso 2

[Paso2 beta.normalize] -> [MotordeEjecucion]: completada

[MotordeEjecucion] -> [Paso3 beta.calculate]: ejecuta paso 3

[Paso3 beta.calculate] -> [MotordeEjecucion]: completada

[MotordeEjecucion] -> [Paso4 foco.execute]: ejecuta paso 4

[Paso4 foco.execute] -> [MotordeEjecucion]: completada

[MotordeEjecucion] -> [Paso5 delta.report]: ejecuta paso 5

[Paso5 delta.report] -> [MotordeEjecucion]: completada

[MotordeEjecucion] -> [TrazaEjecucion]: genera traza completa

[TrazaEjecucion] -> [CLI Remora]: devuelve eventos formateados

[CLI Remora] -> [Desarrollador]: presenta resultado:

  ===========================
  EXECUTION COMPLETE
  ===========================
  
  Flujo: cobranza-mensual
  Estado: SUCCESS
  Duracion: 5.550s
  Run ID: run-abc123
  
  RESULTADOS POR PASO:
  
  [1/5] alfa.ingest
    Estado: SUCCESS
    Duracion: 2.050s
    Outputs:
      raw_data: 2.4MB
      schema: 12 campos
  
  [2/5] beta.normalize
    Estado: SUCCESS
    Duracion: 0.200s
    Outputs:
      normalized_data: 1,247 registros
  
  [3/5] beta.calculate
    Estado: SUCCESS
    Duracion: 1.400s
    Outputs:
      notification_batch: 1,247 notificaciones
  
  [4/5] foco.execute
    Estado: SUCCESS
    Duracion: 1.350s
    Outputs:
      delivery_receipts: 1,195 exitosos, 52 fallidos
  
  [5/5] delta.report
    Estado: SUCCESS
    Duracion: 0.300s
    Outputs:
      report_file: /reports/cobranza-2026-05-16.json
  
  ARTIFACTS GENERADOS:
    - /reports/cobranza-2026-05-16.json
    - /logs/cobranza-2026-05-16.log
  
  Use --trace para ver timeline completo
  Use --inspect artifacts para ver outputs
```

### 2.9 ANALIZAR: Validar Flujo

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza-mensual --validate"

[CLI Remora] -> [ValidadorFlujo]: inicia validacion completa

[ValidadorFlujo] -> [GestordeFlujos]: obtiene flujo

[GestordeFlujos] -> [CatalogoFlujos]: obtiene definicion completa

[CatalogoFlujos] -> [ValidadorFlujo]: devuelve flujo con 5 pasos

[ValidadorFlujo] -> [CatalogoFrameworks]: verifica existencia de cada framework

[CatalogoFrameworks] -> [ValidadorFlujo]: confirmacion:
  - alfa: EXISTS (v1.2.0)
  - beta: EXISTS (v0.9.1)
  - foco: EXISTS (v2.0.0)
  - delta: EXISTS (v1.0.0)

[ValidadorFlujo] -> [AnalizadorGAPs]: analiza gaps entre pasos

[AnalizadorGAPs]: analiza flujo paso a paso:

  Paso 1 -> Paso 2: beta.normalize
    Input requerido: raw_data
    Output disponible de paso anterior: raw_data
    MATCH: OK
  
  Paso 2 -> Paso 3: beta.calculate
    Input requerido: normalized_data
    Output disponible de paso anterior: normalized_data
    MATCH: OK
  
  Paso 3 -> Paso 4: foco.execute
    Input requerido: notification_batch
    Output disponible de paso anterior: notification_batch
    MATCH: OK
  
  Paso 4 -> Paso 5: delta.report
    Input requerido: delivery_receipts
    Output disponible de paso anterior: delivery_receipts
    MATCH: OK

[AnalizadorGAPs] -> [ValidadorFlujo]: resultado: SIN GAPS

[ValidadorFlujo] -> [CLI Remora]: devuelve resultado formateado:

  ===========================
  VALIDATE: cobranza-mensual
  ===========================
  
  Estado: VALID
  
  CHECKS REALIZADOS:
  
  [OK] Estructura del flujo
       5 pasos en secuencia lineal
  
  [OK] Frameworks disponibles
       alfa: v1.2.0
       beta: v0.9.1
       foco: v2.0.0
       delta: v1.0.0
  
  [OK] Inputs de usuario
       source_url: proporcionado
       source_type: proporcionado
  
  [OK] Chain de datos
       raw_data -> normalized_data -> notification_batch -> delivery_receipts
       Todos los outputs fluyen correctamente a inputs
  
  [OK] Dependencias resueltas
       Sin ciclos detectados
       Orden topologico: valido
  
  [OK] Permisos
       Lectura API: OK
       Escritura archivos: OK
       Acceso red: OK
  
  ===========================
  RESUMEN: Flujo listo para ejecucion
  ===========================
```

### 2.10 ANALIZAR: Ver Dependencias entre Frameworks

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza-mensual --dependencies"

[CLI Remora] -> [GestordeFlujos]: solicita flujo

[GestordeFlujos] -> [CatalogoFlujos]: obtiene definicion

[CatalogoFlujos] -> [GestordeFlujos]: devuelve flujo

[GestordeFlujos] -> [GestorDependencias]: resuelve graph de dependencias

[GestorDependencias]: construye grafo:

  NODOS:
    alfa (ingestion)
    beta (transformation)
    foco (orchestration)
    delta (delivery)
  
  ARISTAS (dependencias de datos):
    alfa -> beta (raw_data)
    beta -> beta (normalized_data)
    beta -> foco (notification_batch)
    foco -> delta (delivery_receipts)

[GestorDependencias] -> [CLI Remora]: devuelve graph

[CLI Remora] -> [Desarrollador]: presenta:

  ===========================
  DEPENDENCIES: cobranza-mensual
  ===========================
  
  GRAPH:
  
    [alfa] ----raw_data----> [beta] ----normalized_data----> [beta]
                                                                  |
                                                           notification_batch
                                                                  |
                                                                  v
                                                              [foco] ----delivery_receipts----> [delta]
  
  MATRIX:
  
                alfa      beta      foco      delta
    alfa        -        raw_data    -          -
    beta        -          -      notification  -
                                batch
    foco        -          -          -      delivery
                                              receipts
    delta       -          -          -          -
  
  STATS:
    Frameworks unicos: 4 (alfa, beta, foco, delta)
    Pasos totales: 5
   重用 de framework: beta usado 2 veces
    Depth max: 4 niveles
  
  FRAMEWORKS NECESARIOS:
    - alfa (para ingestion de datos)
    - beta (para transformacion)
    - foco (para orquestacion)
    - delta (para delivery)
```

### 2.11 ANALIZAR: Identificar Gaps

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza-mensual --gaps"

[CLI Remora] -> [ValidadorFlujo]: inicia deteccion de gaps

[ValidadorFlujo] -> [GestordeFlujos]: obtiene flujo

[GestordeFlujos] -> [CatalogoFlujos]: obtiene flujo

[CatalogoFlujos] -> [ValidadorFlujo]: devuelve flujo de ejemplo:

  Flujo: cobranza-fallida
  Pasos:
    1. alfa.ingest
    2. beta.calculate (ERROR: no hay normalizacion)
    3. foco.execute

[ValidadorFlujo] -> [AnalizadorGAPs]: ejecuta analisis

[AnalizadorGAPs]: analiza chain de datos:

  Paso 1 (alfa) outputs:
    - raw_data: Buffer
    - schema: Object
  
  Paso 2 (beta) inputs requeridos:
    - normalized_data: Array  <- REQUERIDO
    - fee_rules: Reference    <- REQUERIDO
  
  Paso 2 outputs disponibles desde paso 1:
    - raw_data (disponible) pero no coincide con requerimiento
    - schema (disponible) pero no coincide con requerimiento
  
  GAP DETECTADO:
    Tipo: DATA_MISMATCH
    Paso: 2 (beta.calculate)
    Input faltante: normalized_data
    Razon: El paso anterior (alfa) produce raw_data, pero este paso
           espera normalized_data. Falta el paso de normalizacion.

[AnalizadorGAPs] -> [CLI Remora]: devuelve gaps detectados

[CLI Remora] -> [Desarrollador]: presenta:

  ===========================
  GAP ANALYSIS: cobranza-fallida
  ===========================
  
  GAPS ENCONTRADOS: 1
  
  [GAP-001] DATA_MISMATCH
  -----------------------
  Ubicacion: Paso 2 (beta.calculate)
  
  Problema:
    El input "normalized_data" no esta disponible.
    
    - Paso 2 requiere: normalized_data (Array)
    - Paso 1 produce: raw_data (Buffer), schema (Object)
    - Gap: No hay paso que transforme raw_data -> normalized_data
  
  Sugerencia:
    Insertar paso de normalizacion entre paso 1 y paso 2:
    
    Flujo corregido:
      1. alfa.ingest
      2. beta.normalize    <-- INSERTAR ESTE PASO
      3. beta.calculate
      4. foco.execute
  
  Comando sugerido:
    "remora flow debug --flow cobranza-fallida --fix-gap GAP-001"
  
  Alternativamente, si beta.calculate puede trabajar con raw_data:
    "remora flow debug --flow cobranza-fallida --declare-compatibility"
```

### 2.12 INSPECCIONAR: Artifact Inspection

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza-mensual --step 2 --inspect-outputs"

[CLI Remora] -> [GestordeFlujos]: solicita flujo y outputs del paso 2

[GestordeFlujos] -> [TrazaEjecucion]: busca artifact del paso 2

[TrazaEjecucion]: verifica si el paso ha sido ejecutado

[TrazaEjecucion]: comprueba si existe run previo en cache

[TrazaEjecucion]: devuelve artifact si existe

[TrazaEjecucion] -> [CLI Remora]: devuelve metadata del artifact:

  ===========================
  INSPECT OUTPUTS: paso 2
  ===========================
  
  Paso: beta.normalize
  Framework: beta v0.9.1
  Comando: normalize
  
  OUTPUTS PRODUCIDOS:
  
  [normalized_data]
    Tipo: Array
    Tamano: 1,247 elementos
    Schema:
      - id: integer (not null)
      - deudor_id: string (not null)
      - monto_original: decimal(12,2)
      - monto_normalizado: decimal(12,2)
      - fecha_deuda: date (ISO8601)
      - fecha_vencimiento: date (ISO8601)
      - telefono: string (E164)
      - email: string (valid format)
      - status: enum [active, inactive, paid]
      - observaciones: text (nullable)
    Muestra (primeros 3):
      {id:1, monto_normalizado:1500.00, fecha_deuda:"2026-04-01"...}
      {id:2, monto_normalizado:2300.50, fecha_deuda:"2026-03-15"...}
      {id:3, monto_normalizado:875.25, fecha_deuda:"2026-04-10"...}
  
  [transformation_log]
    Tipo: Object
    Contenido:
      transformations_applied:
        - date_format: "DD/MM/YYYY" -> "ISO8601"
        - decimal_separator: "," -> "."
        - phone_normalization: "E164"
        - email_validation: added
      records_processed: 1247
      records_failed: 0
  
  UBICACIONES:
    - En memoria: disponible (ultima ejecucion run-abc123)
    - En cache: /cache/step-2-normalized-data.json
    - Persistido: no
```

### 2.13 INSPECCIONAR: Ver Inputs Requeridos por un Paso

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza-mensual --step 3 --inspect-inputs"

[CLI Remora] -> [GestordeFlujos]: solicita flujo y definicion del paso 3

[GestordeFlujos] -> [CatalogoFlujos]: obtiene flujo

[CatalogoFlujos] -> [GestordeFlujos]: devuelve flujo con 5 pasos

[GestordeFlujos] -> [GestordeFlujos]: extrae manifest del paso 3

[GestordeFlujos] -> [CLI Remora]: devuelve inputs requeridos:

  ===========================
  INSPECT INPUTS: paso 3
  ===========================
  
  Paso: beta.calculate
  Framework: beta v0.9.1
  Comando: calculate
  
  INPUTS REQUERIDOS:
  
  [normalized_data]
    Tipo: Array<Record>
    Requerido: true
    Source esperada: Paso 2 (beta.normalize)
    Schema esperado:
      - monto_normalizado: decimal(12,2)
      - fecha_vencimiento: date
      - deudor_id: string
    Descripcion: "Datos normalizados listos para calculo de fees"
  
  [fee_rules]
    Tipo: Reference
    Requerido: true
    Source esperada: Configuracion o paso anterior
    Valor default: null
    Descripcion: "Referencia a reglas de fee configuradas"
    Formato esperado: path a archivo YAML o referencia a artifact
  
  [calculation_context]
    Tipo: Object
    Requerido: false
    Source esperada: Opcional
    Valor default: { period: "current_month" }
    Descripcion: "Contexto para calculos (periodo, parametros)"
  
  INPUTS ACTUALMENTE DISPONIBLES:
    - normalized_data: DISPONIBLE (Paso 2 ejecutado)
    - fee_rules: NO DISPONIBLE (no proporcionado)
    - calculation_context: DISPONIBLE (usara default)
  
  FALTANTES:
    - fee_rules: debe ser proporcionado via --input o configuracion
  
  COMO PROVEER:
    "remora flow debug --flow cobranza-mensual --step 3 --input fee_rules=@config/fees.yaml"
```

---

## 3. Casos de Uso en Formato Narrativo

### Caso 1: "Quiero ver qué frameworks necesito para un flujo de cobranza"

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza-mensual --frameworks-only"

[CLI Remora] -> [GestordeFlujos]: solicita flujo

[GestordeFlujos] -> [CatalogoFlujos]: obtiene flujo

[CatalogoFlujos] -> [GestordeFlujos]: devuelve flujo

[GestordeFlujos] -> [CLI Remora]: presenta:

  Frameworks requeridos para "cobranza-mensual":
  
  - alfa (v1.2.0)     [data-ingestion]
    Provee: ingestion
  
  - beta (v0.9.1)     [data-processing]
    Provee: transformation
  
  - foco (v2.0.0)    [core]
    Provee: execution
  
  - delta (v1.0.0)   [delivery]
    Provee: output

[Desarrollador]: "Ahora se que necesito instalar esos 4 frameworks antes de ejecutar."
```

### Caso 2: "Quiero probar solo el paso de Foco sin ejecutar todo el flujo"

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza --step foco --execute"

[CLI Remora] -> [GestordeFlujos]: obtiene flujo

[GestordeFlujos] -> [GestorDependencias]: resuelve inputs del paso foco

[GestorDependencias]: detecta que foco.execute requiere notification_batch

[GestordeFlujos] -> [CLI Remora]: informe de disponibilidad

[CLI Remora] -> [Desarrollador]: presenta opciones:

  Para ejecutar solo el paso "foco.execute" necesitas:
  
  Input requerido: notification_batch
  Estado: NO DISPONIBLE
  
  Opciones:
    [A] Ejecutar todo el flujo desde el inicio
        "remora flow debug --flow cobranza --from-step 1"
    
    [B] Usar datos de una ejecucion anterior
        "remora flow debug --flow cobranza --step foco --from-cache run-abc123"
    
    [C] Proporcionar mock data
        "remora flow debug --flow cobranza --step foco --mock notification_batch=@data/test-batch.json"

[Desarrollador] -> [CLI Remora]: "B"

[CLI Remora] -> [TrazaEjecucion]: recupera run-abc123

[TrazaEjecucion] -> [CLI Remora]: devuelve inputs del cache

[CLI Remora] -> [MotordeEjecucion]: ejecuta solo paso foco con inputs del cache

[MotordeEjecucion] -> [CLI Remora]: resultado

[CLI Remora] -> [Desarrollador]: presenta resultado del paso individual
```

### Caso 3: "Quiero ver exactamente qué inputs necesita Alfa antes de ejecutarlo"

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --manifest alfa --inputs-only"

[CLI Remora] -> [CatalogoFrameworks]: solicita manifest de alfa

[CatalogoFrameworks] -> [FrameworkLoader]: carga manifest

[FrameworkLoader] -> [CatalogoFrameworks]: devuelve manifest completo

[CatalogoFrameworks] -> [CLI Remora]: presenta inputs:

  Inputs requeridos por alfa:
  
  [source_url]
    Tipo: string
    Requerido: true
    Validacion: debe ser URL valida o path a archivo
    Ejemplo: "https://api.example.com/data"
  
  [source_type]
    Tipo: enum
    Requerido: true
    Valores validos: [api, file, stream, database]
    Default: null
  
  [credentials_ref]
    Tipo: string
    Requerido: false (segun source_type)
    Descripcion: "Referencia a credenciales en vault"
    Requerido cuando: source_type = "api" o "database"

[Desarrollador]: "Ahora se exactamente que datos debo proporcionar."
```

### Caso 4: "Quiero hacer dry-run y ver el timeline antes de ejecutar"

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza --dry-run --timeline"

[CLI Remora] -> [Planificador]: genera plan sin ejecutar

[Planificador] -> [GestorDependencias]: resuelve orden

[GestorDependencias] -> [Planificador]: orden resuelto

[Planificador] -> [GeneradorTimeline]: calcula timeline estimado

[GeneradorTimeline] -> [Planificador]: timeline con duraciones estimadas

[Planificador] -> [CLI Remora]: devuelve dry-run completo

[CLI Remora] -> [Desarrollador]: presenta:

  DRY-RUN: Sin ejecucion real
  
  Timeline estimado:
  
  T+0:00.000  [INIT]    Inicializando
  T+0:00.050  [STEP 1]  alfa.ingest         (2.0s)
  T+0:02.050  [STEP 2]  beta.normalize      (0.2s)
  T+0:02.250  [STEP 3]  beta.calculate     (1.4s)
  T+0:03.650  [STEP 4]  foco.execute        (1.3s)
  T+0:04.950  [STEP 5]  delta.report        (0.3s)
  T+0:05.250  [DONE]    Total: 5.25s estimado
  
  Sin efectos secundarios. Sin escritura. Sin red.
  Presiona 'ejecutar' para iniciar o Ctrl+C para cancelar.

[Desarrollador]: "Perfecto, puedo ver exactamente que pasara antes de ejecutar."
```

### Caso 5: "Quiero saber qué capabilities tiene cada framework y cómo se conectan"

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza --capabilities-map"

[CLI Remora] -> [GestordeFlujos]: obtiene flujo

[GestordeFlujos] -> [CatalogoFlujos]: obtiene flujo

[GestordeFlujos] -> [CatalogoFrameworks]: solicita capabilities de cada framework

[CatalogoFrameworks] -> [GestordeFlujos]: devuelve capabilities

[GestordeFlujos] -> [CLI Remora]: presenta mapa:

  CAPABILITIES MAP: cobranza-mensual
  
  [alfa]
    capability: ingestion
    type: SOURCE
    outputs: [raw_data, schema]
         |
         | raw_data
         v
  [beta]
    capability: transformation
    type: TRANSFORM
    inputs: [raw_data] -> outputs: [normalized_data]
    capabilities adicionales: [enrichment]
         |
         | normalized_data
         v
  [beta]
    capability: transformation (reusado)
    type: TRANSFORM
    inputs: [normalized_data] -> outputs: [notification_batch]
         |
         | notification_batch
         v
  [foco]
    capability: execution
    type: ORCHESTRATION
    inputs: [notification_batch] -> outputs: [delivery_receipts]
         |
         | delivery_receipts
         v
  [delta]
    capability: delivery
    type: SINK
    inputs: [delivery_receipts] -> outputs: [report_file]

  CONEXIONES:
    alfa.ingestion -> beta.transformation
    beta.transformation -> beta.transformation (reuso)
    beta.transformation -> foco.execution
    foco.execution -> delta.delivery
  
  FLUJO DE CAPABILITIES:
    SOURCE -> TRANSFORM -> TRANSFORM -> ORCHESTRATION -> SINK

[Desarrollador]: "Ahora entiendo como se conectan las capabilities."
```

---

## 4. Flujo de Datos - Vista General

```
USUARIO                    CLI                         SISTEMA
  |                          |                            |
  |-- list-frameworks ------>|                            |
  |                          |-- consulta Catalogo ----->|
  |                          |<-- devuelve frameworks ---|
  |<-- tabla frameworks -----|                            |
  |                          |                            |
  |-- manifest alfa -------->|                            |
  |                          |-- carga manifest --------->|
  |                          |<-- devuelve manifest -----|
  |<-- YAML manifest --------|                            |
  |                          |                            |
  |-- dry-run cobranza ----->|                            |
  |                          |-- resuelve flujo --------->|
  |                          |-- calcula plan ---------->|
  |                          |<-- timeline estimado ------|
  |<-- dry-run completo ----|                            |
  |                          |                            |
  |-- execute cobranza ----->|                            |
  |                          |-- valida flujo ----------->|
  |                          |<-- validacion OK ----------|
  |                          |<-- confirmacion -----------|
  |--- confirmar ----------->|                            |
  |                          |-- ejecuta motor ---------->|
  |                          |   (por cada paso)          |
  |                          |<-- resultados -------------||
  |<-- trace completo -------|                            |
  |                          |                            |
  |-- inspect-outputs step=2>|                            |
  |                          |-- recupera artifact ----->|
  |                          |<-- metadata outputs -------|
  |<-- outputs paso 2 -------|                            |
```

---

## 5. Estados de Error - Comportamiento Ideal

### Error 1: Framework no encontrado

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --framework zeta"

[CLI Remora] -> [CatalogoFrameworks]: busca "zeta"

[CatalogoFrameworks] -> [CLI Remora]: NOT_FOUND

[CLI Remora] -> [Desarrollador]: error formateado:

  ERROR: Framework no encontrado "zeta"
  
  Frameworks disponibles:
    alfa, beta, delta, epsilon, foco, gamma
  
  Sugerencia:
    "remora flow debug --framework alfa" o
    "remora flow debug --list-frameworks"
```

### Error 2: Flujo no existe

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza-express"

[CLI Remora] -> [CatalogoFlujos]: busca flujo

[CatalogoFlujos] -> [CLI Remora]: NOT_FOUND

[CLI Remora] -> [Desarrollador]: 

  ERROR: Flujo no encontrado "cobranza-express"
  
  Flujos disponibles:
    cobranza-mensual, cobranza-trimestral, recuperacion-activos
  
  Sugerencia:
    "remora flow debug --list-flows" para ver todos
```

### Error 3: Input requerido faltante

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza --execute"

[CLI Remora] -> [ValidadorFlujo]: valida inputs

[ValidadorFlujo]: detecta input faltante

[ValidadorFlujo] -> [CLI Remora]: 

  ERROR: Input requerido faltante
  
  Paso: 1 (alfa.ingest)
  Input: source_url
  Tipo: string
  Estado: NO PROPORCIONADO
  
  ACCIONES:
    [A] Proporcionar ahora:
        --input source_url="https://..."
    
    [B] Configurar en archivo de configuracion:
        "remora flow debug --config init"
    
    [C] Usar variable de entorno:
        Set ALFA_SOURCE_URL=https://...
    
    [D] Solicitar interactivamente:
        "remora flow debug --flow cobranza --execute --ask-prompts"
```

### Error 4: Ciclo en dependencias

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow circular --validate"

[ValidadorFlujo] -> [GestorDependencias]: detecta ciclo

[GestorDependencias]: analisis de dependencias:

  alfa -> beta -> gamma -> alfa
  
  CICLO DETECTADO:
    alfa depende de beta
    beta depende de gamma
    gamma depende de alfa
  
  No es posible resolver orden topologico.

[ValidadorFlujo] -> [CLI Remora]: ERROR_CICLO

[CLI Remora] -> [Desarrollador]:

  ERROR: Ciclo en dependencias detectado
  
  Flujo: circular
  Ciclo: alfa -> beta -> gamma -> alfa
  
  El flujo no puede ejecutarse.
  Elimine la dependencia circular para continuar.
```

---

## 6. Flags Globales de la CLI

```
FLAGS GLOBALES (disponibles en todos los comandos):

  --help, -h
    Muestra ayuda contextual
  
  --format [json|yaml|table]
    Formato de salida (default: table)
  
  --output FILE
    Redirige salida a archivo
  
  --silent
    Suprime output no esencial
  
  --verbose, -v
    Muestra detalles adicionales
  
  --color [always|auto|never]
    Control de colores en terminal
  
  --no-cache
    Ignora cache de ejecuciones anteriores
  
  --cache-dir PATH
    Directorio de cache (default: .remora/cache)

FLAGS DE INPUT:

  --input KEY=VALUE
    Proporciona input directamente
  
  --input KEY=@FILE
    Lee input desde archivo
  
  --ask-prompts
    Solicita inputs interactivamente
  
  --env-prefix PREFIX
    Prefijo para variables de entorno (default: REMORA_)

FLAGS DE EJECUCION:

  --dry-run
    Simula sin ejecutar efectos reales
  
  --from-step N
    Ejecuta desde paso N
  
  --to-step N
    Ejecuta hasta paso N
  
  --step N
    Ejecuta solo paso N
  
  --mock-input KEY=VALUE
    Proporciona mock data para inputs faltantes
  
  --generate-mock
    Genera mock data automaticamente
  
  --timeout SECONDS
    Timeout global (default: 300)
  
  --parallel
    Ejecuta pasos independientes en paralelo si es posible

FLAGS DE TRAZA:

  --trace
    Captura y muestra timeline completo
  
  --trace-file FILE
    Guarda trace a archivo
  
  --filter EXPRESION
    Filtra eventos en trace
  
  --inspect-inputs
    Muestra solo inputs de un paso
  
  --inspect-outputs
    Muestra solo outputs de un paso

FLAGS DE ANALISIS:

  --validate
    Solo valida, no ejecuta
  
  --gaps
    Analiza gaps en el flujo
  
  --dependencies
    Muestra grafo de dependencias
  
  --capabilities-map
    Muestra mapa de capabilities

FLAGS DE CACHE:

  --use-cache
    Usa artifacts de cache si disponibles
  
  --from-cache RUN-ID
    Usa inputs de una ejecucion anterior
  
  --clear-cache
    Limpia cache antes de ejecutar
  
  --cache-ttl HOURS
    TTL del cache en horas (default: 24)
```

---

## 7. Comandos Disponibles - Resumen

```
COMANDOS PRINCIPALES:

remora flow debug --list-frameworks
  Lista todos los frameworks disponibles

remora flow debug --list-flows
  Lista todos los flujos definidos

remora flow debug --manifest FRAMEWORK
  Muestra manifest completo de un framework

remora flow debug --flow FLUJO --dry-run
  Simula ejecucion sin efectos reales

remora flow debug --flow FLUJO --timeline
  Muestra timeline estimado de ejecucion

remora flow debug --flow FLUJO --execute
  Ejecuta flujo completo

remora flow debug --flow FLUJO --step N --execute
  Ejecuta un paso individual

remora flow debug --flow FLUJO --trace
  Muestra traza completa de ejecucion

remora flow debug --flow FLUJO --validate
  Valida estructura y consistencia

remora flow debug --flow FLUJO --dependencies
  Muestra grafo de dependencias

remora flow debug --flow FLUJO --gaps
  Identifica gaps en el chain de datos

remora flow debug --flow FLUJO --capabilities-map
  Muestra mapa de capabilities y conexiones

remora flow debug --flow FLUJO --step N --inspect-inputs
  Muestra inputs requeridos por un paso

remora flow debug --flow FLUJO --step N --inspect-outputs
  Inspecciona outputs de un paso

remora flow debug --framework FRAMEWORK --capabilities
  Lista capabilities de un framework

remora flow debug --framework FRAMEWORK --commands
  Lista comandos disponibles

remora flow debug --history
  Muestra historial de ejecuciones

remora flow debug --cache inspect
  Inspecciona cache de artifacts
```

---

## 8. Formato de Salida por Defecto

### Formato Table (default)

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

### Formato JSON

```json
{
  "command": "list-frameworks",
  "status": "success",
  "timestamp": "2026-05-16T14:30:00Z",
  "data": {
    "frameworks": [...]
  }
}
```

### Formato YAML

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

## 9. Mensajes de Estado - Terminal Ideal

```
EJECUCION EN PROGRESO:

  remora flow debug --flow cobranza --execute
  
  [1/5] alfa.ingest ................... running
  [2/5] beta.normalize ................. waiting
  [3/5] beta.calculate ................. waiting
  [4/5] foco.execute ................... waiting
  [5/5] delta.report ................... waiting
  
  Tiempo: 0:02 / 0:05 estimado
  Output: 2.4MB ingested

PASE A ESTADO DE ÉXITO:

  [1/5] alfa.ingest ................... SUCCESS (2.0s)
  [2/5] beta.normalize ................. SUCCESS (0.2s)
  [3/5] beta.calculate ................. SUCCESS (1.4s)
  [4/5] foco.execute ................... SUCCESS (1.3s)
  [5/5] delta.report ................... SUCCESS (0.3s)
  
  COMPLETADO: 5/5 pasos en 5.2s

PASE A ESTADO DE ERROR:

  [1/5] alfa.ingest ................... SUCCESS
  [2/5] beta.normalize ................. SUCCESS
  [3/5] beta.calculate ................. FAILED (Error en paso 3)
  
  ERROR: beta.calculate - Invalid fee calculation
         Causa: monto_negativo en registro id=892
  
  ACCIONES DISPONIBLES:
    [R] Retry paso 3
    [S] Skip paso 3 y continuar
    [A] Abortar flujo
    [D] Debug paso 3

PROGRESO CON TIMELINE:

  remora flow debug --flow cobranza --trace
  
  Timeline:
  T+0:00.050  [START] alfa.ingest
  T+0:00.100  [INFO]  Starting ingestion from API...
  T+0:02.050  [OK]    alfa.ingest complete: 2.4MB
  T+0:02.100  [START] beta.normalize
  T+0:02.300  [OK]    beta.normalize complete: 1247 records
  T+0:02.350  [START] beta.calculate
  T+0:03.750  [OK]    beta.calculate complete: 1247 notifications
  T+0:03.800  [START] foco.execute
  T+0:05.150  [OK]    foco.execute complete: 1195 delivered
  T+0:05.200  [START] delta.report
  T+0:05.500  [OK]    delta.report complete: report.json
  T+0:05.500  [DONE]  All steps completed
```

---

## 10. Interactividad - Modo Interactivo

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --flow cobranza --interactive"

[CLI Remora]: inicia modo interactivo

Bienvenido a remora flow debug (interactive mode)
================================================================
Comandos disponibles:
  run, dry-run, validate, inspect, trace, history, quit
================================================================

remora> list-frameworks

NOMBRE          VERSION   CAPABILITIES
alfa            1.2.0     ingestion,validation
beta            0.9.1     transformation,enrichment
foco            2.0.0     execution,orchestration
delta           1.0.0     output,notification

remora> manifest alfa

Framework: alfa v1.2.0
Capabilities:
  - ingestion (SOURCE)
  - validation (TRANSFORM)
Inputs:
  - source_url (required)
  - source_type (required)
Outputs:
  - raw_data
  - schema

remora> load cobranza

Flujo cargado: cobranza-mensual
Pasos: 5

remora> dry-run

Timeline estimado:
  alfa.ingest (2.0s) -> beta.normalize (0.2s) -> beta.calculate (1.4s) -> foco.execute (1.3s) -> delta.report (0.3s)
Total estimado: 5.2s

remora> inspect step 2 inputs

Paso 2: beta.normalize
Inputs requeridos:
  - raw_data (from step 1)
  - date_format (required)
Faltantes: date_format
  Proporcionar? (y/n/q): y
  Ingrese valor: DD/MM/YYYY

remora> run

Ejecutando flujo...
  [1/5] alfa.ingest ................... SUCCESS
  [2/5] beta.normalize ................. SUCCESS
  [3/5] beta.calculate ................. SUCCESS
  [4/5] foco.execute ................... SUCCESS
  [5/5] delta.report ................... SUCCESS

Completado en 5.2s

remora> trace step 3

[14:30:02.400] STEP_START: beta.calculate
[14:30:02.410] INPUT_ACQUIRED: raw_data (1247 records)
[14:30:02.420] INPUT_ACQUIRED: fee_rules (from config)
[14:30:02.425] PROCESS_START
[14:30:03.800] PROCESS_COMPLETE: 1247 calculations
[14:30:03.800] STEP_COMPLETE

remora> quit

Hasta luego!
```

---

## 11. Integración con el Sistema de Configuración

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --config init"

[CLI Remora] -> [GestorConfig]: inicia configuracion

[GestorConfig]: detecta entorno

[GestorConfig] -> [CLI Remora]: wizard interactivo

[CLI Remora] -> [Desarrollador]: presenta wizard

  remora flow debug - Configuracion Inicial
  ========================================
  
  Paso 1/5: Directorio de trabajo
  Valor actual: /Users/dev/projects/remora
  Nuevo valor o Enter para aceptar: _
  
  Paso 2/5: Directorio de cache
  Valor actual: .remora/cache
  Nuevo valor o Enter para aceptar: _
  
  Paso 3/5: Formato de salida default
  Valores: json, yaml, table (default: table): _
  
  Paso 4/5: Timeout default (segundos)
  Valor actual: 300
  Nuevo valor o Enter para aceptar: _
  
  Paso 5/5: Color en terminal
  Valores: always, auto, never (default: auto): _
  
  Configuracion guardada en .remora/config.yaml
  
  Para ver configuracion actual:
    remora flow debug --config show
  
  Para modificar valores individuales:
    remora flow debug --config set KEY VALUE
```

---

## 12. Historia y Trazabilidad

```
[Desarrollador] -> [CLI Remora]: "remora flow debug --history"

[CLI Remora] -> [HistorialEjecuciones]: consulta historial

[HistorialEjecuciones] -> [CLI Remora]: devuelve historial

[CLI Remora] -> [Desarrollador]: presenta:

  HISTORIAL DE EJECUCIONES
  ========================
  
  RUN ID        FLUJO                FECHA               DURACION  ESTADO
  ---------     -----                -----               --------  ------
  run-abc123    cobranza-mensual     2026-05-16 14:30    5.2s      SUCCESS
  run-def456    cobranza-mensual     2026-05-15 10:15    5.5s      SUCCESS
  run-ghi789    cobranza-trimestral  2026-05-14 09:00    12.1s     SUCCESS
  run-jkl012    cobranza-mensual     2026-05-13 14:30    5.1s      SUCCESS
  run-mno345    cobranza-fallida     2026-05-12 16:45    ERROR     FAILED
  
  Para ver traza de ejecucion:
    remora flow debug --run run-abc123 --trace
  
  Para ver outputs de ejecucion:
    remora flow debug --run run-abc123 --inspect
  
  Para diff entre ejecuciones:
    remora flow debug --diff run-abc123 run-def456
```

---

## Resumen de la Narrativa

Esta narrativa define el comportamiento IDEAL de una CLI que proporciona:

1. **VISIBILIDAD TOTAL**: Ver qué frameworks existen, qué hacen, qué requieren
2. **CONTROL TOTAL**: Ejecutar flujos completos, pasos individuales, o con mocks
3. **SIMULACION SEGURA**: Dry-run para ver qué pasaría sin efectos reales
4. **TRAZABILIDAD COMPLETA**: Timeline con timestamps, inputs, outputs de cada paso
5. **ANALISIS PROFUNDO**: Validacion, deteccion de gaps, analisis de dependencias
6. **INSPECCION DETALLADA**: Examinar artifacts, ver schema de outputs, mock data

El usuario tiene plena capacidad de:
- Explorar el ecosistema de frameworks antes de usarlos
- Validar sus flujos antes de ejecutarlos
- Ejecutar granularmente (paso a paso, con mocks, desde cache)
- Trazar ejecuciones completas
- Inspeccionar el estado del sistema en cualquier momento

La CLI se comporta como un "docker inspect" para flujos de Remora: herramienta de introspeccion y control para desarrolladores que necesitan entender y depurar sus flujos de trabajo.
