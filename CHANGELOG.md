# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.27] - 2026-05-12

> **Release**: expandir flujo

### Flujo

- **static/index.html**: +649 / -217
- **api_rest/business_artifacts.go**: +3 / -8
- **api_rest/flow_field_evidence.go**: archivo nuevo
- **api_rest/flow_gap_inputs.go**: +15 / -30
- **api_rest/flow_prerequisites.go**: funciones: (s *server) generateFlowPrerequisites, (s *server) refreshFlowPrerequisites, prerequisitesSummary, flowPrerequisiteExecutionBlockers
- **api_rest/flow_proposals.go**: +7
- **api_rest/flow_run_types.go**: tipos: flowInputAction
- **api_rest/flow_smtp.go**: +12 / -33
- **api_rest/flow_store.go**: +3 / -7
- **api_rest/flow_user_interaction.go**: archivo nuevo
- **api_rest/generic_driver.go**: +9 / -25
- **api_rest/main.go**: +5 / -9
- **api_rest/manifest_runtime.go**: archivo nuevo
- **api_rest/single_wrapper.go**: +3 / -42
- **api_rest/flow_cycles.go**: +3 / -9
- **api_rest/flow_data_pipeline.go**: +69 / -36
- **api_rest/flow_execution.go**: funciones: contractNeedsBusinessSQLitePath
- **api_rest/flow_gap_resolution.go**: +17 / -30
- **api_rest/flow_installation.go**: +10 / -8
- **api_rest/flow_interactions.go**: archivo nuevo
- **api_rest/flow_provider_interaction.go**: funciones: (s *server) invokeProviderNextQuestion
- **api_rest/flow_readiness.go**: +23 / -21
- **api_rest/flow_runner.go**: +14
- **api_rest/flow_prerequisites_test.go**: archivo nuevo
- **api_rest/flow_run_test.go**: funciones: TestCollectionFlowRoutesNodeViewAnswerToHosting
- **api_rest/manifest_runtime_test.go**: archivo nuevo

### Repo

- **Makefile**: +8 / -4
- **PROMPT_PROBLEMAS_RONDA2**: archivo nuevo
- **cloudbuild.yaml**: +11
- **data/findings.json**: +1 / -1
- **framework-hosting/framework.manifest.json**: +4 / -3
- **framework-hosting/go.mod**: +4
- **framework-mensajero/framework.manifest.json**: +3 / -2
- **framework-mensajero/go.mod**: +4
- **scripts/bootstrap.sh**: +3 / -2
- **scripts/dev-local.sh**: +1 / -1
- **scripts/restart_api.sh**: archivo nuevo
- **AGENTS.md**: archivo nuevo
- **credentials/smtp.go**: archivo nuevo
- **frameworkhosting/main.go**: funciones: dispatchIntent, handleConnectWizard, questionForState, questionIDForStep | tipos: frameworkQuestion, connectOutcome, smtpImportOutcome
- **frameworkmensajero/main.go**: funciones: smtpCredsFromBundle
- **vault/vault.go**: funciones: discoverRepoVaultDir
- **frameworkhosting/main_test.go**: archivo nuevo

## [0.1.26] - 2026-05-12

> **Release**: expandir flujo

### Flujo

- **api_rest/flow_prerequisites.go**: +15 / -18
- **api_rest/flow_proposals.go**: funciones: parseDataQualityBulk
- **api_rest/flow_gap_resolution.go**: funciones: (s *server) generateBulkMigrationArtifact

### Repo

- **main-multi-modo**: main.go soporta multiples modos (audit, explain, tree) (frameworkauditor/main.go)
- **data/findings.json**: +18052 / -4324
- **checks/checks.go**: +118 / -58
- **checks/sqlite.go**: +8 / -7

## [0.1.25] - 2026-05-11

> **Release**: expandir flujo

### Flujo

- **api_rest/entrypoint.sh**: +1 / -2
- **static/index.html**: +676 / -40
- **api_rest/business_artifacts.go**: funciones: (s *server) loadBusinessScopeTables, filterGapsByScope
- **api_rest/flow_prerequisites.go**: archivo nuevo
- **api_rest/flow_proposals.go**: +3 / -1
- **api_rest/flow_run_types.go**: +1
- **api_rest/flow_artifacts.go**: funciones: (s *server) validateActionOptionsForNode, fallbackActionOptionForBound
- **api_rest/flow_data_pipeline.go**: funciones: (s *server) verifySMTPCredentialsReal, hostingCredentialVerificationSummary, lastHostingCredentialFailure
- **api_rest/flow_dimensions.go**: +12 / -2
- **api_rest/flow_gap_resolution.go**: +39 / -4
- **api_rest/flow_installation.go**: +7 / -1
- **api_rest/flow_provider_interaction.go**: +4 / -1
- **api_rest/flow_runner.go**: funciones: actionBoundTypes
- **api_rest/flow_run_test.go**: funciones: TestRunFlowManifestValidatesActionOptionsAgainstManifestBounds

### Repo

- **main-multi-modo**: main.go soporta multiples modos (audit, explain, tree) (frameworkmecanico/main.go)
- **data/findings.json**: +64928 / -585
- **framework-foco/framework.manifest.json**: +26
- **framework-hosting/framework.manifest.json**: +38
- **framework-hosting/frameworkhosting**: configuracion
- **framework-mecanico/framework.manifest.json**: +8 / -4
- **framework-mensajero/frameworkmensajero**: configuracion
- **biz_vh9T64yrCmdnR5Xf/sabio.business.json**: +15
- **panalbit/sabio.business.json**: +15
- **PROMPT_PROBLEMAS_DETECTADOS.md**: archivo nuevo
- **PROMPT_PROBLEMAS_RONDA2.md**: archivo nuevo
- **manifest/manifest.go**: tipos: ActionBoundSpec
- **foco/main.go**: funciones: actionBoundForID
- **frameworkhosting/main.go**: funciones: cmdVerifySMTP, verifySMTPLogin, vaultEnv, defaultVaultDir
- **frameworkmensajero/main.go**: funciones: vaultEnv, defaultVaultDir

## [0.1.24] - 2026-05-11

> **Release**: expandir flujo

### Flujo

- **static/index.html**: +382 / -150
- **api_rest/flow_backend.go**: tipos: flowIntent
- **api_rest/flow_gap_inputs.go**: +12 / -7
- **api_rest/flow_run_types.go**: +4
- **api_rest/flow_store.go**: funciones: (fs *flowStore) recordRun, (fs *flowStore) recordArtifact, (fs *flowStore) latestArtifactPath, (fs *flowStore) upsertInstallation | tipos: TEXT
- **api_rest/main.go**: +11 / -2
- **api_rest/flow_artifacts.go**: +30 / -1
- **api_rest/flow_channel.go**: archivo nuevo
- **api_rest/flow_cycles.go**: funciones: (s *server) recordCycleResult, cycleResultStatus
- **api_rest/flow_data_pipeline.go**: archivo nuevo
- **api_rest/flow_execution.go**: +5 / -1
- **api_rest/flow_gap_resolution.go**: funciones: (s *server) findProviderWithCommand, (s *server) providerNameForCapabilityOrCommand
- **api_rest/flow_intent.go**: archivo nuevo
- **api_rest/flow_params.go**: +3
- **api_rest/flow_preflight.go**: funciones: (s *server) shouldRunLayeredDataValidation, isBusinessDataArtifact
- **api_rest/flow_runner.go**: +47 / -1
- **api_rest/flow_work_context.go**: archivo nuevo
- **api_rest/flow_run_test.go**: funciones: TestRunFlowManifestEmitsFlowIntentArtifact, TestRunFlowManifestWithoutIntentDoesNotEmitFlowIntentArtifact, TestFlowIntentAvailableToFirstNode, flowIntentTestManifests
- **api_rest/flow_store_test.go**: archivo nuevo

### Repo

- **data/findings.json**: +585 / -64928
- **framework-hosting/frameworkhosting**: configuracion
- **PROMPT_VISION_COMPLETA.md**: archivo nuevo
- **README.md**: +6 / -1
- **frameworkmecanico/main.go**: funciones: questionForGapWithLLM, parseLLMQuestion, fallbackNaturalQuestion, inferGapField

## [0.1.23] - 2026-05-11

> **Release**: expandir flujo

### Flujo

- **api_rest/Dockerfile**: -3
- **api_rest/entrypoint.sh**: -13
- **api_rest/active_task.go**: +7 / -46
- **api_rest/flow_backend.go**: funciones: resolutionModeFromPolicies, resolutionModeForCapability, normalizeFlowLifecycleRoles, prepareFlowManifestLifecycle
- **api_rest/flow_gap_inputs.go**: archivo nuevo
- **api_rest/flow_proposals.go**: archivo nuevo
- **api_rest/flow_run_types.go**: archivo nuevo
- **api_rest/flow_store.go**: funciones: flowUsesInstallableAnalysis, (s *server) flowOperationalSnapshot, (s *server) flowStatePath
- **api_rest/orchestrator.go**: +2 / -2
- **api_rest/tareas.go**: funciones: focoTasksList, focoTasksNext, activeTaskFromFoco, createFocoTask | tipos: focoTaskPlan, focoTaskNote, focoTaskEvent
- **api_rest/flow_artifacts.go**: archivo nuevo
- **api_rest/flow_cycles.go**: archivo nuevo
- **api_rest/flow_data_mediation.go**: archivo nuevo
- **api_rest/flow_dimensions.go**: archivo nuevo
- **api_rest/flow_execution.go**: archivo nuevo
- **api_rest/flow_gap_resolution.go**: archivo nuevo
- **api_rest/flow_installation.go**: archivo nuevo
- **api_rest/flow_legacy_paths.go**: archivo nuevo
- **api_rest/flow_params.go**: archivo nuevo
- **api_rest/flow_preflight.go**: archivo nuevo
- **api_rest/flow_provider_defaults.go**: archivo nuevo
- **api_rest/flow_provider_interaction.go**: archivo nuevo
- **api_rest/flow_readiness.go**: archivo nuevo
- **api_rest/flow_run.go**: -3431
- **api_rest/flow_runner.go**: archivo nuevo
- **api_rest/flow_runtime_approval.go**: archivo nuevo
- **api_rest/flow_summaries.go**: archivo nuevo
- **api_rest/flow_backend_test.go**: funciones: TestFindProviderForCapability, TestFindProviderForCapabilityNotFound, TestResolutionModeFromPolicies, TestGapResolutionRegistryUsesCapabilityNotName
- **api_rest/flow_run_test.go**: +6 / -4

### Repo

- **framework-auditor/framework.manifest.json**: +1
- **framework-foco/framework.manifest.json**: +11 / -2
- **framework-hosting/framework.manifest.json**: +6
- **framework-mecanico/framework.manifest.json**: +3
- **framework-mensajero/framework.manifest.json**: +1
- **framework-radar/framework.manifest.json**: +2
- **framework-sabio/framework.manifest.json**: +3
- **manifest/manifest.go**: tipos: StateSpec
- **foco/cobranza_tasks.go**: funciones: parseTaskNotesMap
- **foco/main.go**: +3 / -3

## [0.1.22] - 2026-05-11

> **Release**: expandir flujo

### Flujo

- **static/index.html**: +103 / -7
- **api_rest/flow_backend.go**: funciones: configuredFlowEntry, applyConfiguredFlowEntry | tipos: flowLifecycle, flowLifecycleEntry
- **api_rest/flow_run.go**: funciones: isCycleTerminalStep, flowAnalysisAccepted, shouldPauseForAnalysisAcceptance, inputRequestForAnalysisAcceptance | tipos: flowStepTrigger
- **api_rest/flow_backend_test.go**: funciones: TestPrepareFlowManifestLifecycleHonorsConfiguredEntry
- **api_rest/flow_run_test.go**: funciones: TestRunFlowManifestPausesForRadarAnalysisAcceptance, TestCycleTerminalPolicyClosesCycle, TestNoCycleWithoutTerminalPolicy, TestFlowBranchLimitUsesEnvironmentCap

### Repo

- **data/findings.json**: +1 / -1
- **framework-foco/framework.manifest.json**: +7 / -3
- **framework-foco/frameworkfoco**: configuracion
- **framework-mensajero/framework.manifest.json**: +1
- **foco/cobranza_sql.go**: +1
- **foco/cobranza_tasks.go**: +1
- **foco/main.go**: funciones: persistentKeyFromConvID, loadPersistentState, mergePersistentCarryOver, taskDueBefore
- **foco/main_test.go**: funciones: TestPriorityCandidatePreservesLedgerTaskID, TestNormalizeActionOptionsFromNil, TestNormalizeActionOptionsFromOneRecommendation, TestNormalizeActionOptionsFromFiveRecommendations

