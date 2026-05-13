package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type semanticPack struct {
	BusinessID        string                    `json:"business_id"`
	Name              string                    `json:"name"`
	PrimaryEntities   map[string]semanticEntity `json:"primary_entities"`
	ScopePolicies     scopePolicies             `json:"scope_policies"`
	CollectionScoring *collectionScoring        `json:"collection_scoring,omitempty"`
}

type semanticEntity struct {
	Table         string `json:"table"`
	Label         string `json:"label"`
	ScopeKey      string `json:"scope_key"`
	ScopeColumn   string `json:"scope_column"`
	DisplayColumn string `json:"display_column"`
}

type scopePolicies struct {
	ScopeEntity string                    `json:"scope_entity"`
	Tables      map[string]scopeTableRule `json:"tables"`
}

type scopeTableRule struct {
	ScopeColumn string `json:"scope_column"`
	JoinToScope string `json:"join_to_scope"`
}

type collectionScoring struct {
	EntityTable       string   `json:"entity_table"`
	EntityIDColumn    string   `json:"entity_id_column"`
	EntityNameColumn  string   `json:"entity_name_column"`
	ItemTable         string   `json:"item_table"`
	ItemEntityColumn  string   `json:"item_entity_column"`
	AmountColumn      string   `json:"amount_column"`
	AmountTable       string   `json:"amount_table"`
	AmountJoinColumn  string   `json:"amount_join_column"`
	AmountDateColumn  string   `json:"amount_date_column"`
	ItemJoinColumn    string   `json:"item_join_column"`
	StatusColumn      string   `json:"status_column"`
	DateColumn        string   `json:"date_column"`
	OpenStatuses      []string `json:"open_statuses"`
	RecentContactDays int      `json:"recent_contact_days"`
}

type tableInfo struct {
	Name    string
	Columns map[string]bool
}

type rawItem struct {
	EntityID   string
	EntityName string
	Amount     float64
	DueDate    string
	Status     string
	ItemID     string
}

type aggregate struct {
	EntityID string
	Name     string
	Amount   float64
	Count    int
	Oldest   time.Time
	HasDate  bool
	Statuses map[string]bool
}

type paymentStats struct {
	Count   int
	Total   float64
	Last    time.Time
	HasLast bool
}

type priorityItem struct {
	ArtifactType    string         `json:"artifact_type"`
	Rank            int            `json:"rank"`
	Score           int            `json:"score"`
	ScoreBreakdown  map[string]int `json:"score_breakdown,omitempty"`
	Strategy        string         `json:"strategy,omitempty"`
	AnalysisOptions []string       `json:"analysis_options,omitempty"`
	DataGaps        []string       `json:"data_gaps,omitempty"`
	EntityRef       entityRef      `json:"entity_ref"`
	Reasons         []string       `json:"reasons"`
	Deudor          string         `json:"deudor,omitempty"`
	DeudorID        string         `json:"deudor_id,omitempty"`
	SaldoTotal      float64        `json:"saldo_total,omitempty"`
	DiasMoraMax     int            `json:"dias_mora_max,omitempty"`
	FacturasCount   int            `json:"facturas_count,omitempty"`
	Razon           string         `json:"razon,omitempty"`
}

type entityRef struct {
	ArtifactType string `json:"artifact_type"`
	Type         string `json:"type"`
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
}

type followupSynthesis struct {
	Text             string
	Findings         []string
	Evidence         []string
	Confidence       string
	DataGaps         []string
	ResidualRisks    []string
	NextBestQuestion string
	Recommendation   string
}

type portfolioEvidenceSummary struct {
	Text           string
	Findings       []string
	Evidence       []string
	Confidence     string
	DataGaps       []string
	ResidualRisks  []string
	Recommendation string
}

func main() {
	if len(os.Args) < 2 {
		fail("uso: frameworkradar prioritize --business-id <id> --dataset-json <json> --semantic-pack <path>")
	}
	switch os.Args[1] {
	case "configure-analysis":
		if err := runConfigureAnalysis(os.Args[2:]); err != nil {
			fail("%v", err)
		}
	case "deep-dive":
		if err := runDeepDive(os.Args[2:]); err != nil {
			fail("%v", err)
		}
	case "analyze-followup":
		if err := runAnalyzeFollowup(os.Args[2:]); err != nil {
			fail("%v", err)
		}
	case "prioritize":
		if err := runPrioritize(os.Args[2:]); err != nil {
			fail("%v", err)
		}
	default:
		fail("comando desconocido: %s", os.Args[1])
	}
}

func runPrioritize(args []string) error {
	fs := flag.NewFlagSet("prioritize", flag.ExitOnError)
	businessID := fs.String("business-id", "", "negocio activo")
	dbPath := fs.String("db", "", "path SQLite (solo debug/admin; en runtime Radar debe recibir dataset mediado por Sabio)")
	semanticPath := fs.String("semantic-pack", "", "path semantic pack")
	datasetArtifact := fs.String("dataset-artifact", "", "path a dataset.raw.v1 exportado por Sabio")
	datasetJSON := fs.String("dataset-json", "", "dataset como JSON string exportado por Sabio")
	contextB64 := fs.String("context-b64", "", "contexto runtime codificado")
	_ = contextB64
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedDB := *dbPath
	if strings.TrimSpace(*datasetJSON) != "" {
		tmp, err := writeTempJSONToDB(*datasetJSON)
		if err != nil {
			emitNeedsConfiguration(*businessID, "dataset_json_error", err.Error())
			return nil
		}
		defer os.Remove(tmp)
		resolvedDB = tmp
	} else if strings.TrimSpace(*datasetArtifact) != "" {
		tmp, err := loadDatasetArtifactToTempDB(*datasetArtifact)
		if err != nil {
			emitNeedsConfiguration(*businessID, "dataset_artifact_error", err.Error())
			return nil
		}
		defer os.Remove(tmp)
		resolvedDB = tmp
	}
	if strings.TrimSpace(resolvedDB) == "" {
		emitNeedsConfiguration(*businessID, "missing_dataset", "Falta dataset.raw.v1 mediado por Sabio para calcular prioridades.")
		return nil
	}
	if strings.TrimSpace(*semanticPath) == "" {
		emitNeedsConfiguration(*businessID, "missing_semantic_pack", "Falta business.semantic_pack.v1; no se infiere scoring sin configuración semántica.")
		return nil
	}

	pack, err := loadSemanticPack(*semanticPath)
	if err != nil {
		emitNeedsConfiguration(*businessID, "invalid_semantic_pack", err.Error())
		return nil
	}
	model, err := inferScoringModel(pack)
	if err != nil {
		emitNeedsConfiguration(firstNonEmpty(*businessID, pack.BusinessID), "needs_configuration", err.Error())
		return nil
	}
	bid := firstNonEmpty(*businessID, pack.BusinessID)
	if persisted, ok := loadPersistedAnalysisPlan(bid); ok {
		model = persisted
	}
	items, resolvedModel, err := scoreSQLite(resolvedDB, model)
	if err != nil {
		emitNeedsConfiguration(bid, "query_error", err.Error())
		return nil
	}
	plan := persistAnalysisPlan(bid, resolvedModel)
	emitPriorityList(bid, items, resolvedModel, plan)
	return nil
}

func runConfigureAnalysis(args []string) error {
	fs := flag.NewFlagSet("configure-analysis", flag.ExitOnError)
	businessID := fs.String("business-id", "", "negocio activo")
	dbPath := fs.String("db", "", "path SQLite")
	semanticPath := fs.String("semantic-pack", "", "path semantic pack")
	contextB64 := fs.String("context-b64", "", "contexto runtime codificado")
	_ = contextB64
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*semanticPath) == "" {
		emitNeedsConfiguration(*businessID, "missing_semantic_pack", "Falta business.semantic_pack.v1 para diseñar el algoritmo de análisis.")
		return nil
	}
	pack, err := loadSemanticPack(*semanticPath)
	if err != nil {
		emitNeedsConfiguration(*businessID, "invalid_semantic_pack", err.Error())
		return nil
	}
	model, err := inferScoringModel(pack)
	if err != nil {
		emitNeedsConfiguration(firstNonEmpty(*businessID, pack.BusinessID), "needs_configuration", err.Error())
		return nil
	}
	resolvedModel := model
	if strings.TrimSpace(*dbPath) != "" {
		if _, resolved, err := scoreSQLite(*dbPath, model); err == nil {
			resolvedModel = resolved
		}
	}
	bid := firstNonEmpty(*businessID, pack.BusinessID)
	plan := persistAnalysisPlan(bid, resolvedModel)
	printJSON(map[string]interface{}{
		"artifact_type": "analysis.schema.v1",
		"artifacts":     []string{"analysis.schema.v1", "analysis.proposal.v1"},
		"business_id":   bid,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"schema_id":     "collection_priority_40_30_30_v1",
		"schema_path":   plan.SchemaPath,
		"plan_path":     plan.PlanPath,
		"sql_path":      plan.SQLPath,
		"weights":       map[string]int{"materialidad": 40, "comportamiento": 30, "riesgo_legal": 30},
		"model":         analysisModelPayload(resolvedModel),
		"text":          "Radar propone analizar la cartera con un algoritmo configurable: materialidad 40%, comportamiento histórico 30% y riesgo legal/antigüedad 30%. Esta configuración queda plasmada como analysis.schema.v1 para reutilizarse en código antes de priorizar el día.",
		"options": []string{
			"Aceptar configuración y calcular lista de hoy",
			"Ajustar ponderaciones del scoring",
			"Agregar o quitar señales de análisis",
		},
	})
	return nil
}

