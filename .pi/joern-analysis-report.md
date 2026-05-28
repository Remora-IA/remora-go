# Análisis Joern - remora-go-lite

## 1. Resumen del Proyecto

**Proyecto:** remora-go-lite  
**Hash Base:** `be4eaac6043a637c14233a19b471e745843cd0a0de4fe2a9ee99c64eb5ebb1e7` (go.mod)  
**Fecha:** 2026-05-16

## 2. Estructura del Código

### Módulos Principales
- `cmd/api_rest/` - API REST principal (servidor Go con handlers de flujo)
- `cmd/flujo/` - CLI de flujo de trabajo
- `remora-flujo/` - Núcleo de flujo con nativeagent, handoff, simulations
- `channel/` - Comunicación entre frameworks
- `framework-*` - Frameworks de terceros (alfa, paladin, pingpong, etc.)

## 3. Métodos Extraídos (1444 métodos)

### Punto de Entrada Principal
- `main` - cmd/api_rest/main.main.go
- `cmd/flujo/main.main`
- `cmd/framework_session/main.main`
- `cmd/llmtest/main.main`

### Handlers HTTP Principales
- `handle` - Router principal
- `handleAuth*` - Autenticación (Register, Login, Logout, Me)
- `handleBusinesses*` - Gestión de negocios
- `handleData*` - Explorador de datos
- `handleFlow*` - Operaciones de flujo

### Gestión de Flujos
- `runFlow` / `runFlowStream` - Ejecución de flujo
- `runFlowManifest` - Ejecución de manifest compilado
- `executeFlowNode` - Ejecuta un nodo específico
- `validateFlowManifest` - Valida manifest
- `simulateFlowManifest` - Simula ejecución
- `compileFlowWorkbench` - Compila diseño de flujo

### Orquestación
- `runLoop` - Loop principal de orquestación
- `executeSessionFollowup` - Manejo de follow-ups
- `executeDelegations` - Ejecución de delegaciones
- `consumePendingQuestionsForFramework` - Cola de preguntas

### Almacenamiento
- `openFlowStore` - Abre store de flujos
- `persistFlowRun` - Persiste ejecución
- `persistFlowArtifact` - Persiste artefactos
- `loadFlowRun` - Carga ejecución
- `loadCompiledRecord` - Carga plan compilado

## 4. Call Graphs

### main → flujo completo
```
main
├── runLoop (orquestador principal)
│   ├── consumePendingQuestionsForFramework
│   │   └── executeSessionFollowup
│   │       └── executeSessionFollowupDetailed
│   │           ├── runFlowManifest
│   │           │   ├── executeFlowNode
│   │           │   ├── runFlowActionBranches
│   │           │   └── executeDelegations
│   │           └── executeDelegations
│   └── handle (routing HTTP)
│       ├── handleFlowCreate
│       ├── handleFlowRun
│       ├── handleFlowValidate
│       └── handleFlowCompile
```

### handleFlowWorkbench
```
handleFlowWorkbench
├── runFlowWorkbench (CLI)
│   ├── runFlowCreate
│   ├── runFlowDraft
│   ├── runFlowCompile
│   ├── runFlowValidate
│   ├── runFlowSimulate
│   └── runFlowRun
└── compileFlowWorkbench
    ├── validateFlowManifest
    ├── compileFlowManifest
    └── buildCapabilityRegistry
```

## 5. Sinks Detectados

### Operaciones Sensibles

| Tipo | Método | Archivo | Riesgo |
|------|--------|---------|--------|
| exec.Command | executeFlowNode | flow_execution.go | Alto |
| exec.Command | toolRunner.execute | main.go:611 | Alto |
| shellCommandFromTextResponse | - | llm.client.go | Alto |
| bashCommandFromInput | - | llm.client.go | Alto |
| SQL | queryInt | data_browser.go | Medio |
| SQL | writeDataTableRows | data_browser.go | Medio |
| File Write | toolWrite | agent.go | Medio |
| File Write | responseText (WriteFile) | main.go:641 | Medio |
| HTTP | request | llm.client.go | Bajo |
| HTTP | doRequest | llm.client.go | Bajo |

### Variables Sensibles Detectadas
- `passwordHash` - auth.go:295

## 6. GAPs Identificados

### Arquitectura
1. **Múltiples CPGs obsoletos** en workspace/ - varios .bin vacíos o duplicados
2. **Drivers externos** - initDriverRegistry, driversFor, runtimeCommand en drivers.go
3. **Integración con frameworks externos** - Cada framework-* tiene su propio módulo

### Seguridad
1. **Entrada de usuario** - os.Args puede fluir a exec.Command
2. **shellCommandFromTextResponse** - Ejecución de comandos shell desde texto
3. **bashCommandFromInput** - Parsing de comandos bash desde input

### Performance
1. **Sesiones en disco** - loadActiveSessionFromDisk, saveFocoTaskPlan
2. **Colas persistentes** - QuestionsQueue con migración legacy
3. **CPG fragmentado** - Múltiples archivos de CPG en workspace/

## 7. Recomendaciones

### Alta Prioridad
1. Revisar `shellCommandFromTextResponse` y `bashCommandFromInput` para validación de comandos
2. Implementar whitelist para exec.Command
3. Consolidar CPGs en workspace/

### Media Prioridad
1. Agregar rate limiting en handlers HTTP
2. Revisar manejo de credenciales en flow_data_pipeline.go
3. Mejorar logging de decisiones de seguridad

### Baja Prioridad
1. Refactorizar QuestionsQueue para usar estructuras modernas
2. Documentar contratos entre módulos
3. Mejorar cobertura de tests

---
*Generado con Joern Analysis Suite*
