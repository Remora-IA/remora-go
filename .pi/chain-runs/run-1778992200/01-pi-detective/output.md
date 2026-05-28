# DETECTIVE: remora-go-lite - 2026-05-17T00:10:00Z

## Módulos analizados

| Módulo | Binarios | Métodos totales | Sinks |
|--------|----------|-----------------|-------|
| remora-cli | remora, devcli | 180 | env:1, http:5 |
| remora-flujo | api_rest, flujo, agentrpc, framework_session, llmtest, nativeagent, autonomia, handoff, llm, frontends | 1696 | sql:59, file:44, env:46, http:31, exec:27 |
| channel | channel, orchestrator, vault, alfa-runner, echo-runner, adapter, credentials, internal, profile, manifest | 225 | env:7, exec:2, file:5, http:5 |

---

## Binarios por módulo

| Módulo | Binario | Entry point | CLI surface |
|--------|---------|-------------|-------------|
| remora-cli | remora | remora/flow_workbench.go:237 (handleFlowWorkbench) | flow, flow create, flow draft, flow inspect, flow simulate, flow debug |
| remora-cli | devcli | devcli/main.go:15 (main) | framework ls, flow list, flow run, flow simulate, health |
| remora-flujo | api_rest | api_rest/main.go (HTTP :8084) | 70+ rutas REST: /auth, /businesses, /flows, /frameworks, /conversations, /tasks |
| remora-flujo | flujo | flujo/flow_workbench.go:338 (cmdFlow) | flow run, flow debug, flow create, flow draft, flow compile, flow validate |
| remora-flujo | agentrpc | agentrpc/main.go:30 (main) | gRPC-style RPC server |
| remora-flujo | framework_session | framework_session/main.go (runStart/runMessage) | session start, session message |
| remora-flujo | llmtest | llmtest/main.go | LLM test harness |
| remora-flujo | nativeagent | nativeagent/agent.go (Prompt/PromptWithImages) | agente con bash/write/read tools |
| remora-flujo | autonomia | autonomia/session.go:95 (HandleMessage) | HandleMessage (sesión autónoma controlada) |
| remora-flujo | handoff | handoff/questions_queue.go (AddQuestion/GetNextPending) | gestión de preguntas pendientes |
| remora-flujo | llm | llm/client.go:65 (New) | Complete, Stream (MiniMax/Groq/OpenRouter) |
| remora-flujo | frontends | frontends/serve.go | sirve frontend estático :8085 |
| channel | channel | channel/main.go:69 (envOr/loadDotEnv) | servidor HTTP :8080 (punto de entrada) |
| channel | orchestrator | orchestrator/main.go (usage/discover/cmdList/cmdRun) | list, chains, run, chain |
| channel | vault | vault/main.go:77 (cmdHas/cmdGet/cmdSet/cmdDelete) | has, get, set, delete, gen-key |
| channel | alfa-runner | alfa-runner/main.go:29 (main/step) | runner de pasos Alfa |
| channel | echo-runner | echo-runner/main.go (runEcho/runEchoSimple) | runner de sesiones Echo |
| channel | adapter | adapter/adapter.go:36 (New) | ExecuteCommand, ReadFile, WriteFile, Grep, Find, HTTPGet |
| channel | credentials | credentials/smtp.go:35 (LoadSMTP/SaveSMTP) | LoadSMTP, SaveSMTP, DeleteSMTP |
| channel | internal | internal/handler.go:29 (NewHandler/Handle) | JSON-RPC: execute_command, read_file, write_file, grep, find, edit_file, http_get |
| channel | profile | profile/profile.go:46 (NewLoader/Load) | Load, LoadNamed, OverlayFor, SystemPromptWithOverlay |
| channel | manifest | manifest/manifest_test.go | test: ResolveArgs |

---

## Módulo: remora-cli

### Binario: remora

#### Métodos (63)

| Método | Archivo | Línea |
|--------|---------|-------|
| handleFlowWorkbench | remora/flow_workbench.go | 237 |
| printDebugUsage | remora/flow_workbench.go | 259 |
| handleDebugCommand | remora/flow_workbench.go | 274 |
| handleDebugFrameworks | remora/flow_workbench.go | 310 |
| handleDebugManifest | remora/flow_workbench.go | 373 |
| handleDebugCommands | remora/flow_workbench.go | 401 |
| handleDebugCapabilities | remora/flow_workbench.go | 444 |
| handleDebugTrace | remora/flow_workbench.go | 505 |
| handleDebugValidate | remora/flow_workbench.go | 539 |
| handleDebugSimulate | remora/flow_workbench.go | 572 |
| handleDebugDependencies | remora/flow_workbench.go | 625 |
| formatFrameworkManifest | remora/flow_workbench.go | 663 |
| printRunTrace | remora/flow_workbench.go | 684 |
| printFlowValidation | remora/flow_workbench.go | 755 |
| printSimulateResult | remora/flow_workbench.go | 781 |
| printFlowDependencies | remora/flow_workbench.go | 826 |
| delegateToCanonicalFlowWorkbench | remora/flow_workbench.go | 880 |
| canonicalFlowWorkbenchCommand | remora/flow_workbench.go | 895 |
| printFlowWorkbenchUsage | remora/flow_workbench.go | 907 |
| flowAutonomySummary | remora/flow_workbench.go | 924 |
| buildIntentFirstDescription | remora/flow_workbench.go | 935 |
| buildFlowCreateSuggestPayload | remora/flow_workbench.go | 949 |
| flowAutonomyConstraints | remora/flow_workbench.go | 972 |
| inferFlowCreateRoles | remora/flow_workbench.go | 977 |
| applyFlowCreateIntentModel | remora/flow_workbench.go | 1006 |
| buildFlowCreateLifecycle | remora/flow_workbench.go | 1032 |
| emptyCLILifecycle | remora/flow_workbench.go | 1045 |
| cliLifecycleBindingLabel | remora/flow_workbench.go | 1049 |
| fetchBusinessArtifacts | remora/flow_workbench.go | 1064 |
| promptFlowCreateAnswers | remora/flow_workbench.go | 1076 |
| promptFlowField | remora/flow_workbench.go | 1100 |
| handleFlowCreate | remora/flow_workbench.go | 1114 |
| printFlowCreatePreview | remora/flow_workbench.go | 1218 |
| handleFlowDraft | remora/flow_workbench.go | 1235 |
| handleFlowInspect | remora/flow_workbench.go | 1312 |
| handleFlowSimulate | remora/flow_workbench.go | 1330 |
| mustFetchFlowRecord | remora/flow_workbench.go | 1387 |
| decodeAPIData | remora/flow_workbench.go | 1402 |
| formatFlowWorkbench | remora/flow_workbench.go | 1414 |
| parseCSVList | remora/flow_workbench.go | 1523 |
| sortedList | remora/flow_workbench.go | 1538 |
| compactStrings | remora/flow_workbench.go | 1544 |
| firstNonEmpty | remora/flow_workbench.go | 1555 |
| printFrameworkName | remora/main.go | 298 |
| printStatus | remora/main.go | 303 |
| formatTime | remora/main.go | 330 |

#### CLI surface

| Comando | Handler | Descripción |
|---------|---------|-------------|
| flow | handleFlowWorkbench | Dispatcher raíz |
| flow create | handleFlowCreate | Wizard interactivo de creación |
| flow draft | handleFlowDraft | Crear desde YAML/JSON spec |
| flow inspect | handleFlowInspect | Inspeccionar flow existente |
| flow simulate | handleFlowSimulate | Simular ejecución |
| flow debug | handleDebugCommand | Dispatcher de debug |
| flow debug frameworks | handleDebugFrameworks | Info de frameworks |
| flow debug manifest | handleDebugManifest | Ver manifest de framework |
| flow debug commands | handleDebugCommands | Listar comandos |
| flow debug capabilities | handleDebugCapabilities | Listar capabilities |
| flow debug trace | handleDebugTrace | Ver trazas de run |
| flow debug validate | handleDebugValidate | Validar flow |
| flow debug simulate | handleDebugSimulate | Simular sin API |
| flow debug dependencies | handleDebugDependencies | Ver dependencias |

#### Sinks

| Tipo | Código | Archivo:Línea |
|------|--------|---------------|
| exec | var flowWorkbenchExecCommand = exec.Command | remora/flow_workbench.go:235 |
| http | get() → http.NewRequest | remora/main.go:73 |
| http | post() → http.NewRequest | remora/main.go:99 |

---

### Binario: devcli

#### Métodos (28)

| Método | Archivo | Línea |
|--------|---------|-------|
| fwColor | devcli/client.go | 47 |
| newClient | devcli/client.go | 60 |
| get | devcli/client.go | 71 |
| post | devcli/client.go | 93 |
| GetFrameworks | devcli/client.go | 251 |
| GetFramework | devcli/client.go | 286 |
| ListFlows | devcli/client.go | 322 |
| GetFlow | devcli/client.go | 343 |
| RunFlow | devcli/client.go | 356 |
| SimulateFlow | devcli/client.go | 369 |
| GetFlowRun | devcli/client.go | 381 |
| GetRules | devcli/client.go | 394 |
| getString | devcli/client.go | 403 |
| getBool | devcli/client.go | 410 |
| toStrings | devcli/client.go | 417 |
| parseFlowManifest | devcli/client.go | 427 |
| parseFlowRunResult | devcli/client.go | 489 |
| GetProviders | devcli/client.go | 588 |
| HealthCheck | devcli/client.go | 608 |
| main | devcli/main.go | 15 |
| printJSON | devcli/main.go | 424 |
| printFlowManifest | devcli/main.go | 428 |
| printFlowRunResult | devcli/main.go | 468 |
| joinStrings | devcli/main.go | 601 |
| formatDuration | devcli/main.go | 612 |
| formatTimestamp | devcli/main.go | 625 |

#### CLI surface