func runDeepDive(args []string) error {
	fs := flag.NewFlagSet("deep-dive", flag.ExitOnError)
	businessID := fs.String("business-id", "", "negocio activo")
	entityRef := fs.String("entity-ref", "", "referencia de entidad")
	entityType := fs.String("entity-type", "customer", "tipo de entidad")
	semanticPath := fs.String("semantic-pack", "", "path semantic pack")
	priorityListPath := fs.String("priority-list-path", "", "path a collection.priority_list.v1")
	priorityListJSON := fs.String("priority-list-json", "", "collection.priority_list.v1 como JSON")
	strategyPath := fs.String("strategy-path", "", "path a strategy.recommendation.v1")
	strategyJSON := fs.String("strategy-json", "", "strategy.recommendation.v1 como JSON")
	contextB64 := fs.String("context-b64", "", "contexto runtime codificado")
	if err := fs.Parse(args); err != nil {
		return err
	}
	bid := strings.TrimSpace(*businessID)
	if strings.TrimSpace(*semanticPath) != "" {
		if pack, err := loadSemanticPack(*semanticPath); err == nil {
			bid = firstNonEmpty(bid, pack.BusinessID)
		}
	}
	target := radarDeepDiveTarget(loadJSONPayload(*priorityListPath, *priorityListJSON), *entityRef, *entityType)
	priorityItem := radarDeepDivePriorityItem(loadJSONPayload(*priorityListPath, *priorityListJSON), jsonStringFromMap(target, "id"))
	contextPayload := decodeContextPayload(*contextB64)
	selectedAction := jsonStringFromMap(contextPayload, "selected_action_id")
	if selectedAction == "" {
		if selected, ok := contextPayload["selected_action"].(map[string]interface{}); ok {
			selectedAction = jsonStringFromMap(selected, "id", "action_id")
		}
	}
	if selectedAction == "" {
		selectedAction = "deep_analysis"
	}
	recommendation := radarDeepDiveRecommendation(loadJSONPayload(*strategyPath, *strategyJSON))
	name := firstNonEmpty(jsonStringFromMap(target, "name", "entity_name"), strings.TrimSpace(*entityRef), "el caso seleccionado")
	whyPrioritized := radarDeepDiveWhyPrioritized(priorityItem, recommendation)
	evidenceSummary := radarDeepDiveEvidenceSummary(target, priorityItem)
	riskSummary := radarDeepDiveRiskSummary(priorityItem, recommendation)
	agingSummary := radarDeepDiveAgingSummary(target, priorityItem)
	invoiceSummary := radarDeepDiveInvoiceSummary(target, priorityItem)
	blockingGaps := radarDeepDiveBlockingGaps(priorityItem)
	nextNonOperational := radarDeepDiveNonOperationalNextSteps(blockingGaps)
	nextOperational := radarDeepDiveOperationalNextSteps(recommendation, blockingGaps)
	openQuestions := radarDeepDiveOpenQuestions(target, priorityItem, blockingGaps)
	summary := fmt.Sprintf("Radar prioriza %s para análisis profundo y mantiene el tramo analítico hasta cerrar evidencia, riesgos y brechas reales antes de operar.", name)
	text := radarDeepDiveNarrative(name, whyPrioritized, evidenceSummary, riskSummary, agingSummary, invoiceSummary, blockingGaps, nextNonOperational, nextOperational, openQuestions)
	printJSON(map[string]interface{}{
		"artifact_type": "analysis.case_review.v1",
		"artifacts":     []string{"analysis.case_review.v1", "answer.grounded.v1"},
		"business_id":   bid,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"owner": map[string]interface{}{
			"framework":        "radar",
			"capability":       "analysis.deep_dive",
			"transfer_control": true,
		},
		"selected_action_id":                     selectedAction,
		"selected":                               target,
		"summary":                                summary,
		"text":                                   text,
		"why_prioritized":                        whyPrioritized,
		"evidence_summary":                       evidenceSummary,
		"risk_summary":                           riskSummary,
		"aging_summary":                          agingSummary,
		"invoice_summary":                        invoiceSummary,
		"blocking_gaps":                          blockingGaps,
		"recommended_next_non_operational_steps": nextNonOperational,
		"recommended_next_operational_steps":     nextOperational,
		"open_questions":                         openQuestions,
		"analysis_focus": []string{
			"confirmar evidencia del caso priorizado",
			"solicitar entity_360 y contexto relacional antes de redactar",
			"validar hallazgos y huecos de datos antes de bajar a ejecución",
		},
		"delegation_plan": []map[string]interface{}{
			{"framework": "sabio", "capability": "data.entity_360", "goal": "obtener contexto 360 del caso"},
			{"framework": "auditor", "capability": "data.quality.audit", "goal": "validar evidencia y huecos del caso"},
		},
		"current_recommendation": recommendation,
		"grounded_answer": map[string]interface{}{
			"artifact_type": "answer.grounded.v1",
			"text":          text,
		},
	})
	return nil
}

func runAnalyzeFollowup(args []string) error {
	fs := flag.NewFlagSet("analyze-followup", flag.ExitOnError)
	businessID := fs.String("business-id", "", "negocio activo")
	input := fs.String("input", "", "follow-up del usuario")
	entityRef := fs.String("entity-ref", "", "referencia de entidad")
	entityType := fs.String("entity-type", "customer", "tipo de entidad")
	semanticPath := fs.String("semantic-pack", "", "path semantic pack")
	previousAnalysisPath := fs.String("previous-analysis-path", "", "path a analysis.case_review.v1 previo")
	previousAnalysisJSON := fs.String("previous-analysis-json", "", "analysis.case_review.v1 como JSON")
	delegationResultsPath := fs.String("delegation-results-path", "", "path a resultados de delegacion")
	delegationResultsJSON := fs.String("delegation-results-json", "", "resultados de delegacion como JSON")
	priorityListPath := fs.String("priority-list-path", "", "path a collection.priority_list.v1")
	priorityListJSON := fs.String("priority-list-json", "", "collection.priority_list.v1 como JSON")
	turnCountStr := fs.String("turn-count", "0", "numero de turno actual")
	llmFollowupPath := fs.String("llm-followup-path", "", "path a la respuesta analitica generada por LLM del owner")
	llmFollowupJSON := fs.String("llm-followup-json", "", "respuesta analitica generada por LLM del owner")
	contextB64 := fs.String("context-b64", "", "contexto runtime codificado")
	_ = contextB64
	if err := fs.Parse(args); err != nil {
		return err
	}
	bid := strings.TrimSpace(*businessID)
	if strings.TrimSpace(*semanticPath) != "" {
		if pack, err := loadSemanticPack(*semanticPath); err == nil {
			bid = firstNonEmpty(bid, pack.BusinessID)
		}
	}
	userInput := strings.TrimSpace(*input)
	if userInput == "" {
		printJSON(map[string]interface{}{
			"artifact_type": "analysis.followup.v1",
			"text":          "No se recibio un follow-up del usuario.",
			"status":        "no_input",
		})
		return nil
	}
	turnCount := 0
	if n, err := fmt.Sscanf(*turnCountStr, "%d", &turnCount); n == 0 || err != nil {
		turnCount = 0
	}

	// Load previous analysis for context
	previousAnalysis := parseJSONObject(loadJSONPayload(*previousAnalysisPath, *previousAnalysisJSON))
	delegationResults := parseJSONObject(loadJSONPayload(*delegationResultsPath, *delegationResultsJSON))
	target := radarDeepDiveTarget(loadJSONPayload(*priorityListPath, *priorityListJSON), *entityRef, *entityType)
	priorityItem := radarDeepDivePriorityItem(loadJSONPayload(*priorityListPath, *priorityListJSON), jsonStringFromMap(target, "id"))
	name := firstNonEmpty(jsonStringFromMap(target, "name", "entity_name"), strings.TrimSpace(*entityRef), "el caso seleccionado")

	llmFollowupRaw := loadJSONPayload(*llmFollowupPath, *llmFollowupJSON)
	llmPayload := parseJSONObject(llmFollowupRaw)
	evidenceSynthesis := buildEvidenceCentricFollowupSynthesis(userInput, name, delegationResults)
	responseText := evidenceSynthesis.Text
	if responseText == "" {
		responseText = followupTextFromLLM(llmFollowupRaw)
	}
	if responseText == "" {
		responseText = buildFollowupResponse(userInput, name, previousAnalysis, delegationResults, priorityItem, target)
	}

	// Phase A (plan): if no delegated evidence yet, Radar can request allowed
	// analytical evidence contracts. Prefer LLM plan output; fallback is deterministic.
	var delegationRequests []map[string]interface{}
	intent := firstNonEmpty(jsonStringFromMap(llmPayload, "analysis_intent"), inferAnalyticalIntent(userInput))
	planAudit := map[string]interface{}{
		"intent":                   intent,
		"llm_delegation_requests":  delegationRequestsFromPayload(llmPayload),
		"required_capabilities":    requiredDelegationCapabilitiesForIntent(intent),
		"completion_applied":       false,
		"added_capabilities":       []string{},
		"fallback_delegation_plan": []map[string]interface{}{},
	}
	if len(delegationResults) == 0 {
		delegationRequests, planAudit = reconcileDelegationPlan(userInput, target, llmPayload)
	}

	output := map[string]interface{}{
		"artifact_type": "analysis.followup.v1",
		"artifacts":     []string{"analysis.followup.v1", "answer.grounded.v1"},
		"business_id":   bid,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"turn_count":    turnCount,
		"user_input":    userInput,
		"entity_ref":    target,
		"text":          responseText,
		"owner": map[string]interface{}{
			"framework":  "radar",
			"capability": "analysis.deep_dive",
		},
		"grounded_answer": map[string]interface{}{
			"artifact_type": "answer.grounded.v1",
			"text":          responseText,
		},
	}
	if len(delegationRequests) > 0 {
		output["analysis_phase"] = "plan"
		output["analysis_intent"] = intent
		output["needs_delegation"] = true
		output["evidence_needed"] = stringSliceFromPayload(llmPayload, "evidence_needed")
		output["reason"] = jsonStringFromMap(llmPayload, "reason")
		output["delegation_requests"] = delegationRequests
		output["plan_audit"] = planAudit
	} else {
		output["analysis_phase"] = "synthesis"
		output["findings"] = uniqueNonEmptyStrings(append(evidenceSynthesis.Findings, stringSliceFromPayload(llmPayload, "findings")...))
		output["evidence"] = uniqueNonEmptyStrings(append(evidenceSynthesis.Evidence, stringSliceFromPayload(llmPayload, "evidence")...))
		output["confidence"] = firstNonEmpty(evidenceSynthesis.Confidence, jsonStringFromMap(llmPayload, "confidence"), "partial")
		output["data_gaps"] = uniqueNonEmptyStrings(append(evidenceSynthesis.DataGaps, stringSliceFromPayload(llmPayload, "data_gaps")...))
		output["residual_risks"] = uniqueNonEmptyStrings(append(evidenceSynthesis.ResidualRisks, stringSliceFromPayload(llmPayload, "residual_risks")...))
		output["next_best_question"] = firstNonEmpty(evidenceSynthesis.NextBestQuestion, jsonStringFromMap(llmPayload, "next_best_question"))
		output["recommendation"] = firstNonEmpty(evidenceSynthesis.Recommendation, jsonStringFromMap(llmPayload, "recommendation"))
	}
	printJSON(output)
	return nil
}

// --- Data-driven followup response ---
//
// Instead of keyword-branching, the followup collects ALL available data
// sections, scores each for relevance against the user's input, and
// includes sections above a threshold. This makes the system extensible
// without adding more keyword branches.

// analyticalSection represents one block of analytical content that can
// be included in a followup response.
type analyticalSection struct {
	Label    string   // heading for this section
	Keywords []string // terms that boost this section's relevance
	Content  func(target, priorityItem map[string]interface{}) string
}

