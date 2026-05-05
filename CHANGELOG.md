# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.18] - 2026-05-05

> **Release**: actualizar proyecto

### Repo

- **framework-pingpong/pingpong**: configuracion
- **framework-pingpong/pingpong_progress.json**: +23 / -29
- **framework-pingpong/INITIAL_PROMPT.md**: +160 / -12
- **framework-pingpong/main.go**: funciones: cmdSubdivide, cmdClean, cmdPeek, cmdScan
- **pingpong/client.go**: funciones: (c *Client) Clean, (c *Client) Peek, (c *Client) Subdivide, (c *Client) Scan | tipos: ScanResult
- **pingpong/verifier.go**: funciones: extractSnippet, VerifyFileLenient, DetectNoiseNames, RemoveDeclarations | tipos: removal, declInfo
- **servidor-rpc/cliente.go**: archivo nuevo
- **servidor-rpc/main.go**: funciones: main | tipos: Args, Reply, Servicio
- **framework-pingpong/main.go**: codigo modificado
- **servidor-rpc/server.go**: codigo modificado

## [0.1.17] - 2026-05-05

> **Release**: expandir flujo, paladin, quine

### Paladin

- **framework-paladin/framework.manifest.json**: archivo nuevo
- **framework-paladin/frameworkpaladin**: archivo nuevo

### Quine

- **quine/main.go**: +2

### Flujo

- **remora-flujo/.remora_session**: archivo nuevo
- **flujo_api/flow.rules.json**: +12
- **flujo_api/main.go**: funciones: (s *server) postMessageSSE | tipos: loopResult
- **flujo_api/streaming.go**: archivo nuevo
- **nativeagent/agent.go**: +41 / -26
- **frontends/serve.go**: -2

### Repo

- **main-multi-modo**: main.go soporta multiples modos (audit, explain, tree) (remora/main.go)
- **framework-arquitecto/framework.manifest.json**: archivo nuevo
- **framework-arquitecto/frameworkarquitecto**: archivo nuevo
- **framework-arquitecto/go.mod**: archivo nuevo
- **framework-critico/framework.manifest.json**: archivo nuevo
- **framework-critico/frameworkcritico**: archivo nuevo
- **framework-critico/go.mod**: archivo nuevo
- **framework-pingpong/0**: archivo nuevo
- **framework-pingpong/Makefile**: archivo nuevo
- **framework-pingpong/framework-pingpong**: archivo nuevo
- **framework-pingpong/go.mod**: archivo nuevo
- **framework-pingpong/pingpong**: archivo nuevo
- **framework-pingpong/pingpong_progress.json**: archivo nuevo
- **framework-pingpong/progress.json**: archivo nuevo
- **framework-pingpong/two_sum**: archivo nuevo
- **remora-cli/.remora_session**: archivo nuevo
- **remora-cli/go.mod**: archivo nuevo
- **remora-cli/remora**: archivo nuevo
- **scripts/dev-local.sh**: archivo nuevo
- **scripts/install-remora.sh**: archivo nuevo
- **HANDOFF_PROMPT.md**: archivo nuevo
- **framework-arquitecto/AGENTS.md**: archivo nuevo
- **framework-arquitecto/INITIAL_PROMPT.md**: archivo nuevo
- **framework-arquitecto/README.md**: archivo nuevo
- **framework-critico/AGENTS.md**: archivo nuevo
- **framework-critico/INITIAL_PROMPT.md**: archivo nuevo
- **framework-critico/README.md**: archivo nuevo
- **framework-pingpong/AGENTS.md**: archivo nuevo
- **framework-pingpong/INITIAL_PROMPT.md**: archivo nuevo
- **framework-pingpong/README.md**: archivo nuevo
- **framework-pingpong/WHY.md**: archivo nuevo
- **internal/handler.go**: funciones: (h *Handler) executeCommand
- **frameworkarquitecto/llm.go**: archivo nuevo
- **frameworkarquitecto/llm_minimax.go**: archivo nuevo
- **frameworkarquitecto/llm_stream.go**: archivo nuevo
- **frameworkarquitecto/main.go**: archivo nuevo
- **frameworkarquitecto/tools.go**: archivo nuevo
- **frameworkcritico/main.go**: archivo nuevo
- **framework-pingpong/main.go**: archivo nuevo
- **pingpong/client.go**: archivo nuevo
- **pingpong/runner.go**: archivo nuevo
- **pingpong/verifier.go**: archivo nuevo
- **internal/exec.go**: funciones: ExecuteCommandWithEnv
- **internal/whitelist.go**: +3 / -1
- **framework-pingpong/main.go**: archivo nuevo
- **framework-pingpong/palindrome.go**: funciones: main
- **framework-pingpong/roman_to_integer.go**: archivo nuevo
- **servidor-rpc/main.go**: archivo nuevo
- **servidor-rpc/server.go**: archivo nuevo
- **framework-pingpong/two_sum.go**: archivo nuevo
- **pingpong/client_test.go**: archivo nuevo
- **pingpong/runner_test.go**: archivo nuevo
- **pingpong/verifier_strict_test.go**: archivo nuevo