## [0.1.21] - 2026-05-10

> **Release**: expandir alfa, bravo, charlie, echo, excel, flujo, gmail, paladin, quine

### Charlie

- **framework-charlie/.charlieignore**: +1 / -1
- **framework-charlie/framework.manifest.json**: archivo nuevo
- **framework-charlie/WHY.md**: archivo nuevo
- **charlie/charlie.go**: +2 / -2
- **llm/client.go**: archivo nuevo

- **framework-charlie/.charlieignore**: +3 / -1
- **framework-charlie/framework.manifest.json**: archivo nuevo
- **framework-charlie/WHY.md**: archivo nuevo
- **charlie/charlie.go**: +2 / -2
- **llm/client.go**: archivo nuevo
- **paladin-server**: servidor HTTP para recibir traces (server/main.go)
- **framework-paladin/framework.manifest.json**: +6
- **llm/client.go**: archivo nuevo
- **paladin/lint.go**: funciones: lintManifestCapabilities, hasPolicy, capabilityUsesMultipleEngines, capabilityLooksGrounded | tipos: lintCapability
- **paladin/lint_test.go**: funciones: TestLintManifestsCatchesMissingTypedCapabilities, TestLintManifestsValidatesTypedCapabilityContract
- **framework-quine/framework.manifest.json**: archivo nuevo
- **framework-quine/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **review/review.go**: +2 / -2
- **types/types.go**: +1 / -1
- **api_rest/.dockerignore**: +31
- **api_rest/Dockerfile**: -3
- **api_rest/Dockerfile.dev**: +31
- **api_rest/deploy.sh**: +111
- **api_rest/entrypoint.sh**: +76
- **api_rest/flow.rules.json**: +72
- **api_rest/flujo_api**: configuracion
- **static/data.html**: archivo nuevo
- **static/index.html**: archivo nuevo
- **flujo_api/Dockerfile**: -84
- **static/index.html**: -3492
- **remora-flujo/framework_session**: archivo nuevo
- **frontend-chat/index.html**: +469 / -6
- **remora-flujo/go.mod**: +25 / -1
- **remora-flujo/go.sum**: +67
- **api_rest/active_task.go**: funciones: activeTaskContext, invalidateActiveTaskCache, tareasBinPath, buildActiveTaskLine | tipos: ActiveTask
- **api_rest/api_connections.go**: archivo nuevo
- **api_rest/auth.go**: funciones: defaultAuthDBPath
- **api_rest/auth_handlers.go**: archivo nuevo
- **api_rest/business_artifacts.go**: archivo nuevo
- **api_rest/contactos.go**: archivo nuevo
- **api_rest/data_browser.go**: +4 / -1
- **api_rest/drivers.go**: funciones: keysOfManifests, initDriverRegistry, keysOf, driversFor | tipos: FrameworkDriver, QueuedAnswerCtx, nextQuestionResponse
- **api_rest/flow_backend.go**: archivo nuevo
- **api_rest/flow_run.go**: funciones: (s *server) runFlowStream, (s *server) runFlowManifest, resetCycleArtifacts, (s *server) shouldSkipInstalledAnalysis | tipos: flowBranchRun, flowRequiredInput, flowInputField
- **api_rest/flow_smtp.go**: archivo nuevo
- **api_rest/flow_store.go**: archivo nuevo
- **api_rest/flow_suggest.go**: archivo nuevo
- **api_rest/framework_session.go**: archivo nuevo
- **api_rest/generic_driver.go**: funciones: newGenericDriver, (g *genericDriver) Name, (g *genericDriver) fullArgs, (g *genericDriver) resolveCommandArgs | tipos: genericDriver, historyTurn
- **api_rest/main.go**: funciones: getRuntimeInfo, main, envOr, loadDotEnv | tipos: APIResponse, fwInfo, server
- **api_rest/multimodal.go**: funciones: loadFrameworkManifest, modelSpecFor, preprocessVision, appendImageAnalysis | tipos: entry
- **api_rest/orchestrator.go**: funciones: runLoop, driverNames, truncate, hasImageResource | tipos: fullPoller
- **api_rest/rules.go**: funciones: loadFlowRules, (fr *FlowRules) Match, condMatches, reorderDrivers | tipos: FlowRules, FlowRule, FlowCondition
- **api_rest/single_wrapper.go**: funciones: (s *server) createUniversalSingleMessage, (s *server) runUniversalSingle, selectUniversalCommand, universalCommandScore | tipos: universalSingleResult
- **api_rest/store.go**: funciones: convPath, metaPath, messagesPath, queuePath | tipos: Conversation, Message, MessageArtifact
- **api_rest/streaming.go**: funciones: wantsSSE, newSSEWriter, (s *sseWriter) emit, liveFilePath | tipos: sseWriter
- **api_rest/tareas.go**: funciones: resolveTareas, runTareas, currentProfile, emitTaskEvent | tipos: createTaskReq, taskEventReq
- **api_rest/traces.go**: funciones: (s *server) handleTracesLatest | tipos: entry
- **framework_session/main.go**: archivo nuevo
- **llm/client.go**: funciones: (g *groqClient) Stream, (m *minimaxClient) Stream, handleMiniStreamData | tipos: StreamEvent
- **nativeagent/agent.go**: +43 / -8
- **api_rest/email_sanitize.go**: funciones: dedupeSubjectPrefix, unresolvedPlaceholders
- **api_rest/initial_prompt.go**: archivo nuevo
- **api_rest/intent.go**: funciones: classifyIntent, providerOfModelCapability, providerOfProducedCapability
- **flujo_api/contactos.go**: -64
- **api_rest/auth_test.go**: archivo nuevo
- **api_rest/business_artifacts_test.go**: archivo nuevo
- **api_rest/data_browser_test.go**: archivo nuevo
- **api_rest/flow_backend_test.go**: funciones: TestPrepareFlowManifestLifecyclePromotesPriorityListBeforeFoco, TestValidateFlowManifestMissingRequirementAllowsRuntimeCredentials, TestValidateFlowManifestUsesMecanicoForDraftArtifact, TestNormalizeFlowLifecycleRoles
- **api_rest/flow_run_test.go**: funciones: TestRunFlowManifestDryRunExecutesSafeNodesAndStopsBeforeSideEffect, TestSummarizeAuditorGapsCompactsMissingContactsAndCounts, TestRunFlowManifestTestModeExecutesSideEffectAgainstTestRecipient, TestRunFlowManifestUsesInteractiveModeForApprovalPolicy
- **api_rest/root_test.go**: archivo nuevo
- **api_rest/single_wrapper_test.go**: funciones: TestUniversalSingleMessageExposesStandardSession, TestEncodeConversationRuntimeContextIncludesBusinessAndScope, TestSelectUniversalCommandPrefersInputCommand, TestInferUniversalParamsFromFreeTextAndJSON
- **flujo_api/root_test.go**: -36
- **nativeagent/agent_test.go**: funciones: TestResolveProviderPrefersOpenRouter, TestResolveProviderExplicitOpenRouter
- **framework-echo/framework.manifest.json**: +4 / -5
- **framework-echo/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **framework-echo/main.go**: +4 / -5
- **frameworkecho/automation.go**: funciones: generateEchoQuestion, buildTreeContext
- **framework-gmail/framework.manifest.json**: archivo nuevo
- **framework-gmail/WHY.md**: archivo nuevo
- **framework-gmail/client.go**: +1 / -4
- **framework-gmail/types.go**: +1 / -1
- **llm/client.go**: archivo nuevo
- **framework-gmail/main.go**: funciones: printJSON
- **framework-alfa/framework.manifest.json**: +1 / -2
- **framework-alfa/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **frameworkalfa/automation.go**: funciones: generateAlfaQuestion
- **prospectos-seguimiento/go.mod**: +5 / -1
- **framework-bravo/framework.manifest.json**: archivo nuevo
- **framework-bravo/go.mod**: +2 / -2
- **framework-bravo/WHY.md**: archivo nuevo
- **bravo/trace.go**: +1 / -1
- **prospectos-seguimiento/main.go**: +2 / -2
- **llm/client.go**: archivo nuevo
- **framework-excel/framework.manifest.json**: archivo nuevo
- **framework-excel/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **main-multi-modo**: main.go soporta multiples modos (audit, explain, tree) (frameworkauditor/main.go, frameworkmecanico/main.go, frameworksabio/main.go)
- **.env.example**: +12 / -5
- **.gitignore**: +3 / -4
- **CHANGELOG.md**: +319 / -28
- **Makefile**: +9 / -9
- **cloudbuild.yaml**: +48 / -9
- **docker-compose.yml**: +5 / -5
- **framework-arquitecto/framework.manifest.json**: +4 / -6
- **framework-arquitecto/frameworkarquitecto**: configuracion
- **data/findings.json**: +65135 / -259
- **framework-auditor/framework.manifest.json**: +106 / -22
- **framework-auditor/go.mod**: +14 / -1
- **framework-auditor/go.sum**: archivo nuevo
- **framework-contactos/framework.manifest.json**: -74
- **framework-contactos/go.mod**: -17
- **framework-contactos/go.sum**: -43
- **framework-critico/framework.manifest.json**: +4 / -5
- **framework-critico/frameworkcritico**: configuracion
- **framework-deployer/framework.manifest.json**: +6
- **framework-foco/framework.manifest.json**: +356 / -18
- **framework-foco/frameworkfoco**: archivo nuevo
- **framework-foco/go.mod**: -17
- **framework-foco/go.sum**: -51
- **framework-hosting/framework.manifest.json**: +336 / -31
- **framework-hosting/frameworkhosting**: configuracion
- **data/dump.json**: +1 / -1
- **data/sync_meta.json**: +3 / -3
- **framework-indexa/framework.manifest.json**: +9 / -4
- **data/applied.jsonl**: +8956 / -13
- **data/proposals.json**: +1 / -1
- **framework-mecanico/framework.manifest.json**: +349 / -33
- **framework-mensajero/framework.manifest.json**: +156 / -20
- **framework-mensajero/frameworkmensajero**: configuracion
- **framework-pingpong/framework.manifest.json**: archivo nuevo
- **servidor-rpc/pingpong_progress.json**: +3 / -2
- **framework-radar/framework.manifest.json**: +82 / -14
- **framework-radar/frameworkradar**: configuracion
- **panalbit/sabio.business.json**: archivo nuevo
- **data/qa_cobranza_chile_ideal.json**: archivo nuevo
- **framework-sabio/framework.manifest.json**: +609 / -25
- **semantic/catalog.json**: archivo nuevo
- **semantic/profile.json**: archivo nuevo
- **semantic/relationships.mmd**: archivo nuevo
- **semantic/relationships_full.mmd**: archivo nuevo
- **semantic/views.sql**: archivo nuevo
- **framework-tareas/framework.manifest.json**: +4 / -2
- **scripts/bootstrap.sh**: +1 / -1
- **scripts/demo_aceleradora.sh**: +1 / -1
- **scripts/dev-local.sh**: +4 / -4
- **scripts/install-remora.sh**: +2 / -2
- **scripts/smoke_test_api_rest.sh**: +106
- **.claude/CLAUDE.md**: archivo nuevo
- **.codex/instructions.md**: archivo nuevo
- **ARCHITECTURE.md**: +7 / -5
- **HANDOFF_PROMPT.md**: +13 / -14
- **README.md**: +3 / -3
- **SKILL_COLABORACION.md**: archivo nuevo
- **docs/AXIOMS.md**: archivo nuevo
- **docs/CAPABILITIES.md**: +1 / -1
- **framework-arquitecto/WHY.md**: archivo nuevo
- **framework-auditor/INITIAL_PROMPT.md**: archivo nuevo
- **framework-auditor/WHY.md**: archivo nuevo
- **framework-critico/WHY.md**: archivo nuevo
- **framework-deployer/README.md**: +55 / -28
- **framework-deployer/WHY.md**: archivo nuevo
- **framework-hosting/INITIAL_PROMPT.md**: archivo nuevo
- **framework-hosting/WHY.md**: archivo nuevo
- **framework-indexa/INITIAL_PROMPT.md**: archivo nuevo
- **framework-indexa/WHY.md**: archivo nuevo
- **framework-mecanico/INITIAL_PROMPT.md**: archivo nuevo
- **framework-mecanico/WHY.md**: archivo nuevo
- **framework-mensajero/INITIAL_PROMPT.md**: archivo nuevo
- **framework-mensajero/WHY.md**: archivo nuevo
- **framework-sabio/AXIOMS.md**: archivo nuevo
- **framework-sabio/INITIAL_PROMPT.md**: archivo nuevo
- **framework-sabio/WHY.md**: archivo nuevo
- **semantic/profile.md**: archivo nuevo
- **framework-tareas/INITIAL_PROMPT.md**: archivo nuevo
- **framework-tareas/WHY.md**: archivo nuevo
- **adapter/adapter.go**: funciones: (c *Client) Grep, (c *Client) Find, (c *Client) EditFile
- **channel/main.go**: funciones: loadDotEnv
- **internal/handler.go**: funciones: (h *Handler) grep, (h *Handler) find, (h *Handler) editFile, intParam
- **internal/jsonrpc.go**: +6 / -2
- **manifest/manifest.go**: tipos: CapabilitySpec
- **frameworkarquitecto/llm.go**: +22 / -11
- **frameworkarquitecto/llm_stream.go**: +13 / -4
- **llm/client.go**: archivo nuevo
- **checks/checks.go**: funciones: InferFKRelations, InferRequiredStringFields, InferRequiredNonNullFields, InferDateFields | tipos: EndpointField, FKRelation
- **llm/client.go**: archivo nuevo
- **frameworkcritico/main.go**: funciones: resolveCriticoProvider, evaluateWithLLM, firstNonEmpty, loadEnvFiles | tipos: msg
- **llm/client.go**: archivo nuevo
- **deployer/config.go**: archivo nuevo
- **deployer/diagnose.go**: archivo nuevo
- **deployer/runbook.go**: archivo nuevo
- **deployer/runner.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **foco/cobranza_sql.go**: +1 / -137
- **foco/cobranza_tasks.go**: +1 / -1
- **foco/main.go**: funciones: interpretFocoInput, runSessionStart, sessionStartSystem, runPriorities | tipos: sessionStartEvent, sessionStartResponse, priorityCandidate
- **llm/client.go**: funciones: (c *Client) Provider, (c *Client) Model, (c *Client) generateOAICompat
- **frameworkhosting/main.go**: funciones: dispatchIntent, handleConnectWizard, sanitizeHostAnswer, doConnectFromText | tipos: cpanelDiscoveryResp, cpanelCandidateResult
- **cpanel/client.go**: funciones: normalizeHost, isPlaceholderHost
- **cpanel/domain.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **frameworkindexa/main.go**: funciones: cmdAPIPlan, planConnectorWithLLM, connectorPlanFromOpenAPI, extractJSONObjectStrings | tipos: connectorSpec, connectorResource, docSource
- **llm/client.go**: archivo nuevo
- **auditdata/auditdata.go**: funciones: ParseFindings, ParseDataset
- **llm/client.go**: archivo nuevo
- **frameworkmensajero/main.go**: funciones: subjectWithDevPrefix
- **llm/client.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **servidor-rpc/servidor.go**: funciones: main | tipos: Servicio
- **frameworkradar/main.go**: funciones: runConfigureAnalysis, scoreSQLite, fetchPaymentStats, findPaymentTable | tipos: paymentStats, analysisPlanPaths
- **llm/client.go**: funciones: (c *Client) generateOAICompat
- **sqlqa/sqlqa.go**: tipos: relation
- **llm/client.go**: archivo nuevo
- **remora/main.go**: +8 / -8
- **orchestrator/main.go**: +2 / -1
- **internal/whitelist.go**: +37 / -34
- **checks/sqlite.go**: archivo nuevo
- **frameworkcontactos/main.go**: -432
- **deployer/main.go**: funciones: printJSON, exitErr, hasFlag, flagValue
- **deployer/deploy.go**: -180
- **framework-pingpong/palindrome.go**: funciones: palindromeDemo
- **framework-pingpong/roman_to_integer.go**: funciones: romanToIntegerDemo
- **servidor-rpc/client.go**: +2 / -6
- **servidor-rpc/cliente.go**: archivo nuevo
- **framework-pingpong/two_sum.go**: funciones: twoSumDemo, twoSum
- **manifest/manifest_test.go**: archivo nuevo
- **checks/sqlite_test.go**: archivo nuevo
- **frameworkauditor/main_test.go**: archivo nuevo
- **deployer/runbook_test.go**: archivo nuevo
- **foco/main_test.go**: archivo nuevo
- **frameworkradar/main_test.go**: funciones: TestPersistAnalysisPlanWritesTangibleJSONAndSQL, TestLoadPersistedAnalysisPlanReusesConfiguredModel
- **frameworksabio/main_test.go**: archivo nuevo
- **framework-sabio/qa_fixture_test.go**: archivo nuevo