// followupSections defines the available analytical sections.
// Each section declares its relevance keywords. The system picks sections
// whose keywords match the user's input, plus a generic context section
// as fallback.
var followupSections = []analyticalSection{
	{
		Label:    "Mora/Antiguedad",
		Keywords: []string{"mora", "dias", "día", "antigued", "aging", "vencid", "atraso"},
		Content: func(target, item map[string]interface{}) string {
			if s := radarDeepDiveAgingSummary(target, item); s != "" {
				return s
			}
			return "No hay datos suficientes de mora/antiguedad. Radar puede solicitar un entity_360 a Sabio."
		},
	},
	{
		Label:    "Evaluacion de riesgo",
		Keywords: []string{"riesgo", "risk", "peligro", "critico", "crítico", "grave"},
		Content: func(_, item map[string]interface{}) string {
			return radarDeepDiveRiskSummary(item, nil)
		},
	},
	{
		Label:    "Evidencia y facturas",
		Keywords: []string{"evidencia", "factura", "invoice", "comprobante", "documento", "soporte"},
		Content: func(target, item map[string]interface{}) string {
			var parts []string
			for _, e := range radarDeepDiveEvidenceSummary(target, item) {
				parts = append(parts, "  - "+e)
			}
			if s := radarDeepDiveInvoiceSummary(target, item); s != "" {
				parts = append(parts, s)
			}
			return strings.Join(parts, "\n")
		},
	},
	{
		Label:    "Razon de priorizacion",
		Keywords: []string{"prioriz", "por que", "por qué", "scoring", "puntaje", "ranking"},
		Content: func(_, item map[string]interface{}) string {
			return radarDeepDiveWhyPrioritized(item, nil)
		},
	},
	{
		Label:    "Brechas de datos",
		Keywords: []string{"dato", "brecha", "gap", "falta", "pendiente", "incompleto"},
		Content: func(_, item map[string]interface{}) string {
			gaps := radarDeepDiveBlockingGaps(item)
			if len(gaps) == 0 {
				return ""
			}
			var parts []string
			for _, g := range gaps {
				parts = append(parts, "  - "+g)
			}
			return strings.Join(parts, "\n")
		},
	},
}

// buildFollowupResponse constructs a follow-up analytical response using a
// data-driven approach: collect all sections, score relevance, include matches.
func buildFollowupResponse(userInput, entityName string, previousAnalysis, delegationResults, priorityItem, target map[string]interface{}) string {
	var sb strings.Builder

	if len(delegationResults) > 0 {
		if integrated := buildEvidenceCentricFollowupResponse(userInput, entityName, delegationResults); integrated != "" {
			return integrated + "\n\n---\nRadar mantiene el tramo analitico. Podes seguir preguntando o decir 'avanza' para pasar a accion."
		}
	}

	inputLower := strings.ToLower(userInput)

	// Score and collect relevant sections.
	type scored struct {
		label   string
		content string
	}
	var matched []scored
	for _, sec := range followupSections {
		relevant := false
		for _, kw := range sec.Keywords {
			if strings.Contains(inputLower, kw) {
				relevant = true
				break
			}
		}
		if !relevant {
			continue
		}
		content := sec.Content(target, priorityItem)
		if strings.TrimSpace(content) == "" {
			continue
		}
		matched = append(matched, scored{label: sec.Label, content: content})
	}

	if len(matched) > 0 {
		sb.WriteString(fmt.Sprintf("Analisis de seguimiento sobre %s:\n\n", entityName))
		for _, m := range matched {
			sb.WriteString(m.label + ":\n")
			sb.WriteString(m.content)
			sb.WriteString("\n\n")
		}
	} else {
		// Fallback: dump all available context so the user gets something useful.
		sb.WriteString(fmt.Sprintf("Sobre %s, respondiendo a '%s':\n\n", entityName, userInput))
		for _, sec := range followupSections {
			content := sec.Content(target, priorityItem)
			if strings.TrimSpace(content) != "" {
				sb.WriteString(sec.Label + ": " + content + "\n\n")
			}
		}
		if sb.Len() <= len(fmt.Sprintf("Sobre %s, respondiendo a '%s':\n\n", entityName, userInput))+5 {
			sb.WriteString("Radar no tiene datos adicionales para esta pregunta. Podes delegar a Sabio para obtener un entity_360.\n")
		}
	}

	sb.WriteString("---\nRadar mantiene el tramo analitico. Podes seguir preguntando o decir 'avanza' para pasar a accion.")
	return sb.String()
}

func buildEvidenceCentricFollowupResponse(userInput, entityName string, delegationResults map[string]interface{}) string {
	return buildEvidenceCentricFollowupSynthesis(userInput, entityName, delegationResults).Text
}

func buildEvidenceCentricFollowupSynthesis(userInput, entityName string, delegationResults map[string]interface{}) followupSynthesis {
	results := delegationEvidenceResults(delegationResults)
	if len(results) == 0 {
		if text := jsonStringFromMap(delegationResults, "text", "answer"); text != "" {
			return followupSynthesis{
				Text:       fmt.Sprintf("Sobre %s, integrando datos adicionales:\n\n%s", entityName, text),
				Confidence: "partial",
			}
		}
		return followupSynthesis{}
	}
	intent := inferAnalyticalIntent(userInput)
	var (
		case360Verified        map[string]interface{}
		portfolioVerified      map[string]interface{}
		sensitivityVerified    map[string]interface{}
		counterfactualVerified map[string]interface{}
		behaviorVerified       map[string]interface{}
		failures               []string
	)
	for _, entry := range results {
		capability := firstNonEmpty(jsonStringFromMap(entry, "capability"), jsonStringFromMap(entry, "resolved_capability"))
		verified, _ := entry["verified"].(bool)
		if !verified {
			failures = append(failures, summarizeDelegationFailure(entry))
			continue
		}
		switch capability {
		case "evidence.case_360", "data.entity_360":
			case360Verified = entry
		case "evidence.portfolio_comparison":
			portfolioVerified = entry
		case "evidence.score_sensitivity":
			sensitivityVerified = entry
		case "evidence.counterfactual":
			counterfactualVerified = entry
		case "evidence.payment_behavior_summary":
			behaviorVerified = entry
		}
	}
	var (
		parts            []string
		findings         []string
		evidence         []string
		dataGaps         []string
		residualRisks    []string
		recommendation   string
		nextBestQuestion string
		confidence       = "partial"
		verifiedCount    int
	)
	switch intent {
	case "portfolio_comparison":
		if case360Verified != nil {
			summary := summarizeCase360Evidence(case360Verified)
			parts = append(parts, summary)
			findings = append(findings, "La línea base del caso quedó verificada con evidencia directa del entity_360.")
			evidence = append(evidence, summary)
			verifiedCount++
		}
		if portfolioVerified != nil {
			portfolio := summarizePortfolioEvidenceDetails(portfolioVerified)
			parts = append(parts, portfolio.Text)
			findings = append(findings, portfolio.Findings...)
			evidence = append(evidence, portfolio.Evidence...)
			dataGaps = append(dataGaps, portfolio.DataGaps...)
			residualRisks = append(residualRisks, portfolio.ResidualRisks...)
			recommendation = firstNonEmpty(recommendation, portfolio.Recommendation)
			confidence = firstNonEmpty(portfolio.Confidence, confidence)
			verifiedCount++
		} else {
			parts = append(parts, "No pude verificar todavía la comparación contra la cartera, así que no voy a afirmar percentiles ni clientes similares como hecho.")
			dataGaps = append(dataGaps, "Falta comparación de cartera verificada; no corresponde afirmar percentiles ni peers como hecho.")
		}
		nextBestQuestion = "¿Quieres que profundice por qué el caso pesa más por materialidad que por mora relativa, o que baje al detalle de los peers comparables?"
	case "score_sensitivity":
		if case360Verified != nil {
			summary := summarizeCase360Evidence(case360Verified)
			parts = append(parts, summary)
			evidence = append(evidence, summary)
			verifiedCount++
		}
		if sensitivityVerified != nil {
			summary := summarizeGenericEvidence(sensitivityVerified)
			parts = append(parts, summary)
			findings = append(findings, "Hay evidencia cuantitativa para sensibilidad del score.")
			evidence = append(evidence, summary)
			confidence = "moderate"
			verifiedCount++
		} else {
			parts = append(parts, "No pude verificar todavía la sensibilidad del score con evidencia cuantitativa.")
			dataGaps = append(dataGaps, "Falta una simulación verificada de sensibilidad del score.")
		}
	case "counterfactual_scenario":
		if case360Verified != nil {
			summary := summarizeCase360Evidence(case360Verified)
			parts = append(parts, summary)
			evidence = append(evidence, summary)
			verifiedCount++
		}
		if counterfactualVerified != nil {
			summary := summarizeGenericEvidence(counterfactualVerified)
			parts = append(parts, summary)
			findings = append(findings, "El escenario contrafactual quedó soportado por evidencia verificada.")
			evidence = append(evidence, summary)
			confidence = "moderate"
			verifiedCount++
		} else {
			parts = append(parts, "No pude verificar todavía el escenario contrafactual pedido.")
			dataGaps = append(dataGaps, "Falta una simulación contrafactual verificada.")
		}
	default:
		if case360Verified != nil {
			summary := summarizeCase360Evidence(case360Verified)
			parts = append(parts, summary)
			evidence = append(evidence, summary)
			verifiedCount++
		}
		if behaviorVerified != nil {
			summary := summarizeGenericEvidence(behaviorVerified)
			parts = append(parts, summary)
			findings = append(findings, "El comportamiento de pago quedó resumido con evidencia verificada.")
			evidence = append(evidence, summary)
			confidence = "moderate"
			verifiedCount++
		}
	}
	if len(failures) > 0 {
		parts = append(parts, "Delegaciones no verificadas: "+strings.Join(failures, " "))
		dataGaps = append(dataGaps, "Persisten delegaciones no verificadas para parte del análisis.")
		residualRisks = append(residualRisks, "La respuesta combina evidencia verificada con componentes aún no verificados.")
	}
	if len(parts) == 0 {
		return followupSynthesis{}
	}
	if verifiedCount >= 2 && confidence == "partial" {
		confidence = "moderate"
	}
	return followupSynthesis{
		Text:             fmt.Sprintf("Sobre %s, integrando evidencia delegada:\n\n- %s", entityName, strings.Join(parts, "\n- ")),
		Findings:         uniqueNonEmptyStrings(findings),
		Evidence:         uniqueNonEmptyStrings(evidence),
		Confidence:       confidence,
		DataGaps:         uniqueNonEmptyStrings(dataGaps),
		ResidualRisks:    uniqueNonEmptyStrings(residualRisks),
		NextBestQuestion: nextBestQuestion,
		Recommendation:   recommendation,
	}
}