urfave/cli app con subcomandos: framework ls, framework show, flow list, flow show, flow run, flow simulate, flow run-get, rules, health. Backend: REMORA_API_URL (default http://localhost:8084).

#### Sinks

| Tipo | Código | Archivo:Línea |
|------|--------|---------------|
| env | os.Getenv("REMORA_API_URL") | devcli/client.go:61 |
| http | get() → http.NewRequest → Do | devcli/client.go:73,80 |
| http | post() → http.NewRequest → Do | devcli/client.go:99,107 |
| http | HealthCheck() → Do | devcli/client.go:613 |

---

## Módulo: remora-flujo

### Binario: api_rest

#### Métodos (987) — selección de handlers clave

| Método | Archivo | Línea |
|--------|---------|-------|
| handleAuthRegister | api_rest/auth_handlers.go | 42 |
| handleAuthLogin | api_rest/auth_handlers.go | 68 |
| handleAuthLogout | api_rest/auth_handlers.go | 94 |
| handleAuthMe | api_rest/auth_handlers.go | 103 |
| handleBusinesses | api_rest/auth_handlers.go | 117 |
| handleBusinessCreate | api_rest/auth_handlers.go | 130 |
| handleAdminUsers | api_rest/auth_handlers.go | 157 |
| handleAdminTeam | api_rest/auth_handlers.go | 169 |
| handleAdminRemoraInviteCreate | api_rest/auth_handlers.go | 181 |
| handleRemoraInviteLookup | api_rest/auth_handlers.go | 203 |
| handleRemoraInviteAccept | api_rest/auth_handlers.go | 213 |
| handleBusinessMembers | api_rest/auth_handlers.go | 232 |
| handleBusinessInviteCreate | api_rest/auth_handlers.go | 245 |
| handleInviteLookup | api_rest/auth_handlers.go | 273 |
| handleInviteAccept | api_rest/auth_handlers.go | 283 |
| handleBusinessArtifacts | api_rest/business_artifacts.go | 19 |
| handleDataTables | api_rest/data_browser.go | 44 |
| handleDataTableRows | api_rest/data_browser.go | 64 |
| handleBusinessDataTables | api_rest/data_browser.go | 114 |
| handleBusinessDataTableRows | api_rest/data_browser.go | 133 |
| handleBusinessDataUpload | api_rest/data_browser.go | 150 |
| ensureChannel | api_rest/flow_channel.go | 56 |
| buildChannelBinary | api_rest/flow_channel.go | 134 |
| handleGetCompiledFlow | api_rest/flow_compiled_store.go | 98 |
| handleGetFlowRun | api_rest/flow_compiled_store.go | 113 |
| contactosLookupProfile | api_rest/contactos.go | 51 |
| contactosStoreProfile | api_rest/contactos.go | 83 |
| handleSMTPCheck | api_rest/flow_smtp.go | 16 |
| handleHostingConnect | api_rest/flow_smtp.go | 79 |
| handleSMTPImport | api_rest/flow_smtp.go | 147 |
| handleSMTPDelete | api_rest/flow_smtp.go | 224 |
| handleListFlows | api_rest/flow_store.go | 745 |
| handleCreateFlow | api_rest/flow_store.go | 763 |
| handleGetFlow | api_rest/flow_store.go | 796 |
| handleInstallFlow | api_rest/flow_store.go | 811 |
| handleUpdateFlow | api_rest/flow_store.go | 831 |
| handleDeleteFlow | api_rest/flow_store.go | 868 |
| handleListFlowTemplates | api_rest/flow_templates.go | 97 |
| handleConfig | api_rest/main.go | 1141 |
| handleSendEmail | api_rest/main.go | 1150 |
| handleAutonomiaBootstrap | api_rest/simulation_autonomia.go | 15 |
| handleAutonomiaMessage | api_rest/simulation_autonomia.go | 23 |
| handleTasksList | api_rest/tareas.go | 89 |
| handleTasksNext | api_rest/tareas.go | 99 |
| handleTasksCreate | api_rest/tareas.go | 117 |
| handleTaskEvent | api_rest/tareas.go | 144 |
| handleTracesLatest | api_rest/traces.go | 20 |
| openAuthStore | api_rest/auth.go | 104 |
| migrate | api_rest/auth.go | 133 |
| seedDefaults | api_rest/auth.go | 242 |
| createUser | api_rest/auth.go | 257 |
| authenticate | api_rest/auth.go | 291 |
| createSession | api_rest/auth.go | 306 |
| activeTaskContext | api_rest/active_task.go | 35 |
| sanitizeForArg | api_rest/active_task.go | 107 |

#### Rutas HTTP registradas (api_rest — puerto :8084)

| Método | Ruta | Handler |
|--------|------|---------|
| GET | /health | health |
| GET | /healthz | healthz |
| POST | /api/v1/auth/register | handleAuthRegister |
| POST | /api/v1/auth/login | handleAuthLogin |
| POST | /api/v1/auth/logout | handleAuthLogout |
| GET | /api/v1/auth/me | handleAuthMe |
| GET | /api/v1/businesses | handleBusinesses |
| POST | /api/v1/businesses | handleBusinessCreate |
| GET | /api/v1/businesses/{business_id}/members | handleBusinessMembers |
| POST | /api/v1/businesses/{business_id}/invites | handleBusinessInviteCreate |
| GET | /api/v1/invites/lookup | handleInviteLookup |
| POST | /api/v1/invites/accept | handleInviteAccept |
| GET | /api/v1/admin/users | handleAdminUsers |
| GET | /api/v1/admin/team | handleAdminTeam |
| POST | /api/v1/admin/remora-invites | handleAdminRemoraInviteCreate |
| GET | /api/v1/remora-invites/lookup | handleRemoraInviteLookup |
| POST | /api/v1/remora-invites/accept | handleRemoraInviteAccept |
| GET | /api/v1/frameworks | listFrameworks |
| GET | /api/v1/frameworks/testable | listTestableFrameworks |
| GET | /api/v1/frameworks/chainable | listChainableFrameworks |
| GET | /api/v1/capabilities | listCapabilities |
| GET | /api/v1/capabilities/{id}/providers | listCapabilityProviders |
| POST | /api/v1/flows/validate | validateFlow |
| POST | /api/v1/flows/simulate | simulateFlow |
| POST | /api/v1/flows/run | runFlow |
| POST | /api/v1/flows/run/stream | runFlowStream |
| GET | /api/v1/flows/runs/{id} | handleGetFlowRun |
| POST | /api/v1/flows/suggest | suggestFlowCapabilities |
| POST | /api/v1/flows/workbench/compile | compileFlowWorkbench |
| GET | /api/v1/flows/compiled/{compiled_id} | handleGetCompiledFlow |
| GET | /api/v1/businesses/{business_id}/flow-templates | handleListFlowTemplates |
| GET | /api/v1/businesses/{business_id}/flows | handleListFlows |
| POST | /api/v1/businesses/{business_id}/flows | handleCreateFlow |
| POST | /api/v1/businesses/{business_id}/hosting/connect | handleHostingConnect |
| GET | /api/v1/businesses/{business_id}/smtp/check | handleSMTPCheck |
| POST | /api/v1/businesses/{business_id}/smtp/import | handleSMTPImport |
| DELETE | /api/v1/businesses/{business_id}/smtp | handleSMTPDelete |
| POST | /api/v1/flows/{id}/install | handleInstallFlow |
| GET | /api/v1/flows/{id} | handleGetFlow |
| PUT | /api/v1/flows/{id} | handleUpdateFlow |
| DELETE | /api/v1/flows/{id} | handleDeleteFlow |
| GET | /api/v1/conversations | listConversations |
| POST | /api/v1/conversations | createConversation |
| GET | /api/v1/conversations/{id} | getConversation |
| DELETE | /api/v1/conversations/{id} | deleteConversation |
| GET | /api/v1/conversations/{id}/messages | getMessages |
| POST | /api/v1/conversations/{id}/messages | postMessage |
| GET | /api/v1/conversations/{id}/queue | getQueue |
| GET | /api/v1/rules | getRules |
| PUT,POST | /api/v1/rules | updateRules |
| GET | /api/v1/frameworks/{name} | getFramework |
| POST | /api/v1/frameworks/{name}/commands/{command}/run | runFrameworkCommand |
| POST | /api/v1/conversations-single | createSingleConversation |
| POST | /api/v1/conversations-single/{id}/messages | postSingleMessage |
| GET | /api/v1/conversations-single/{id}/live | getFrameworkSessionLiveEvents |
| GET | /api/v1/runtime | getRuntime |
| GET | /api/v1/models | listModels |
| POST | /api/v1/send-email | handleSendEmail |
| GET | /api/v1/config | handleConfig |
| GET | /api/v1/traces/latest | handleTracesLatest |
| GET | /api/v1/data/tables | handleDataTables |
| GET | /api/v1/data/tables/{table} | handleDataTableRows |
| POST | /api/v1/businesses/{business_id}/data/upload | handleBusinessDataUpload |
| GET | /api/v1/businesses/{business_id}/data/tables | handleBusinessDataTables |
| GET | /api/v1/businesses/{business_id}/data/tables/{table} | handleBusinessDataTableRows |
| GET | /api/v1/businesses/{business_id}/artifacts | handleBusinessArtifacts |
| GET | /api/v1/businesses/{business_id}/api-connections | handleAPIConnectionsList |
| POST | /api/v1/businesses/{business_id}/api-connections | handleAPIConnectionCreate |
| POST | /api/v1/businesses/{business_id}/api-connections/plan | handleAPIConnectionPlan |
| POST | /api/v1/businesses/{business_id}/api-connections/{connection_id}/sync | handleAPIConnectionSync |
| GET | /api/v1/simulations/autonomia-controlada/bootstrap | handleAutonomiaBootstrap |
| POST | /api/v1/simulations/autonomia-controlada/message | handleAutonomiaMessage |
| GET | /api/v1/tasks | handleTasksList |
| POST | /api/v1/tasks | handleTasksCreate |
| GET | /api/v1/tasks/next | handleTasksNext |
| POST | /api/v1/tasks/{id}/event | handleTaskEvent |
| GET | / | Frontend canvas (static index.html) |
| GET | /data | Frontend data browser |
| GET | /app | Frontend app |

#### Sinks (api_rest — selección por tipo)

| Tipo | Código | Archivo:Línea |
|------|--------|---------------|
| exec | exec.Command(channelBin, ...) | api_rest/flow_channel.go:56 |
| exec | exec.Command("go", "build", ...) | api_rest/flow_channel.go:134 |
| exec | exec.Command(bin, ...) | api_rest/contactos.go:51 |
| exec | exec.Command(bin, ...) | api_rest/contactos.go:83 |
| exec | exec.Command(resolveVaultBin(), "set", ...) | api_rest/main.go:1111 |
| exec | exec.Command(binPath, args...) | api_rest/main.go:1254 |
| exec | exec.Command(resolveVaultBin(), "has", ...) | api_rest/main.go:1334 |
| sql | db.Exec() — migrate | api_rest/auth.go:224 |
| sql | db.QueryRow() — authenticate | api_rest/auth.go:293 |
| sql | db.Exec() — createUser | api_rest/auth.go:280 |
| sql | db.Query() — memberships | api_rest/auth.go:350 |
| file | os.WriteFile — persistFlowRun | api_rest/flow_artifacts.go:337 |
| file | os.WriteFile — persistFlowArtifact | api_rest/flow_artifacts.go:374 |
| http | http.Get() — authTokenFromRequest | api_rest/auth.go:701 |
| env | os.Getenv — seedDefaults | api_rest/auth.go:244 |

---

### Binario: flujo

#### Métodos (107) — selección

| Método | Archivo | Línea |
|--------|---------|-------|
| cmdFlow | flujo/flow_workbench.go | 338 |
| runFlowWorkbench | flujo/flow_workbench.go | 344 |
| newFlowWorkbenchClient | flujo/flow_workbench.go | 381 |
| get | flujo/flow_workbench.go | 393 |
| post | flujo/flow_workbench.go | 397 |
| stream | flujo/flow_workbench.go | 401 |
| doJSON | flujo/flow_workbench.go | 469 |
| printFlowWorkbenchUsage | flujo/flow_workbench.go | 510 |
| flowAutonomySummary | flujo/flow_workbench.go | 527 |
| buildIntentFirstDescription | flujo/flow_workbench.go | 538 |
| buildFlowCreateIntent | flujo/flow_workbench.go | 552 |
| buildFlowCreateSuggestPayload | flujo/flow_workbench.go | 565 |
| applyFlowCreateIntentHints | flujo/flow_workbench.go | 580 |
| buildFlowCreateLifecycle | flujo/flow_workbench.go | 604 |
| cliLifecycleBindingLabel | flujo/flow_workbench.go | 617 |
| emptyCLILifecycle | flujo/flow_workbench.go | 632 |
| fetchBusinessArtifacts | flujo/flow_workbench.go | 636 |
| promptFlowCreateAnswers | flujo/flow_workbench.go | 647 |
| promptFlowField | flujo/flow_workbench.go | 671 |
| printFlowCreatePreview | flujo/flow_workbench.go | 685 |
| runFlowCreate | flujo/flow_workbench.go | 704 |
| runFlowDraft | flujo/flow_workbench.go | 806 |
| buildFlowDraftSuggestPayload | flujo/flow_workbench.go | 870 |
| inferCLIIntentRoles | flujo/flow_workbench.go | 886 |
| runFlowCompile | flujo/flow_workbench.go | 915 |
| runFlowInspect | flujo/flow_workbench.go | 936 |
| runFlowValidate | flujo/flow_workbench.go | 957 |
| runFlowSimulate | flujo/flow_workbench.go | 983 |
| runFlowRun | flujo/flow_workbench.go | 1015 |
| runFlowInstall | flujo/flow_workbench.go | 1058 |

#### CLI surface

Subcomandos via `flujo flow <cmd>`: run, debug, create, draft, compile, inspect, validate, simulate, install. HTTP client a REMORA_API_URL (default :8084). También abre stream SSE a /api/v1/flows/run/stream.

#### Sinks

| Tipo | Código | Archivo:Línea |
|------|--------|---------------|
| exec | exec.Command("/bin/zsh", "-lc", "frameworkecho readiness") | flujo/main.go:319 |
| http | stream() → SSE /api/v1/flows/run/stream | flujo/flow_workbench.go:401 |

---

### Binario: agentrpc

#### Métodos (5)

| Método | Archivo | Línea |
|--------|---------|-------|
| main | agentrpc/main.go | 30 |
| handle | agentrpc/main.go | 54 |
| allowedTools | agentrpc/main.go | 84 |
| write | agentrpc/main.go | 93 |

#### CLI surface

Servidor RPC HTTP que expone handle() — llama nativeagent.Prompt() con tools. Recibe requests de agentes externos y responde con completion LLM.

---

### Binario: framework_session

#### Métodos (45) — selección

| Método | Archivo | Línea |
|--------|---------|-------|
| runStart | framework_session/main.go | 93 |
| runMessage | framework_session/main.go | 172 |
| newLiveEventWriter | framework_session/main.go | 274 |
| ensureWorkspace | framework_session/main.go | 315 |
| newToolRunner | framework_session/main.go | 326 |
| runAgentLoop | framework_session/main.go | 332 |
| streamFinal | framework_session/main.go | 393 |
| parseToolDecision | framework_session/main.go | 409 |
| parseToolDecisions | framework_session/main.go | 424 |
| execute | framework_session/main.go | 611 |
| proposeConfiguration | framework_session/main.go | 671 |
| commitConfiguration | framework_session/main.go | 690 |
| injectFrameworkCommandContext | framework_session/main.go | 750 |
| normalizeCommandAndArgs | framework_session/main.go | 800 |

#### CLI surface

Invocado por api_rest vía exec (runFrameworkCommand). Gestiona el loop de agente para sesiones de framework: start → runAgentLoop → runMessage. Emite SSE vía newLiveEventWriter.

---

### Binario: nativeagent

#### Métodos (59) — selección

| Método | Archivo | Línea |
|--------|---------|-------|
| resolveProvider | nativeagent/agent.go | 113 |
| Prompt | nativeagent/agent.go | 210 |
| PromptWithImages | nativeagent/agent.go | 214 |
| request | nativeagent/agent.go | 395 |
| requestMiniMax | nativeagent/agent.go | 415 |
| requestGroq | nativeagent/agent.go | 476 |
| toolCommandFromText | nativeagent/agent.go | 564 |
| shellCommandFromTextResponse | nativeagent/agent.go | 598 |
| successfulTerminalHandoff | nativeagent/agent.go | 657 |
| bashCommandFromInput | nativeagent/agent.go | 668 |
| runTool | nativeagent/agent.go | 956 |
| tools | nativeagent/agent.go | 999 |
| toolBash | nativeagent/agent.go | 1013 |
| validateBashPolicy | nativeagent/agent.go | 1031 |
| doRequest | nativeagent/agent.go | 716 |
| firstEnv | nativeagent/agent.go | 826 |
| loadDefaultEnvFiles | nativeagent/agent.go | 844 |

#### Sinks

| Tipo | Código | Archivo:Línea |
|------|--------|---------------|
| exec | exec.Command("/bin/zsh", "-lc", command) — toolBash | nativeagent/agent.go:1021 |
| http | requestMiniMax() → http.Do | nativeagent/agent.go:435 |
| http | requestGroq() → http.Do | nativeagent/agent.go:499 |

---

### Binario: autonomia

#### Métodos (23) — selección

| Método | Archivo | Línea |
|--------|---------|-------|
| InitialState | autonomia/session.go | 75 |
| Bootstrap | autonomia/session.go | 82 |
| HandleMessage | autonomia/session.go | 95 |
| generalResponse | autonomia/session.go | 179 |
| generalSocialResponse | autonomia/session.go | 193 |
| ensureState | autonomia/session.go | 242 |
| ensureEntity | autonomia/session.go | 255 |
| isGreeting | autonomia/session.go | 262 |
| isRepairPrompt | autonomia/session.go | 284 |
| isForecastPrompt | autonomia/session.go | 293 |
| normalize | autonomia/session.go | 301 |

#### CLI surface

Sesión autónoma controlada. Consumida por api_rest vía /api/v1/simulations/autonomia-controlada/{bootstrap,message}.

---

### Binario: handoff

#### Métodos (36) — selección

| Método | Archivo | Línea |
|--------|---------|-------|
| NewQuestionsQueue | handoff/questions_queue.go | 60 |
| LoadQuestionsQueue | handoff/questions_queue.go | 75 |
| SaveQuestionsQueue | handoff/questions_queue.go | 104 |
| AddQuestion | handoff/questions_queue.go | 146 |
| AddQuestionWithReasoning | handoff/questions_queue.go | 151 |
| GetNextPending | handoff/questions_queue.go | 174 |
| MarkAnswered | handoff/questions_queue.go | 194 |
| MarkAsked | handoff/questions_queue.go | 207 |
| HasPending | handoff/questions_queue.go | 221 |
| AddAlfaQuestion | handoff/questions_queue.go | 291 |
| AddEchoQuestion | handoff/questions_queue.go | 296 |
| GetNextAlfaQuestion | handoff/questions_queue.go | 301 |
| GetNextEchoQuestion | handoff/questions_queue.go | 306 |
| AskQuestion | handoff/questions_queue.go | 327 |
| NewState | handoff/state.go | 64 |
| Load | handoff/state.go | 77 |
| Save | handoff/state.go | 98 |
| Start | handoff/state.go | 109 |

#### CLI surface

Librería interna — gestiona cola de preguntas pendientes entre frameworks (Echo/Alfa). No tiene entry point propio; usada por api_rest y framework_session.

---

### Binario: llm

#### Métodos (10)

| Método | Archivo | Línea |
|--------|---------|-------|
| New | llm/client.go | 65 |
| HasCapability | llm/client.go | 87 |
| Provider | llm/client.go | 106 |
| Model | llm/client.go | 107 |
| Capabilities | llm/client.go | 108 |
| Complete | llm/client.go | 144 |
| Stream | llm/client.go | 218 |
| imageToDataURL | llm/client.go | 305 |
| handleMiniStreamData | llm/client.go | 486 |

#### Sinks

| Tipo | Código | Archivo:Línea |
|------|--------|---------------|
| http | Complete() → http.Do | llm/client.go:192 |
| http | Stream() → http.Do | llm/client.go:242 |

---

## Módulo: channel

### Binario: channel

#### Métodos (3)

| Método | Archivo | Línea |
|--------|---------|-------|
| envOr | channel/main.go | 69 |
| loadDotEnv | channel/main.go | 76 |

#### CLI surface

Punto de entrada del servidor HTTP channel. Puerto configurable via PORT (default :8080). Expone Handle en `/` y `/health`. Autenticación vía CHANNEL_API_KEYS.

---

### Binario: internal (handler JSON-RPC)

#### Métodos (43) — selección

| Método | Archivo | Línea |
|--------|---------|-------|
| ExecuteCommandWithEnv | internal/exec.go | 22 |
| NewHandler | internal/handler.go | 29 |
| Handle | internal/handler.go | 50 |
| writeError | internal/handler.go | 123 |
| executeCommand | internal/handler.go | 133 |
| readFile | internal/handler.go | 192 |
| writeFile | internal/handler.go | 209 |
| listDir | internal/handler.go | 229 |
| grep | internal/handler.go | 250 |
| find | internal/handler.go | 293 |
| editFile | internal/handler.go | 340 |
| httpGet | internal/handler.go | 377 |
| resolveWithinBase | internal/handler.go | 411 |
| walkReadableFiles | internal/handler.go | 464 |
| shouldSkipDir | internal/handler.go | 506 |
| ValidateJSONRPC | internal/jsonrpc.go | 14 |
| IsMethodAllowed | internal/jsonrpc.go | 45 |
| LogRequest | internal/logging.go | 9 |
| LogSecurityReject | internal/logging.go | 25 |
| ObfuscateAPIKey | internal/logging.go | 40 |
| NewSuccessResponse | internal/response.go | 17 |
| NewErrorResponse | internal/response.go | 30 |

#### JSON-RPC methods expuestos

| Método JSON-RPC | Implementación | Archivo:Línea |
|-----------------|----------------|---------------|
| execute_command | executeCommand → ExecuteCommandWithEnv | internal/handler.go:133 |
| read_file | readFile → os.ReadFile | internal/handler.go:192 |
| write_file | writeFile → os.WriteFile | internal/handler.go:209 |
| list_dir | listDir → os.ReadDir | internal/handler.go:229 |
| grep | grep → exec(grep/rg) | internal/handler.go:250 |
| find | find → walkReadableFiles | internal/handler.go:293 |
| edit_file | editFile → os.WriteFile | internal/handler.go:340 |
| http_get | httpGet → http.Do | internal/handler.go:377 |

#### Sinks

| Tipo | Código | Archivo:Línea |
|------|--------|---------------|
| exec | exec.CommandContext(ctx, cmd, args...) | internal/exec.go:26 |
| file | os.WriteFile — writeFile | internal/handler.go:223 |
| file | os.WriteFile — editFile | internal/handler.go:371 |
| http | http.Do — httpGet | internal/handler.go:393 |

---

### Binario: orchestrator

#### Métodos (12)

| Método | Archivo | Línea |
|--------|---------|-------|
| usage | orchestrator/main.go | 69 |
| discover | orchestrator/main.go | 92 |
| cmdList | orchestrator/main.go | 116 |
| cmdChains | orchestrator/main.go | 141 |
| cmdRun | orchestrator/main.go | 162 |
| cmdChain | orchestrator/main.go | 208 |
| absPathsFromPorts | orchestrator/main.go | 222 |
| resolveInputs | orchestrator/main.go | 233 |
| commandNames | orchestrator/main.go | 259 |
| parseKV | orchestrator/main.go | 267 |
| indent | orchestrator/main.go | 277 |

#### CLI surface

`orchestrator list` — listar frameworks disponibles. `orchestrator chains` — listar chains. `orchestrator run <fw> <cmd>` — ejecutar comando de framework. `orchestrator chain <chain>` — ejecutar cadena completa.

---

### Binario: vault

#### Métodos (19)

| Método | Archivo | Línea |
|--------|---------|-------|
| cmdHas | vault/main.go | 77 |
| cmdGet | vault/main.go | 91 |
| cmdSet | vault/main.go | 109 |
| cmdDelete | vault/main.go | 164 |
| cmdGenKey | vault/main.go | 177 |
| requireConvKey | vault/main.go | 186 |
| DefaultBaseDir | vault/vault.go | 51 |
| discoverRepoVaultDir | vault/vault.go | 64 |
| Path | vault/vault.go | 89 |
| sanitize | vault/vault.go | 99 |
| masterKey | vault/vault.go | 121 |
| Set | vault/vault.go | 145 |
| Get | vault/vault.go | 176 |
| Has | vault/vault.go | 210 |
| List | vault/vault.go | 219 |
| Delete | vault/vault.go | 249 |
| GenerateKey | vault/vault.go | 259 |

#### CLI surface

`vault has --conv <id> --key <k>` | `vault get --conv <id> --key <k>` | `vault set --conv <id> --key <k> --stdin` | `vault delete --conv <id> --key <k>` | `vault gen-key`. Cifrado AES-GCM. Invocado por api_rest vía exec.Command.

#### Sinks

| Tipo | Código | Archivo:Línea |
|------|--------|---------------|
| env | os.Getenv("REMORA_VAULT_DIR") | vault/vault.go:52 |
| env | os.Getenv("REMORA_VAULT_KEY") | vault/vault.go:122 |
| file | os.WriteFile — Set | vault/vault.go:168 |
| file | os.Remove — Delete | vault/vault.go:251 |
| http | http.Get — cmdGet | vault/main.go:97 |

---

### Binario: alfa-runner

#### Métodos (4)

| Método | Archivo | Línea |
|--------|---------|-------|
| main | alfa-runner/main.go | 29 |
| step | alfa-runner/main.go | 80 |
| mustOk | alfa-runner/main.go | 84 |

#### CLI surface

Runner de pasos para framework Alfa. Invocado por orchestrator o directamente. Ejecuta pasos secuenciales del pipeline Alfa.

---

### Binario: echo-runner

#### Métodos (8)

| Método | Archivo | Línea |
|--------|---------|-------|
| runEcho | echo-runner/main.go | 101 |
| runEchoSimple | echo-runner/main.go | 112 |
| extractID | echo-runner/main.go | 122 |
| firstLine | echo-runner/main.go | 137 |
| parseTree | echo-runner/main.go | 144 |
| firstByType | echo-runner/main.go | 152 |
| mustExec | echo-runner/main.go | 161 |

#### CLI surface

Runner de sesiones Echo. Invocado por flujo CLI cuando necesita verificar readiness de framework-echo. Extrae árbol Echo (echo.tree.v1).

---

### Binario: adapter

#### Métodos (11)

| Método | Archivo | Línea |
|--------|---------|-------|
| New | adapter/adapter.go | 36 |
| ExecuteCommand | adapter/adapter.go | 48 |
| ReadFile | adapter/adapter.go | 60 |
| WriteFile | adapter/adapter.go | 65 |
| ListDir | adapter/adapter.go | 73 |
| Grep | adapter/adapter.go | 77 |
| Find | adapter/adapter.go | 85 |
| EditFile | adapter/adapter.go | 93 |
| HTTPGet | adapter/adapter.go | 103 |
| call | adapter/adapter.go | 108 |

#### CLI surface

Wrapper Go del protocolo JSON-RPC de channel. Usado programáticamente por frameworks o tests para llamar al servidor channel.

---

### Binario: credentials

#### Métodos (5)

| Método | Archivo | Línea |
|--------|---------|-------|
| LoadSMTP | credentials/smtp.go | 35 |
| SaveSMTP | credentials/smtp.go | 86 |
| DeleteSMTP | credentials/smtp.go | 97 |
| ApplyDefaults | credentials/smtp.go | 104 |

#### CLI surface

Librería de credenciales SMTP. Carga desde vault vía HTTP GET. Usada por api_rest para operaciones SMTP.

---

### Binario: profile

#### Métodos (7)

| Método | Archivo | Línea |
|--------|---------|-------|
| NewLoader | profile/profile.go | 46 |
| Load | profile/profile.go | 55 |
| LoadNamed | profile/profile.go | 64 |
| OverlayFor | profile/profile.go | 125 |
| SystemPromptWithOverlay | profile/profile.go | 131 |
| Active | profile/profile.go | 146 |

#### CLI surface

Librería de perfiles de configuración. Lee REMORA_PROFILE del entorno. Usada por channel/internal/handler.go para personalizar system prompt por perfil de conversación.

---

## Interfaces detectadas

### remora-cli → api_rest (HTTP)

- **Desde:** remora/main.go:73 (get) y remora/main.go:99 (post)
- **Base URL:** REMORA_API_URL (default http://localhost:8084)
- **Endpoints usados:** /api/v1/frameworks, /api/v1/frameworks/{name}/manifest, /api/v1/flows/validate, /api/v1/flows/simulate, /api/v1/businesses/{id}/flows, /api/v1/flows/suggest, /api/v1/businesses/{id}/artifacts
- **Receptores inferidos:** listFrameworks, getFramework, validateFlow, simulateFlow, handleCreateFlow, suggestFlowCapabilities, handleBusinessArtifacts

### devcli → api_rest (HTTP)

- **Desde:** devcli/client.go:73 (get) y devcli/client.go:99 (post)
- **Base URL:** REMORA_API_URL (default http://localhost:8084)
- **Endpoints usados:** /health, /api/v1/frameworks, /api/v1/flows, /api/v1/flows/run, /api/v1/flows/simulate
- **Receptores inferidos:** health, listFrameworks, handleListFlows, runFlow, simulateFlow

### remora → flujo (exec.Command)

- **Desde:** remora/flow_workbench.go:235 (var flowWorkbenchExecCommand)
- **Código:** `var flowWorkbenchExecCommand = exec.Command` (wrappable para tests)
- **Receptor inferido:** cmd/flujo/main.go — cmdFlow() [flujo/flow_workbench.go:338]
- **Protocolo:** args passthrough, stdin/stdout/stderr heredados

### api_rest → channel (exec.Command)

- **Desde:** api_rest/flow_channel.go:56 (ensureChannel)
- **Código:** `exec.Command(channelBin, ...)` — inicia channel como proceso hijo
- **Receptor inferido:** channel/cmd/channel/main.go:48 — Handle en /
- **Protocolo:** proceso hijo HTTP, luego pingChannel() para health check

### api_rest → channel (compilación)

- **Desde:** api_rest/flow_channel.go:134 (buildChannelBinary)
- **Código:** `exec.Command("go", "build", "-buildvcs=false", "-o", outBin, "./cmd/channel")`
- **Propósito:** construir el binario channel on-demand si no existe

### api_rest → vault (exec.Command)

- **Desde:** api_rest/main.go:1111 (bootstrapSMTPFromEnvIfNeeded)
- **Código:** `exec.Command(resolveVaultBin(), "set", "--conv", ..., "--key", "credentials.smtp", "--stdin")`
- **Receptor inferido:** vault/main.go:109 — cmdSet()

- **Desde:** api_rest/main.go:1334 (vaultHasFromAPI)
- **Código:** `exec.Command(resolveVaultBin(), "has", "--conv", ..., "--key", key)`
- **Receptor inferido:** vault/main.go:77 — cmdHas()

### api_rest → mensajero (exec.Command)

- **Desde:** api_rest/main.go:1254 (handleSendEmail)
- **Código:** `exec.Command(binPath, args...)`  donde binPath = REMORA_MENSAJERO_BIN
- **Receptor inferido:** framework-mensajero binary (fuera de estos 3 módulos)

### api_rest → contactos (exec.Command)

- **Desde:** api_rest/contactos.go:51,83
- **Código:** `exec.Command(bin, ...)` — bin = ruta al binario contactos
- **Receptor inferido:** binario externo de contactos (fuera de estos 3 módulos)

### flujo → api_rest (HTTP/SSE)

- **Desde:** flujo/flow_workbench.go:401 (stream)
- **Endpoint:** /api/v1/flows/run/stream
- **Receptor inferido:** runFlowStream — SSE
- **Protocolo:** Server-Sent Events

### flujo → framework-echo (exec.Command)

- **Desde:** flujo/main.go:319 (echoReadyForAlfa)
- **Código:** `exec.Command("/bin/zsh", "-lc", "cd ... && ./frameworkecho readiness")`
- **Receptor:** framework-echo binary (fuera de módulo, en path hardcodeado)

### nativeagent → bash (exec.Command)

- **Desde:** nativeagent/agent.go:1021 (toolBash)
- **Código:** `exec.Command("/bin/zsh", "-lc", command)` — bash arbitrario
- **Riesgo:** entrada del usuario puede llegar sin sanitizar si validateBashPolicy no la rechaza

### channel (internal) → OS (exec.CommandContext)

- **Desde:** internal/exec.go:26 (ExecuteCommandWithEnv)
- **Código:** `exec.CommandContext(ctx, cmd, args...)`
- **Riesgo:** cmd viene del JSON-RPC request; ValidateJSONRPC + IsCommandAllowed son las guardas

---

## Catálogo de Frameworks

Los frameworks son plugins ejecutados vía exec.Command o API REST. Su contrato viene de `framework.manifest.json`.
_(Tabla generada automáticamente desde `.pi/framework-catalog.json` — 21 frameworks)_

| Framework | Descripción | Comandos | Inputs | Outputs | Modo | Modelo |
|-----------|-------------|----------|--------|---------|------|--------|
| framework-alfa | Compilador semántico Echo→Bravo. Traduce árbol Echo a flujo ideal. | compile, inspect, export-bravo, next-question, ingest-answer | echo_tree(echo.tree.v1) | alfa_spec(alfa.spec.v1), ideal_flow(alfa.ideal_flow.v1) | sync | groq/llama-4-scout |
| framework-arquitecto | Comprende e indexa codebases Go. Mantiene modelo mental del repo. | init, index-repo, query-structure, trace-flow, status, readiness, next-question, ingest-answer | — | repo_model(arquitecto.model.v1) | sync | groq/llama-4-scout |
| framework-auditor | Auditor proactivo de ERP. Lee datasets JSON, corre checks y emite findings. | reset, scan, list, detail, next-question, ingest-answer | dataset(external.api.dump.v1) | findings(auditor.findings.v1), data_gaps(data.gaps.v1) | sync_chain | groq/llama-4-scout |
| framework-bravo | Verificador de flujos ideales vs trazas reales. | — | — | — | async_trigger | groq/llama-4-scout |
| framework-charlie | Gestión ciclo de vida de repos Git: doctor, plan, backup, preflight. | doctor, plan, preflight, status, changelog, backup | — | — | async_trigger | groq/llama-4-scout |
| framework-critico | Framework adversarial para evaluar propuestas de cambio en codebases. | init, evaluate, challenge, status, readiness, next-question, ingest-answer | repo_model(arquitecto.model.v1) | evaluation(critico.eval.v1) | sync | groq/llama-4-scout |
| framework-deployer | Deploya remora-go a Cloud Run dev. Nunca toca producción. | plan, apply | — | — | async_trigger | groq/llama-4-scout |
| framework-echo | Guía reuniones de descubrimiento de procesos. | init, add-axiom, add-theory, add-task, add-pain, add-opportunity, validate, show-tree | — | tree(echo.tree.v1) | sync | groq/llama-4-scout |
| framework-excel | Conector de archivos Excel. Lee, escribe y manipula .xlsx. | — | — | — | async_trigger | groq/llama-4-scout |
| framework-foco | Foco operativo del día. Convierte prioridades en tareas. | next-question, session-start, ingest-answer, query, priorities, next-task, complete-cycle | — | — | sync_chain | groq/llama-4-scout |
| framework-gmail | Gestor de emails. Prepara borradores y gestiona operaciones de correo. | send-email, get-unread-emails, search-emails, list-labels, list-drafts, create-draft | — | — | async_trigger | groq/llama-4-scout |
| framework-hosting | Conecta panel hosting cPanel UAPI. Opera email/DNS/archivos. | next-question, ingest-answer, connect, list-emails, provision-smtp, import-smtp, has-smtp, verify-smtp | — | — | async_trigger | groq/llama-4-scout |
| framework-indexa | Ingesta data de API externa, genera embeddings, persiste en vector store. | init, index, status, api-plan | source_json(external.api.dump.v1) | vector_store(indexa.store.v1) | async_trigger | groq/llama-4-scout |
| framework-mecanico | Agente reparador. Toma findings de Auditor, propone remediaciones. | propose, propose-all-auto, list-proposals, apply, apply-all, reset, next-question, ingest-answer | findings(auditor.findings.v1), dataset(external.api.dump.v1) | proposals(mecanico.proposals.v1), applied_log(mecanico.applied.v1) | sync_chain | groq/llama-4-scout |
| framework-mensajero | Envía mensajes salientes (email/sms/whatsapp). Agnóstico al negocio. | next-question, ingest-answer, can-send, send | — | — | async_trigger | groq/llama-4-scout |
| framework-paladin | Tracing semántico para Go. Audita repos, explica traces. | audit, explain, status, readiness | — | audit_report(paladin.audit.v1) | async_trigger | groq/llama-4-scout |
| framework-pingpong | Tutor 80/20 para aprender haciendo. Guía ejercicios de código. | next, review, check, accept, init, verify, peek, clean | — | — | sync_chain | groq/llama-4-scout |
| framework-quine | Generador y revisor de frameworks. Crea nuevos frameworks desde spec. | create, review, list, spec, fix | — | — | async_trigger | groq/llama-4-scout |
| framework-radar | Radar analítico data-aware. Scoring, prioridad y esquema de análisis. | prioritize, configure-analysis, deep-dive, analyze-followup | — | — | async_trigger | — |
| framework-sabio | Experto en datos indexados. Responde preguntas usando SQLite. | next-question, ingest-answer, query, explain-capabilities, inspect-source, validate-business-config, reset, contact-lookup | data_sqlite_db(data.sqlite_db.v1) | — | sync_chain | groq/llama-4-scout |
| framework-tareas | Task ledger canónico por perfil. Mantiene tabla tasks + task_events. | next-question, ingest-answer, list, next, create, complete, event, seed-from-foco | — | — | async_trigger | groq/llama-4-scout |

---

## Superficies de Usuario

### SUPERFICIE: cli-remora
- Tipo: CLI
- Entry: remora-cli/cmd/remora/main.go (flow_workbench.go:237)
- Comandos disponibles: flow, flow create, flow draft, flow inspect, flow simulate, flow debug [frameworks|manifest|commands|capabilities|trace|validate|simulate|dependencies]
- Output: texto plano + ANSI colors, JSON opcional con --json flag

### SUPERFICIE: cli-devcli
- Tipo: CLI
- Entry: remora-cli/cmd/devcli/main.go:15
- Comandos disponibles: framework ls, framework show, flow list, flow show, flow run, flow simulate, flow run-get, rules, health
- Output: texto plano + JSON

### SUPERFICIE: api-rest
- Tipo: HTTP REST
- Entry: remora-flujo/cmd/api_rest/main.go
- Se sirve en: http://localhost:8084 (REMORA_API_URL)
- Pantallas/rutas: 70+ endpoints REST (ver tabla de rutas)
- Interacciones: auth, businesses, frameworks, flows, conversations, tasks, data browser

### SUPERFICIE: frontend-chat
- Tipo: Web
- Archivos: remora-flujo/frontends/frontend-chat/index.html (serve.go :8085), remora-flujo/cmd/api_rest/static/index.html (:8084/)
- Se sirve en: localhost:8084 (canvas) y localhost:8085 (chat)
- Endpoints consumidos: /api/v1/flows/run/stream (SSE), /api/v1/conversations, /api/v1/frameworks
- Interacciones principales: canvas visual de flows, chat interface, data browser

### SUPERFICIE: channel-rpc
- Tipo: HTTP JSON-RPC
- Entry: channel/cmd/channel/main.go (Handle en /)
- Se sirve en: http://localhost:8080 (PORT env var)
- Métodos JSON-RPC: execute_command, read_file, write_file, list_dir, grep, find, edit_file, http_get

---

## GAPs detectados

| Símbolo | Componente | Query | Status |
|---------|------------|-------|--------|
| handleFlowRun (local en remora) | remora-cli | grep "^func handleFlowRun" remora/ | DELEGADO → flujo vía exec.Command |
| main en flujo/main.go | remora-flujo | flujo binary entry point | NO_EN_GRAFO — el grafo no expone main() de flujo, solo cmdFlow |
| framework-mensajero binary | remora-flujo | exec.Command(REMORA_MENSAJERO_BIN) en main.go:1254 | EXTERNO — no en los 3 módulos analizados |
| contactos binary | remora-flujo | exec.Command(bin) en contactos.go:51,83 | EXTERNO — binario no encontrado en repo |
| framework-echo binary en path hardcodeado | remora-flujo | /Users/alcless_a1234_cursor/remora-go/framework-echo | EXTERNO — path absoluto fuera de repo |
| llmtest main() | remora-flujo | llmtest/main.go — CPG solo muestra 1 método (marcador) | PARCIAL — entry point no resolvible por grafo |
| framework-radar model | framework-catalog | model.provider en framework-radar | AUSENTE — campo model vacío en catalog.json |

---

## Variables de Entorno

| Variable | Componente | Archivo | Descripción |
|----------|-----------|---------|-------------|
| REMORA_API_URL | remora-cli, remora-flujo | main.go:61 | URL del backend (default: http://localhost:8084) |
| REMORA_API_TOKEN | remora-cli, remora-flujo | main.go:67 | Token Bearer de autenticación |
| REMORA_ROOT | remora-flujo | api_rest/main.go:365 | Directorio raíz del proyecto |
| REMORA_LLM_PROVIDER | remora-flujo | api_rest/flow_runtime_approval.go | Proveedor LLM (minimax/groq/openrouter) |
| REMORA_BOOTSTRAP_EMAIL | remora-flujo | api_rest/auth.go:243 | Email del usuario bootstrap |
| REMORA_BOOTSTRAP_PASSWORD | remora-flujo | api_rest/auth.go:244 | Contraseña del usuario bootstrap |
| REMORA_SECRET_KEY | remora-flujo | api_rest/api_connections.go | Clave secreta para API connections |
| REMORA_AUTH_SECURE_COOKIE | remora-flujo | api_rest/auth.go:716 | Habilitar secure cookie |
| REMORA_DEV_STATIC | remora-flujo | api_rest/main.go:271 | Servir estáticos desde disco |
| REMORA_DEV_MODE | remora-flujo | api_rest/main.go:1129 | Modo desarrollo |
| REMORA_MENSAJERO_BIN | remora-flujo | api_rest/main.go:1313 | Path al binario mensajero |
| REMORA_VAULT_BIN | remora-flujo | api_rest/main.go:1325 | Path al binario vault |
| CHANNEL_BIN | remora-flujo | api_rest/flow_channel.go:106 | Path al binario channel |
| CHANNEL_API_KEYS | channel | channel/main.go:26 | API keys permitidas (CSV) |
| CHANNEL_EXEC_TIMEOUT | channel | internal/handler.go:36 | Timeout de exec |
| PORT | channel | channel/main.go:20 | Puerto del servidor channel (default :8080) |
| REMORA_PROFILE | channel | profile/profile.go:56 | Perfil de configuración activo |
| REMORA_VAULT_DIR | channel | vault/vault.go:52 | Directorio del vault |
| REMORA_VAULT_KEY | channel | vault/vault.go:122 | Clave maestra del vault |
| HOSTING_VAULT_KEY | channel | vault/vault.go:124 | Clave vault de hosting (fallback) |
| SABIO_DB | remora-flujo | api_rest/data_browser.go:229 | Path a DB SQLite de datos |
| TASKS_DB_PATH | remora-flujo | api_rest/tareas.go:537 | Path DB legacy de tareas |
| K_SERVICE | remora-flujo | api_rest | Cloud Run service name |

## Mecanismos Internos

### MECANISMO: run-flow-manifest — Orquesta la ejecución completa de un flujo declarativo multi-nodo con ciclos

- Función: runFlowManifest() [cmd/api_rest/flow_runner.go:12]
- Fan-out: 98 callees
- Módulo: remora-flujo
- Disparado por: runFlow(), runFlowStream()

#### Pipeline interno:
1. Carga businessArtifacts y compilación del manifest (loadCompiledRecord o compileAndPersistFlowManifest)
2. Genera runID, valida el manifest con artefactos disponibles (validateFlowManifestWithArtifacts)
3. Calcula orden de ejecución topológica (flowExecutionOrder)
4. Inicializa artefactos de sistema y provided; inyecta flow.intent.v1 si hay intent declarado
5. Inicializa segmento semántico (ensureSemanticSegmentInitialized) e inyecta nodo owner si aplica
6. Itera nodos en orden: para cada nodo resuelve contrato (resolveFlowNodeContract), verifica artefactos faltantes
7. Ante artefactos faltantes just-in-time: dispara ensureDataPipeline o resolveMissingFlowArtifacts
8. Ejecuta preflight audit si se requiere (ensureFlowPreflightAudit) antes de nodos con side-effect
9. Verifica si el nodo requiere aprobación humana (nodeRequiresRuntimeApproval)
10. Ejecuta el nodo via executeFlowNode y registra artefactos producidos (recordNodeArtifacts)
11. Gestiona segmentos semánticos (delegación, retorno, refresh)
12. Detecta cycle-terminal y registra ciclo completado; si maxCycles permite, resetea y vuelve a cycleStart
13. Al finalizar: normaliza timeline, persiste el run, emite readiness

#### Invariantes (axiomas):
- El flujo SIEMPRE se compila antes de ejecutar; si no existe compiled previo, se genera uno nuevo
- Validación inválida aborta inmediatamente (status=invalid, sin ejecución de nodos)
- Los artefactos de sistema se inyectan ANTES de iterar nodos; ningún nodo puede alterar este conjunto base
- Un nodo NUNCA se ejecuta si tiene artefactos requeridos faltantes sin resolución posible
- La aprobación runtime es un gate que SIEMPRE detiene la ejecución antes de side-effects no autorizados
- Ciclos se resetan con resetCycleArtifacts; artefactos de bootstrap/infra persisten entre ciclos
- El pipeline es breakable: cualquier fallo, needs_input o needs_approval DETIENE la iteración

#### Callees clave:
- executeFlowNode() — ejecuta un nodo individual contra channel
- resolveFlowGapsIteratively() — resuelve gaps de datos post-auditor iterativamente
- ensureFlowPreflightAudit() — valida calidad de datos antes de side-effects
- resolveMissingFlowArtifacts() — intenta resolver artefactos faltantes vía múltiples estrategias
- compileAndPersistFlowManifest() — compila y persiste la versión final del manifest
- flowExecutionOrder() — determina el orden topológico de nodos

---

### MECANISMO: run-loop — Orquesta el ciclo de pregunta-respuesta multi-framework conversacional

- Función: runLoop() [cmd/api_rest/orchestrator.go:37]
- Fan-out: 60 callees
- Módulo: remora-flujo
- Disparado por: postMessage(), postSingleMessage()

#### Pipeline interno:
1. Crea Paladin trace para observabilidad del ciclo completo
2. Carga la cola de preguntas (loadQueue) y los drivers activos (driversFor)
3. Detecta sesión analítica activa: si existe, clasifica intención del segmento (exit/operational/continue)
4. Si intent=continue: ejecuta followup del owner (executeSessionFollowup) y retorna directo
5. Si intent=operational: cierra tramo analítico y genera handoff artifact para Foco
6. Clasifica intent del usuario contra intent_examples de manifests (classifyIntent) — reordena drivers
7. Evalúa reglas de composición declarativas (flow.rules.json) — puede reordenar/delegar
8. Si hay pregunta pendiente: entrega respuesta al driver dueño (IngestAnswer) con posible preprocesamiento vision
9. Pollea drivers en orden por la próxima pregunta (PollQuestion/PollQuestionFull)
10. Retorna la siguiente pregunta encolada al cliente

#### Invariantes (axiomas):
- Un solo framework queda elegido como próximo speaker por ciclo (nunca responden dos simultáneamente)
- Las sesiones activas tienen prioridad ABSOLUTA sobre el pipeline normal de drivers
- La clasificación de intent (capability-based) PRECEDE a las reglas declarativas
- La whitelist de allowed_delegates del session owner NUNCA se puede saltar
- Vision preprocessing se aplica SOLO si la regla lo pide Y hay imágenes en resources
- El claim de sesión para una conversación es atómico (una vez asignada, no se reasigna)

#### Callees clave:
- classifyIntent() — matchea intención del usuario contra intent_examples de manifests activos
- executeSessionFollowup() — ejecuta el followup command del owner del segmento analítico
- driversFor() — instancia los drivers activos para la conversación
- loadQueue()/saveQueue() — persistencia de la cola de preguntas

---

### MECANISMO: resolve-flow-gaps-iteratively — Resuelve brechas de datos iterativamente post-auditoría

- Función: resolveFlowGapsIteratively() [cmd/api_rest/flow_gap_resolution.go:13]
- Fan-out: 52 callees
- Módulo: remora-flujo
- Disparado por: ensureFlowPreflightAudit(), runFlowManifest()

#### Pipeline interno:
1. Carga tablas de scope del negocio y el semantic pack (loadBusinessScopeTables, businessSemanticPackPath)
2. Calcula requisitos terminales y campos requeridos (flowTerminalRequirementsForArtifacts, flowRequiredDataFields)
3. En cada pass (max 2): parsea gaps del artefacto data.gaps.v1, filtra por scope, propósito del flow, y datos existentes de la entidad
4. Parsea bulk-quality findings separadamente; genera artefacto de observabilidad en pass 0
5. Separa gaps user-completable de bulk-migration (los bulk NUNCA van a Mecánico)
6. Para gaps de contacto: intenta lookup Sabio; si falla, invoca Mecánico conversacional; si falla, pide input directo
7. Para gaps hybrid data-quality: busca provider con resolutionMode=hybrid, ejecuta resolución vía executeFlowNode
8. Si Mecánico genera proposals: solicita aprobación humana (requestMecanicoProposalApproval)
9. Si hubo resolución en pass 0: re-ejecuta Auditor para validación post-resolución
10. Si no hubo progreso en un pass: rompe el loop (no-progress breakout)

#### Invariantes (axiomas):
- Máximo 2 passes de resolución (maxResolutionPasses=2); NUNCA se hace loop infinito
- Gaps de tipo bulk_migration NUNCA se envían a resolución conversacional; son solo observabilidad
- Si no hay requisitos terminales NI campos requeridos, gaps se anulan (gaps = nil)
- El re-audit SOLO se ejecuta en pass 0 tras resolución exitosa
- El breakout por no-progress es obligatorio: si ningún gap se resolvió en un pass, se termina

#### Callees clave:
- invokeMecanicoResolveGaps() — invoca resolución conversacional de gaps via Mecánico
- executeFlowNode() — ejecuta el nodo de remediación hybrid
- filterGapsByScope() — filtra gaps por las tablas de scope del negocio
- filterGapsByExistingEntityData() — elimina gaps donde la entidad ya tiene dato
- lookupSabioContactDestination() — busca contacto en Sabio antes de pedir al usuario

---

### MECANISMO: install-flow-analysis — Instala la configuración de análisis de un flujo (Radar)

- Función: installFlowAnalysis() [cmd/api_rest/flow_store.go:537]
- Fan-out: 41 callees
- Módulo: remora-flujo
- Disparado por: handleInstallFlow()

#### Pipeline interno:
1. Valida que el flow tenga manifest; si compiled_id proporcionado, lo carga y verifica pertenencia al business
2. Si no hay compiled_id: compila el manifest (compileAndPersistFlowManifest)
3. Si Radar ya está instalado (radarAnalysisInstalled) y no es reconfigure: retorna early con status installed
4. Busca el nodo instalable en el flow compilado (findInstallableFlowNode)
5. Resuelve contrato del nodo, prepara params (business_id, semantic_pack, db)
6. Ejecuta el command del nodo instalable contra channel con timeout de 120s
7. Parsea el resultado: espera analysis.schema.v1 en stdout
8. Persiste artefactos analysis.schema.v1 y analysis.plan.v1
9. Registra el run, upsert installation snapshot, actualiza status del flow a "installed"

#### Invariantes (axiomas):
- Si el flow ya está instalado Y no es reconfigure, SIEMPRE retorna early sin re-ejecutar
- El compiled_id DEBE pertenecer al mismo business_id; cross-business está prohibido
- El nodo instalable DEBE devolver artifact_type=analysis.schema.v1; otro tipo es error
- El timeout es de 120s hard-coded; no configurable por manifest
- Si channel está unavailable: retorna error específico de channel-unavailable con URL

#### Callees clave:
- findInstallableFlowNode() — busca en el flow compilado el nodo con role=install
- resolveFlowNodeContract() — resuelve inputs/outputs/command del nodo
- compileAndPersistFlowManifest() — compila si no hay compiled_id previo
- radarAnalysisInstalled() — verifica si ya hay plan de análisis activo
- persistFlowArtifact() — persiste los artefactos generados en disco

---

### MECANISMO: chat-echo — Ejecuta el loop interactivo CLI de discovery conversacional Echo-Alfa

- Función: chatEcho() [cmd/flujo/main.go:408]
- Fan-out: 37 callees
- Módulo: remora-flujo
- Disparado por: cmdRun(), cmdChat()

#### Pipeline interno:
1. Carga estado del handoff (mustLoad), inicia fase Echo
2. Genera prompt inicial para Echo y ejecuta promptRole contra LLM
3. Imprime pregunta de Echo al usuario (printEchoQuestionIfMissing)
4. Entra en loop interactivo leyendo stdin con bufio.Scanner
5. En cada iteración verifica echoReadyToHandOff() — si Echo declaró readiness, hace handoff a Alfa
6. Si hay pregunta pendiente de otro role: rutea respuesta al role correcto (runRole)
7. Cuenta respuestas reales del usuario (countUserResponses); si >= 2, auto-handoff a Alfa
8. Si hay imagen: parsea input (parseImageInput), usa promptRoleWithImages
9. Imprime respuesta de Echo y vuelve al loop

#### Invariantes (axiomas):
- Echo SIEMPRE habla primero; Alfa se activa SOLO si Echo declara readiness O hay 2+ respuestas de usuario
- El handoff de Echo a Alfa es irreversible dentro de una sesión chatEcho
- /salir, /exit, /quit SIEMPRE terminan el loop sin handoff
- El estado se persiste a disco (mustSave) después de cada transición
- Las imágenes se procesan SOLO si el input del usuario contiene markup de imagen

#### Callees clave:
- mustLoad()/mustSave() — carga/persiste el estado del handoff
- promptRole()/promptRoleWithImages() — invoca el LLM con el prompt del role
- runRole() — ejecuta un role arbitrario (Alfa, Bravo) tras handoff
- echoReadyToHandOff() — verifica si Echo marcó EventEchoReadyForAlfa
- countUserResponses() — cuenta turnos reales para trigger de auto-handoff

---

### MECANISMO: execute-session-followup-detailed — Ejecuta un turno analítico con delegaciones y síntesis

- Función: executeSessionFollowupDetailed() [cmd/api_rest/orchestrator.go:508]
- Fan-out: 36 callees
- Módulo: remora-flujo
- Disparado por: executeSessionFollowup(), simulateDeepAnalysisConversation()

#### Pipeline interno:
1. Resuelve manifest y command del session owner (session.Framework + session.FollowupCmd)
2. Incrementa turn count en disco (incrementSessionOnDisk)
3. Prepara params: input, business_id, turn_count, semantic_pack, history, context_b64, artefactos previos
4. Genera draft LLM para el followup si el command lo declara (generateOwnerFollowupWithLLM, fase "plan")
5. Resuelve args portables y ejecuta el followup command contra channel
6. Parsea respuesta: extrae texto y delegation_requests (extractFollowupTextAndDelegations)
7. Si hay delegations: ejecuta cada una (executeDelegations) respetando allowed_delegates
8. Con resultados de delegación: regenera draft LLM (fase "synthesis") y re-ejecuta followup command
9. Si la síntesis falla: genera artefacto de runtime failure (synthesizeFollowupRuntimeFailureArtifact)
10. Persiste artefacto analysis.followup.v1 y construye la respuesta como QueuedQuestion con chips

#### Invariantes (axiomas):
- El turn count SIEMPRE se incrementa antes de ejecutar; nunca se repite un turno
- Las delegaciones SOLO se ejecutan si están en allowed_delegates del session owner
- La síntesis (second pass) SIEMPRE borra el draft del plan; nunca reutiliza el draft pre-delegación
- Si la síntesis falla o no llega a phase=synthesis: se genera un artefacto de failure explícito
- El session followup SIEMPRE produce un QueuedQuestion con speaker = session.Framework

#### Callees clave:
- executeDelegations() — ejecuta cada delegation request contra el provider correspondiente
- generateOwnerFollowupWithLLM() — genera draft via LLM auxiliar (plan o synthesis)
- extractFollowupTextAndDelegations() — parsea stdout del command en texto + delegaciones
- persistFollowupArtifact() — persiste el analysis.followup.v1 con metadata de síntesis
- resolvePortableCommandArgs() — materializa params como archivos para commands que lo requieren

---

### MECANISMO: resolve-missing-flow-artifacts — Resuelve artefactos faltantes por tipo con estrategias escalonadas

- Función: resolveMissingFlowArtifacts() [cmd/api_rest/flow_gap_inputs.go:11]
- Fan-out: 34 callees
- Módulo: remora-flujo
- Disparado por: runFlowManifest()

#### Pipeline interno:
1. Itera cada artefacto faltante del nodo actual
2. Para contact.destination.v1: intenta extraer de artefactos existentes; luego Sabio lookup; luego Mecánico conversacional; fallback a input request
3. Para credentials.smtp: verifica vault check de artefactos; luego invoca provider credentials.smtp.check via command "has-smtp"; fallback a Hosting wizard (invokeProviderNextQuestion)
4. Para cualquier otro tipo: genera input request genérico (inputRequestsForMissingArtifacts)
5. Retorna lista de resueltos y lista de needs (inputs requeridos del usuario)

#### Invariantes (axiomas):
- Cada tipo de artefacto tiene su propia cadena de resolución; NUNCA se aplica una estrategia genérica a contact.destination
- El orden de intento es siempre: artefactos existentes > lookup externo > resolución conversacional > input directo
- Un artefacto resuelto se marca en available[] Y se persiste inmediatamente (persistFlowArtifact)
- Si Mecánico genera preguntas, esas se retornan como needs SIN detener el loop (el caller decide si breaks)
- El vault check de credentials.smtp tiene timeout de 10s hard-coded

#### Callees clave:
- lookupSabioContactDestination() — busca email en Sabio antes de pedir al usuario
- invokeMecanicoResolveGaps() — resolución conversacional como alternativa a input directo
- invokeProviderNextQuestion() — obtiene la próxima pregunta del wizard de Hosting
- persistFlowArtifact() — persiste artefacto resuelto en disco
- credentialAvailableFromArtifacts() — verifica si un credential ya está disponible

---

### MECANISMO: execute-delegations — Ejecuta delegaciones a otros frameworks con whitelist y envelope

- Función: executeDelegations() [cmd/api_rest/orchestrator.go:737]
- Fan-out: 33 callees
- Módulo: remora-flujo
- Disparado por: executeSessionFollowupDetailed()

#### Pipeline interno:
1. Construye whitelist de allowed_delegates en mapa lowercase para O(1) lookup
2. Inicializa envelope con requests[], results[], summary counters
3. Para cada delegation request: resuelve target ejecutable (resolveDelegationExecutionTarget)
4. Verifica que resolved_capability esté en allowed_delegates; si no, bloquea silenciosamente
5. Busca manifest del framework resuelto; fallback: busca por capability routing (providerOfProducedCapability)
6. Resuelve command del manifest para la capability (findManifestCapability -> cap.Command)
7. Prepara params: business_id, capability, entity_ref, semantic_pack, db, context_b64, analysis_intent
8. Ejecuta el command delegado contra channel (ExecuteCommand)
9. Parsea output: JSON estructurado o text/plain; marca verified/error/partial
10. Acumula resultados en envelope con contadores de éxito/fallo/parcial

#### Invariantes (axiomas):
- La whitelist de allowed_delegates es OBLIGATORIA; una capability no listada NUNCA se ejecuta
- El envelope SIEMPRE contiene requests[] y results[] del mismo largo (uno por cada request)
- Capabilities bloqueadas se reportan como failure silenciosa (no se propagan como error al caller)
- Si el manifest no tiene command para la capability: se intenta fallback a "query", "entity-360", "audit", "analyze"
- Los resultados parciales se contabilizan separadamente de éxitos y fallos

#### Callees clave:
- resolveDelegationExecutionTarget() — determina framework+capability ejecutable desde el request semántico
- findManifestCapability() — busca la capability en el manifest para obtener su command
- ExecuteCommand() — ejecuta contra channel con args resueltos
- delegationOutputVerified()/delegationOutputError()/delegationOutputPartial() — clasifica resultado

---

### MECANISMO: ensure-contact-destination-pipeline — Pipeline JIT para resolver destino de contacto

- Función: ensureContactDestinationPipeline() [cmd/api_rest/flow_data_pipeline.go:39]
- Fan-out: 32 callees
- Módulo: remora-flujo
- Disparado por: ensureDataPipeline()

#### Pipeline interno:
1. Intenta extraer contacto de artefactos existentes (contactDestinationFromArtifacts) — early return si existe
2. Verifica si hay una interaction answer que matchea contact.destination.v1 y campo email
3. Si el input del request es un email válido: lo acepta como contacto directo
4. Si ninguna fuente local resuelve: ejecuta Sabio lookup (lookupSabioContactDestination)
5. Si Sabio encuentra: registra, emite step, retorna true
6. Si Sabio no encuentra: invoca Mecánico conversacional (invokeMecanicoResolveGaps con gap missing_contact_destination)
7. Construye NeedsInput con pregunta contextual ("Para enviar el cobro a X necesito su email")
8. Marca result.Status = needs_input y registra readiness

#### Invariantes (axiomas):
- El pipeline se cortocircuita en cuanto CUALQUIER fuente resuelve el contacto (early return true)
- El orden de prioridad es: artefactos > interaction answer > input directo > Sabio > Mecánico > input request
- Si se resuelve vía Sabio: se emite step de persistencia contra Sabio (emitSabioPersistContactStep)
- La pregunta al usuario SIEMPRE incluye el entity display name para contexto
- Un email válido en req.Input se acepta DIRECTAMENTE sin verificación adicional (isLikelyEmail)

#### Callees clave:
- lookupSabioContactDestination() — consulta a Sabio por el email de la entidad actual
- invokeMecanicoResolveGaps() — resolución conversacional si Sabio no tiene el dato
- contactDestinationFromArtifacts() — extrae contacto de artefactos ya disponibles
- recordContactDestination() — registra el contacto resuelto en artefactos y available
- flowInteractionAnswerFromArtifacts() — busca respuestas previas del usuario en artefactos

---

### MECANISMO: invoke-mecanico-resolve-gaps — Invoca resolución conversacional de gaps via Mecánico

- Función: invokeMecanicoResolveGaps() [cmd/api_rest/flow_gap_resolution.go:289]
- Fan-out: 31 callees
- Módulo: remora-flujo
- Disparado por: ensureContactDestinationPipeline(), resolveMissingFlowArtifacts(), resolveFlowGapsIteratively()

#### Pipeline interno:
1. Busca provider con capability "action.fix.resolve_gaps_conversational"; fallback a command "resolve-gaps"
2. Serializa gaps como JSON payload con type, description, field, endpoint
3. Enriquece params: findings_json (si <= 20 gaps), entity_ref_json, scope_tables_json
4. Materializa params portables en disco (materializePortableArtifactParams)
5. Ejecuta command "resolve-gaps" contra channel con timeout 60s
6. Parsea stdout JSON: extrae array "questions" como preguntas conversacionales
7. Registra nodo dinámico y artefacto producido (recordDynamicFlowNode, persistFlowArtifact)
8. Emite step de infraestructura con human summary indicando cantidad de preguntas generadas
9. Retorna (questions, true) si hay preguntas; (nil, false) si no

#### Invariantes (axiomas):
- El timeout de ejecución es 60s hard-coded; NUNCA se extiende
- Si el provider no existe NI por capability NI por command "resolve-gaps": retorna (nil, false) silenciosamente
- Findings JSON se incluyen SOLO si hay <= 20 gaps (para no exceder límites de args)
- El artefacto producido se persiste SIEMPRE que el provider devuelva artifact_type en su respuesta
- Exit code != 0 retorna (nil, false) sin propagar error; el caller decide el fallback

#### Callees clave:
- findProviderForCapability() — busca manifest que declare la capability
- findProviderWithCommand() — fallback: busca manifest que tenga command "resolve-gaps"
- materializePortableArtifactParams() — escribe params JSON grandes como archivos en disco
- ExecuteCommand() — ejecuta contra channel con context y timeout
- recordDynamicFlowNode() — registra el nodo en el resultado del flow para observabilidad

---

### MECANISMO: run-message — Ejecuta un turno completo de agent loop para un framework individual

- Función: runMessage() [cmd/framework_session/main.go:199]
- Fan-out: 30 callees
- Módulo: remora-flujo
- Disparado por: (entry point CLI, invocado externamente)

#### Pipeline interno:
1. Parsea flags: --framework, --conv-id, --message (o --message-b64), --history, --context-b64
2. Resuelve root del proyecto y carga manifest del framework (loadManifest)
3. Lee initial prompt del framework (readInitialPrompt) y spec del modelo (specFor)
4. Crea cliente LLM (llm.New) y workspace para el framework+conv (ensureWorkspace)
5. Abre writer de eventos live (newLiveEventWriter) para streaming
6. Construye el mensaje user combinando history, session context, y mensaje actual
7. Emite eventos de framework.initial_prompt_active y llm.request_start
8. Ejecuta agent loop completo (agentloop.Run) con tool runner, max turns, max tokens
9. Parsea resultado: extrae texto de respuesta, emite eventos de completion
10. Ejecuta assisted setup completion si aplica (assistedSetupCompletionEvents)
11. Escribe respuesta final como JSON a stdout

#### Invariantes (axiomas):
- --framework es OBLIGATORIO; sin él la función retorna error inmediatamente
- El timeout del context es 120s hard-coded para todo el agent loop
- maxTurns viene del manifest (Agent.MaxTurns) con default 30 si no está declarado
- MaxTokens es 1200 hard-coded para respuestas concisas
- Si el LLM devuelve respuesta vacía: retorna error (no se acepta respuesta nula)
- El workspace se crea o reutiliza por (framework, conv_id); es idempotente

#### Callees clave:
- agentloop.Run() — ejecuta el loop completo de agent con tools
- newToolRunner() — instancia el runner de tools para el framework en su workspace
- newLiveEventWriter() — crea writer para eventos SSE/streaming
- llm.New() — instancia el cliente LLM según spec del manifest
- decodeSessionContext() — decodifica contexto de sesión para enriquecer el prompt

---

### MECANISMO: ensure-flow-preflight-audit — Valida calidad de datos antes de ejecutar nodos con side-effect

- Función: ensureFlowPreflightAudit() [cmd/api_rest/flow_preflight.go:92]
- Fan-out: 27 callees
- Módulo: remora-flujo
- Disparado por: runFlowManifest()

#### Pipeline interno:
1. Si no hay dataset pero sí sqlite_db: media datos via Sabio (ensureSabioDataMediation)
2. Si no hay external.api.dump NI dataset.raw: retorna true (no hay qué auditar)
3. Busca provider de capability "data.quality.audit" (findProviderForCapability)
4. Registra nodo dinámico preflight_audit_{target_id} (recordDynamicFlowNode)
5. Resuelve contrato del auditor; verifica que sus inputs estén disponibles
6. Si faltan inputs del auditor: skip con human summary explicativo
7. Ejecuta el auditor via executeFlowNode; parsea output con extractHumanSummary
8. Si auditor falla: marca result.Status=failed, registra preflight no-ready
9. Si auditor completa: registra artefactos, agrega gap summary al human summary
10. Invoca resolveFlowGapsIteratively para resolver gaps detectados
11. Determina readiness final: ready sólo si status != needs_input && status != failed
12. Registra flow.preflight.v1 con detalle de readiness y blockers

#### Invariantes (axiomas):
- Si no existe dataset NI dump: retorna true INMEDIATAMENTE (asume no hay datos que auditar)
- El preflight SIEMPRE ejecuta Auditor antes de intentar resolución de gaps
- Si Auditor falla: el flujo se DETIENE; no se intenta resolución de gaps con datos corruptos
- El resultado de readiness se persiste SIEMPRE (flow.preflight.v1), tanto si ready como si no
- Sabio mediation se dispara SOLO si hay sqlite_db pero no hay dump/dataset (convierte DB a dataset)

#### Callees clave:
- executeFlowNode() — ejecuta el auditor contra channel
- resolveFlowGapsIteratively() — resuelve gaps encontrados por el auditor
- ensureSabioDataMediation() — media datos de sqlite a formato auditable
- recordFlowPreflight() — persiste el artefacto flow.preflight.v1 con estado de readiness
- summarizeAuditorGaps() — genera resumen legible de los gaps para el UI

<!-- CHAIN_RUN_ID: run-1778992200 -->