- **framework-charlie/.charlieignore**: +3 / -1
- **framework-charlie/framework.manifest.json**: archivo nuevo
- **framework-charlie/WHY.md**: archivo nuevo
- **charlie/charlie.go**: +2 / -2
- **llm/client.go**: archivo nuevo
- **paladin-server**: servidor HTTP para recibir traces (server/main.go)
- **framework-paladin/framework.manifest.json**: +6
- **llm/client.go**: archivo nuevo
- **paladin/lint.go**: funciones: lintManifestCapabilities, hasPolicy, capabilityUsesMultipleEngines, capabilityLooksGrounded | tipos: lintCapability
- **paladin/lint_test.go**: funciones: TestLintManifestsCatchesMissingTypedCapabilities, TestLintManifestsValidatesTypedCapabilityContract
- **framework-quine/framework.manifest.json**: archivo nuevo
- **framework-quine/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **review/review.go**: +2 / -2
- **types/types.go**: +1 / -1
- **api_rest/.dockerignore**: +31
- **api_rest/Dockerfile**: -3
- **api_rest/Dockerfile.dev**: +31
- **api_rest/deploy.sh**: +111
- **api_rest/entrypoint.sh**: +76
- **api_rest/flow.rules.json**: +72
- **api_rest/flujo_api**: configuracion
- **static/data.html**: archivo nuevo
- **static/index.html**: archivo nuevo
- **flujo_api/Dockerfile**: -84
- **static/index.html**: -3492
- **remora-flujo/framework_session**: archivo nuevo
- **frontend-chat/index.html**: +469 / -6
- **remora-flujo/go.mod**: +25 / -1
- **remora-flujo/go.sum**: +67
- **api_rest/active_task.go**: funciones: activeTaskContext, invalidateActiveTaskCache, tareasBinPath, buildActiveTaskLine | tipos: ActiveTask
- **api_rest/api_connections.go**: archivo nuevo
- **api_rest/auth.go**: funciones: defaultAuthDBPath
- **api_rest/auth_handlers.go**: archivo nuevo
- **api_rest/business_artifacts.go**: archivo nuevo
- **api_rest/contactos.go**: archivo nuevo
- **api_rest/data_browser.go**: +4 / -1
- **api_rest/drivers.go**: funciones: keysOfManifests, initDriverRegistry, keysOf, driversFor | tipos: FrameworkDriver, QueuedAnswerCtx, nextQuestionResponse
- **api_rest/flow_backend.go**: archivo nuevo
- **api_rest/flow_run.go**: funciones: (s *server) runFlowStream, (s *server) runFlowManifest, resetCycleArtifacts, (s *server) shouldSkipInstalledAnalysis | tipos: flowBranchRun, flowRequiredInput, flowInputField
- **api_rest/flow_smtp.go**: archivo nuevo
- **api_rest/flow_store.go**: archivo nuevo
- **api_rest/flow_suggest.go**: archivo nuevo
- **api_rest/framework_session.go**: archivo nuevo
- **api_rest/generic_driver.go**: funciones: newGenericDriver, (g *genericDriver) Name, (g *genericDriver) fullArgs, (g *genericDriver) resolveCommandArgs | tipos: genericDriver, historyTurn
- **api_rest/main.go**: funciones: getRuntimeInfo, main, envOr, loadDotEnv | tipos: APIResponse, fwInfo, server
- **api_rest/multimodal.go**: funciones: loadFrameworkManifest, modelSpecFor, preprocessVision, appendImageAnalysis | tipos: entry
- **api_rest/orchestrator.go**: funciones: runLoop, driverNames, truncate, hasImageResource | tipos: fullPoller
- **api_rest/rules.go**: funciones: loadFlowRules, (fr *FlowRules) Match, condMatches, reorderDrivers | tipos: FlowRules, FlowRule, FlowCondition
- **api_rest/single_wrapper.go**: funciones: (s *server) createUniversalSingleMessage, (s *server) runUniversalSingle, selectUniversalCommand, universalCommandScore | tipos: universalSingleResult
- **api_rest/store.go**: funciones: convPath, metaPath, messagesPath, queuePath | tipos: Conversation, Message, MessageArtifact
- **api_rest/streaming.go**: funciones: wantsSSE, newSSEWriter, (s *sseWriter) emit, liveFilePath | tipos: sseWriter
- **api_rest/tareas.go**: funciones: resolveTareas, runTareas, currentProfile, emitTaskEvent | tipos: createTaskReq, taskEventReq
- **api_rest/traces.go**: funciones: (s *server) handleTracesLatest | tipos: entry
- **framework_session/main.go**: archivo nuevo
- **llm/client.go**: funciones: (g *groqClient) Stream, (m *minimaxClient) Stream, handleMiniStreamData | tipos: StreamEvent
- **nativeagent/agent.go**: +43 / -8
- **api_rest/email_sanitize.go**: funciones: dedupeSubjectPrefix, unresolvedPlaceholders
- **api_rest/initial_prompt.go**: archivo nuevo
- **api_rest/intent.go**: funciones: classifyIntent, providerOfModelCapability, providerOfProducedCapability
- **flujo_api/contactos.go**: -64
- **api_rest/auth_test.go**: archivo nuevo
- **api_rest/business_artifacts_test.go**: archivo nuevo
- **api_rest/data_browser_test.go**: archivo nuevo
- **api_rest/flow_backend_test.go**: funciones: TestPrepareFlowManifestLifecyclePromotesPriorityListBeforeFoco, TestValidateFlowManifestMissingRequirementAllowsRuntimeCredentials, TestValidateFlowManifestUsesMecanicoForDraftArtifact, TestNormalizeFlowLifecycleRoles
- **api_rest/flow_run_test.go**: funciones: TestRunFlowManifestDryRunExecutesSafeNodesAndStopsBeforeSideEffect, TestSummarizeAuditorGapsCompactsMissingContactsAndCounts, TestRunFlowManifestTestModeExecutesSideEffectAgainstTestRecipient, TestRunFlowManifestUsesInteractiveModeForApprovalPolicy
- **api_rest/root_test.go**: archivo nuevo
- **api_rest/single_wrapper_test.go**: funciones: TestUniversalSingleMessageExposesStandardSession, TestEncodeConversationRuntimeContextIncludesBusinessAndScope, TestSelectUniversalCommandPrefersInputCommand, TestInferUniversalParamsFromFreeTextAndJSON
- **flujo_api/root_test.go**: -36
- **nativeagent/agent_test.go**: funciones: TestResolveProviderPrefersOpenRouter, TestResolveProviderExplicitOpenRouter
- **framework-echo/framework.manifest.json**: +4 / -5
- **framework-echo/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **framework-echo/main.go**: +4 / -5
- **frameworkecho/automation.go**: funciones: generateEchoQuestion, buildTreeContext
- **framework-gmail/framework.manifest.json**: archivo nuevo
- **framework-gmail/WHY.md**: archivo nuevo
- **framework-gmail/client.go**: +1 / -4
- **framework-gmail/types.go**: +1 / -1
- **llm/client.go**: archivo nuevo
- **framework-gmail/main.go**: funciones: printJSON
- **framework-alfa/framework.manifest.json**: +1 / -2
- **framework-alfa/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **frameworkalfa/automation.go**: funciones: generateAlfaQuestion
- **prospectos-seguimiento/go.mod**: +5 / -1
- **framework-bravo/framework.manifest.json**: archivo nuevo
- **framework-bravo/go.mod**: +2 / -2
- **framework-bravo/WHY.md**: archivo nuevo
- **bravo/trace.go**: +1 / -1
- **prospectos-seguimiento/main.go**: +2 / -2
- **llm/client.go**: archivo nuevo
- **framework-excel/framework.manifest.json**: archivo nuevo
- **framework-excel/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **main-multi-modo**: main.go soporta multiples modos (audit, explain, tree) (frameworkauditor/main.go, frameworkmecanico/main.go, frameworksabio/main.go)
- **.env.example**: +12 / -5
- **.gitignore**: +3 / -4
- **CHANGELOG.md**: +232
- **Makefile**: +9 / -9
- **cloudbuild.yaml**: +48 / -9
- **docker-compose.yml**: +5 / -5
- **framework-arquitecto/framework.manifest.json**: +4 / -6
- **framework-arquitecto/frameworkarquitecto**: configuracion
- **data/findings.json**: +65135 / -259
- **framework-auditor/framework.manifest.json**: +106 / -22
- **framework-auditor/go.mod**: +14 / -1
- **framework-auditor/go.sum**: archivo nuevo
- **framework-contactos/framework.manifest.json**: -74
- **framework-contactos/go.mod**: -17
- **framework-contactos/go.sum**: -43
- **framework-critico/framework.manifest.json**: +4 / -5
- **framework-critico/frameworkcritico**: configuracion
- **framework-deployer/framework.manifest.json**: +6
- **framework-foco/framework.manifest.json**: +356 / -18
- **framework-foco/frameworkfoco**: archivo nuevo
- **framework-foco/go.mod**: -17
- **framework-foco/go.sum**: -51
- **framework-hosting/framework.manifest.json**: +336 / -31
- **framework-hosting/frameworkhosting**: configuracion
- **data/dump.json**: +1 / -1
- **data/sync_meta.json**: +3 / -3
- **framework-indexa/framework.manifest.json**: +9 / -4
- **data/applied.jsonl**: +8956 / -13
- **data/proposals.json**: +1 / -1
- **framework-mecanico/framework.manifest.json**: +349 / -33
- **framework-mensajero/framework.manifest.json**: +156 / -20
- **framework-mensajero/frameworkmensajero**: configuracion
- **framework-pingpong/framework.manifest.json**: archivo nuevo
- **servidor-rpc/pingpong_progress.json**: +3 / -2
- **framework-radar/framework.manifest.json**: +82 / -14
- **framework-radar/frameworkradar**: configuracion
- **panalbit/sabio.business.json**: archivo nuevo
- **data/qa_cobranza_chile_ideal.json**: archivo nuevo
- **framework-sabio/framework.manifest.json**: +609 / -25
- **semantic/catalog.json**: archivo nuevo
- **semantic/profile.json**: archivo nuevo
- **semantic/relationships.mmd**: archivo nuevo
- **semantic/relationships_full.mmd**: archivo nuevo
- **semantic/views.sql**: archivo nuevo
- **framework-tareas/framework.manifest.json**: +4 / -2
- **scripts/bootstrap.sh**: +1 / -1
- **scripts/demo_aceleradora.sh**: +1 / -1
- **scripts/dev-local.sh**: +4 / -4
- **scripts/install-remora.sh**: +2 / -2
- **scripts/smoke_test_api_rest.sh**: +106
- **.claude/CLAUDE.md**: archivo nuevo
- **.codex/instructions.md**: archivo nuevo
- **ARCHITECTURE.md**: +7 / -5
- **HANDOFF_PROMPT.md**: +13 / -14
- **README.md**: +3 / -3
- **SKILL_COLABORACION.md**: archivo nuevo
- **docs/AXIOMS.md**: archivo nuevo
- **docs/CAPABILITIES.md**: +1 / -1
- **framework-arquitecto/WHY.md**: archivo nuevo
- **framework-auditor/INITIAL_PROMPT.md**: archivo nuevo
- **framework-auditor/WHY.md**: archivo nuevo
- **framework-critico/WHY.md**: archivo nuevo
- **framework-deployer/README.md**: +55 / -28
- **framework-deployer/WHY.md**: archivo nuevo
- **framework-hosting/INITIAL_PROMPT.md**: archivo nuevo
- **framework-hosting/WHY.md**: archivo nuevo
- **framework-indexa/INITIAL_PROMPT.md**: archivo nuevo
- **framework-indexa/WHY.md**: archivo nuevo
- **framework-mecanico/INITIAL_PROMPT.md**: archivo nuevo
- **framework-mecanico/WHY.md**: archivo nuevo
- **framework-mensajero/INITIAL_PROMPT.md**: archivo nuevo
- **framework-mensajero/WHY.md**: archivo nuevo
- **framework-sabio/AXIOMS.md**: archivo nuevo
- **framework-sabio/INITIAL_PROMPT.md**: archivo nuevo
- **framework-sabio/WHY.md**: archivo nuevo
- **semantic/profile.md**: archivo nuevo
- **framework-tareas/INITIAL_PROMPT.md**: archivo nuevo
- **framework-tareas/WHY.md**: archivo nuevo
- **adapter/adapter.go**: funciones: (c *Client) Grep, (c *Client) Find, (c *Client) EditFile
- **channel/main.go**: funciones: loadDotEnv
- **internal/handler.go**: funciones: (h *Handler) grep, (h *Handler) find, (h *Handler) editFile, intParam
- **internal/jsonrpc.go**: +6 / -2
- **manifest/manifest.go**: tipos: CapabilitySpec
- **frameworkarquitecto/llm.go**: +22 / -11
- **frameworkarquitecto/llm_stream.go**: +13 / -4
- **llm/client.go**: archivo nuevo
- **checks/checks.go**: funciones: InferFKRelations, InferRequiredStringFields, InferRequiredNonNullFields, InferDateFields | tipos: EndpointField, FKRelation
- **llm/client.go**: archivo nuevo
- **frameworkcritico/main.go**: funciones: resolveCriticoProvider, evaluateWithLLM, firstNonEmpty, loadEnvFiles | tipos: msg
- **llm/client.go**: archivo nuevo
- **deployer/config.go**: archivo nuevo
- **deployer/diagnose.go**: archivo nuevo
- **deployer/runbook.go**: archivo nuevo
- **deployer/runner.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **foco/cobranza_sql.go**: +1 / -137
- **foco/cobranza_tasks.go**: +1 / -1
- **foco/main.go**: funciones: interpretFocoInput, runSessionStart, sessionStartSystem, runPriorities | tipos: sessionStartEvent, sessionStartResponse, priorityCandidate
- **llm/client.go**: funciones: (c *Client) Provider, (c *Client) Model, (c *Client) generateOAICompat
- **frameworkhosting/main.go**: funciones: dispatchIntent, handleConnectWizard, sanitizeHostAnswer, doConnectFromText | tipos: cpanelDiscoveryResp, cpanelCandidateResult
- **cpanel/client.go**: funciones: normalizeHost, isPlaceholderHost
- **cpanel/domain.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **frameworkindexa/main.go**: funciones: cmdAPIPlan, planConnectorWithLLM, connectorPlanFromOpenAPI, extractJSONObjectStrings | tipos: connectorSpec, connectorResource, docSource
- **llm/client.go**: archivo nuevo
- **auditdata/auditdata.go**: funciones: ParseFindings, ParseDataset
- **llm/client.go**: archivo nuevo
- **frameworkmensajero/main.go**: funciones: subjectWithDevPrefix
- **llm/client.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **servidor-rpc/servidor.go**: funciones: main | tipos: Servicio
- **frameworkradar/main.go**: funciones: runConfigureAnalysis, scoreSQLite, fetchPaymentStats, findPaymentTable | tipos: paymentStats, analysisPlanPaths
- **llm/client.go**: funciones: (c *Client) generateOAICompat
- **sqlqa/sqlqa.go**: tipos: relation
- **llm/client.go**: archivo nuevo
- **remora/main.go**: +8 / -8
- **orchestrator/main.go**: +2 / -1
- **internal/whitelist.go**: +37 / -34
- **checks/sqlite.go**: archivo nuevo
- **frameworkcontactos/main.go**: -432
- **deployer/main.go**: funciones: printJSON, exitErr, hasFlag, flagValue
- **deployer/deploy.go**: -180
- **framework-pingpong/palindrome.go**: funciones: palindromeDemo
- **framework-pingpong/roman_to_integer.go**: funciones: romanToIntegerDemo
- **servidor-rpc/client.go**: +2 / -6
- **servidor-rpc/cliente.go**: archivo nuevo
- **framework-pingpong/two_sum.go**: funciones: twoSumDemo, twoSum
- **manifest/manifest_test.go**: archivo nuevo
- **checks/sqlite_test.go**: archivo nuevo
- **frameworkauditor/main_test.go**: archivo nuevo
- **deployer/runbook_test.go**: archivo nuevo
- **foco/main_test.go**: archivo nuevo
- **frameworkradar/main_test.go**: funciones: TestPersistAnalysisPlanWritesTangibleJSONAndSQL, TestLoadPersistedAnalysisPlanReusesConfiguredModel
- **frameworksabio/main_test.go**: archivo nuevo
- **framework-sabio/qa_fixture_test.go**: archivo nuevo