func delegationEvidenceResults(payload map[string]interface{}) []map[string]interface{} {
	raw, ok := payload["results"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

func summarizeDelegationFailure(entry map[string]interface{}) string {
	capability := firstNonEmpty(jsonStringFromMap(entry, "capability"), jsonStringFromMap(entry, "resolved_capability"))
	msg := firstNonEmpty(jsonStringFromMap(entry, "error", "text"), "falló sin detalle")
	return fmt.Sprintf("%s: %s", capability, msg)
}

func summarizeCase360Evidence(entry map[string]interface{}) string {
	payload, _ := entry["payload"].(map[string]interface{})
	if payload == nil {
		return summarizeGenericEvidence(entry)
	}
	entity, _ := payload["entity"].(map[string]interface{})
	financial, _ := payload["financial_position"].(map[string]interface{})
	aging, _ := payload["aging"].(map[string]interface{})
	history, _ := payload["history"].(map[string]interface{})
	states := stringifyMap(payload["charges_by_state"])
	name := firstNonEmpty(jsonStringFromMap(entity, "name"), "el caso")
	return fmt.Sprintf("%s quedó soportado así: saldo abierto %s, mora abierta aproximada %s días, %s pagos históricos por %s, con estados de cargos %s.",
		name,
		formatValue(financial["open_amount"]),
		formatValue(aging["oldest_open_debt_days"]),
		formatValue(history["payments_count"]),
		formatValue(history["payments_total"]),
		firstNonEmpty(states, "sin detalle de estados"),
	)
}

func summarizePortfolioEvidence(entry map[string]interface{}) string {
	return summarizePortfolioEvidenceDetails(entry).Text
}

func summarizePortfolioEvidenceDetails(entry map[string]interface{}) portfolioEvidenceSummary {
	payload, _ := entry["payload"].(map[string]interface{})
	if payload == nil {
		text := summarizeGenericEvidence(entry)
		return portfolioEvidenceSummary{Text: text, Evidence: []string{text}, Confidence: "partial"}
	}
	structured, _ := payload["structured"].(map[string]interface{})
	if structured == nil {
		text := summarizeGenericEvidence(entry)
		return portfolioEvidenceSummary{Text: text, Evidence: []string{text}, Confidence: "partial"}
	}
	entityName := firstNonEmpty(jsonStringFromMap(structured, "entity_name"), jsonStringFromMap(payload, "entity_name"), "el caso")
	openAmount := jsonFloatFromAny(structured["open_amount"])
	daysPastDue := jsonIntFromAny(structured["days_past_due"])
	paymentCount := jsonIntFromAny(structured["payment_count"])
	paymentTotal := jsonFloatFromAny(structured["payment_total"])
	openPercentile := jsonIntFromAny(structured["open_amount_percentile"])
	moraPercentile := jsonIntFromAny(structured["days_past_due_percentile"])
	peers := portfolioPeersFromEvidence(structured["peers"])
	driver := portfolioPriorityDriver(openPercentile, moraPercentile)
	driverInsight := portfolioPriorityDriverInsight(driver)
	peerSummary := portfolioPeersNarrative(peers)
	cohortGap := ""
	if len(peers) < 3 {
		cohortGap = "La cohorte comparable es chica, así que la lectura contra peers conviene tomarla como orientativa."
	}
	paymentClause := ""
	if paymentCount > 0 || paymentTotal > 0 {
		paymentClause = fmt.Sprintf(" En comportamiento de pago aparecen %d pagos históricos por %s.", paymentCount, formatCurrencyCompact(paymentTotal))
	}
	textParts := []string{
		fmt.Sprintf("%s muestra una comparación de cartera soportada: saldo abierto %s (percentil %d) frente a mora %d días (percentil %d).", entityName, formatCurrencyCompact(openAmount), openPercentile, daysPastDue, moraPercentile),
		driverInsight,
	}
	if paymentClause != "" {
		textParts = append(textParts, strings.TrimSpace(paymentClause))
	}
	if peerSummary != "" {
		textParts = append(textParts, "Peers comparables: "+peerSummary+".")
	}
	if cohortGap != "" {
		textParts = append(textParts, cohortGap)
	}
	return portfolioEvidenceSummary{
		Text: strings.Join(textParts, " "),
		Findings: uniqueNonEmptyStrings([]string{
			fmt.Sprintf("El saldo abierto del caso está en percentil %d dentro de la cartera comparable.", openPercentile),
			fmt.Sprintf("La mora del caso está en percentil %d dentro de la cartera comparable.", moraPercentile),
			driverInsight,
		}),
		Evidence: uniqueNonEmptyStrings([]string{
			fmt.Sprintf("Saldo abierto %s con percentil %d.", formatCurrencyCompact(openAmount), openPercentile),
			fmt.Sprintf("Mora %d días con percentil %d.", daysPastDue, moraPercentile),
			firstNonEmpty(peerSummary, ""),
		}),
		Confidence: firstNonEmpty(portfolioEvidenceConfidence(peers), "moderate"),
		DataGaps: uniqueNonEmptyStrings([]string{
			cohortGap,
		}),
		ResidualRisks: uniqueNonEmptyStrings([]string{
			portfolioResidualRisk(driver, len(peers)),
		}),
		Recommendation: portfolioRecommendation(driver),
	}
}

func summarizeGenericEvidence(entry map[string]interface{}) string {
	if text := jsonStringFromMap(entry, "text"); text != "" {
		return text
	}
	if payload, ok := entry["payload"].(map[string]interface{}); ok {
		if text := jsonStringFromMap(payload, "text", "summary", "answer"); text != "" {
			return text
		}
	}
	return "La delegación devolvió evidencia estructurada, pero sin texto resumido."
}

func portfolioPeersFromEvidence(raw interface{}) []map[string]interface{} {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

func portfolioPeersNarrative(peers []map[string]interface{}) string {
	if len(peers) == 0 {
		return ""
	}
	limit := len(peers)
	if limit > 4 {
		limit = 4
	}
	parts := make([]string, 0, limit)
	for _, peer := range peers[:limit] {
		name := firstNonEmpty(jsonStringFromMap(peer, "name"), jsonStringFromMap(peer, "id"), "peer")
		parts = append(parts, fmt.Sprintf("%s (saldo %s, mora %d días)", name, formatCurrencyCompact(jsonFloatFromAny(peer["open_amount"])), jsonIntFromAny(peer["days_past_due"])))
	}
	return strings.Join(parts, "; ")
}

func portfolioPriorityDriver(openPercentile, moraPercentile int) string {
	diff := openPercentile - moraPercentile
	switch {
	case diff >= 15:
		return "materialidad"
	case diff <= -15:
		return "mora"
	default:
		return "balanceado"
	}
}

func portfolioPriorityDriverInsight(driver string) string {
	switch driver {
	case "materialidad":
		return "La prioridad parece venir más por materialidad que por mora relativa."
	case "mora":
		return "La prioridad parece venir más por envejecimiento/mora que por materialidad pura."
	default:
		return "La prioridad parece combinar materialidad y mora sin que una domine claramente."
	}
}

func portfolioRecommendation(driver string) string {
	switch driver {
	case "materialidad":
		return "Tratar este caso como prioritario por materialidad/exposición económica; conviene validar contactabilidad antes de ejecutar acción."
	case "mora":
		return "Tratar este caso como prioritario por envejecimiento relativo; conviene confirmar recuperabilidad y contacto antes de operar."
	default:
		return "Mantener el caso en prioridad alta, pero validando contacto y recuperabilidad porque la señal combina materialidad y mora."
	}
}

func portfolioResidualRisk(driver string, peerCount int) string {
	if peerCount < 3 {
		return "La cohorte comparable es reducida, así que la inferencia sobre peers puede estar sesgada."
	}
	switch driver {
	case "materialidad":
		return "La prioridad por materialidad no garantiza urgencia operativa si la recuperabilidad o el contacto son débiles."
	case "mora":
		return "La prioridad por mora no garantiza el mayor impacto económico si la exposición es menor que otros casos."
	default:
		return "La combinación de señales puede cambiar si aparecen nuevos peers o datos de recuperabilidad."
	}
}

func portfolioEvidenceConfidence(peers []map[string]interface{}) string {
	if len(peers) >= 3 {
		return "moderate"
	}
	return "partial"
}

func formatValue(v interface{}) string {
	switch n := v.(type) {
	case float64:
		if n == float64(int64(n)) {
			return fmt.Sprintf("%d", int64(n))
		}
		return fmt.Sprintf("%.2f", n)
	case int:
		return fmt.Sprintf("%d", n)
	case int64:
		return fmt.Sprintf("%d", n)
	case string:
		return n
	default:
		return fmt.Sprintf("%v", v)
	}
}

func stringifyMap(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, formatValue(m[k])))
	}
	return strings.Join(parts, ", ")
}

// --- Delegation inference ---
//
// Instead of keyword-driven hardcodes, delegation needs are inferred from
// a declarative mapping of data domains → capabilities. The engine checks
// which domains the user is asking about and maps them to the appropriate
// delegate capability.

// inferDelegationNeeds determines if the follow-up requires analytical
// evidence contracts instead of raw technical capabilities.
func inferDelegationNeeds(userInput string, existingDelegationResults, target map[string]interface{}) []map[string]interface{} {
	if len(existingDelegationResults) > 0 {
		return nil
	}
	intent := inferAnalyticalIntent(userInput)
	entityRef := jsonStringFromMap(target, "id", "entity_id")
	entityType := firstNonEmpty(jsonStringFromMap(target, "type", "entity_type"), "client")
	case360 := func(reason string) map[string]interface{} {
		return map[string]interface{}{
			"framework":  "sabio",
			"capability": "evidence.case_360",
			"params": map[string]interface{}{
				"entity_ref":      entityRef,
				"entity_type":     entityType,
				"analysis_intent": "case_baseline",
				"question":        fmt.Sprintf("Construye una vista 360 verificable del cliente %s.", firstNonEmpty(entityRef, "activo")),
			},
			"reason": reason,
		}
	}
	switch intent {
	case "portfolio_comparison":
		return []map[string]interface{}{
			case360("establecer la línea base verificable del caso"),
			{
				"framework":  "sabio",
				"capability": "evidence.portfolio_comparison",
				"params": map[string]interface{}{
					"entity_ref":      entityRef,
					"entity_type":     entityType,
					"analysis_intent": intent,
					"metrics":         []interface{}{"open_amount", "days_past_due", "payment_behavior"},
					"peer_strategy":   "similar_clients",
					"question":        fmt.Sprintf("Compara el cliente %s contra la cartera por saldo abierto, mora y comportamiento relativo.", firstNonEmpty(entityRef, "activo")),
				},
				"reason": "comparar el caso contra la cartera requiere percentiles y clientes comparables",
			},
		}
	case "data_reconciliation":
		return []map[string]interface{}{
			case360("obtener la línea base del caso antes de auditar contradicciones"),
			{
				"framework":  "auditor",
				"capability": "evidence.data_reconciliation",
				"params": map[string]interface{}{
					"entity_ref":      entityRef,
					"entity_type":     entityType,
					"analysis_intent": intent,
					"question":        fmt.Sprintf("Audita contradicciones, gaps y límites de inferencia del cliente %s.", firstNonEmpty(entityRef, "activo")),
				},
				"reason": "validar calidad y contradicciones relevantes para la conclusión analítica",
			},
		}
	case "score_sensitivity":
		return []map[string]interface{}{
			case360("necesito la línea base del caso antes de medir sensibilidad"),
			{
				"framework":  "sabio",
				"capability": "evidence.score_sensitivity",
				"params": map[string]interface{}{
					"entity_ref":      entityRef,
					"entity_type":     entityType,
					"analysis_intent": intent,
					"metrics":         []interface{}{"materiality", "payment_behavior", "legal_risk"},
					"question":        fmt.Sprintf("Calcula sensibilidad del score para el cliente %s.", firstNonEmpty(entityRef, "activo")),
				},
				"reason": "medir qué variable cambiaría más la prioridad requiere evidencia contrafactual",
			},
		}
	case "counterfactual_scenario":
		return []map[string]interface{}{
			case360("necesito la línea base del caso antes de simular contrafactuales"),
			{
				"framework":  "sabio",
				"capability": "evidence.counterfactual",
				"params": map[string]interface{}{
					"entity_ref":      entityRef,
					"entity_type":     entityType,
					"analysis_intent": intent,
					"question":        fmt.Sprintf("Evalúa escenarios contrafactuales para el cliente %s.", firstNonEmpty(entityRef, "activo")),
				},
				"reason": "la pregunta pide escenarios contrafactuales verificables",
			},
		}
	case "alternative_hypothesis":
		return []map[string]interface{}{
			case360("necesito la línea base del caso para contrastar hipótesis"),
			{
				"framework":  "auditor",
				"capability": "evidence.claim_audit",
				"params": map[string]interface{}{
					"entity_ref":      entityRef,
					"entity_type":     entityType,
					"analysis_intent": intent,
					"question":        fmt.Sprintf("Audita qué claims sobre el cliente %s son débiles o no verificables.", firstNonEmpty(entityRef, "activo")),
				},
				"reason": "las hipótesis alternativas requieren validar claims y límites de inferencia",
			},
		}
	default:
		if strings.Contains(strings.ToLower(userInput), "pago") || strings.Contains(strings.ToLower(userInput), "comportam") {
			return []map[string]interface{}{
				{
					"framework":  "sabio",
					"capability": "evidence.payment_behavior_summary",
					"params": map[string]interface{}{
						"entity_ref":      entityRef,
						"entity_type":     entityType,
						"analysis_intent": "payment_behavior_summary",
						"metrics":         []interface{}{"payment_count", "payment_total", "payment_residue_total"},
						"question":        fmt.Sprintf("Resume el comportamiento de pago del cliente %s.", firstNonEmpty(entityRef, "activo")),
					},
					"reason": "el usuario pide evidencia del comportamiento de pagos",
				},
			}
		}
		return nil
	}
}

