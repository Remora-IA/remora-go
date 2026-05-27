# framework-charlie

> Git-safe versioning — los agentes no pueden romper el repo

| Campo | Valor |
|---|---|
| Package | `charlie` |
| Pain weight | 0.70 |
| Agente | `agent-gamma` |
| Archivos | charlie.go |
| Líneas | 2638 |
| Tags | versioning, safety |

## Funciones exportadas

### `ValidateSafeToOperate`

### `ChangeToRepoRoot`

### `ResetToCommit`
ResetToCommit exists only as a guardrail for older callers. Charlie must not

### `BackupWorkingTree`

### `CurrentBranch`

### `HeadCommit`

### `FullCommit`

### `TagCommit`

### `RemoteTagCommit`

### `RemoteBranchCommit`

### `IsAncestor`

### `CommitMessage`

### `TagsAt`

### `StashList`

### `UpstreamDivergence`

### `UpstreamRef`

### `UnmergedPaths`

### `Preflight`

### `BuildAmendPlan`

### `BuildReconcileDraftPlan`

### `BuildRepairReleasePlan`

### `ApplyRepairRelease`

### `BuildPublishDraftPlan`

### `ApplyPublishDraft`

### `BuildPublishTagPlan`

### `ApplyPublishTag`

### `BuildPublishMainPlan`

### `ApplyPublishMain`

### `Status`

### `CheckIfClean`

### `GetCurrentTag`

### `BuildReport`

### `ClassifyChanges`

### `ShouldIgnore`

### `ClassifyFile`

### `SummarizeDiff`

### `GetScope`

### `GenerateCommitMessage`

### `NextVersion`

### `GenerateChangelogSection`

### `FormatStatus`

### `FormatPreflight`

### `FormatProposal`

### `FormatAmendPlan`

### `FormatReconcilePlan`

### `FormatRepairReleasePlan`

### `FormatPublishDraftPlan`

### `FormatPublishTagPlan`

### `FormatPublishMainPlan`

### `ValidateReport`

### `FormatValidation`

### `GetFilesInCommit`

### `HasUncommittedChanges`

### `SuggestConsolidateCommits`

## Tipos

- **`ChangeType`** (type)
- **`Change`** (struct)
- **`DiffSummary`** (struct)
- **`Report`** (struct)
- **`PreflightReport`** (struct)
- **`AmendPlan`** (struct)
- **`ReconcilePlan`** (struct)
- **`RepairReleasePlan`** (struct)
- **`PublishDraftPlan`** (struct)
- **`PublishTagPlan`** (struct)
- **`PublishMainPlan`** (struct)

---
_Generado por Remora Doc-Swarm · agente `agent-gamma` · 2026-05-27 06:25:03_