- **framework-charlie/.charlieignore**: +6 / -1
- **framework-charlie/framework.manifest.json**: archivo nuevo
- **framework-charlie/WHY.md**: archivo nuevo
- **charlie/charlie.go**: +2 / -2
- **llm/client.go**: archivo nuevo
- **paladin-server**: servidor HTTP para recibir traces (server/main.go)
- **framework-paladin/framework.manifest.json**: +6
- **llm/client.go**: archivo nuevo
- **paladin/lint.go**: funciones: lintManifestCapabilities, hasPolicy, capabilityUsesMultipleEngines, capabilityLooksGrounded | tipos: lintCapability
- **paladin/lint_test.go**: funciones: TestLintManifestsCatchesMissingTypedCapabilities, TestLintManifestsValidatesTypedCapabilityContract
- **framework-quine/framework.manifest.json**: archivo nuevo
- **framework-quine/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **review/review.go**: +2 / -2
- **types/types.go**: +1 / -1
- **api_rest/.dockerignore**: +31
- **api_rest/Dockerfile**: -3
- **api_rest/Dockerfile.dev**: +31
- **api_rest/deploy.sh**: +111
- **api_rest/entrypoint.sh**: +76
- **api_rest/flow.rules.json**: +72
- **api_rest/flujo_api**: configuracion
- **static/data.html**: archivo nuevo
- **static/index.html**: archivo nuevo
- **flujo_api/Dockerfile**: -84
- **static/index.html**: -3492
- **remora-flujo/framework_session**: archivo nuevo
- **frontend-chat/index.html**: +469 / -6
- **remora-flujo/go.mod**: +25 / -1
- **remora-flujo/go.sum**: +67
- **api_rest/active_task.go**: funciones: activeTaskContext, invalidateActiveTaskCache, tareasBinPath, buildActiveTaskLine | tipos: ActiveTask
- **api_rest/api_connections.go**: archivo nuevo
- **api_rest/auth.go**: funciones: defaultAuthDBPath
- **api_rest/auth_handlers.go**: archivo nuevo
- **api_rest/business_artifacts.go**: archivo nuevo
- **api_rest/contactos.go**: archivo nuevo
- **api_rest/data_browser.go**: +4 / -1
- **api_rest/drivers.go**: funciones: keysOfManifests, initDriverRegistry, keysOf, driversFor | tipos: FrameworkDriver, QueuedAnswerCtx, nextQuestionResponse
- **api_rest/flow_backend.go**: archivo nuevo
- **api_rest/flow_run.go**: funciones: (s *server) runFlowStream, (s *server) runFlowManifest, resetCycleArtifacts, (s *server) shouldSkipInstalledAnalysis | tipos: flowBranchRun, flowRequiredInput, flowInputField
- **api_rest/flow_smtp.go**: archivo nuevo
- **api_rest/flow_store.go**: archivo nuevo
- **api_rest/flow_suggest.go**: archivo nuevo
- **api_rest/framework_session.go**: archivo nuevo
- **api_rest/generic_driver.go**: funciones: newGenericDriver, (g *genericDriver) Name, (g *genericDriver) fullArgs, (g *genericDriver) resolveCommandArgs | tipos: genericDriver, historyTurn
- **api_rest/main.go**: funciones: getRuntimeInfo, main, envOr, loadDotEnv | tipos: APIResponse, fwInfo, server
- **api_rest/multimodal.go**: funciones: loadFrameworkManifest, modelSpecFor, preprocessVision, appendImageAnalysis | tipos: entry
- **api_rest/orchestrator.go**: funciones: runLoop, driverNames, truncate, hasImageResource | tipos: fullPoller
- **api_rest/rules.go**: funciones: loadFlowRules, (fr *FlowRules) Match, condMatches, reorderDrivers | tipos: FlowRules, FlowRule, FlowCondition
- **api_rest/single_wrapper.go**: funciones: (s *server) createUniversalSingleMessage, (s *server) runUniversalSingle, selectUniversalCommand, universalCommandScore | tipos: universalSingleResult
- **api_rest/store.go**: funciones: convPath, metaPath, messagesPath, queuePath | tipos: Conversation, Message, MessageArtifact
- **api_rest/streaming.go**: funciones: wantsSSE, newSSEWriter, (s *sseWriter) emit, liveFilePath | tipos: sseWriter
- **api_rest/tareas.go**: funciones: resolveTareas, runTareas, currentProfile, emitTaskEvent | tipos: createTaskReq, taskEventReq
- **api_rest/traces.go**: funciones: (s *server) handleTracesLatest | tipos: entry
- **framework_session/main.go**: archivo nuevo
- **llm/client.go**: funciones: (g *groqClient) Stream, (m *minimaxClient) Stream, handleMiniStreamData | tipos: StreamEvent
- **nativeagent/agent.go**: +43 / -8
- **api_rest/email_sanitize.go**: funciones: dedupeSubjectPrefix, unresolvedPlaceholders
- **api_rest/initial_prompt.go**: archivo nuevo
- **api_rest/intent.go**: funciones: classifyIntent, providerOfModelCapability, providerOfProducedCapability
- **flujo_api/contactos.go**: -64
- **api_rest/auth_test.go**: archivo nuevo
- **api_rest/business_artifacts_test.go**: archivo nuevo
- **api_rest/data_browser_test.go**: archivo nuevo
- **api_rest/flow_backend_test.go**: funciones: TestPrepareFlowManifestLifecyclePromotesPriorityListBeforeFoco, TestValidateFlowManifestMissingRequirementAllowsRuntimeCredentials, TestValidateFlowManifestUsesMecanicoForDraftArtifact, TestNormalizeFlowLifecycleRoles
- **api_rest/flow_run_test.go**: funciones: TestRunFlowManifestDryRunExecutesSafeNodesAndStopsBeforeSideEffect, TestSummarizeAuditorGapsCompactsMissingContactsAndCounts, TestRunFlowManifestTestModeExecutesSideEffectAgainstTestRecipient, TestRunFlowManifestUsesInteractiveModeForApprovalPolicy
- **api_rest/root_test.go**: archivo nuevo
- **api_rest/single_wrapper_test.go**: funciones: TestUniversalSingleMessageExposesStandardSession, TestEncodeConversationRuntimeContextIncludesBusinessAndScope, TestSelectUniversalCommandPrefersInputCommand, TestInferUniversalParamsFromFreeTextAndJSON
- **flujo_api/root_test.go**: -36
- **nativeagent/agent_test.go**: funciones: TestResolveProviderPrefersOpenRouter, TestResolveProviderExplicitOpenRouter
- **framework-echo/framework.manifest.json**: +4 / -5
- **framework-echo/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **framework-echo/main.go**: +4 / -5
- **frameworkecho/automation.go**: funciones: generateEchoQuestion, buildTreeContext
- **framework-gmail/framework.manifest.json**: archivo nuevo
- **framework-gmail/WHY.md**: archivo nuevo
- **framework-gmail/client.go**: +1 / -4
- **framework-gmail/types.go**: +1 / -1
- **llm/client.go**: archivo nuevo
- **framework-gmail/main.go**: funciones: printJSON
- **framework-alfa/framework.manifest.json**: +1 / -2
- **framework-alfa/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **frameworkalfa/automation.go**: funciones: generateAlfaQuestion
- **prospectos-seguimiento/go.mod**: +5 / -1
- **framework-bravo/framework.manifest.json**: archivo nuevo
- **framework-bravo/go.mod**: +2 / -2
- **framework-bravo/WHY.md**: archivo nuevo
- **bravo/trace.go**: +1 / -1
- **prospectos-seguimiento/main.go**: +2 / -2
- **llm/client.go**: archivo nuevo
- **framework-excel/framework.manifest.json**: archivo nuevo
- **framework-excel/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **main-multi-modo**: main.go soporta multiples modos (audit, explain, tree) (frameworkauditor/main.go, frameworkmecanico/main.go, frameworksabio/main.go)
- **.env.example**: +12 / -5
- **.gitignore**: +3 / -4
- **CHANGELOG.md**: +464
- **Makefile**: +9 / -9
- **cloudbuild.yaml**: +48 / -9
- **docker-compose.yml**: +5 / -5
- **framework-arquitecto/framework.manifest.json**: +4 / -6
- **framework-arquitecto/frameworkarquitecto**: configuracion
- **data/findings.json**: +65135 / -259
- **framework-auditor/framework.manifest.json**: +106 / -22
- **framework-auditor/go.mod**: +14 / -1
- **framework-auditor/go.sum**: archivo nuevo
- **framework-critico/framework.manifest.json**: +4 / -5
- **framework-critico/frameworkcritico**: configuracion
- **framework-deployer/framework.manifest.json**: +6
- **framework-foco/framework.manifest.json**: +356 / -18
- **framework-foco/frameworkfoco**: archivo nuevo
- **framework-foco/go.mod**: -17
- **framework-foco/go.sum**: -51
- **framework-hosting/framework.manifest.json**: +336 / -31
- **framework-hosting/frameworkhosting**: configuracion
- **data/dump.json**: +1 / -1
- **data/sync_meta.json**: +3 / -3
- **framework-indexa/framework.manifest.json**: +9 / -4
- **data/applied.jsonl**: +8956 / -13
- **data/proposals.json**: +1 / -1
- **framework-mecanico/framework.manifest.json**: +349 / -33
- **framework-mensajero/framework.manifest.json**: +156 / -20
- **framework-mensajero/frameworkmensajero**: configuracion
- **framework-pingpong/framework.manifest.json**: archivo nuevo
- **servidor-rpc/pingpong_progress.json**: +3 / -2
- **framework-radar/framework.manifest.json**: +82 / -14
- **framework-radar/frameworkradar**: configuracion
- **panalbit/sabio.business.json**: archivo nuevo
- **data/qa_cobranza_chile_ideal.json**: archivo nuevo
- **framework-sabio/framework.manifest.json**: +609 / -25
- **semantic/catalog.json**: archivo nuevo
- **semantic/profile.json**: archivo nuevo
- **semantic/relationships.mmd**: archivo nuevo
- **semantic/relationships_full.mmd**: archivo nuevo
- **semantic/views.sql**: archivo nuevo
- **framework-tareas/framework.manifest.json**: +4 / -2
- **scripts/bootstrap.sh**: +1 / -1
- **scripts/demo_aceleradora.sh**: +1 / -1
- **scripts/dev-local.sh**: +4 / -4
- **scripts/install-remora.sh**: +2 / -2
- **scripts/smoke_test_api_rest.sh**: +106
- **.claude/CLAUDE.md**: archivo nuevo
- **.codex/instructions.md**: archivo nuevo
- **ARCHITECTURE.md**: +7 / -5
- **HANDOFF_PROMPT.md**: +13 / -14
- **README.md**: +3 / -3
- **SKILL_COLABORACION.md**: archivo nuevo
- **docs/AXIOMS.md**: archivo nuevo
- **docs/CAPABILITIES.md**: +1 / -1
- **framework-arquitecto/WHY.md**: archivo nuevo
- **framework-auditor/INITIAL_PROMPT.md**: archivo nuevo
- **framework-auditor/WHY.md**: archivo nuevo
- **framework-critico/WHY.md**: archivo nuevo
- **framework-deployer/README.md**: +55 / -28
- **framework-deployer/WHY.md**: archivo nuevo
- **framework-hosting/INITIAL_PROMPT.md**: archivo nuevo
- **framework-hosting/WHY.md**: archivo nuevo
- **framework-indexa/INITIAL_PROMPT.md**: archivo nuevo
- **framework-indexa/WHY.md**: archivo nuevo
- **framework-mecanico/INITIAL_PROMPT.md**: archivo nuevo
- **framework-mecanico/WHY.md**: archivo nuevo
- **framework-mensajero/INITIAL_PROMPT.md**: archivo nuevo
- **framework-mensajero/WHY.md**: archivo nuevo
- **framework-sabio/AXIOMS.md**: archivo nuevo
- **framework-sabio/INITIAL_PROMPT.md**: archivo nuevo
- **framework-sabio/WHY.md**: archivo nuevo
- **semantic/profile.md**: archivo nuevo
- **framework-tareas/INITIAL_PROMPT.md**: archivo nuevo
- **framework-tareas/WHY.md**: archivo nuevo
- **adapter/adapter.go**: funciones: (c *Client) Grep, (c *Client) Find, (c *Client) EditFile
- **channel/main.go**: funciones: loadDotEnv
- **internal/handler.go**: funciones: (h *Handler) grep, (h *Handler) find, (h *Handler) editFile, intParam
- **internal/jsonrpc.go**: +6 / -2
- **manifest/manifest.go**: tipos: CapabilitySpec
- **frameworkarquitecto/llm.go**: +22 / -11
- **frameworkarquitecto/llm_stream.go**: +13 / -4
- **llm/client.go**: archivo nuevo
- **checks/checks.go**: funciones: InferFKRelations, InferRequiredStringFields, InferRequiredNonNullFields, InferDateFields | tipos: EndpointField, FKRelation
- **llm/client.go**: archivo nuevo
- **frameworkcritico/main.go**: funciones: resolveCriticoProvider, evaluateWithLLM, firstNonEmpty, loadEnvFiles | tipos: msg
- **llm/client.go**: archivo nuevo
- **deployer/config.go**: archivo nuevo
- **deployer/diagnose.go**: archivo nuevo
- **deployer/runbook.go**: archivo nuevo
- **deployer/runner.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **foco/cobranza_sql.go**: +1 / -137
- **foco/cobranza_tasks.go**: +1 / -1
- **foco/main.go**: funciones: interpretFocoInput, runSessionStart, sessionStartSystem, runPriorities | tipos: sessionStartEvent, sessionStartResponse, priorityCandidate
- **llm/client.go**: funciones: (c *Client) Provider, (c *Client) Model, (c *Client) generateOAICompat
- **frameworkhosting/main.go**: funciones: dispatchIntent, handleConnectWizard, sanitizeHostAnswer, doConnectFromText | tipos: cpanelDiscoveryResp, cpanelCandidateResult
- **cpanel/client.go**: funciones: normalizeHost, isPlaceholderHost
- **cpanel/domain.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **frameworkindexa/main.go**: funciones: cmdAPIPlan, planConnectorWithLLM, connectorPlanFromOpenAPI, extractJSONObjectStrings | tipos: connectorSpec, connectorResource, docSource
- **llm/client.go**: archivo nuevo
- **auditdata/auditdata.go**: funciones: ParseFindings, ParseDataset
- **llm/client.go**: archivo nuevo
- **frameworkmensajero/main.go**: funciones: subjectWithDevPrefix
- **llm/client.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **servidor-rpc/servidor.go**: funciones: main | tipos: Servicio
- **frameworkradar/main.go**: funciones: runConfigureAnalysis, scoreSQLite, fetchPaymentStats, findPaymentTable | tipos: paymentStats, analysisPlanPaths
- **llm/client.go**: funciones: (c *Client) generateOAICompat
- **sqlqa/sqlqa.go**: tipos: relation
- **llm/client.go**: archivo nuevo
- **remora/main.go**: +8 / -8
- **orchestrator/main.go**: +2 / -1
- **internal/whitelist.go**: +37 / -34
- **checks/sqlite.go**: archivo nuevo
- **deployer/main.go**: funciones: printJSON, exitErr, hasFlag, flagValue
- **deployer/deploy.go**: -180
- **framework-pingpong/palindrome.go**: funciones: palindromeDemo
- **framework-pingpong/roman_to_integer.go**: funciones: romanToIntegerDemo
- **servidor-rpc/client.go**: +2 / -6
- **servidor-rpc/cliente.go**: archivo nuevo
- **framework-pingpong/two_sum.go**: funciones: twoSumDemo, twoSum
- **manifest/manifest_test.go**: archivo nuevo
- **checks/sqlite_test.go**: archivo nuevo
- **frameworkauditor/main_test.go**: archivo nuevo
- **deployer/runbook_test.go**: archivo nuevo
- **foco/main_test.go**: archivo nuevo
- **frameworkradar/main_test.go**: funciones: TestPersistAnalysisPlanWritesTangibleJSONAndSQL, TestLoadPersistedAnalysisPlanReusesConfiguredModel
- **frameworksabio/main_test.go**: archivo nuevo
- **framework-sabio/qa_fixture_test.go**: archivo nuevo