func followupTextFromLLM(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var payload map[string]interface{}
	if json.Unmarshal([]byte(raw), &payload) != nil {
		return ""
	}
	text := jsonStringFromMap(payload, "text", "answer", "analysis")
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return strings.TrimSpace(text) + "\n\n---\nRadar mantiene el tramo analitico. Podes seguir preguntando o decir 'avanza' para pasar a accion."
}

func delegationRequestsFromPayload(payload map[string]interface{}) []map[string]interface{} {
	raw, ok := payload["delegation_requests"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

func reconcileDelegationPlan(userInput string, target, llmPayload map[string]interface{}) ([]map[string]interface{}, map[string]interface{}) {
	intent := firstNonEmpty(jsonStringFromMap(llmPayload, "analysis_intent"), inferAnalyticalIntent(userInput))
	llmRequests := delegationRequestsFromPayload(llmPayload)
	fallback := inferDelegationNeeds(userInput, nil, target)
	required := requiredDelegationCapabilitiesForIntent(intent)
	added := []string{}
	merged := append([]map[string]interface{}{}, llmRequests...)
	existing := map[string]bool{}
	for _, req := range llmRequests {
		existing[jsonStringFromMap(req, "capability")] = true
	}
	for _, req := range fallback {
		capability := jsonStringFromMap(req, "capability")
		if capability == "" || existing[capability] {
			continue
		}
		if !stringSliceContains(required, capability) && len(llmRequests) > 0 {
			continue
		}
		merged = append(merged, req)
		existing[capability] = true
		added = append(added, capability)
	}
	if len(merged) == 0 {
		merged = fallback
	}
	return merged, map[string]interface{}{
		"intent":                   intent,
		"llm_delegation_requests":  llmRequests,
		"fallback_delegation_plan": fallback,
		"required_capabilities":    required,
		"completion_applied":       len(added) > 0,
		"added_capabilities":       added,
	}
}

func requiredDelegationCapabilitiesForIntent(intent string) []string {
	switch strings.TrimSpace(strings.ToLower(intent)) {
	case "portfolio_comparison":
		return []string{"evidence.portfolio_comparison"}
	default:
		return nil
	}
}

func stringSliceContains(items []string, want string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == strings.TrimSpace(want) {
			return true
		}
	}
	return false
}

func stringSliceFromPayload(payload map[string]interface{}, key string) []string {
	raw, ok := payload[key].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func inferAnalyticalIntent(input string) string {
	lower := strings.ToLower(input)
	switch {
	case strings.Contains(lower, "compara") || strings.Contains(lower, "cartera"):
		return "portfolio_comparison"
	case strings.Contains(lower, "contradic") || strings.Contains(lower, "calidad") || strings.Contains(lower, "gap"):
		return "data_reconciliation"
	case strings.Contains(lower, "sensibilidad") || strings.Contains(lower, "score"):
		return "score_sensitivity"
	case strings.Contains(lower, "hipótesis") || strings.Contains(lower, "hipotesis"):
		return "alternative_hypothesis"
	case strings.Contains(lower, "contrafactual") || strings.Contains(lower, "qué pasaría"):
		return "counterfactual_scenario"
	case strings.Contains(lower, "recom"):
		return "recommendation_readiness"
	default:
		return "general_deep_analysis"
	}
}

func loadSemanticPack(path string) (semanticPack, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return semanticPack{}, err
	}
	var pack semanticPack
	if err := json.Unmarshal(raw, &pack); err != nil {
		return semanticPack{}, err
	}
	if len(pack.PrimaryEntities) == 0 && pack.CollectionScoring == nil {
		return semanticPack{}, errors.New("semantic pack no declara primary_entities ni collection_scoring")
	}
	return pack, nil
}

func inferScoringModel(pack semanticPack) (collectionScoring, error) {
	if pack.CollectionScoring != nil {
		return *pack.CollectionScoring, nil
	}
	scopeEntity := pack.ScopePolicies.ScopeEntity
	entity := pack.PrimaryEntities[scopeEntity]
	if entity.Table == "" {
		for key, candidate := range pack.PrimaryEntities {
			if strings.Contains(strings.ToLower(key), "client") || strings.Contains(strings.ToLower(candidate.Label), "cliente") {
				entity = candidate
				break
			}
		}
	}
	if entity.Table == "" {
		return collectionScoring{}, errors.New("semantic pack no declara entidad cobrable primaria")
	}
	item := pack.PrimaryEntities["charge"]
	if item.Table == "" {
		for key, candidate := range pack.PrimaryEntities {
			k := strings.ToLower(key + " " + candidate.Label + " " + candidate.Table)
			if strings.Contains(k, "charge") || strings.Contains(k, "cobro") || strings.Contains(k, "invoice") || strings.Contains(k, "debt") {
				item = candidate
				break
			}
		}
	}
	if item.Table == "" {
		return collectionScoring{}, errors.New("semantic pack no declara tabla de cobros/deuda para priorizar")
	}
	return collectionScoring{
		EntityTable:      entity.Table,
		EntityIDColumn:   firstNonEmpty(entity.ScopeKey, "id"),
		EntityNameColumn: firstNonEmpty(entity.DisplayColumn, "name"),
		ItemTable:        item.Table,
		ItemEntityColumn: firstNonEmpty(item.ScopeColumn, singular(entity.Table)+"_id", "client_id", "customer_id"),
	}, nil
}

func scoreSQLite(dbPath string, model collectionScoring) ([]priorityItem, collectionScoring, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_pragma=query_only(true)")
	if err != nil {
		return nil, model, err
	}
	defer db.Close()

	tables, err := inspectTables(db)
	if err != nil {
		return nil, model, err
	}
	entityTable, ok := tables[model.EntityTable]
	if !ok {
		return nil, model, fmt.Errorf("tabla de entidad no existe: %s", model.EntityTable)
	}
	itemTable, ok := tables[model.ItemTable]
	if !ok {
		return nil, model, fmt.Errorf("tabla de cobros no existe: %s", model.ItemTable)
	}

	model.EntityIDColumn = existingColumn(entityTable, model.EntityIDColumn, "id", "code")
	model.EntityNameColumn = existingColumn(entityTable, model.EntityNameColumn, "name", "nombre", "code", model.EntityIDColumn)
	model.ItemEntityColumn = existingColumn(itemTable, model.ItemEntityColumn, singular(model.EntityTable)+"_id", "client_id", "customer_id", "entity_id")
	model.ItemJoinColumn = existingColumn(itemTable, model.ItemJoinColumn, "id")
	model.StatusColumn = existingColumn(itemTable, model.StatusColumn, "status", "state", "estado")
	model.DateColumn = existingColumn(itemTable, model.DateColumn, "due_date", "date", "date_to", "created_at")
	model.AmountColumn = existingColumn(itemTable, model.AmountColumn, "amount", "balance", "saldo", "total", "residue")
	if model.AmountColumn == "" {
		model.AmountTable, model.AmountJoinColumn, model.AmountColumn, model.AmountDateColumn = findAmountTable(tables, model.ItemTable)
	}
	if model.ItemEntityColumn == "" || model.ItemJoinColumn == "" {
		return nil, model, errors.New("no se pudo mapear la relación entre entidad y cobros")
	}
	if model.AmountColumn == "" {
		return nil, model, errors.New("no se pudo mapear columna de monto para scoring")
	}

	rows, err := fetchRawItems(db, model)
	if err != nil {
		return nil, model, err
	}
	payments := fetchPaymentStats(db, tables, model)
	return aggregateItems(rows, payments), model, nil
}

func inspectTables(db *sql.DB) (map[string]tableInfo, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type IN ('table','view')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]tableInfo{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols, err := tableColumns(db, name)
		if err != nil {
			return nil, err
		}
		out[name] = tableInfo{Name: name, Columns: cols}
	}
	return out, rows.Err()
}

func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + quoteIdent(table) + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

func fetchRawItems(db *sql.DB, model collectionScoring) ([]rawItem, error) {
	selects := []string{
		"e." + quoteIdent(model.EntityIDColumn),
		"e." + quoteIdent(model.EntityNameColumn),
		"i." + quoteIdent(model.ItemEntityColumn),
		"i." + quoteIdent(model.ItemJoinColumn),
	}
	if model.StatusColumn != "" {
		selects = append(selects, "i."+quoteIdent(model.StatusColumn))
	} else {
		selects = append(selects, "''")
	}
	if model.AmountTable != "" && model.AmountDateColumn != "" {
		selects = append(selects, "MIN(a."+quoteIdent(model.AmountDateColumn)+")")
	} else if model.DateColumn != "" {
		selects = append(selects, "i."+quoteIdent(model.DateColumn))
	} else {
		selects = append(selects, "''")
	}
	if model.AmountTable != "" {
		selects = append(selects, "COALESCE(SUM(CAST(a."+quoteIdent(model.AmountColumn)+" AS REAL)), 0)")
	} else {
		selects = append(selects, "COALESCE(CAST(i."+quoteIdent(model.AmountColumn)+" AS REAL), 0)")
	}
	query := "SELECT " + strings.Join(selects, ", ") +
		" FROM " + quoteIdent(model.ItemTable) + " i" +
		" JOIN " + quoteIdent(model.EntityTable) + " e ON e." + quoteIdent(model.EntityIDColumn) + " = i." + quoteIdent(model.ItemEntityColumn)
	if model.AmountTable != "" {
		query += " LEFT JOIN " + quoteIdent(model.AmountTable) + " a ON a." + quoteIdent(model.AmountJoinColumn) + " = i." + quoteIdent(model.ItemJoinColumn)
		query += " GROUP BY i." + quoteIdent(model.ItemJoinColumn)
	}
	query += " LIMIT 5000"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []rawItem
	for rows.Next() {
		var entityID, name, fk, itemID, status, dueDate sql.NullString
		var amount sql.NullFloat64
		if err := rows.Scan(&entityID, &name, &fk, &itemID, &status, &dueDate, &amount); err != nil {
			return nil, err
		}
		if !isEligibleStatus(status.String) || amount.Float64 <= 0 {
			continue
		}
		out = append(out, rawItem{
			EntityID:   firstNonEmpty(entityID.String, fk.String),
			EntityName: firstNonEmpty(name.String, entityID.String, fk.String),
			Amount:     amount.Float64,
			DueDate:    dueDate.String,
			Status:     status.String,
			ItemID:     itemID.String,
		})
	}
	return out, rows.Err()
}