## [0.1.16] - 2026-05-03

> **Release**: expandir charlie

### Charlie

- **framework-charlie/INITIAL_PROMPT.md**: +11 / -11
- **charlie/main.go**: -35
- **charlie/deploy.go**: -156

### Repo

- **Makefile**: +4 / -4
- **framework-deployer/framework.manifest.json**: archivo nuevo
- **framework-deployer/go.mod**: archivo nuevo
- **README.md**: +19
- **framework-deployer/INITIAL_PROMPT.md**: archivo nuevo
- **framework-deployer/README.md**: archivo nuevo
- **deployer/deploy.go**: archivo nuevo
- **deployer/main.go**: archivo nuevo

## [0.1.15] - 2026-05-03

> **Release**: expandir charlie

### Charlie

- **framework-charlie/INITIAL_PROMPT.md**: +15
- **charlie/deploy.go**: archivo nuevo
- **charlie/main.go**: +35

## [0.1.14] - 2026-05-03

> **Release**: actualizar proyecto

### Repo

- **scripts/setup-prod.sh**: +18
- **scripts/setup-secrets.sh**: +14 / -1

## [0.1.13] - 2026-05-03

> **Release**: expandir charlie

### Charlie

- **charlie/charlie.go**: +1 / -1

### Repo

- **Makefile**: +6 / -2
- **scripts/setup-prod.sh**: archivo nuevo
- **README.md**: +33 / -9

## [0.1.12] - 2026-05-03

> **Release**: expandir flujo

### Flujo

- **flujo_api/main.go**: funciones: (s *server) healthz | tipos: checkResult

### Repo

- **cloudbuild-ci.yaml**: archivo nuevo
- **scripts/setup-secrets.sh**: archivo nuevo
- **README.md**: +49 / -5

## [0.1.11] - 2026-05-03

> **Release**: expandir charlie

### Charlie

- **framework-charlie/INITIAL_PROMPT.md**: +15
- **charlie/clean.go**: archivo nuevo
- **charlie/main.go**: +24

### Repo

- **.gitignore**: -3
- **Makefile**: archivo nuevo
- **scripts/bootstrap.sh**: archivo nuevo

## [0.1.10] - 2026-05-03

> **Release**: profesionalizar repo (Fase 1)

### Repo

- **.gitignore**: archivo nuevo (regla unica: solo codigo fuente)
- **.env.example**: archivo nuevo (plantilla env vars para local + prod)
- **README.md**: archivo nuevo (onboarding 5 pasos + convenciones)

### Limpieza (untrack)