- **framework-charlie/.charlieignore**: +10 / -2
- **framework-charlie/framework.manifest.json**: archivo nuevo
- **framework-charlie/WHY.md**: archivo nuevo
- **charlie/charlie.go**: +2 / -2
- **llm/client.go**: archivo nuevo
- **paladin-server**: servidor HTTP para recibir traces (server/main.go)
- **framework-paladin/framework.manifest.json**: +6
- **llm/client.go**: archivo nuevo
- **paladin/lint.go**: funciones: lintManifestCapabilities, hasPolicy, capabilityUsesMultipleEngines, capabilityLooksGrounded | tipos: lintCapability
- **paladin/lint_test.go**: funciones: TestLintManifestsCatchesMissingTypedCapabilities, TestLintManifestsValidatesTypedCapabilityContract
- **framework-quine/framework.manifest.json**: archivo nuevo
- **framework-quine/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **review/review.go**: +2 / -2
- **types/types.go**: +1 / -1
- **api_rest/.dockerignore**: +31
- **api_rest/Dockerfile**: -3
- **api_rest/Dockerfile.dev**: +31
- **api_rest/deploy.sh**: +111
- **api_rest/entrypoint.sh**: +76
- **api_rest/flow.rules.json**: +72
- **api_rest/flujo_api**: configuracion
- **static/data.html**: archivo nuevo
- **static/index.html**: archivo nuevo
- **remora-flujo/framework_session**: archivo nuevo
- **frontend-chat/index.html**: +469 / -6
- **remora-flujo/go.mod**: +25 / -1
- **remora-flujo/go.sum**: +67
- **api_rest/active_task.go**: funciones: activeTaskContext, invalidateActiveTaskCache, tareasBinPath, buildActiveTaskLine | tipos: ActiveTask
- **api_rest/api_connections.go**: archivo nuevo
- **api_rest/auth.go**: funciones: defaultAuthDBPath
- **api_rest/auth_handlers.go**: archivo nuevo
- **api_rest/business_artifacts.go**: archivo nuevo
- **api_rest/contactos.go**: archivo nuevo
- **api_rest/data_browser.go**: +4 / -1
- **api_rest/drivers.go**: funciones: keysOfManifests, initDriverRegistry, keysOf, driversFor | tipos: FrameworkDriver, QueuedAnswerCtx, nextQuestionResponse
- **api_rest/flow_backend.go**: archivo nuevo
- **api_rest/flow_run.go**: funciones: (s *server) runFlowStream, (s *server) runFlowManifest, resetCycleArtifacts, (s *server) shouldSkipInstalledAnalysis | tipos: flowBranchRun, flowRequiredInput, flowInputField
- **api_rest/flow_smtp.go**: archivo nuevo
- **api_rest/flow_store.go**: archivo nuevo
- **api_rest/flow_suggest.go**: archivo nuevo
- **api_rest/framework_session.go**: archivo nuevo
- **api_rest/generic_driver.go**: funciones: newGenericDriver, (g *genericDriver) Name, (g *genericDriver) fullArgs, (g *genericDriver) resolveCommandArgs | tipos: genericDriver, historyTurn
- **api_rest/main.go**: funciones: getRuntimeInfo, main, envOr, loadDotEnv | tipos: APIResponse, fwInfo, server
- **api_rest/multimodal.go**: funciones: loadFrameworkManifest, modelSpecFor, preprocessVision, appendImageAnalysis | tipos: entry
- **api_rest/orchestrator.go**: funciones: runLoop, driverNames, truncate, hasImageResource | tipos: fullPoller
- **api_rest/rules.go**: funciones: loadFlowRules, (fr *FlowRules) Match, condMatches, reorderDrivers | tipos: FlowRules, FlowRule, FlowCondition
- **api_rest/single_wrapper.go**: funciones: (s *server) createUniversalSingleMessage, (s *server) runUniversalSingle, selectUniversalCommand, universalCommandScore | tipos: universalSingleResult
- **api_rest/store.go**: funciones: convPath, metaPath, messagesPath, queuePath | tipos: Conversation, Message, MessageArtifact
- **api_rest/streaming.go**: funciones: wantsSSE, newSSEWriter, (s *sseWriter) emit, liveFilePath | tipos: sseWriter
- **api_rest/tareas.go**: funciones: resolveTareas, runTareas, currentProfile, emitTaskEvent | tipos: createTaskReq, taskEventReq
- **api_rest/traces.go**: funciones: (s *server) handleTracesLatest | tipos: entry
- **framework_session/main.go**: archivo nuevo
- **llm/client.go**: funciones: (g *groqClient) Stream, (m *minimaxClient) Stream, handleMiniStreamData | tipos: StreamEvent
- **nativeagent/agent.go**: +43 / -8
- **api_rest/email_sanitize.go**: funciones: dedupeSubjectPrefix, unresolvedPlaceholders
- **api_rest/initial_prompt.go**: archivo nuevo
- **api_rest/intent.go**: funciones: classifyIntent, providerOfModelCapability, providerOfProducedCapability
- **api_rest/auth_test.go**: archivo nuevo
- **api_rest/business_artifacts_test.go**: archivo nuevo
- **api_rest/data_browser_test.go**: archivo nuevo
- **api_rest/flow_backend_test.go**: funciones: TestPrepareFlowManifestLifecyclePromotesPriorityListBeforeFoco, TestValidateFlowManifestMissingRequirementAllowsRuntimeCredentials, TestValidateFlowManifestUsesMecanicoForDraftArtifact, TestNormalizeFlowLifecycleRoles
- **api_rest/flow_run_test.go**: funciones: TestRunFlowManifestDryRunExecutesSafeNodesAndStopsBeforeSideEffect, TestSummarizeAuditorGapsCompactsMissingContactsAndCounts, TestRunFlowManifestTestModeExecutesSideEffectAgainstTestRecipient, TestRunFlowManifestUsesInteractiveModeForApprovalPolicy
- **api_rest/root_test.go**: archivo nuevo
- **api_rest/single_wrapper_test.go**: funciones: TestUniversalSingleMessageExposesStandardSession, TestEncodeConversationRuntimeContextIncludesBusinessAndScope, TestSelectUniversalCommandPrefersInputCommand, TestInferUniversalParamsFromFreeTextAndJSON
- **nativeagent/agent_test.go**: funciones: TestResolveProviderPrefersOpenRouter, TestResolveProviderExplicitOpenRouter
- **framework-echo/framework.manifest.json**: +4 / -5
- **framework-echo/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **framework-echo/main.go**: +4 / -5
- **frameworkecho/automation.go**: funciones: generateEchoQuestion, buildTreeContext
- **framework-gmail/framework.manifest.json**: archivo nuevo
- **framework-gmail/WHY.md**: archivo nuevo
- **framework-gmail/client.go**: +1 / -4
- **framework-gmail/types.go**: +1 / -1
- **llm/client.go**: archivo nuevo
- **framework-gmail/main.go**: funciones: printJSON
- **framework-alfa/framework.manifest.json**: +1 / -2
- **framework-alfa/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **frameworkalfa/automation.go**: funciones: generateAlfaQuestion
- **prospectos-seguimiento/go.mod**: +5 / -1
- **framework-bravo/framework.manifest.json**: archivo nuevo
- **framework-bravo/go.mod**: +2 / -2
- **framework-bravo/WHY.md**: archivo nuevo
- **bravo/trace.go**: +1 / -1
- **prospectos-seguimiento/main.go**: +2 / -2
- **llm/client.go**: archivo nuevo
- **framework-excel/framework.manifest.json**: archivo nuevo
- **framework-excel/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **main-multi-modo**: main.go soporta multiples modos (audit, explain, tree) (frameworkauditor/main.go, frameworkmecanico/main.go, frameworksabio/main.go)
- **.env.example**: +12 / -5
- **.gitignore**: +3 / -4
- **CHANGELOG.md**: +692
- **Makefile**: +9 / -9
- **cloudbuild.yaml**: +48 / -9
- **docker-compose.yml**: +5 / -5
- **framework-arquitecto/framework.manifest.json**: +4 / -6
- **framework-arquitecto/frameworkarquitecto**: configuracion
- **data/findings.json**: +65135 / -259
- **framework-auditor/framework.manifest.json**: +106 / -22
- **framework-auditor/go.mod**: +14 / -1
- **framework-auditor/go.sum**: archivo nuevo
- **framework-critico/framework.manifest.json**: +4 / -5
- **framework-critico/frameworkcritico**: configuracion
- **framework-deployer/framework.manifest.json**: +6
- **framework-foco/framework.manifest.json**: +356 / -18
- **framework-foco/frameworkfoco**: archivo nuevo
- **framework-foco/go.mod**: -17
- **framework-foco/go.sum**: -51
- **framework-hosting/framework.manifest.json**: +336 / -31
- **framework-hosting/frameworkhosting**: configuracion
- **framework-indexa/framework.manifest.json**: +9 / -4
- **data/applied.jsonl**: +8956 / -13
- **data/proposals.json**: +1 / -1
- **framework-mecanico/framework.manifest.json**: +349 / -33
- **framework-mensajero/framework.manifest.json**: +156 / -20
- **framework-mensajero/frameworkmensajero**: configuracion
- **framework-pingpong/framework.manifest.json**: archivo nuevo
- **servidor-rpc/pingpong_progress.json**: +3 / -2
- **framework-radar/framework.manifest.json**: +82 / -14
- **framework-radar/frameworkradar**: configuracion
- **panalbit/sabio.business.json**: archivo nuevo
- **data/qa_cobranza_chile_ideal.json**: archivo nuevo
- **framework-sabio/framework.manifest.json**: +609 / -25
- **semantic/catalog.json**: archivo nuevo
- **semantic/profile.json**: archivo nuevo
- **semantic/relationships.mmd**: archivo nuevo
- **semantic/relationships_full.mmd**: archivo nuevo
- **semantic/views.sql**: archivo nuevo
- **framework-tareas/framework.manifest.json**: +4 / -2
- **scripts/bootstrap.sh**: +1 / -1
- **scripts/demo_aceleradora.sh**: +1 / -1
- **scripts/dev-local.sh**: +4 / -4
- **scripts/install-remora.sh**: +2 / -2
- **scripts/smoke_test_api_rest.sh**: +106
- **.claude/CLAUDE.md**: archivo nuevo
- **.codex/instructions.md**: archivo nuevo
- **ARCHITECTURE.md**: +7 / -5
- **HANDOFF_PROMPT.md**: +13 / -14
- **README.md**: +3 / -3
- **SKILL_COLABORACION.md**: archivo nuevo
- **docs/AXIOMS.md**: archivo nuevo
- **docs/CAPABILITIES.md**: +1 / -1
- **framework-arquitecto/WHY.md**: archivo nuevo
- **framework-auditor/INITIAL_PROMPT.md**: archivo nuevo
- **framework-auditor/WHY.md**: archivo nuevo
- **framework-critico/WHY.md**: archivo nuevo
- **framework-deployer/README.md**: +55 / -28
- **framework-deployer/WHY.md**: archivo nuevo
- **framework-hosting/INITIAL_PROMPT.md**: archivo nuevo
- **framework-hosting/WHY.md**: archivo nuevo
- **framework-indexa/INITIAL_PROMPT.md**: archivo nuevo
- **framework-indexa/WHY.md**: archivo nuevo
- **framework-mecanico/INITIAL_PROMPT.md**: archivo nuevo
- **framework-mecanico/WHY.md**: archivo nuevo
- **framework-mensajero/INITIAL_PROMPT.md**: archivo nuevo
- **framework-mensajero/WHY.md**: archivo nuevo
- **framework-sabio/AXIOMS.md**: archivo nuevo
- **framework-sabio/INITIAL_PROMPT.md**: archivo nuevo
- **framework-sabio/WHY.md**: archivo nuevo
- **semantic/profile.md**: archivo nuevo
- **framework-tareas/INITIAL_PROMPT.md**: archivo nuevo
- **framework-tareas/WHY.md**: archivo nuevo
- **adapter/adapter.go**: funciones: (c *Client) Grep, (c *Client) Find, (c *Client) EditFile
- **channel/main.go**: funciones: loadDotEnv
- **internal/handler.go**: funciones: (h *Handler) grep, (h *Handler) find, (h *Handler) editFile, intParam
- **internal/jsonrpc.go**: +6 / -2
- **manifest/manifest.go**: tipos: CapabilitySpec
- **frameworkarquitecto/llm.go**: +22 / -11
- **frameworkarquitecto/llm_stream.go**: +13 / -4
- **llm/client.go**: archivo nuevo
- **checks/checks.go**: funciones: InferFKRelations, InferRequiredStringFields, InferRequiredNonNullFields, InferDateFields | tipos: EndpointField, FKRelation
- **llm/client.go**: archivo nuevo
- **frameworkcritico/main.go**: funciones: resolveCriticoProvider, evaluateWithLLM, firstNonEmpty, loadEnvFiles | tipos: msg
- **llm/client.go**: archivo nuevo
- **deployer/config.go**: archivo nuevo
- **deployer/diagnose.go**: archivo nuevo
- **deployer/runbook.go**: archivo nuevo
- **deployer/runner.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **foco/cobranza_sql.go**: +1 / -137
- **foco/cobranza_tasks.go**: +1 / -1
- **foco/main.go**: funciones: interpretFocoInput, runSessionStart, sessionStartSystem, runPriorities | tipos: sessionStartEvent, sessionStartResponse, priorityCandidate
- **llm/client.go**: funciones: (c *Client) Provider, (c *Client) Model, (c *Client) generateOAICompat
- **frameworkhosting/main.go**: funciones: dispatchIntent, handleConnectWizard, sanitizeHostAnswer, doConnectFromText | tipos: cpanelDiscoveryResp, cpanelCandidateResult
- **cpanel/client.go**: funciones: normalizeHost, isPlaceholderHost
- **cpanel/domain.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **frameworkindexa/main.go**: funciones: cmdAPIPlan, planConnectorWithLLM, connectorPlanFromOpenAPI, extractJSONObjectStrings | tipos: connectorSpec, connectorResource, docSource
- **llm/client.go**: archivo nuevo
- **auditdata/auditdata.go**: funciones: ParseFindings, ParseDataset
- **llm/client.go**: archivo nuevo
- **frameworkmensajero/main.go**: funciones: subjectWithDevPrefix
- **llm/client.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **servidor-rpc/servidor.go**: funciones: main | tipos: Servicio
- **frameworkradar/main.go**: funciones: runConfigureAnalysis, scoreSQLite, fetchPaymentStats, findPaymentTable | tipos: paymentStats, analysisPlanPaths
- **llm/client.go**: funciones: (c *Client) generateOAICompat
- **sqlqa/sqlqa.go**: tipos: relation
- **llm/client.go**: archivo nuevo
- **remora/main.go**: +8 / -8
- **orchestrator/main.go**: +2 / -1
- **internal/whitelist.go**: +37 / -34
- **checks/sqlite.go**: archivo nuevo
- **deployer/main.go**: funciones: printJSON, exitErr, hasFlag, flagValue
- **deployer/deploy.go**: -180
- **framework-pingpong/palindrome.go**: funciones: palindromeDemo
- **framework-pingpong/roman_to_integer.go**: funciones: romanToIntegerDemo
- **servidor-rpc/client.go**: +2 / -6
- **servidor-rpc/cliente.go**: archivo nuevo
- **framework-pingpong/two_sum.go**: funciones: twoSumDemo, twoSum
- **manifest/manifest_test.go**: archivo nuevo
- **checks/sqlite_test.go**: archivo nuevo
- **frameworkauditor/main_test.go**: archivo nuevo
- **deployer/runbook_test.go**: archivo nuevo
- **foco/main_test.go**: archivo nuevo
- **frameworkradar/main_test.go**: funciones: TestPersistAnalysisPlanWritesTangibleJSONAndSQL, TestLoadPersistedAnalysisPlanReusesConfiguredModel
- **frameworksabio/main_test.go**: archivo nuevo
- **framework-sabio/qa_fixture_test.go**: archivo nuevo