func fetchPaymentStats(db *sql.DB, tables map[string]tableInfo, model collectionScoring) map[string]paymentStats {
	tableName, entityColumn, amountColumn, dateColumn := findPaymentTable(tables, model)
	if tableName == "" || entityColumn == "" {
		return map[string]paymentStats{}
	}
	selects := []string{
		quoteIdent(entityColumn),
		"COUNT(*)",
	}
	if amountColumn != "" {
		selects = append(selects, "COALESCE(SUM(CAST("+quoteIdent(amountColumn)+" AS REAL)), 0)")
	} else {
		selects = append(selects, "0")
	}
	if dateColumn != "" {
		selects = append(selects, "MAX("+quoteIdent(dateColumn)+")")
	} else {
		selects = append(selects, "''")
	}
	query := "SELECT " + strings.Join(selects, ", ") + " FROM " + quoteIdent(tableName) + " GROUP BY " + quoteIdent(entityColumn)
	rows, err := db.Query(query)
	if err != nil {
		return map[string]paymentStats{}
	}
	defer rows.Close()
	out := map[string]paymentStats{}
	for rows.Next() {
		var entityID, lastDate sql.NullString
		var count int
		var total sql.NullFloat64
		if err := rows.Scan(&entityID, &count, &total, &lastDate); err != nil {
			continue
		}
		st := paymentStats{Count: count, Total: total.Float64}
		if parsed, ok := parseDate(lastDate.String); ok {
			st.Last = parsed
			st.HasLast = true
		}
		if entityID.String != "" {
			out[entityID.String] = st
		}
	}
	return out
}

func findPaymentTable(tables map[string]tableInfo, model collectionScoring) (table, entityColumn, amountColumn, dateColumn string) {
	candidates := []tableInfo{}
	for _, t := range tables {
		name := strings.ToLower(t.Name)
		if strings.Contains(name, "payment") || strings.Contains(name, "pago") || strings.Contains(name, "receipt") {
			candidates = append(candidates, t)
		}
	}
	for _, t := range candidates {
		entity := existingColumn(t, model.ItemEntityColumn, singular(model.EntityTable)+"_id", "client_id", "customer_id", "entity_id")
		if entity == "" {
			continue
		}
		amount := existingColumn(t, "amount", "monto", "total", "paid_amount", "value")
		date := existingColumn(t, "date", "paid_at", "payment_date", "created_at", "updated_at")
		return t.Name, entity, amount, date
	}
	return "", "", "", ""
}

func aggregateItems(rows []rawItem, payments map[string]paymentStats) []priorityItem {
	now := time.Now()
	agg := map[string]*aggregate{}
	for _, row := range rows {
		if row.EntityID == "" {
			continue
		}
		a := agg[row.EntityID]
		if a == nil {
			a = &aggregate{EntityID: row.EntityID, Name: row.EntityName, Statuses: map[string]bool{}}
			agg[row.EntityID] = a
		}
		a.Amount += row.Amount
		a.Count++
		if row.Status != "" {
			a.Statuses[row.Status] = true
		}
		if parsed, ok := parseDate(row.DueDate); ok && (!a.HasDate || parsed.Before(a.Oldest)) {
			a.Oldest = parsed
			a.HasDate = true
		}
	}
	portfolioTotal := 0.0
	for _, a := range agg {
		portfolioTotal += a.Amount
	}
	items := make([]priorityItem, 0, len(agg))
	for _, a := range agg {
		days := 0
		if a.HasDate {
			days = int(now.Sub(a.Oldest).Hours() / 24)
			if days < 0 {
				days = 0
			}
		}
		pay := payments[a.EntityID]
		breakdown, score, gaps := computeRiskScore(a.Amount, days, portfolioTotal, pay, now)
		reasons := []string{fmt.Sprintf("Saldo total %.0f", a.Amount)}
		if days > 0 {
			reasons = append(reasons, fmt.Sprintf("Más de %d días desde la fecha más antigua", days))
		}
		if a.Count > 1 {
			reasons = append(reasons, fmt.Sprintf("%d documentos/cobros abiertos", a.Count))
		}
		if pay.Count > 0 {
			reasons = append(reasons, fmt.Sprintf("%d pagos históricos registrados", pay.Count))
		}
		items = append(items, priorityItem{
			ArtifactType:    "collection.priority_item.v1",
			Score:           score,
			ScoreBreakdown:  breakdown,
			Strategy:        recommendedStrategy(score, days, gaps),
			AnalysisOptions: recommendedAnalysisOptions(gaps),
			DataGaps:        gaps,
			EntityRef:       entityRef{ArtifactType: "entity.ref.v1", Type: "customer", ID: a.EntityID, Name: a.Name},
			Reasons:         reasons,
			Deudor:          a.Name,
			DeudorID:        a.EntityID,
			SaldoTotal:      a.Amount,
			DiasMoraMax:     days,
			FacturasCount:   a.Count,
			Razon:           strings.Join(reasons, "; "),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].SaldoTotal > items[j].SaldoTotal
		}
		return items[i].Score > items[j].Score
	})
	if len(items) > 10 {
		items = items[:10]
	}
	for i := range items {
		items[i].Rank = i + 1
	}
	return items
}

func loadDatasetArtifactToTempDB(artifactPath string) (string, error) {
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		return "", fmt.Errorf("read artifact: %w", err)
	}
	return writeTempJSONToDB(string(raw))
}

