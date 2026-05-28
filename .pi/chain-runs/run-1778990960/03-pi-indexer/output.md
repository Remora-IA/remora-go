# PI-INDEXER: Índice Semántico Consolidado

**Run ID:** run-1778990960  
**Fecha:** 2026-05-17

---

## NARRATIVA (Índice Navegable)

### Superficies

| ID | Nombre | Tipo | Entry Point |
|----|--------|------|-------------|
| S1 | remora CLI | Terminal | handleFlowWorkbench |
| S2 | API REST | HTTP :8084 | main (api_rest) |
| S3 | Flow Workbench CLI | HTTP Client | runFlowWorkbench |
| S4 | Channel RPC | JSON-RPC | Handle() |
| S5 | Alfa Runner | Subprocess | cmd/alfa-runner |
| S6 | Echo Runner | Subprocess | cmd/echo-runner |

### Flujos

| ID | Superficie | Nombre | Descripción |
|----|------------|--------|-------------|
| F1.1 | S1 | flow-create | Crear flow interactivo |
| F1.2 | S1 | flow-simulate | Dry-run de flow |
| F1.3 | S1 | flow-run | Ejecutar flow real |
| F2.1 | S2 | auth-register | Registrar usuario |
| F2.2 | S2 | business-create | Crear workspace |
| F2.3 | S2 | data-upload | Subir archivo Excel |
| F2.4 | S2 | flow-execution | Ejecutar flow vía API |
| F3.1 | S4 | execute-command | Ejecutar comando en workspace |
| F4.1 | S5 | framework-compile | Compilar spec de Alfa |
| F4.2 | S6 | echo-simple | Ejecutar Echo simple |

### Escenas

| ID | Flujo | Momento |
|----|-------|---------|
| E1.1.1 | F1.1 | Usuario inicia remora sin args → ve usage |
| E1.1.2 | F1.1 | Usuario responde prompts → ve preview |
| E2.3.1 | F2.3 | Usuario sube Excel → ve columnas detectadas |
| E2.4.1 | F2.4 | Flow se ejecuta → nodos silenciosos |
| E3.1.1 | F3.1 | Command validation → allowed/rejected |
| E4.1.1 | F4.1 | Compile spec → stdout/stderr |

### Gaps

| ID | Título | Severidad |
|----|--------|-----------|
| G1 | runFlow silent — sin streaming | blocking |
| G2 | Channel sin log persistente | important |
| G3 | Auth token sin refresh | important |

### Cables

| ID | Limitación |
|----|------------|
| C1 | CLI requiere backend :8084 |
| C2 | Channel timeout fijo 30s |
| C3 | Orchestrator single-provider (Groq) |

---

## MÉTODOS PRINCIPALES (por superficie)

### remora CLI (S1)
- handleFlowWorkbench, delegateToCanonicalFlowWorkbench
- handleFlowCreate, handleFlowInspect, handleFlowSimulate, handleFlowRun, handleFlowInstall
- handleDebugCommand, handleDebugTrace, handleDebugManifest

### API REST (S2)
- handleAuthRegister, handleAuthLogin, handleAuthLogout
- handleBusinessCreate, handleBusinesses, handleBusinessMembers
- handleCreateFlow, handleGetFlow, handleInstallFlow
- handleBusinessDataUpload, handleBusinessArtifacts
- handleSMTPCheck, handleHostingConnect
- runFlow, runFlowManifest, executeFlowNode
- classifySegmentIntent, executeDelegations, runLoop

### Channel (S4)
- Handle(), executeCommand()
- ValidateSecurity, ValidatePath, isPathSafe
- IsCommandAllowed, IsDestructiveCommand
- ExecuteCommand, ReadFile, WriteFile, Grep, Find, EditFile, HTTPGet

---

## COMPONENTES ANALIZADOS

| Componente | CPG Size | Methods | binary |
|------------|-----------|---------|--------|
| remora-cli | 588K | 124 | remora, devcli |
| remora-flujo | 5.0M | 1411 | api_rest, flujo, agentrpc |
| channel | 420K | 129 | channel, orchestrator, vault |
| framework-alfa | 372K | sample | plugin |

---

## LINKS

- Detective: `.pi/chain-runs/run-1778990960/01-pi-detective/output.md`
- Pre-Narrador: `.pi/chain-runs/run-1778990960/02-pi-pre-narrador/output.md`

<!-- CHAIN_RUN_ID: run-1778990960 -->
<!-- STEP: 03-pi-indexer -->