### Paladin

- **paladin-server**: servidor HTTP para recibir traces (server/main.go)
- **framework-paladin/framework.manifest.json**: +6
- **llm/client.go**: archivo nuevo
- **paladin/lint.go**: funciones: lintManifestCapabilities, hasPolicy, capabilityUsesMultipleEngines, capabilityLooksGrounded | tipos: lintCapability
- **paladin/lint_test.go**: funciones: TestLintManifestsCatchesMissingTypedCapabilities, TestLintManifestsValidatesTypedCapabilityContract

### Quine

- **framework-quine/framework.manifest.json**: archivo nuevo
- **framework-quine/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **review/review.go**: +2 / -2
- **types/types.go**: +1 / -1

### Flujo

- **api_rest/.dockerignore**: archivo nuevo
- **api_rest/Dockerfile**: -3
- **api_rest/Dockerfile.dev**: archivo nuevo
- **api_rest/deploy.sh**: archivo nuevo
- **api_rest/entrypoint.sh**: archivo nuevo
- **api_rest/flow.rules.json**: archivo nuevo
- **api_rest/flujo_api**: archivo nuevo
- **static/data.html**: archivo nuevo
- **static/index.html**: archivo nuevo
- **flujo_api/.dockerignore**: -31
- **flujo_api/Dockerfile**: -84
- **flujo_api/Dockerfile.dev**: -31
- **flujo_api/deploy.sh**: -111
- **flujo_api/entrypoint.sh**: -76
- **flujo_api/flow.rules.json**: -72
- **static/index.html**: -3492
- **remora-flujo/framework_session**: archivo nuevo
- **frontend-chat/index.html**: +469 / -6
- **remora-flujo/go.mod**: +25 / -1
- **remora-flujo/go.sum**: +67
- **api_rest/active_task.go**: archivo nuevo
- **api_rest/api_connections.go**: archivo nuevo
- **api_rest/auth.go**: funciones: defaultAuthDBPath
- **api_rest/auth_handlers.go**: archivo nuevo
- **api_rest/business_artifacts.go**: archivo nuevo
- **api_rest/contactos.go**: archivo nuevo
- **api_rest/data_browser.go**: +4 / -1
- **api_rest/drivers.go**: archivo nuevo
- **api_rest/flow_backend.go**: archivo nuevo
- **api_rest/flow_run.go**: funciones: (s *server) runFlowStream, (s *server) runFlowManifest, resetCycleArtifacts, (s *server) shouldSkipInstalledAnalysis | tipos: flowBranchRun, flowRequiredInput, flowInputField
- **api_rest/flow_smtp.go**: archivo nuevo
- **api_rest/flow_store.go**: archivo nuevo
- **api_rest/flow_suggest.go**: archivo nuevo
- **api_rest/framework_session.go**: archivo nuevo
- **api_rest/generic_driver.go**: archivo nuevo
- **api_rest/main.go**: archivo nuevo
- **api_rest/multimodal.go**: archivo nuevo
- **api_rest/orchestrator.go**: archivo nuevo
- **api_rest/rules.go**: archivo nuevo
- **api_rest/single_wrapper.go**: archivo nuevo
- **api_rest/store.go**: archivo nuevo
- **api_rest/streaming.go**: archivo nuevo
- **api_rest/tareas.go**: archivo nuevo
- **api_rest/traces.go**: archivo nuevo
- **framework_session/main.go**: archivo nuevo
- **llm/client.go**: funciones: (g *groqClient) Stream, (m *minimaxClient) Stream, handleMiniStreamData | tipos: StreamEvent
- **nativeagent/agent.go**: +43 / -8
- **api_rest/email_sanitize.go**: archivo nuevo
- **api_rest/initial_prompt.go**: archivo nuevo
- **api_rest/intent.go**: archivo nuevo
- **flujo_api/active_task.go**: -156
- **flujo_api/contactos.go**: -64
- **flujo_api/drivers.go**: -356
- **flujo_api/email_sanitize.go**: -89
- **flujo_api/generic_driver.go**: -258
- **flujo_api/intent.go**: -116
- **flujo_api/main.go**: -1599
- **flujo_api/multimodal.go**: -175
- **flujo_api/orchestrator.go**: -310
- **flujo_api/rules.go**: -205
- **flujo_api/single_wrapper.go**: -288
- **flujo_api/store.go**: -229
- **flujo_api/streaming.go**: -170
- **flujo_api/tareas.go**: -175
- **flujo_api/traces.go**: -84
- **api_rest/auth_test.go**: archivo nuevo
- **api_rest/business_artifacts_test.go**: archivo nuevo
- **api_rest/data_browser_test.go**: archivo nuevo
- **api_rest/flow_backend_test.go**: funciones: TestPrepareFlowManifestLifecyclePromotesPriorityListBeforeFoco, TestValidateFlowManifestMissingRequirementAllowsRuntimeCredentials, TestValidateFlowManifestUsesMecanicoForDraftArtifact, TestNormalizeFlowLifecycleRoles
- **api_rest/flow_run_test.go**: funciones: TestRunFlowManifestDryRunExecutesSafeNodesAndStopsBeforeSideEffect, TestSummarizeAuditorGapsCompactsMissingContactsAndCounts, TestRunFlowManifestTestModeExecutesSideEffectAgainstTestRecipient, TestRunFlowManifestUsesInteractiveModeForApprovalPolicy
- **api_rest/root_test.go**: archivo nuevo
- **api_rest/single_wrapper_test.go**: archivo nuevo
- **flujo_api/root_test.go**: -36
- **flujo_api/single_wrapper_test.go**: -72
- **nativeagent/agent_test.go**: funciones: TestResolveProviderPrefersOpenRouter, TestResolveProviderExplicitOpenRouter