func writeTempJSONToDB(jsonStr string) (string, error) {
	var payload struct {
		Tables map[string][]map[string]interface{} `json:"tables"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		// Try wrapped artifact format (dataset.raw.v1 with tables nested).
		var wrap map[string]interface{}
		if err2 := json.Unmarshal([]byte(jsonStr), &wrap); err2 != nil {
			return "", fmt.Errorf("parse artifact: %w", err)
		}
		if t, ok := wrap["tables"].(map[string]interface{}); ok {
			payload.Tables = make(map[string][]map[string]interface{}, len(t))
			for name, rows := range t {
				if arr, ok := rows.([]interface{}); ok {
					var records []map[string]interface{}
					for _, r := range arr {
						if rec, ok := r.(map[string]interface{}); ok {
							records = append(records, rec)
						}
					}
					payload.Tables[name] = records
				}
			}
		}
	}
	if len(payload.Tables) == 0 {
		return "", errors.New("artifact has no tables")
	}

	f, err := os.CreateTemp("", "radar_dataset_*.db")
	if err != nil {
		return "", err
	}
	f.Close()
	db, err := sql.Open("sqlite", f.Name())
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	defer db.Close()

	for tableName, rows := range payload.Tables {
		if len(rows) == 0 {
			continue
		}
		cols := make([]string, 0, len(rows[0]))
		for k := range rows[0] {
			cols = append(cols, k)
		}
		sort.Strings(cols)
		var createCols []string
		for _, c := range cols {
			createCols = append(createCols, quoteIdent(c)+" TEXT")
		}
		createSQL := "CREATE TABLE " + quoteIdent(tableName) + " (" + strings.Join(createCols, ", ") + ")"
		if _, err := db.Exec(createSQL); err != nil {
			continue
		}
		placeholders := make([]string, len(cols))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		insertSQL := "INSERT INTO " + quoteIdent(tableName) + " (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
		stmt, err := db.Prepare(insertSQL)
		if err != nil {
			continue
		}
		for _, row := range rows {
			vals := make([]interface{}, len(cols))
			for i, c := range cols {
				if v, ok := row[c]; ok {
					vals[i] = fmt.Sprint(v)
				} else {
					vals[i] = nil
				}
			}
			_, _ = stmt.Exec(vals...)
		}
		stmt.Close()
	}
	return f.Name(), nil
}

type analysisPlanPaths struct {
	SchemaPath string
	PlanPath   string
	SQLPath    string
}

func emitPriorityList(businessID string, items []priorityItem, model collectionScoring, plan analysisPlanPaths) {
	out := map[string]interface{}{
		"artifact_type":  "collection.priority_list.v1",
		"artifacts":      []string{"collection.priority_list.v1", "collection.priority_item.v1", "entity.ref.v1", "risk.score.v1", "strategy.recommendation.v1", "data.gaps.v1", "analysis.schema.v1"},
		"business_id":    businessID,
		"generated_at":   time.Now().UTC().Format(time.RFC3339),
		"scoring_model":  "collection_priority_40_30_30_v1",
		"scoring_source": "framework-radar",
		"analysis_schema": map[string]interface{}{
			"artifact_type": "analysis.schema.v1",
			"schema_id":     "collection_priority_40_30_30_v1",
			"weights":       map[string]int{"materialidad": 40, "comportamiento": 30, "riesgo_legal": 30},
			"path":          plan.SchemaPath,
			"plan_path":     plan.PlanPath,
			"sql_path":      plan.SQLPath,
		},
		"items": items,
		"count": len(items),
		"trace": map[string]string{
			"entity_table": model.EntityTable,
			"item_table":   model.ItemTable,
		},
	}
	if len(items) > 0 {
		out["selected"] = items[0].EntityRef
		out["priority_item"] = items[0]
	}
	printJSON(out)
}

func analysisModelPayload(model collectionScoring) map[string]string {
	return map[string]string{
		"entity_table":       model.EntityTable,
		"entity_id_column":   model.EntityIDColumn,
		"entity_name_column": model.EntityNameColumn,
		"item_table":         model.ItemTable,
		"item_entity_column": model.ItemEntityColumn,
		"item_join_column":   model.ItemJoinColumn,
		"amount_table":       model.AmountTable,
		"amount_column":      model.AmountColumn,
		"date_column":        firstNonEmpty(model.AmountDateColumn, model.DateColumn),
		"status_column":      model.StatusColumn,
	}
}

func decodeContextPayload(contextB64 string) map[string]interface{} {
	contextB64 = strings.TrimSpace(contextB64)
	if contextB64 == "" {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(contextB64)
	if err != nil {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload
}

func radarDeepDiveTarget(priorityListJSON, entityRef, entityType string) map[string]interface{} {
	target := map[string]interface{}{
		"artifact_type": "entity.ref.v1",
		"type":          firstNonEmpty(strings.TrimSpace(entityType), "customer"),
		"id":            strings.TrimSpace(entityRef),
	}
	if payload := parseJSONObject(priorityListJSON); len(payload) > 0 {
		if selected, ok := payload["selected"].(map[string]interface{}); ok {
			mergeStringAnyMaps(target, selected)
		}
		if item := radarDeepDivePriorityItem(priorityListJSON, strings.TrimSpace(entityRef)); len(item) > 0 {
			if name := jsonStringFromMap(item, "deudor", "name"); name != "" && target["name"] == nil {
				target["name"] = name
			}
			if saldo, ok := item["saldo_total"]; ok {
				target["saldo_total"] = saldo
			}
			if dias, ok := item["dias_mora_max"]; ok {
				target["dias_mora_max"] = dias
			}
			if rank, ok := item["rank"]; ok {
				target["rank"] = rank
			}
		}
	}
	if target["id"] == nil || strings.TrimSpace(fmt.Sprint(target["id"])) == "" {
		target["id"] = strings.TrimSpace(entityRef)
	}
	return target
}

func radarDeepDivePriorityItem(priorityListJSON, entityRef string) map[string]interface{} {
	payload := parseJSONObject(priorityListJSON)
	if len(payload) == 0 {
		return nil
	}
	entityRef = strings.TrimSpace(entityRef)
	if item, ok := payload["priority_item"].(map[string]interface{}); ok {
		if entityRef == "" || radarDeepDivePriorityItemMatches(item, entityRef) {
			return item
		}
	}
	if items, ok := payload["items"].([]interface{}); ok {
		for _, raw := range items {
			item, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if entityRef == "" || radarDeepDivePriorityItemMatches(item, entityRef) {
				return item
			}
		}
	}
	return nil
}

func radarDeepDivePriorityItemMatches(item map[string]interface{}, entityRef string) bool {
	entityRef = strings.TrimSpace(entityRef)
	if entityRef == "" {
		return true
	}
	if jsonStringFromMap(item, "deudor_id", "entity_id", "id") == entityRef {
		return true
	}
	if ref, ok := item["entity_ref"].(map[string]interface{}); ok {
		return jsonStringFromMap(ref, "id") == entityRef
	}
	return false
}

func radarDeepDiveRecommendation(strategyJSON string) map[string]interface{} {
	payload := parseJSONObject(strategyJSON)
	if len(payload) == 0 {
		return map[string]interface{}{
			"recommended_action": "deep_analysis",
			"reason":             "Sin strategy.recommendation.v1 explícita, Radar mantiene el tramo analítico abierto.",
		}
	}
	return payload
}

func radarDeepDiveWhyPrioritized(priorityItem, recommendation map[string]interface{}) string {
	reasons := interfaceStringSlice(priorityItem["reasons"])
	score := jsonIntFromAny(priorityItem["score"])
	rank := jsonIntFromAny(priorityItem["rank"])
	scoreBits := []string{}
	if rank > 0 {
		scoreBits = append(scoreBits, fmt.Sprintf("quedó rankeado #%d", rank))
	}
	if score > 0 {
		scoreBits = append(scoreBits, fmt.Sprintf("score %d/100", score))
	}
	if len(reasons) > 0 {
		if len(scoreBits) > 0 {
			return fmt.Sprintf("Se priorizó porque %s y %s.", joinNaturalList(scoreBits), strings.ToLower(strings.Join(reasons, "; ")))
		}
		return "Se priorizó porque " + strings.ToLower(strings.Join(reasons, "; ")) + "."
	}
	if reason := jsonStringFromMap(recommendation, "reason", "summary"); reason != "" {
		if len(scoreBits) > 0 {
			return fmt.Sprintf("Se priorizó porque %s; %s", joinNaturalList(scoreBits), strings.ToLower(reason))
		}
		return reason
	}
	if len(scoreBits) > 0 {
		return "Se priorizó porque " + joinNaturalList(scoreBits) + "."
	}
	return "Se priorizó por la combinación de exposición, antigüedad y señales de riesgo disponibles."
}

func radarDeepDiveEvidenceSummary(target, priorityItem map[string]interface{}) []string {
	evidence := []string{}
	if saldo := jsonFloatFromAny(firstNonNil(priorityItem["saldo_total"], target["saldo_total"])); saldo > 0 {
		evidence = append(evidence, fmt.Sprintf("saldo abierto estimado: %s", formatCurrencyCompact(saldo)))
	}
	if dias := jsonIntFromAny(firstNonNil(priorityItem["dias_mora_max"], target["dias_mora_max"])); dias > 0 {
		evidence = append(evidence, fmt.Sprintf("mora máxima observada: %d días", dias))
	}
	if count := jsonIntFromAny(priorityItem["facturas_count"]); count > 0 {
		evidence = append(evidence, fmt.Sprintf("documentos abiertos detectados: %d", count))
	}
	for key, value := range intMapFromAny(priorityItem["score_breakdown"]) {
		if value <= 0 {
			continue
		}
		evidence = append(evidence, fmt.Sprintf("señal %s: %d puntos", key, value))
	}
	if len(evidence) == 0 {
		evidence = append(evidence, "el caso aparece como seleccionado en la lista priorizada actual")
	}
	return uniqueNonEmptyStrings(evidence)
}

func radarDeepDiveRiskSummary(priorityItem, recommendation map[string]interface{}) string {
	score := jsonIntFromAny(priorityItem["score"])
	strategy := jsonStringFromMap(priorityItem, "strategy")
	switch {
	case strategy != "" && score > 0:
		return fmt.Sprintf("Riesgo estimado %d/100. %s", score, strategy)
	case strategy != "":
		return strategy
	case score >= 75:
		return fmt.Sprintf("Riesgo alto (%d/100): la combinación de exposición y antigüedad justifica frenar acciones operativas hasta validar evidencia.", score)
	case score >= 50:
		return fmt.Sprintf("Riesgo medio (%d/100): conviene confirmar contexto y huecos de datos antes de accionar.", score)
	case score > 0:
		return fmt.Sprintf("Riesgo preliminar %d/100: el caso merece lectura adicional, pero con menor urgencia relativa.", score)
	}
	if reason := jsonStringFromMap(recommendation, "reason"); reason != "" {
		return reason
	}
	return "Riesgo preliminar sin score explícito: Radar conserva el tramo analítico hasta reunir más contexto."
}

func radarDeepDiveAgingSummary(target, priorityItem map[string]interface{}) string {
	dias := jsonIntFromAny(firstNonNil(priorityItem["dias_mora_max"], target["dias_mora_max"]))
	if dias <= 0 {
		return "No hay antigüedad de mora explícita en el artifact actual."
	}
	switch {
	case dias >= 120:
		return fmt.Sprintf("La mora ya está en %d días o más, una señal fuerte de exposición de cobranza.", dias)
	case dias >= 60:
		return fmt.Sprintf("La mora alcanza %d días, suficiente para justificar revisión detallada antes de actuar.", dias)
	default:
		return fmt.Sprintf("La mora observada es de %d días; no es extrema, pero sí relevante para el contexto del caso.", dias)
	}
}

func radarDeepDiveInvoiceSummary(target, priorityItem map[string]interface{}) string {
	saldo := jsonFloatFromAny(firstNonNil(priorityItem["saldo_total"], target["saldo_total"]))
	count := jsonIntFromAny(priorityItem["facturas_count"])
	switch {
	case saldo > 0 && count > 0:
		return fmt.Sprintf("Se observan %d documentos abiertos por un saldo total aproximado de %s.", count, formatCurrencyCompact(saldo))
	case saldo > 0:
		return fmt.Sprintf("Se observa un saldo abierto aproximado de %s.", formatCurrencyCompact(saldo))
	case count > 0:
		return fmt.Sprintf("Se observan %d documentos abiertos en el caso.", count)
	default:
		return "No hay detalle consolidado de documentos en el artifact actual."
	}
}

func radarDeepDiveBlockingGaps(priorityItem map[string]interface{}) []string {
	gaps := uniqueNonEmptyStrings(interfaceStringSlice(priorityItem["data_gaps"]))
	if len(gaps) == 0 {
		return []string{"No aparece un gap crítico estructurado en la priorización; queda pendiente validarlo con Auditor."}
	}
	out := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		out = append(out, fmt.Sprintf("falta validar %s", strings.TrimSpace(gap)))
	}
	return out
}

func radarDeepDiveNonOperationalNextSteps(blockingGaps []string) []string {
	steps := []string{
		"pedir a Sabio contexto 360 del cliente para entender relación, historial y señales adicionales",
		"pedir a Auditor validación de calidad para confirmar qué evidencia falta o reduce confianza",
		"comparar este caso contra otros casos priorizados antes de moverlo a ejecución",
	}
	if len(blockingGaps) > 0 {
		steps = append(steps, "cerrar primero las brechas críticas de información antes de redactar o enviar nada")
	}
	return uniqueNonEmptyStrings(steps)
}

func radarDeepDiveOperationalNextSteps(recommendation map[string]interface{}, blockingGaps []string) []string {
	action := strings.ToLower(jsonStringFromMap(recommendation, "recommended_action", "action"))
	reason := jsonStringFromMap(recommendation, "reason")
	switch action {
	case "send_email", "send":
		return []string{"si luego quisieras actuar, preparar un borrador de email revisado y verificar destinatario/credenciales antes del envío"}
	case "quick_action", "proceed":
		return []string{"si luego quisieras actuar, convertir este análisis en una acción rápida controlada una vez cerradas las brechas"}
	}
	if len(blockingGaps) > 0 {
		return []string{"si luego quisieras actuar, primero resolver los gaps de datos y después evaluar draft + envío como paso separado"}
	}
	if reason != "" {
		return []string{"si luego quisieras actuar, usar esta recomendación como guía operativa posterior: " + strings.ToLower(reason)}
	}
	return []string{"si luego quisieras actuar, el paso natural sería preparar un borrador o contactar al cliente, pero sin ejecutarlo desde este tramo"}
}

func radarDeepDiveOpenQuestions(target, priorityItem map[string]interface{}, blockingGaps []string) []string {
	questions := []string{}
	if len(blockingGaps) > 0 {
		questions = append(questions, "¿qué impacto tienen las brechas de datos sobre la confianza del scoring?")
	}
	if jsonIntFromAny(firstNonNil(priorityItem["dias_mora_max"], target["dias_mora_max"])) == 0 {
		questions = append(questions, "¿cuál es la antigüedad real de la mora de este caso?")
	}
	if jsonFloatFromAny(firstNonNil(priorityItem["saldo_total"], target["saldo_total"])) == 0 {
		questions = append(questions, "¿cuál es el saldo exigible real que explica la prioridad?")
	}
	questions = append(questions, "¿este caso está por encima de pares similares por exposición, comportamiento o riesgo de cobro?")
	return uniqueNonEmptyStrings(questions)
}

func radarDeepDiveNarrative(name, whyPrioritized string, evidence []string, riskSummary, agingSummary, invoiceSummary string, blockingGaps, nextNonOperational, nextOperational, openQuestions []string) string {
	sections := []string{
		"Resumen del análisis profundo de " + name,
		"Por qué se priorizó: " + strings.TrimSpace(whyPrioritized),
		"Qué sabemos: " + strings.Join(uniqueNonEmptyStrings(append(evidence, riskSummary, agingSummary, invoiceSummary)), " | "),
		"Qué falta: " + strings.Join(uniqueNonEmptyStrings(blockingGaps), " | "),
		"Qué haría después, sin ejecutarlo: " + strings.Join(uniqueNonEmptyStrings(append(nextNonOperational, nextOperational...)), " | "),
	}
	if len(openQuestions) > 0 {
		sections = append(sections, "Preguntas abiertas: "+strings.Join(openQuestions, " | "))
	}
	return strings.Join(sections, "\n\n")
}

func interfaceStringSlice(raw interface{}) []string {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func intMapFromAny(raw interface{}) map[string]int {
	items, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	out := map[string]int{}
	for key, value := range items {
		if n := jsonIntFromAny(value); n != 0 {
			out[key] = n
		}
	}
	return out
}

func jsonIntFromAny(raw interface{}) int {
	switch v := raw.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return 0
}

func jsonFloatFromAny(raw interface{}) float64 {
	switch v := raw.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	}
	return 0
}

func firstNonNil(values ...interface{}) interface{} {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func formatCurrencyCompact(v float64) string {
	if v == 0 {
		return "0"
	}
	if math.Abs(v-math.Round(v)) < 0.01 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func uniqueNonEmptyStrings(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func joinNaturalList(items []string) string {
	items = uniqueNonEmptyStrings(items)
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " y " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " y " + items[len(items)-1]
	}
}

func loadJSONPayload(pathArg, inline string) string {
	if strings.TrimSpace(pathArg) != "" {
		if raw, err := os.ReadFile(pathArg); err == nil {
			return string(raw)
		}
	}
	return strings.TrimSpace(inline)
}

func parseJSONObject(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return payload
}

func mergeStringAnyMaps(dst map[string]interface{}, src map[string]interface{}) {
	for key, value := range src {
		if value == nil {
			continue
		}
		dst[key] = value
	}
}

func jsonStringFromMap(payload map[string]interface{}, fields ...string) string {
	for _, field := range fields {
		if value, ok := payload[field]; ok && value != nil {
			if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func radarAnalysisDir(businessID string) string {
	return filepath.Join("temp", "radar", safePathPart(firstNonEmpty(businessID, "default")))
}

func loadPersistedAnalysisPlan(businessID string) (collectionScoring, bool) {
	path := filepath.Join(radarAnalysisDir(businessID), "collection_analysis_plan.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return collectionScoring{}, false
	}
	var plan struct {
		Model collectionScoring `json:"model"`
	}
	if err := json.Unmarshal(raw, &plan); err != nil {
		return collectionScoring{}, false
	}
	if plan.Model.EntityTable == "" || plan.Model.ItemTable == "" {
		return collectionScoring{}, false
	}
	return plan.Model, true
}

func emitNeedsConfiguration(businessID, code, message string) {
	out := map[string]interface{}{
		"artifact_type":        "collection.priority_list.v1",
		"artifacts":            []string{"collection.priority_list.v1"},
		"business_id":          businessID,
		"generated_at":         time.Now().UTC().Format(time.RFC3339),
		"items":                []interface{}{},
		"count":                0,
		"needs_configuration":  true,
		"configuration_code":   code,
		"configuration_reason": message,
	}
	if strings.Contains(code, "db") || strings.Contains(code, "dataset") || strings.Contains(code, "query") {
		out["artifacts"] = []string{"collection.priority_list.v1", "data.request.v1"}
		out["request"] = map[string]interface{}{
			"artifact_type": "data.request.v1",
			"target":        "sabio",
			"capability":    "dataset.export",
			"reason":        message,
			"code":          code,
		}
	}
	printJSON(out)
}

func persistAnalysisPlan(businessID string, model collectionScoring) analysisPlanPaths {
	dir := radarAnalysisDir(businessID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return analysisPlanPaths{}
	}
	schemaPath := filepath.Join(dir, "collection_analysis_schema.json")
	planPath := filepath.Join(dir, "collection_analysis_plan.json")
	sqlPath := filepath.Join(dir, "collection_priority_query.sql")
	sqlText := analysisSQLTemplate(model)
	schema := map[string]interface{}{
		"artifact_type": "analysis.schema.v1",
		"schema_id":     "collection_priority_40_30_30_v1",
		"framework":     "radar",
		"weights":       map[string]int{"materialidad": 40, "comportamiento": 30, "riesgo_legal": 30},
		"model":         analysisModelPayload(model),
		"plan_path":     planPath,
		"sql_path":      sqlPath,
		"notes": []string{
			"Esquema genérico inferido desde semantic pack/dataset y persistido como plan tangible.",
			"Radar no ejecuta acciones operativas; solo analiza y prioriza.",
			"Los ciclos siguientes reutilizan collection_analysis_plan.json salvo que el usuario reconfigure el análisis.",
		},
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	plan := map[string]interface{}{
		"artifact_type": "analysis.plan.v1",
		"plan_id":       "collection_priority_40_30_30_v1",
		"framework":     "radar",
		"business_id":   businessID,
		"model":         model,
		"weights":       map[string]int{"materialidad": 40, "comportamiento": 30, "riesgo_legal": 30},
		"sql_file":      sqlPath,
		"schema_file":   schemaPath,
		"reconfigure_by": []string{
			"Ejecutar configure-analysis con un semantic pack actualizado.",
			"Reemplazar este plan mediante una acción aprobada por el usuario/staff.",
		},
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	if raw, err := json.MarshalIndent(schema, "", "  "); err == nil {
		_ = os.WriteFile(schemaPath, raw, 0644)
	}
	if raw, err := json.MarshalIndent(plan, "", "  "); err == nil {
		_ = os.WriteFile(planPath, raw, 0644)
	}
	_ = os.WriteFile(sqlPath, []byte(sqlText), 0644)
	return analysisPlanPaths{SchemaPath: schemaPath, PlanPath: planPath, SQLPath: sqlPath}
}

func analysisSQLTemplate(model collectionScoring) string {
	var sb strings.Builder
	sb.WriteString("-- Radar collection priority query\n")
	sb.WriteString("-- Generated from business semantic pack and persisted as the tangible analysis plan.\n")
	sb.WriteString("-- Runtime scoring weights: materialidad=40, comportamiento=30, riesgo_legal=30.\n")
	sb.WriteString("SELECT\n")
	sb.WriteString("  e." + quoteIdent(model.EntityIDColumn) + " AS entity_id,\n")
	sb.WriteString("  e." + quoteIdent(model.EntityNameColumn) + " AS entity_name,\n")
	sb.WriteString("  i." + quoteIdent(model.ItemJoinColumn) + " AS item_id,\n")
	if model.StatusColumn != "" {
		sb.WriteString("  i." + quoteIdent(model.StatusColumn) + " AS status,\n")
	} else {
		sb.WriteString("  '' AS status,\n")
	}
	dateExpr := "''"
	if model.AmountTable != "" && model.AmountDateColumn != "" {
		dateExpr = "MIN(a." + quoteIdent(model.AmountDateColumn) + ")"
	} else if model.DateColumn != "" {
		dateExpr = "i." + quoteIdent(model.DateColumn)
	}
	sb.WriteString("  " + dateExpr + " AS due_date,\n")
	if model.AmountTable != "" {
		sb.WriteString("  COALESCE(SUM(CAST(a." + quoteIdent(model.AmountColumn) + " AS REAL)), 0) AS amount\n")
	} else {
		sb.WriteString("  COALESCE(CAST(i." + quoteIdent(model.AmountColumn) + " AS REAL), 0) AS amount\n")
	}
	sb.WriteString("FROM " + quoteIdent(model.ItemTable) + " i\n")
	sb.WriteString("JOIN " + quoteIdent(model.EntityTable) + " e ON e." + quoteIdent(model.EntityIDColumn) + " = i." + quoteIdent(model.ItemEntityColumn) + "\n")
	if model.AmountTable != "" {
		sb.WriteString("LEFT JOIN " + quoteIdent(model.AmountTable) + " a ON a." + quoteIdent(model.AmountJoinColumn) + " = i." + quoteIdent(model.ItemJoinColumn) + "\n")
		sb.WriteString("GROUP BY i." + quoteIdent(model.ItemJoinColumn) + "\n")
	}
	sb.WriteString("LIMIT 5000;\n")
	return sb.String()
}

func safePathPart(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '_'
		}
	}, s)
}

func findAmountTable(tables map[string]tableInfo, itemTable string) (table, joinColumn, amountColumn, dateColumn string) {
	itemFK := singular(itemTable) + "_id"
	for _, t := range tables {
		join := existingColumn(t, itemFK, "charge_id", "invoice_id", "debt_id", "item_id")
		amount := existingColumn(t, "amount", "balance", "saldo", "total", "residue")
		if join != "" && amount != "" {
			date := existingColumn(t, "due_date", "date", "created_at", "updated_at")
			return t.Name, join, amount, date
		}
	}
	return "", "", "", ""
}

func existingColumn(table tableInfo, candidates ...string) string {
	for _, c := range candidates {
		if c != "" && table.Columns[c] {
			return c
		}
	}
	return ""
}

func isEligibleStatus(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "" {
		return true
	}
	blocked := []string{"pagado", "paid", "cancel", "anulad", "void", "closed", "cerrad", "cobrado"}
	for _, token := range blocked {
		if strings.Contains(s, token) {
			return false
		}
	}
	return true
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if len(s) >= 10 {
		s = s[:10]
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006", "2006/01/02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func computeRiskScore(amount float64, days int, portfolioTotal float64, pay paymentStats, now time.Time) (map[string]int, int, []string) {
	gaps := []string{}
	materiality := 0
	if portfolioTotal > 0 {
		materiality = clampScore((amount / portfolioTotal) * 300)
	} else if amount > 0 {
		materiality = 50
		gaps = append(gaps, "No hay total de portafolio suficiente para materialidad relativa.")
	}
	legal := clampScore(float64(days) / 365.0 * 100)
	behavior := 0
	if pay.Count == 0 {
		behavior = 70
		gaps = append(gaps, "No hay pagos históricos para calcular desviación de comportamiento.")
	} else if pay.HasLast {
		daysSincePayment := int(now.Sub(pay.Last).Hours() / 24)
		if daysSincePayment < 0 {
			daysSincePayment = 0
		}
		behavior = clampScore(float64(daysSincePayment) / 180.0 * 100)
	} else {
		behavior = 50
		gaps = append(gaps, "Hay pagos históricos, pero sin fecha confiable de último pago.")
	}
	breakdown := map[string]int{
		"materialidad":   materiality,
		"comportamiento": behavior,
		"riesgo_legal":   legal,
	}
	score := int(math.Round(float64(materiality)*0.40 + float64(behavior)*0.30 + float64(legal)*0.30))
	return breakdown, score, gaps
}

func clampScore(v float64) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return int(math.Round(v))
}

func recommendedStrategy(score, days int, gaps []string) string {
	switch {
	case len(gaps) > 0:
		return "Lectura de riesgo incompleta: los datos faltantes reducen confianza del scoring."
	case score >= 75 || days >= 120:
		return "Riesgo alto: alta materialidad o antigüedad explican la prioridad del caso."
	case score >= 50:
		return "Riesgo medio: conviene revisar comportamiento histórico y evidencia antes de operar."
	default:
		return "Riesgo bajo: prioridad menor frente a casos de mayor materialidad o exposición."
	}
}

func recommendedAnalysisOptions(gaps []string) []string {
	if len(gaps) > 0 {
		return []string{"Revisar datos faltantes", "Ajustar criterio de scoring", "Aceptar esquema y continuar"}
	}
	return []string{"Aceptar esquema de análisis", "Ver explicación del scoring", "Ajustar ponderaciones"}
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func singular(s string) string {
	s = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), "s")
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func printJSON(v interface{}) {
	raw, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(raw))
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "radar_error: "+format+"\n", args...)
	os.Exit(1)
}