- **8 .DS_Store**: removidos del index (macOS junk)
- **5 binarios compilados**: removidos (channel, frameworkecho, frameworksabio, flujo_api, flujo_api/channel)
- **framework-indexa/data/panalbit.db**: removido (data del cliente, no versionable)
- **framework-paladin/.env**: removido (patron de leak, ahora .env.example es el template)
- **22 archivos en temp/**: removidos (traces de paladin + uploads regenerables)

## [0.1.9] - 2026-05-03

> **Release**: expandir flujo

### Flujo

- **frontend-chat/index.html**: +510 / -223
- **handoff/questions_queue.go**: funciones: (q *QuestionsQueue) AddQuestion

### Repo

- **framework-foco/framework.manifest.json**: archivo nuevo
- **framework-foco/go.mod**: +20 / -1
- **framework-hosting/go.mod**: archivo nuevo
- **framework-mecanico/framework.manifest.json**: +5
- **framework-mecanico/go.mod**: +2
- **framework-mecanico/go.sum**: archivo nuevo
- **framework-mecanico/mecanico**: archivo nuevo
- **framework-sabio/go.mod**: +3
- **cobranza-chile/flow.rules.json**: archivo nuevo
- **cobranza-chile/profile.json**: archivo nuevo
- **views/action_proposal.json**: archivo nuevo
- **views/deudor_card.json**: archivo nuevo
- **views/priority_list.json**: archivo nuevo
- **cobranza-chile/foco.md**: archivo nuevo
- **cobranza-chile/glossary.md**: archivo nuevo
- **cobranza-chile/mecanico.md**: archivo nuevo
- **cobranza-chile/sabio.md**: archivo nuevo
- **profile/profile.go**: archivo nuevo
- **foco/cobranza_sql.go**: archivo nuevo
- **foco/main.go**: funciones: runNextQuestion, runIngestAnswer, runQuery, runPriorities
- **cpanel/client.go**: archivo nuevo
- **cpanel/email.go**: archivo nuevo
- **creds/store.go**: archivo nuevo
- **frameworkmecanico/main.go**: funciones: cmdDraftEmail, generateEmailDraft, formatDraftForUser, urlEncode | tipos: emailDraft
- **frameworksabio/main.go**: funciones: getProfile, systemPromptWithOverlay
- **internal/whitelist.go**: +2

## [0.1.8] - 2026-05-01

> **Release**: expandir charlie

### Charlie

- **charlie-bloqueo-git**: bloqueo de operaciones git peligrosas (reset --hard, push --force) (charlie/apply_propose.go, charlie/doctor.go)
- **main-multi-modo**: main.go soporta multiples modos (audit, explain, tree) (charlie/main.go)
- **framework-charlie/.charlieignore**: archivo nuevo
- **framework-charlie/INITIAL_PROMPT.md**: +40 / -4
- **framework-charlie/README.md**: +30 / -1
- **charlie/charlie.go**: +8 / -1
- **charlie/ignore.go**: archivo nuevo
- **charlie/intent.go**: archivo nuevo
- **charlie/audit.go**: archivo nuevo
- **charlie/doctor_test.go**: archivo nuevo


## [0.1.7] - 2026-05-01

> **Release**: expandir echo, excel, flujo, quine

### Quine

- **framework-quine/go.mod**: +4
- **quine/main.go**: +4 / -6
- **review/review.go**: +33
- **types/types.go**: +12

### Flujo

- **flujo_api/.dockerignore**: archivo nuevo
- **flujo_api/Dockerfile**: archivo nuevo
- **flujo_api/channel**: archivo nuevo
- **flujo_api/deploy.sh**: archivo nuevo
- **flujo_api/entrypoint.sh**: archivo nuevo
- **flujo_api/flow.rules.json**: +15 / -8
- **static/index.html**: archivo nuevo
- **remora-flujo/flujo_test**: archivo nuevo
- **frontend-chat/index.html**: +2093 / -503
- **flujo_api/drivers.go**: funciones: initDriverRegistry, keysOf
- **flujo_api/generic_driver.go**: archivo nuevo
- **flujo_api/main.go**: funciones: getRuntimeInfo, (s *server) getRuntime, (s *server) listModels, (s *server) getRules | tipos: runtimeInfo, createSingleConvRequest

### Echo

- **framework-echo/frameworkecho.json**: +5 / -90

### Excel

- **framework-excel/excel**: configuracion

### Repo

- **main-multi-modo**: main.go soporta multiples modos (audit, explain, tree) (frameworkauditor/main.go, frameworkmecanico/main.go)
- **channel/channel**: configuracion
- **channel/channel-new**: archivo nuevo
- **channel/orchestrator**: archivo nuevo
- **cloudbuild.yaml**: archivo nuevo
- **data/dataset.golden.json**: archivo nuevo
- **data/dataset.working.json**: archivo nuevo
- **data/findings.json**: archivo nuevo
- **framework-auditor/framework.manifest.json**: archivo nuevo
- **framework-auditor/frameworkauditor**: archivo nuevo
- **framework-auditor/go.mod**: archivo nuevo
- **framework-foco/foco**: configuracion
- **framework-foco/foco_state.json**: -75
- **data/dump.json**: archivo nuevo
- **data/panalbit.db**: archivo nuevo
- **data/store.json**: archivo nuevo
- **framework-indexa/framework.manifest.json**: archivo nuevo
- **framework-indexa/frameworkindexa**: archivo nuevo
- **framework-indexa/go.mod**: archivo nuevo
- **framework-indexa/go.sum**: archivo nuevo
- **framework-indexa/panalbit-sync**: archivo nuevo
- **data/applied.jsonl**: archivo nuevo
- **data/proposals.json**: archivo nuevo
- **framework-mecanico/framework.manifest.json**: archivo nuevo
- **framework-mecanico/frameworkmecanico**: archivo nuevo
- **framework-mecanico/go.mod**: archivo nuevo
- **framework-sabio/framework.manifest.json**: archivo nuevo
- **framework-sabio/frameworksabio**: archivo nuevo
- **framework-sabio/go.mod**: archivo nuevo
- **framework-sabio/go.sum**: archivo nuevo
- **scripts/demo_aceleradora.sh**: archivo nuevo
- **sessions/conv_1777508171200582000.jsonl**: -30
- **sessions/conv_1777508580699470000.jsonl**: -10
- **sessions/conv_1777508715131471000.jsonl**: -33
- **sessions/conv_1777509252828881000.jsonl**: -32
- **sessions/conv_1777510967690659000.jsonl**: -12
- **sessions/conv_1777511106511589000.jsonl**: -20
- **sessions/conv_1777511380221364000.jsonl**: -15
- **sessions/conv_1777511522670900000.jsonl**: -15
- **sessions/conv_1777588675507545000.jsonl**: archivo nuevo
- **sessions/conv_1777590457233338000.jsonl**: archivo nuevo
- **sessions/conv_1777590556636254000.jsonl**: archivo nuevo
- **sessions/conv_1777590615947918000.jsonl**: archivo nuevo
- **sessions/conv_1777590961468041000.jsonl**: archivo nuevo
- **sessions/conv_1777591066591723000.jsonl**: archivo nuevo
- **sessions/conv_1777591126226500000.jsonl**: archivo nuevo
- **sessions/conv_1777591289579492000.jsonl**: archivo nuevo
- **sessions/conv_1777596871412664000.jsonl**: archivo nuevo
- **sessions/conv_1777596933569136000.jsonl**: archivo nuevo
- **sessions/conv_1777596938610675000.jsonl**: archivo nuevo
- **sessions/conv_1777597083632122000.jsonl**: archivo nuevo
- **sessions/demo-completo-1777505315.jsonl**: -3
- **manifest/manifest.go**: funciones: (m *Manifest) EffectiveExecutionMode, Discover, (m *Manifest) Validate | tipos: CapabilitiesSemantic
- **checks/checks.go**: archivo nuevo
- **panalbit-sync/main.go**: archivo nuevo
- **sqlbuilder/sqlbuilder.go**: archivo nuevo
- **store/store.go**: archivo nuevo
- **fixers/fixers.go**: archivo nuevo
- **frameworksabio/main.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **sqlqa/sqlqa.go**: archivo nuevo
- **internal/whitelist.go**: +10
- **frameworkindexa/main.go**: archivo nuevo

## [0.1.4] - 2026-04-26

> **Release**: expandir charlie, echo, flujo, gmail, paladin, quine

### Charlie

- **charlie-validar-operacion**: validacion de directorio antes de operar
- **charlie-bloqueo-git**: bloqueo de operaciones git peligrosas (reset --hard, push --force)
- **charlie-report**: generacion de reporte con version y changelog
- **semantic-tags**: detector de patrones semanticos para changelogs descriptivos

- **charlie-validar-operacion**: validacion de directorio antes de operar (charlie/charlie.go)
- **framework-charlie/INITIAL_PROMPT.md**: +67 / -2
- **framework-charlie/README.md**: +121 / -6
- **charlie/main.go**: +100 / -1
- **charlie/charlie_test.go**: funciones: TestBackupSkipsGitAndGeneratedArtifacts, TestFormatAmendPlanBlocksUnsafeReleaseRewrite, TestSameReleaseCommitMessage, TestCommitMessageVersion

- **framework-charlie/INITIAL_PROMPT.md**: +7
- **framework-charlie/README.md**: +8
- **charlie/charlie.go**: funciones: FullCommit, RemoteTagCommit, BuildPublishTagPlan, buildPublishTagPlan | tipos: PublishTagPlan
- **charlie/main.go**: +24
- **charlie/charlie_test.go**: funciones: TestFormatPublishTagPlanShowsApplyCommand

- **framework-foco/go.mod**: archivo nuevo
- **framework-foco/INITIAL_PROMPT.md**: archivo nuevo
- **framework-foco/README.md**: archivo nuevo
- **framework-foco/WHY.md**: archivo nuevo
- **foco/main.go**: archivo nuevo

- **framework-charlie/INITIAL_PROMPT.md**: +8
- **framework-charlie/README.md**: +19 / -1
- **charlie/charlie.go**: funciones: RemoteBranchCommit, IsAncestor, shortSHA, BuildPublishMainPlan | tipos: PublishMainPlan
- **charlie/main.go**: +20
- **charlie/charlie_test.go**: funciones: TestFormatPublishMainPlanShowsApplyCommand

### Paladin

- **API semantica**: Nuevos metodos en Context para declarar logica de negocio:
  - `Actor(name, responsibility)`: Quién actúa
  - `Goal(goal)`: Intención del span
  - `Event(subject, summary, meta)`: Evento de negocio
  - `Rule(name, summary, meta)`: Regla aplicada
  - `Check(rule, expected, actual, passed)`: Evaluación de regla
  - `Expect(subject, expected)`: Estado esperado después
  - `Handoff(from, to, reason)`: Transferencia de control
  - `Violation(subject, expected, actual)`: Inconsistencia detectada
- **tipo SemanticEvent**: Estructura para eventos de negocio
- **paladin-audit**: comando `audit` para evaluar si un repo implementa Paladin correctamente
- **paladin-explain**: comando `explain` para traducir trace a lenguaje humano
- **paladin-client**: TraceClient para enviar traces al servidor
- **paladin-server**: Servidor HTTP para recibir traces
- **examples/03_semantic_flow**: Nuevo ejemplo de flujo semantico

### Quine

- **quine-taxonomia-comandos**: Taxonomia semantica de comandos
- **quine-checklist-comandos**: Checklist para verificar que INITIAL_PROMPT sea ejecutable
- **quine-why**: Generacion automatica de WHY.md en nuevos frameworks
- **quine-metodos-integracion**: Metodos Register, Connect, Validate para frameworks de integracion

### Flujo

- **cola-preguntas**: Sistema de cola de preguntas para control de turnos entre Alfa y Echo
- **eventos nuevos**: `echo_user_answered`, `alfa_ceded_to_echo`, `alfa_asks_question`
- **flujo-terminal-handoff**: Comandos `done` y `ask-echo` detienen el agente
- **flujo-groq-fallback**: Recuperacion de errores `failed_generation` de Groq
- **flujo-shell-fallback**: Extraccion de comandos shell de texto y tool calls

### Gmail

- **nuevo framework**: Framework Gmail con cliente, types, cmd y documentacion

### Echo

- **frameworkecho.json**: Rediseño del árbol de conocimiento

---

## [0.1.3] - 2026-04-25

> **Release**: Actualizar Framework Charlie con formato de commit grupal

### Actualización: Framework Charlie

- **Nuevo formato de commit**: `chore: commit vVERSION - descripción`
- **Regla de un solo commit por versión**: No más commits insignificantes
- **CHANGELOG.md obligatorio**: Siempre actualizar después de cada commit
- **Detección de nuevos frameworks**: Automatic detection para minor bumps
- **Ignorar archivos**: .DS_Store, binarios, examples/, temp/
- **Scope detection**: Detecta automáticamente framework desde file path
- **Lógica de versión**: Nuevos frameworks → minor, cambios → patch

---

## [0.1.2] - 2026-04-25

> **Release**: 5 nuevos frameworks + expansiones de Paladin y Echo

### Nuevo: Framework Charlie

- **Framework de versionado y changelog** para el proyecto Remora
- Sistema de clasificación de cambios (feat, fix, docs, test, chore, etc.)
- Reglas SemVer integradas (major, minor, patch bumps)
- Changelog automático en formato Keep a Changelog
- CLI para verificar estado del repo y proponer commits
- Archivos: INITIAL_PROMPT.md, README.md, frameworkcharlie.json, go.mod, charlie.go, charlie_test.go

### Nuevo: Framework Excel

- **Framework para conectar, leer y escribir archivos Excel**
- Soporte para lectura de archivos Excel completos
- Soporte para leer hojas específicas
- Acceso a valores de celdas individuales
- Cliente con tracing integrado (Paladin)
- Estructura modular: cmd/, internal/, temp/

### Nuevo: Framework Quine

- **Framework de quines auto-replicantes** para el proyecto Remora
- Sistema de revisión de código
- Integración con Paladin para tracing
- AGENTS.md e INITIAL_PROMPT.md para guías de uso
- Estructura: cmd/, internal/quine/, internal/review/, internal/types/

### Expansión: Framework Paladin

- **SYSTEM.md**: Nuevo prompt del sistema con documentación completa
- **docs/MERE.md**: Documentación de la estructura MERE
- **examples/**: Dos ejemplos nuevos:
  - `01_basic/`: Uso básico de tracing
  - `02_decisions/`: Ejemplo de decisiones lógicas con contexto

### Expansión: Framework Echo

- **cmd/framework-echo/**: Nuevo ejecutable principal
- **internal/paladin/**: Módulo de tracing integrado en Echo
  - console.go, context.go, span.go, trace.go
- **docs/SYSTEM_PROMPT.md**: Actualizado con nuevas instrucciones

---

## [0.1.1] - 2026-04-25

> **Important**: This release generalizes the MERE data model to work across any business domain, replacing domain-specific entities (payments, invoices, etc.) with generic patterns.

### Framework Alfa

#### Breaking Changes

- **Generic MERE Entities**: Replaced domain-specific entities with domain-agnostic patterns:
  - Removed: `recurso_recibido`, `planilla_actual`, `documento_comercial_actual`, `movimiento_de_dinero_actual`, `contraparte`, `documento_comercial`, `pago`, `aplicacion_pago`
  - Added: `artefacto_actual`, `registro_actual`, `actor_actual`, `objeto_operativo_actual` (current state)
  - Added: `actor`, `entidad_negocio`, `evento_operativo`, `relacion_normalizada`, `evidencia`, `estado_historial` (normalized target)

- **Generic Cardinality Questions**: Replaced "pagos parciales" question with:
  > "Cuando relacionan esos elementos, ¿la relación es siempre 1 a 1, puede ser 1 a muchos, muchos a muchos, parcial o con excepciones?"

#### New Features

- **Domain-Agnostic Data Model**: Alfa now compiles generic MERE structures that can apply to any business:
  - `actor`: Person, organization, area or system with a defined role
  - `entidad_negocio`: Main thing the business needs to track, classify or decide about (name not fixed without Echo evidence)
  - `evento_operativo`: Event that changes state, history, amount, priority, responsible or decision on an entity
  - `relacion_normalizada`: Generic associative entity for crosses between two or more elements when cardinality is unconfirmed
  - `evidencia`: Original artifact that backs structured data and enables auditing
  - `estado_historial`: History of states or stages when process depends on temporal tracking

- **Advanced Gap Detection Functions**:
  - `evidenceLikely()`: Detects unstructured resources (whatsapp, capture, image, photo, receipt, email, pdf, file, paper, message)
  - `relationshipLikely()`: Detects relationship needs (cruzar, cruce, relacionar, associate, calzar, conciliar, match)
  - `identifierGapLikely()`: Detects potential duplicate issues
  - `cardinalityConfirmed()`: Checks if cardinality (1:1, 1:N, N:M, partial, etc.) was explicitly mentioned

- **Generic Business Rules**:
  - `data_rule_001`: Don't invent domain entities (Alfa can propose generic MERE but cannot fix names, fields or rules without Echo evidence)
  - `data_rule_002`: Don't assume cardinality (when flow relates elements, Alfa cannot assume 1:1, 1:N or N:M without confirmation)
  - `data_rule_003`: Preserve original evidence (automation must maintain link between normalized data and original resource)

- **Updated Compilation Logic**:
  - `normalizedEntities()`: Builds generic entity structure from conversation patterns
  - `normalizedRelationships()`: Builds relationship model with cardinality_unconfirmed flags
  - `dataModelGaps()`: Generates open questions based on detected gaps in relationships, identifiers and evidence context

- **MERE Verbalization for Bravo**: Export now includes full generic MERE structure with entities, relationships and business rules

#### Improvements

- `INITIAL_PROMPT.md`: Updated MERE section to emphasize generic patterns over domain-specific examples
- `compile.go`: Refactored with helper functions for cleaner, more maintainable code
- `compile_test.go`: Updated tests to use generic relationship patterns instead of payment-specific scenarios

### Framework Echo

- **Enhanced Resource Detection**: `readiness.go` now detects more evidence sources (message, chat, email, pdf, file, document, paper)
- **Generic Context Commitment**: Updated wording to handle any resource-to-record linking, not just payments/invoices
  - Before: "Para automatizar esto necesito unir transferencia, factura y cliente..."
  - After: "Para automatizar esto necesito unir cada recurso con el registro correcto..."
- **Updated Prompts**: AGENTS.md, INITIAL_PROMPT.md, SYSTEM_PROMPT.md all refined with generic patterns

### Documentation

- `nuevo_mapa.md`: Added clarification that cardinality rules are general, not specific to payments:
  > "La regla general no es 'pagos y facturas'. La regla general es: cuando la automatización necesita relacionar elementos, Alfa debe saber si la relación es 1 a 1, 1 a muchos, muchos a muchos, parcial, temporal o con excepciones."

---

## [0.1.0] - 2026-04-24

### Initial Release

#### Frameworks

- **Framework Alfa**: Compilation engine for translating Echo's validated intent into verifiable flow specs for Bravo
- **Framework Bravo**: Flow verification and validation framework
- **Framework Echo**: Discovery and opportunity validation framework
- **Remora Flujo**: Main orchestration layer

#### Core Capabilities

- Tree-based opportunity validation (OPPORTUNITY -> PAIN -> TASK -> THEORY -> AXIOM lineage)
- Draft compilation for early-stage opportunities
- Readiness-based compilation gates
- Export-ready validation before Bravo handoff
- Open questions tracking for incomplete information