### Echo

- **framework-echo/framework.manifest.json**: +4 / -5
- **framework-echo/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **framework-echo/main.go**: +4 / -5
- **frameworkecho/automation.go**: funciones: generateEchoQuestion, buildTreeContext

### Gmail

- **framework-gmail/framework.manifest.json**: archivo nuevo
- **framework-gmail/WHY.md**: archivo nuevo
- **framework-gmail/client.go**: +1 / -4
- **framework-gmail/types.go**: +1 / -1
- **llm/client.go**: archivo nuevo
- **framework-gmail/main.go**: funciones: printJSON

### Alfa

- **framework-alfa/framework.manifest.json**: +1 / -2
- **framework-alfa/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **frameworkalfa/automation.go**: funciones: generateAlfaQuestion

### Bravo

- **prospectos-seguimiento/go.mod**: +5 / -1
- **framework-bravo/framework.manifest.json**: archivo nuevo
- **framework-bravo/go.mod**: +2 / -2
- **framework-bravo/WHY.md**: archivo nuevo
- **bravo/trace.go**: +1 / -1
- **prospectos-seguimiento/main.go**: +2 / -2
- **llm/client.go**: archivo nuevo

### Excel

- **framework-excel/framework.manifest.json**: archivo nuevo
- **framework-excel/WHY.md**: archivo nuevo
- **llm/client.go**: archivo nuevo

### Repo

- **main-multi-modo**: main.go soporta multiples modos (audit, explain, tree) (frameworkauditor/main.go, frameworkmecanico/main.go, frameworksabio/main.go)
- **.env.example**: +12 / -5
- **.gitignore**: +3 / -4
- **CHANGELOG.md**: +28 / -28
- **Makefile**: +9 / -9
- **cloudbuild.yaml**: +48 / -9
- **docker-compose.yml**: +5 / -5
- **framework-arquitecto/framework.manifest.json**: +4 / -6
- **framework-arquitecto/frameworkarquitecto**: configuracion
- **data/findings.json**: +65135 / -259
- **framework-auditor/framework.manifest.json**: +106 / -22
- **framework-auditor/go.mod**: +14 / -1
- **framework-auditor/go.sum**: archivo nuevo
- **framework-contactos/framework.manifest.json**: -74
- **framework-contactos/frameworkcontactos**: configuracion
- **framework-contactos/go.mod**: -17
- **framework-contactos/go.sum**: -43
- **framework-critico/framework.manifest.json**: +4 / -5
- **framework-critico/frameworkcritico**: configuracion
- **framework-deployer/framework.manifest.json**: +6
- **framework-foco/framework.manifest.json**: +356 / -18
- **framework-foco/frameworkfoco**: archivo nuevo
- **framework-foco/go.mod**: -17
- **framework-foco/go.sum**: -51
- **framework-hosting/framework.manifest.json**: +336 / -31
- **framework-hosting/frameworkhosting**: configuracion
- **data/dump.json**: +1 / -1
- **data/sync_meta.json**: +3 / -3
- **framework-indexa/framework.manifest.json**: +9 / -4
- **data/applied.jsonl**: +8956 / -13
- **data/proposals.json**: +1 / -1
- **framework-mecanico/framework.manifest.json**: +349 / -33
- **framework-mensajero/framework.manifest.json**: +156 / -20
- **framework-mensajero/frameworkmensajero**: configuracion
- **framework-pingpong/framework.manifest.json**: archivo nuevo
- **servidor-rpc/pingpong_progress.json**: +3 / -2
- **framework-radar/framework.manifest.json**: +82 / -14
- **framework-radar/frameworkradar**: configuracion
- **panalbit/sabio.business.json**: archivo nuevo
- **data/qa_cobranza_chile_ideal.json**: archivo nuevo
- **framework-sabio/framework.manifest.json**: +609 / -25
- **semantic/catalog.json**: archivo nuevo
- **semantic/profile.json**: archivo nuevo
- **semantic/relationships.mmd**: archivo nuevo
- **semantic/relationships_full.mmd**: archivo nuevo
- **semantic/views.sql**: archivo nuevo
- **framework-tareas/framework.manifest.json**: +4 / -2
- **scripts/bootstrap.sh**: +1 / -1
- **scripts/demo_aceleradora.sh**: +1 / -1
- **scripts/dev-local.sh**: +4 / -4
- **scripts/install-remora.sh**: +2 / -2
- **scripts/smoke_test_api_rest.sh**: archivo nuevo
- **scripts/smoke_test_flujo_api.sh**: -106
- **tmp/channel.log**: archivo nuevo
- **tmp/channel.pid**: archivo nuevo
- **tmp/flujo_api.log**: archivo nuevo
- **tmp/flujo_api.pid**: archivo nuevo
- **.claude/CLAUDE.md**: archivo nuevo
- **.codex/instructions.md**: archivo nuevo
- **ARCHITECTURE.md**: +7 / -5
- **HANDOFF_PROMPT.md**: +13 / -14
- **README.md**: +3 / -3
- **SKILL_COLABORACION.md**: archivo nuevo
- **docs/AXIOMS.md**: archivo nuevo
- **docs/CAPABILITIES.md**: +1 / -1
- **framework-arquitecto/WHY.md**: archivo nuevo
- **framework-auditor/INITIAL_PROMPT.md**: archivo nuevo
- **framework-auditor/WHY.md**: archivo nuevo
- **framework-critico/WHY.md**: archivo nuevo
- **framework-deployer/README.md**: +55 / -28
- **framework-deployer/WHY.md**: archivo nuevo
- **framework-hosting/INITIAL_PROMPT.md**: archivo nuevo
- **framework-hosting/WHY.md**: archivo nuevo
- **framework-indexa/INITIAL_PROMPT.md**: archivo nuevo
- **framework-indexa/WHY.md**: archivo nuevo
- **framework-mecanico/INITIAL_PROMPT.md**: archivo nuevo
- **framework-mecanico/WHY.md**: archivo nuevo
- **framework-mensajero/INITIAL_PROMPT.md**: archivo nuevo
- **framework-mensajero/WHY.md**: archivo nuevo
- **framework-sabio/AXIOMS.md**: archivo nuevo
- **framework-sabio/INITIAL_PROMPT.md**: archivo nuevo
- **framework-sabio/WHY.md**: archivo nuevo
- **semantic/profile.md**: archivo nuevo
- **framework-tareas/INITIAL_PROMPT.md**: archivo nuevo
- **framework-tareas/WHY.md**: archivo nuevo
- **adapter/adapter.go**: funciones: (c *Client) Grep, (c *Client) Find, (c *Client) EditFile
- **channel/main.go**: funciones: loadDotEnv
- **internal/handler.go**: funciones: (h *Handler) grep, (h *Handler) find, (h *Handler) editFile, intParam
- **internal/jsonrpc.go**: +6 / -2
- **manifest/manifest.go**: tipos: CapabilitySpec
- **frameworkarquitecto/llm.go**: +22 / -11
- **frameworkarquitecto/llm_stream.go**: +13 / -4
- **llm/client.go**: archivo nuevo
- **checks/checks.go**: funciones: InferFKRelations, InferRequiredStringFields, InferRequiredNonNullFields, InferDateFields | tipos: EndpointField, FKRelation
- **llm/client.go**: archivo nuevo
- **frameworkcritico/main.go**: funciones: resolveCriticoProvider, evaluateWithLLM, firstNonEmpty, loadEnvFiles | tipos: msg
- **llm/client.go**: archivo nuevo
- **deployer/config.go**: archivo nuevo
- **deployer/diagnose.go**: archivo nuevo
- **deployer/runbook.go**: archivo nuevo
- **deployer/runner.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **foco/cobranza_sql.go**: +1 / -137
- **foco/cobranza_tasks.go**: +1 / -1
- **foco/main.go**: funciones: interpretFocoInput, runSessionStart, sessionStartSystem, runPriorities | tipos: sessionStartEvent, sessionStartResponse, priorityCandidate
- **llm/client.go**: funciones: (c *Client) Provider, (c *Client) Model, (c *Client) generateOAICompat
- **frameworkhosting/main.go**: funciones: dispatchIntent, handleConnectWizard, sanitizeHostAnswer, doConnectFromText | tipos: cpanelDiscoveryResp, cpanelCandidateResult
- **cpanel/client.go**: funciones: normalizeHost, isPlaceholderHost
- **cpanel/domain.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **frameworkindexa/main.go**: funciones: cmdAPIPlan, planConnectorWithLLM, connectorPlanFromOpenAPI, extractJSONObjectStrings | tipos: connectorSpec, connectorResource, docSource
- **llm/client.go**: archivo nuevo
- **auditdata/auditdata.go**: funciones: ParseFindings, ParseDataset
- **llm/client.go**: archivo nuevo
- **frameworkmensajero/main.go**: funciones: subjectWithDevPrefix
- **llm/client.go**: archivo nuevo
- **llm/client.go**: archivo nuevo
- **servidor-rpc/servidor.go**: funciones: main | tipos: Servicio
- **frameworkradar/main.go**: funciones: runConfigureAnalysis, scoreSQLite, fetchPaymentStats, findPaymentTable | tipos: paymentStats, analysisPlanPaths
- **llm/client.go**: funciones: (c *Client) generateOAICompat
- **sqlqa/sqlqa.go**: tipos: relation
- **llm/client.go**: archivo nuevo
- **remora/main.go**: +8 / -8
- **orchestrator/main.go**: +2 / -1
- **internal/whitelist.go**: +37 / -34
- **checks/sqlite.go**: archivo nuevo
- **frameworkcontactos/main.go**: -432
- **deployer/main.go**: funciones: printJSON, exitErr, hasFlag, flagValue
- **deployer/deploy.go**: -180
- **framework-pingpong/palindrome.go**: funciones: palindromeDemo
- **framework-pingpong/roman_to_integer.go**: funciones: romanToIntegerDemo
- **servidor-rpc/client.go**: +2 / -6
- **servidor-rpc/cliente.go**: archivo nuevo
- **framework-pingpong/two_sum.go**: funciones: twoSumDemo, twoSum
- **manifest/manifest_test.go**: archivo nuevo
- **checks/sqlite_test.go**: archivo nuevo
- **frameworkauditor/main_test.go**: archivo nuevo
- **deployer/runbook_test.go**: archivo nuevo
- **foco/main_test.go**: archivo nuevo
- **frameworkradar/main_test.go**: funciones: TestPersistAnalysisPlanWritesTangibleJSONAndSQL, TestLoadPersistedAnalysisPlanReusesConfiguredModel
- **frameworksabio/main_test.go**: archivo nuevo
- **framework-sabio/qa_fixture_test.go**: archivo nuevo

## [0.1.20] - 2026-05-06

> **Release**: expandir flujo, paladin

### Paladin

- **paladin/lint.go**: +10
- **paladin/lint_test.go**: funciones: TestLintLocalIntegrationCatchesWorkspaceRootDefaults, main, TestLintLocalIntegrationCatchesHardcodedWorkspaceDrivers, run

### Flujo

- **api_rest/drivers.go**: +30 / -14
- **api_rest/main.go**: funciones: resolveRemoraRoot, findRemoraRoot, looksLikeRemoraRoot, singleNoQuestionMessage
- **api_rest/root_test.go**: archivo nuevo

## [0.1.19] - 2026-05-06

> **Release**: expandir flujo, paladin

### Paladin

- **paladin/lint.go**: archivo nuevo
- **paladin/main.go**: funciones: hasLintFailures
- **paladin/lint_test.go**: archivo nuevo

### Flujo

- **cola-preguntas**: sistema de cola de preguntas para control de turnos (handoff/questions_queue.go)
- **api_rest/Dockerfile.dev**: archivo nuevo
- **api_rest/flow.rules.json**: +1 / -1
- **static/index.html**: +776 / -408
- **frontend-chat/index.html**: +643 / -19
- **api_rest/drivers.go**: funciones: (e *echoDriver) PollQuestion, (a *alfaDriver) PollQuestion
- **api_rest/generic_driver.go**: funciones: (g *genericDriver) resolveCommandArgs, (g *genericDriver) PollQuestion
- **api_rest/main.go**: funciones: (s *server) collectFrameworkInfos, (s *server) listTestableFrameworks, (s *server) listChainableFrameworks, (s *server) listFrameworksFiltered | tipos: fwInfo, runFrameworkCommandRequest
- **api_rest/orchestrator.go**: +24 / -4
- **api_rest/rules.go**: +37 / -4
- **api_rest/single_wrapper.go**: archivo nuevo
- **api_rest/store.go**: funciones: convPath, metaPath, messagesPath, queuePath | tipos: MessageArtifact, MessageEvent
- **api_rest/intent.go**: funciones: providerOfProducedCapability
- **api_rest/single_wrapper_test.go**: archivo nuevo

### Repo

- **main-multi-modo**: main.go soporta multiples modos (audit, explain, tree) (frameworkmecanico/main.go)
- **channel/Dockerfile**: archivo nuevo
- **docker-compose.yml**: archivo nuevo
- **framework-deployer/framework.manifest.json**: +26 / -5
- **framework-foco/framework.manifest.json**: +10 / -6
- **framework-mecanico/go.mod**: -4
- **framework-pingpong/framework-pingpong**: configuracion
- **framework-pingpong/pingpong**: configuracion
- **framework-pingpong/pingpong_progress.json**: -59
- **servidor-rpc/pingpong_progress.json**: archivo nuevo
- **framework-sabio/go.mod**: -3
- **cobranza-chile/flow.rules.json**: +21 / -19
- **framework-pingpong/INITIAL_PROMPT.md**: +131 / -63
- **foco/main.go**: funciones: configureFocoSession, safePathSegment, newSessionPlan, loadOrNewSessionPlan | tipos: focoAIIntent, focoHistoryTurn
- **llm/client.go**: archivo nuevo
- **fixers/fixers.go**: funciones: ProposeForFinding, proposeDeriveFromRelated, proposeSetNull, proposeNextSequence
- **auditdata/auditdata.go**: archivo nuevo
- **framework-pingpong/main.go**: funciones: cmdConfigure, cmdNext, cmdCheck, cmdAccept
- **pingpong/client.go**: funciones: NewWithTrace, (c *Client) Configure, (c *Client) Clean, (c *Client) Scan | tipos: Detour, BatchInfo, BatchStep
- **pingpong/flow80.go**: archivo nuevo
- **pingpong/inspector.go**: archivo nuevo
- **pingpong/runner.go**: funciones: RunFile, runConfigured, execRun
- **pingpong/verifier.go**: funciones: CompileCheck, FileHash, ReadFileContent, extractSnippet | tipos: LangConfig
- **servidor-rpc/servidor.go**: archivo nuevo
- **frameworksabio/local_store.go**: archivo nuevo
- **frameworksabio/main.go**: +14 / -15
- **servidor-rpc/client.go**: archivo nuevo
- **servidor-rpc/cliente.go**: -23
- **servidor-rpc/main.go**: -25
- **pingpong/batch_test.go**: archivo nuevo
- **pingpong/clean_test.go**: archivo nuevo
- **pingpong/flow80_test.go**: archivo nuevo
- **pingpong/inspector_test.go**: archivo nuevo
- **pingpong/verifier_strict_test.go**: funciones: TestCompileCheckPythonOK, TestCompileCheckPythonError

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
- **api_rest/flow.rules.json**: +12
- **api_rest/main.go**: funciones: (s *server) postMessageSSE | tipos: loopResult
- **api_rest/streaming.go**: archivo nuevo
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

- **api_rest/main.go**: funciones: (s *server) healthz | tipos: checkResult

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
- **5 binarios compilados**: removidos (channel, frameworkecho, frameworksabio, api_rest, api_rest/channel)
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

- **api_rest/.dockerignore**: archivo nuevo
- **api_rest/Dockerfile**: archivo nuevo
- **api_rest/channel**: archivo nuevo
- **api_rest/deploy.sh**: archivo nuevo
- **api_rest/entrypoint.sh**: archivo nuevo
- **api_rest/flow.rules.json**: +15 / -8
- **static/index.html**: archivo nuevo
- **remora-flujo/flujo_test**: archivo nuevo
- **frontend-chat/index.html**: +2093 / -503
- **api_rest/drivers.go**: funciones: initDriverRegistry, keysOf
- **api_rest/generic_driver.go**: archivo nuevo
- **api_rest/main.go**: funciones: getRuntimeInfo, (s *server) getRuntime, (s *server) listModels, (s *server) getRules | tipos: runtimeInfo, createSingleConvRequest

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