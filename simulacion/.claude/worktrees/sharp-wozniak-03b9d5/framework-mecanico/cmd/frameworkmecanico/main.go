// frameworkmecanico: agente reparador. Toma findings producidos por
// framework-auditor, propone fixes, y los aplica solo cuando el usuario
// los confirma.
//
// Comandos:
//
//	./frameworkmecanico propose --finding-id F-001
//	./frameworkmecanico propose-all-auto
//	./frameworkmecanico list-proposals [--json]
//	./frameworkmecanico apply --proposal-id P-001
//	./frameworkmecanico apply-all
//	./frameworkmecanico reset
//	./frameworkmecanico next-question
//	./frameworkmecanico ingest-answer --question-id <id> --answer <text>
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"framework-mecanico/fixers"
	"framework-mecanico/internal/auditdata"
	"framework-mecanico/internal/llm"
)

const (
	defaultFindings   = "../framework-auditor/data/findings.json"
	defaultDataset    = "../framework-auditor/data/dataset.working.json"
	defaultProposals  = "data/proposals.json"
	defaultAppliedLog = "data/applied.jsonl"
	defaultState      = "temp/state.json"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "propose":
		cmdPropose(os.Args[2:])
	case "propose-all-auto":
		cmdProposeAllAuto(os.Args[2:])
	case "list-proposals":
		cmdListProposals(os.Args[2:])
	case "apply":
		cmdApply(os.Args[2:])
	case "apply-all":
		cmdApplyAll(os.Args[2:])
	case "draft-email":
		cmdDraftEmail(os.Args[2:])
	case "resolve-gaps":
		cmdResolveGaps(os.Args[2:])
	case "reset":
		cmdReset(os.Args[2:])
	case "next-question":
		cmdNextQuestion(os.Args[2:])
	case "ingest-answer":
		cmdIngestAnswer(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`frameworkmecanico: aplica fixes propuestos a partir de findings del auditor

Uso:
  frameworkmecanico propose --finding-id F-001
  frameworkmecanico propose-all-auto
  frameworkmecanico list-proposals [--json]
  frameworkmecanico apply --proposal-id P-001
  frameworkmecanico apply-all
  frameworkmecanico draft-email --deudor <nombre> [--to <email>] [--saldo <monto>] [--dias-mora <dias>]
  frameworkmecanico resolve-gaps --data-gaps-json <json> [--findings-json <json>] [--entity-ref-json <json>]
  frameworkmecanico reset
  frameworkmecanico next-question
  frameworkmecanico ingest-answer --question-id <id> --answer <text>

Variables de entorno:
  MECANICO_FINDINGS  override findings.json
  MECANICO_DATASET   override dataset.working.json
  MECANICO_PROPOSALS override proposals.json
  MECANICO_APPLIED   override applied.jsonl
  MECANICO_STATE     override state.json
`)
}

func resolvePath(flagVal, envKey, def string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return def
}

func paths() (findingsP, datasetP, proposalsP, appliedP, stateP string) {
	findingsP = resolvePath("", "MECANICO_FINDINGS", defaultFindings)
	datasetP = resolvePath("", "MECANICO_DATASET", defaultDataset)
	proposalsP = resolvePath("", "MECANICO_PROPOSALS", defaultProposals)
	appliedP = resolvePath("", "MECANICO_APPLIED", defaultAppliedLog)
	stateP = resolvePath("", "MECANICO_STATE", defaultState)
	return
}

// loadFindingsOrJSON loads findings from a JSON string if non-empty,
// then from MECANICO_FINDINGS_JSON env var, otherwise from file path.
func loadFindingsOrJSON(jsonStr, path string) ([]auditdata.Finding, error) {
	if strings.TrimSpace(jsonStr) != "" {
		return auditdata.ParseFindings([]byte(jsonStr))
	}
	if v := os.Getenv("MECANICO_FINDINGS_JSON"); v != "" {
		return auditdata.ParseFindings([]byte(v))
	}
	return auditdata.LoadFindings(path)
}

func loadFindingsOrJSONPath(jsonStr, pathArg, defaultPath string) ([]auditdata.Finding, error) {
	if strings.TrimSpace(pathArg) != "" {
		raw, err := os.ReadFile(pathArg)
		if err != nil {
			return nil, err
		}
		return auditdata.ParseFindings(raw)
	}
	return loadFindingsOrJSON(jsonStr, defaultPath)
}

// loadDatasetOrJSON loads dataset from a JSON string if non-empty,
// then from MECANICO_DATASET_JSON env var, otherwise from file path.
func loadDatasetOrJSON(jsonStr, path string) (*auditdata.Dataset, error) {
	if strings.TrimSpace(jsonStr) != "" {
		return auditdata.ParseDataset([]byte(jsonStr))
	}
	if v := os.Getenv("MECANICO_DATASET_JSON"); v != "" {
		return auditdata.ParseDataset([]byte(v))
	}
	return auditdata.LoadDataset(path)
}

func loadDatasetOrJSONPath(jsonStr, pathArg, defaultPath string) (*auditdata.Dataset, error) {
	if strings.TrimSpace(pathArg) != "" {
		raw, err := os.ReadFile(pathArg)
		if err != nil {
			return nil, err
		}
		return auditdata.ParseDataset(raw)
	}
	return loadDatasetOrJSON(jsonStr, defaultPath)
}

// ---------- propose ----------

func cmdPropose(args []string) {
	fs := flag.NewFlagSet("propose", flag.ExitOnError)
	findingID := fs.String("finding-id", "", "id del finding")
	findingsPath := fs.String("findings-path", "", "ruta a archivo JSON de findings")
	findingsJSON := fs.String("findings-json", "", "findings como JSON string (artifact)")
	datasetPath := fs.String("dataset-path", "", "ruta a archivo JSON de dataset")
	datasetJSON := fs.String("dataset-json", "", "dataset como JSON string (artifact)")
	fs.Parse(args)
	if *findingID == "" {
		fail("propose: --finding-id requerido")
	}
	fp, dp, pp, _, _ := paths()
	finds, err := loadFindingsOrJSONPath(*findingsJSON, *findingsPath, fp)
	if err != nil {
		fail("load findings: %v", err)
	}
	var target *auditdata.Finding
	for i := range finds {
		if finds[i].ID == *findingID {
			target = &finds[i]
			break
		}
	}
	if target == nil {
		fail("finding %s no existe", *findingID)
	}
	ds, err := loadDatasetOrJSONPath(*datasetJSON, *datasetPath, dp)
	if err != nil {
		fail("load dataset: %v", err)
	}
	existing, _ := fixers.LoadProposals(pp)
	idx := nextProposalIdx(existing)
	prop := fixers.ProposeForFinding(*target, ds, idx)
	if prop == nil {
		fail("no hay estrategia auto para %s (rule=%s, auto=%v)", target.ID, target.Rule, target.AutoFixable)
	}
	existing = append(existing, *prop)
	if err := fixers.SaveProposals(pp, existing); err != nil {
		fail("save proposals: %v", err)
	}
	printProposal(*prop)
}

func cmdProposeAllAuto(args []string) {
	fs := flag.NewFlagSet("propose-all-auto", flag.ExitOnError)
	findingsJSON := fs.String("findings-json", "", "findings como JSON string (artifact)")
	findingsPath := fs.String("findings-path", "", "ruta a archivo JSON de findings")
	datasetJSON := fs.String("dataset-json", "", "dataset como JSON string (artifact)")
	datasetPath := fs.String("dataset-path", "", "ruta a archivo JSON de dataset")
	fs.Parse(args)
	fp, dp, pp, _, _ := paths()
	finds, err := loadFindingsOrJSONPath(*findingsJSON, *findingsPath, fp)
	if err != nil {
		fail("load findings: %v", err)
	}
	ds, err := loadDatasetOrJSONPath(*datasetJSON, *datasetPath, dp)
	if err != nil {
		fail("load dataset: %v", err)
	}
	existing, _ := fixers.LoadProposals(pp)
	// Construimos un set de finding_ids ya propuestos para no duplicar.
	already := map[string]bool{}
	for _, p := range existing {
		already[p.FindingID] = true
	}
	idx := nextProposalIdx(existing)
	added := 0
	for _, f := range finds {
		if !f.AutoFixable {
			continue
		}
		if already[f.ID] {
			continue
		}
		prop := fixers.ProposeForFinding(f, ds, idx)
		if prop == nil {
			continue
		}
		existing = append(existing, *prop)
		idx++
		added++
	}
	if err := fixers.SaveProposals(pp, existing); err != nil {
		fail("save proposals: %v", err)
	}
	fmt.Printf("Generadas %d propuestas nuevas (total pendientes: %d).\n", added, len(existing))
	for _, p := range fixers.SortedProposals(existing) {
		fmt.Printf("  %s [%s:%s.%s] %v → %v\n", p.ID, p.Endpoint, p.RecordID, p.Field,
			displayValue(p.CurrentValue), displayValue(p.ProposedValue))
	}
}

func nextProposalIdx(existing []fixers.Proposal) int {
	max := 0
	for _, p := range existing {
		var n int
		if _, err := fmt.Sscanf(p.ID, "P-%d", &n); err == nil {
			if n > max {
				max = n
			}
		}
	}
	return max + 1
}

// ---------- list-proposals ----------

func cmdListProposals(args []string) {
	fs := flag.NewFlagSet("list-proposals", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "salida JSON")
	fs.Parse(args)
	_, _, pp, _, _ := paths()
	props, err := fixers.LoadProposals(pp)
	if err != nil {
		fail("load proposals: %v", err)
	}
	if *asJSON {
		emitJSON(props)
		return
	}
	if len(props) == 0 {
		fmt.Println("Sin propuestas pendientes.")
		return
	}
	for _, p := range fixers.SortedProposals(props) {
		printProposal(p)
		fmt.Println("---")
	}
}

func printProposal(p fixers.Proposal) {
	fmt.Printf("%s  (corrige %s)\n", p.ID, p.FindingID)
	fmt.Printf("  endpoint: %s   record: %s   campo: %s\n", p.Endpoint, p.RecordID, p.Field)
	fmt.Printf("  estrategia: %s\n", p.Strategy)
	fmt.Printf("  valor actual:    %v\n", displayValue(p.CurrentValue))
	fmt.Printf("  valor propuesto: %v\n", displayValue(p.ProposedValue))
	fmt.Printf("  por qué: %s\n", p.Rationale)
	if p.RequiresUser {
		fmt.Println("  ⚠ requiere confirmación explícita del usuario antes de aplicar")
	}
}

func displayValue(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch x := v.(type) {
	case string:
		if x == "" {
			return "\"\" (vacío)"
		}
		return fmt.Sprintf("%q", x)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// ---------- apply ----------

func cmdApply(args []string) {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	proposalID := fs.String("proposal-id", "", "id de la propuesta")
	datasetPath := fs.String("dataset-path", "", "ruta a archivo JSON de dataset")
	datasetJSON := fs.String("dataset-json", "", "dataset como JSON string (artifact)")
	fs.Parse(args)
	if *proposalID == "" {
		fail("apply: --proposal-id requerido")
	}
	_, dp, pp, ap, _ := paths()
	props, err := fixers.LoadProposals(pp)
	if err != nil {
		fail("load proposals: %v", err)
	}
	var target *fixers.Proposal
	for i := range props {
		if props[i].ID == *proposalID {
			target = &props[i]
			break
		}
	}
	if target == nil {
		fail("propuesta %s no existe", *proposalID)
	}
	dsPath := dp
	if strings.TrimSpace(*datasetPath) != "" {
		dsPath = *datasetPath
	} else if strings.TrimSpace(*datasetJSON) != "" {
		tmp, err := writeTempDatasetJSON(*datasetJSON)
		if err != nil {
			fail("write temp dataset: %v", err)
		}
		defer os.Remove(tmp)
		dsPath = tmp
	}
	rec, err := fixers.Apply(*target, dsPath, ap)
	if err != nil {
		fail("apply: %v", err)
	}
	if err := fixers.RemoveProposal(pp, target.ID); err != nil {
		fail("remove proposal: %v", err)
	}
	updated := readJSONMap(dsPath)
	emitJSON(map[string]interface{}{
		"artifact_type":   "mecanico.applied.v1",
		"artifacts":       []string{"mecanico.applied.v1", "dataset.raw.v1", "external.api.dump.v1"},
		"applied":         rec,
		"applied_count":   1,
		"failed_count":    0,
		"updated_dataset": updated,
		"human_summary":   fmt.Sprintf("Mecánico aplicó %s sobre %s:%s.%s.", target.ID, rec.Endpoint, rec.RecordID, rec.Field),
	})
}

func cmdApplyAll(args []string) {
	fs := flag.NewFlagSet("apply-all", flag.ExitOnError)
	datasetJSON := fs.String("dataset-json", "", "dataset como JSON string (artifact)")
	datasetPath := fs.String("dataset-path", "", "ruta a archivo JSON de dataset")
	fs.Parse(args)
	_, dp, pp, ap, _ := paths()
	props, err := fixers.LoadProposals(pp)
	if err != nil {
		fail("load proposals: %v", err)
	}
	if len(props) == 0 {
		emitJSON(map[string]interface{}{
			"artifact_type": "mecanico.applied.v1",
			"artifacts":     []string{"mecanico.applied.v1"},
			"applied":       []interface{}{},
			"applied_count": 0,
			"failed_count":  0,
			"human_summary": "No había propuestas pendientes para aplicar.",
		})
		return
	}
	dsPath := dp
	if strings.TrimSpace(*datasetPath) != "" {
		dsPath = *datasetPath
	} else if strings.TrimSpace(*datasetJSON) != "" {
		tmp, err := writeTempDatasetJSON(*datasetJSON)
		if err != nil {
			fail("write temp dataset: %v", err)
		}
		defer os.Remove(tmp)
		dsPath = tmp
	}
	applied := 0
	failed := 0
	appliedRecords := []interface{}{}
	failures := []map[string]interface{}{}
	for _, p := range props {
		rec, err := fixers.Apply(p, dsPath, ap)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s falló: %v\n", p.ID, err)
			failed++
			failures = append(failures, map[string]interface{}{"proposal_id": p.ID, "error": err.Error()})
			continue
		}
		appliedRecords = append(appliedRecords, rec)
		applied++
	}
	// Limpiamos las propuestas aplicadas. apply-all aplica todo, así que
	// vaciamos el archivo (las que fallaron quedan registradas en stderr).
	_ = fixers.SaveProposals(pp, nil)
	updated := readJSONMap(dsPath)
	emitJSON(map[string]interface{}{
		"artifact_type":   "mecanico.applied.v1",
		"artifacts":       []string{"mecanico.applied.v1", "dataset.raw.v1", "external.api.dump.v1"},
		"applied":         appliedRecords,
		"failures":        failures,
		"applied_count":   applied,
		"failed_count":    failed,
		"updated_dataset": updated,
		"human_summary":   fmt.Sprintf("Mecánico aplicó %d propuesta(s). Fallidas: %d.", applied, failed),
	})
}

func writeTempDatasetJSON(jsonStr string) (string, error) {
	f, err := os.CreateTemp("", "mecanico_dataset_*.json")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(jsonStr); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func readJSONMap(path string) map[string]interface{} {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

// ---------- resolve-gaps (conversational gap resolver) ----------

func cmdResolveGaps(args []string) {
	fs := flag.NewFlagSet("resolve-gaps", flag.ExitOnError)
	dataGapsPath := fs.String("data-gaps-path", "", "path a data.gaps.v1 como JSON")
	dataGapsJSON := fs.String("data-gaps-json", "", "data.gaps.v1 como JSON string")
	findingsPath := fs.String("findings-path", "", "path a auditor.findings.v1 como JSON")
	findingsJSON := fs.String("findings-json", "", "auditor.findings.v1 como JSON string")
	entityRefPath := fs.String("entity-ref-path", "", "path a entity.ref.v1 como JSON")
	entityRefJSON := fs.String("entity-ref-json", "", "entity.ref.v1 como JSON string")
	scopeTablesPath := fs.String("scope-tables-path", "", "path a scope tables como JSON array")
	scopeTablesJSON := fs.String("scope-tables-json", "", "tables in scope for current entity (JSON array of strings)")
	fs.Parse(args)
	resolvedDataGapsJSON, err := loadJSONArg(*dataGapsPath, *dataGapsJSON)
	if err != nil {
		fail("read data-gaps-path: %v", err)
	}
	if strings.TrimSpace(resolvedDataGapsJSON) == "" {
		fail("resolve-gaps: --data-gaps-json requerido")
	}
	var gaps []map[string]interface{}
	if err := json.Unmarshal([]byte(resolvedDataGapsJSON), &gaps); err != nil {
		fail("parse data-gaps-json: %v", err)
	}
	// Filter gaps by scope tables if provided.
	var scopeTables map[string]bool
	resolvedScopeTablesJSON, err := loadJSONArg(*scopeTablesPath, *scopeTablesJSON)
	if err != nil {
		fail("read scope-tables-path: %v", err)
	}
	if strings.TrimSpace(resolvedScopeTablesJSON) != "" {
		var tableNames []string
		if err := json.Unmarshal([]byte(resolvedScopeTablesJSON), &tableNames); err == nil && len(tableNames) > 0 {
			scopeTables = make(map[string]bool, len(tableNames))
			for _, t := range tableNames {
				scopeTables[t] = true
			}
			var filtered []map[string]interface{}
			for _, gap := range gaps {
				endpoint := jsonString(gap, "endpoint", "table")
				if endpoint == "" || scopeTables[endpoint] {
					filtered = append(filtered, gap)
				}
			}
			gaps = filtered
		}
	}
	var findings []auditdata.Finding
	resolvedFindingsJSON, err := loadJSONArg(*findingsPath, *findingsJSON)
	if err != nil {
		fail("read findings-path: %v", err)
	}
	if strings.TrimSpace(resolvedFindingsJSON) != "" {
		findings, err = auditdata.ParseFindings([]byte(resolvedFindingsJSON))
		if err != nil {
			fail("parse findings-json: %v", err)
		}
	}
	var entityRef map[string]interface{}
	resolvedEntityRefJSON, err := loadJSONArg(*entityRefPath, *entityRefJSON)
	if err != nil {
		fail("read entity-ref-path: %v", err)
	}
	if strings.TrimSpace(resolvedEntityRefJSON) != "" {
		_ = json.Unmarshal([]byte(resolvedEntityRefJSON), &entityRef)
	}
	entityName := "esta entidad"
	if entityRef != nil {
		if n, ok := entityRef["name"].(string); ok && n != "" {
			entityName = n
		}
	}
	// Filter out gaps where the entity already has a value for the field.
	// This prevents asking for data that already exists in the DB (e.g.
	// asking for the client name when we already have it in entity ref).
	gaps = filterGapsByEntityData(gaps, entityRef)

	questions := []map[string]interface{}{}
	plan := []map[string]interface{}{}
	for idx, gap := range gaps {
		gapType := ""
		if t, ok := gap["type"].(string); ok {
			gapType = t
		}
		q := questionForGapWithLLM(gap, entityRef, entityName, idx)
		if q != nil {
			questions = append(questions, q)
			plan = append(plan, map[string]interface{}{
				"gap_type":      gapType,
				"action":        "ask_user",
				"question_id":   q["id"],
				"question_text": q["text"],
			})
		} else {
			plan = append(plan, map[string]interface{}{
				"gap_type": gapType,
				"action":   "auto_resolve",
			})
		}
	}
	emitJSON(map[string]interface{}{
		"artifact_type":   "mecanico.resolution_plan.v1",
		"artifacts":       []string{"mecanico.resolution_plan.v1", "framework.question.v1"},
		"entity_name":     entityName,
		"questions_count": len(questions),
		"questions":       questions,
		"plan":            plan,
		"findings_count":  len(findings),
		"generated_at":    time.Now().Format(time.RFC3339),
		"human_summary":   fmt.Sprintf("Mecánico generó %d preguntas para resolver gaps de %s.", len(questions), entityName),
	})
}

func loadJSONArg(pathArg, jsonArg string) (string, error) {
	if strings.TrimSpace(pathArg) != "" {
		raw, err := os.ReadFile(pathArg)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
	return strings.TrimSpace(jsonArg), nil
}

func questionForGapWithLLM(gap map[string]interface{}, entityRef map[string]interface{}, entityName string, idx int) map[string]interface{} {
	gapType := jsonString(gap, "type", "kind", "rule", "gap_type")
	field := jsonString(gap, "field")
	if field == "" {
		field = inferGapField(gapType, jsonString(gap, "description", "message"))
	}
	system := `Eres un asistente operativo que ayuda a completar tareas de negocio. Cuando falta un dato para continuar, lo pides de forma clara, directa y amable. Nunca mencionas terminos tecnicos como "base de datos", "gap", "framework", "artifact", "tabla" ni "campo". Hablas como un companero de trabajo que necesita un dato para hacer su trabajo. Siempre explicas POR QUE necesitas el dato (ej: "para poder enviar el correo de cobranza"). Hablas en espanol.

Devuelve solamente JSON valido con esta forma:
{"text":"pregunta natural","field":"dato faltante","gap_type":"tipo interno"}`
	payload := map[string]interface{}{
		"gap":         gap,
		"entity":      entityRef,
		"entity_name": entityName,
		"flow_context": map[string]interface{}{
			"purpose": "continuar una tarea operativa del negocio",
			"channel": "conversacional",
		},
	}
	rawPayload, _ := json.Marshal(payload)
	text := ""
	if client, err := llm.NewClient(); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if out, err := client.Generate(ctx, system, string(rawPayload)); err == nil {
			text, field = parseLLMQuestion(out, field)
		}
	}
	if strings.TrimSpace(text) == "" {
		text = fallbackNaturalQuestion(gapType, field, entityName)
	}
	return map[string]interface{}{
		"artifact_type": "framework.question.v1",
		"id":            fmt.Sprintf("mecanico_%s_%d_%d", safeToken(firstNonEmpty(gapType, field, "question")), time.Now().Unix(), idx),
		"text":          scrubTechnicalQuestion(text),
		"gap_type":      gapType,
		"field":         field,
	}
}

func parseLLMQuestion(raw, fallbackField string) (string, string) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		return strings.TrimSpace(raw), fallbackField
	}
	field := jsonString(parsed, "field")
	if field == "" {
		field = fallbackField
	}
	return jsonString(parsed, "text", "question", "message"), field
}

func fallbackNaturalQuestion(gapType, field, entityName string) string {
	needle := strings.ToLower(gapType + " " + field)
	switch {
	case strings.Contains(needle, "smtp") || strings.Contains(needle, "credential"):
		return "Para poder enviar correos necesito acceso a tu cuenta de email. ¿Me pasás los datos SMTP o de tu hosting?"
	case strings.Contains(needle, "email") || strings.Contains(needle, "contact"):
		return fmt.Sprintf("Para enviar el cobro a %s necesito su email. ¿Cuál es?", entityName)
	case strings.Contains(needle, "draft"):
		return fmt.Sprintf("Para avanzar con %s necesito preparar el texto del mensaje. ¿Querés que lo redacte ahora?", entityName)
	default:
		return fmt.Sprintf("Para continuar con %s necesito completar un dato. ¿Me lo pasás?", entityName)
	}
}

func inferGapField(gapType, description string) string {
	needle := strings.ToLower(gapType + " " + description)
	switch {
	case strings.Contains(needle, "smtp") || strings.Contains(needle, "credential"):
		return "credentials.smtp"
	case strings.Contains(needle, "email"):
		return "email"
	case strings.Contains(needle, "contact"):
		return "contact"
	case strings.Contains(needle, "draft"):
		return "message.draft"
	default:
		return ""
	}
}

// filterGapsByEntityData removes gaps where the entity ref already has
// a non-empty value for the gap's field. This prevents asking the user
// for data that already exists in the database (e.g. asking for client
// name when entity ref already has name="Thiel-Effertz").
//
// Schema-level gaps (like schema_contact_gap) are never filtered since
// they represent structural issues, not missing values.
func filterGapsByEntityData(gaps []map[string]interface{}, entityRef map[string]interface{}) []map[string]interface{} {
	if entityRef == nil || len(gaps) == 0 {
		return gaps
	}
	var filtered []map[string]interface{}
	for _, gap := range gaps {
		gapType := jsonString(gap, "type", "kind", "rule", "gap_type")
		field := jsonString(gap, "field")
		// Schema-level gaps are structural -- always keep them.
		if gapType == "schema_contact_gap" || gapType == "missing_contact_destination" || gapType == "missing_contact" {
			filtered = append(filtered, gap)
			continue
		}
		// Bulk data quality gaps (empty_required / null_required) on
		// non-entity tables are noise for conversational resolution.
		if gapType == "empty_required" || gapType == "null_required" {
			endpoint := jsonString(gap, "endpoint", "table")
			// If the entity ref already has the field value, skip.
			if field != "" && entityRefHasValue(entityRef, field) {
				continue
			}
			// If the gap is about a table that is not the entity table,
			// skip -- these are mass data quality issues.
			if endpoint != "" {
				entityTable := jsonString(entityRef, "table", "entity_table")
				if entityTable != "" && endpoint != entityTable {
					continue
				}
			}
		}
		// For any other gap, check if entity ref has the field value.
		if field != "" && entityRefHasValue(entityRef, field) {
			filtered = append(filtered, gap)
			// Mark it but still include -- the question generator may
			// decide to skip it based on the value.
			continue
		}
		filtered = append(filtered, gap)
	}
	return filtered
}

// entityRefHasValue checks if the entity ref has a non-empty value for
// the given field name, trying common aliases.
func entityRefHasValue(entityRef map[string]interface{}, field string) bool {
	if entityRef == nil || field == "" {
		return false
	}
	// Direct match
	if v, ok := entityRef[field].(string); ok && strings.TrimSpace(v) != "" {
		return true
	}
	// Try aliases for common fields
	switch field {
	case "name":
		if v, ok := entityRef["display_name"].(string); ok && strings.TrimSpace(v) != "" {
			return true
		}
	case "code":
		for _, k := range []string{"code", "client_code", "entity_code"} {
			if v, ok := entityRef[k].(string); ok && strings.TrimSpace(v) != "" {
				return true
			}
		}
	}
	return false
}

func scrubTechnicalQuestion(text string) string {
	text = strings.TrimSpace(text)
	replacements := map[string]string{
		"Gap detectado": "Necesito un dato",
		"gap":           "dato faltante",
		"framework":     "sistema",
		"artifact":      "dato",
		"tabla":         "registro",
		"campo":         "dato",
		"base de datos": "informacion guardada",
	}
	for from, to := range replacements {
		text = strings.ReplaceAll(text, from, to)
		text = strings.ReplaceAll(text, strings.Title(from), to)
	}
	return text
}

func jsonString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func safeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(".", "_", "-", "_", " ", "_", "/", "_")
	value = replacer.Replace(value)
	if value == "" {
		return "question"
	}
	return value
}

// ---------- reset ----------

func cmdReset(args []string) {
	_, _, pp, ap, sp := paths()
	_ = os.Remove(pp)
	_ = os.Remove(ap)
	_ = os.Remove(sp)
	fmt.Println("mecanico reset OK: proposals.json, applied.jsonl y state.json limpiados.")
}

// ---------- conversacional ----------

type state struct {
	GreetingSent bool      `json:"greeting_sent"`
	PendingText  string    `json:"pending_text"`
	PendingID    string    `json:"pending_id"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func loadState(path string) *state {
	s := &state{}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, s)
	return s
}

func saveState(path string, s *state) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	s.UpdatedAt = time.Now()
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(path, data, 0644)
}

func cmdNextQuestion(args []string) {
	fs := flag.NewFlagSet("next-question", flag.ExitOnError)
	statePath := fs.String("state", "", "path state")
	fs.Parse(args)
	sp := resolvePath(*statePath, "MECANICO_STATE", defaultState)
	s := loadState(sp)
	if s.PendingText != "" {
		out := map[string]string{"id": s.PendingID, "text": s.PendingText}
		s.PendingText = ""
		s.PendingID = ""
		_ = saveState(sp, s)
		emitJSON(out)
		return
	}
	if !s.GreetingSent {
		// Saludo: anuncia capacidades y consulta findings actuales.
		text := buildGreeting()
		s.GreetingSent = true
		_ = saveState(sp, s)
		emitJSON(map[string]string{
			"id":   fmt.Sprintf("mecanico_intro_%d", time.Now().Unix()),
			"text": text,
		})
		return
	}
	fmt.Println("{}")
}

func buildGreeting() string {
	fp, dp, pp, _, _ := paths()
	finds, _ := loadFindingsOrJSON("", fp)
	autoCount := 0
	for _, f := range finds {
		if f.AutoFixable {
			autoCount++
		}
	}
	props, _ := fixers.LoadProposals(pp)
	var sb strings.Builder
	sb.WriteString("Soy el mecánico. Tomo los hallazgos del auditor y propongo fixes; nunca toco el dataset sin tu OK.\n")
	if len(finds) == 0 {
		sb.WriteString("No hay findings cargados todavía. Pediile al auditor que escanee primero.")
		return sb.String()
	}
	// Si hay autos pendientes y no hay propuestas, generamos las propuestas
	// proactivamente para que el user vea el plan completo en el primer turno.
	if len(props) == 0 && autoCount > 0 {
		ds, err := loadDatasetOrJSON("", dp)
		if err != nil {
			return fmt.Sprintf("No pude leer el dataset: %v.", err)
		}
		already := map[string]bool{}
		for _, p := range props {
			already[p.FindingID] = true
		}
		idx := nextProposalIdx(props)
		for _, f := range finds {
			if !f.AutoFixable || already[f.ID] {
				continue
			}
			prop := fixers.ProposeForFinding(f, ds, idx)
			if prop == nil {
				continue
			}
			props = append(props, *prop)
			idx++
		}
		_ = fixers.SaveProposals(pp, props)
	}
	if len(props) > 0 {
		fmt.Fprintf(&sb, "Generé un plan de %d fix(es) auto-corregibles. Antes de aplicar nada, te lo muestro:\n\n", len(props))
		shown := 0
		for _, p := range fixers.SortedProposals(props) {
			if shown >= 8 {
				fmt.Fprintf(&sb, "  …y %d más.\n", len(props)-shown)
				break
			}
			fmt.Fprintf(&sb, "  %s — %s.%s en %s:%s    %v → %v\n",
				p.ID, p.Endpoint, p.Field, p.Endpoint, p.RecordID,
				displayValue(p.CurrentValue), displayValue(p.ProposedValue))
			shown++
		}
		sb.WriteString("\n¿Aplico todo? Decime \"sí, aplicá todo\" para ejecutar, o \"aplicá P-XXX\" para uno puntual.")
	} else {
		fmt.Fprintf(&sb, "De los %d hallazgos, ninguno es auto-corregible: necesitan revisión humana antes de tocarlos.", len(finds))
	}
	return sb.String()
}

func cmdIngestAnswer(args []string) {
	fs := flag.NewFlagSet("ingest-answer", flag.ExitOnError)
	questionID := fs.String("question-id", "", "id de la pregunta")
	answer := fs.String("answer", "", "respuesta del user")
	statePath := fs.String("state", "", "path state")
	fs.Parse(args)
	_ = *questionID
	if *answer == "" {
		fail("ingest-answer: --answer requerido")
	}
	sp := resolvePath(*statePath, "MECANICO_STATE", defaultState)
	s := loadState(sp)

	reply := interpretMecanico(strings.TrimSpace(*answer))
	if reply == sentinelDelegateToAuditor {
		// Quedamos idle: el orquestador hará polling y el auditor hablará.
		// Además, reseteamos GreetingSent del auditor para forzar que su
		// próximo next-question vuelva a correr scan + saludo.
		s.PendingText = ""
		s.PendingID = ""
		_ = resetAuditorGreeting()
	} else {
		s.PendingText = reply
		s.PendingID = fmt.Sprintf("mecanico_reply_%d", time.Now().Unix())
	}
	if err := saveState(sp, s); err != nil {
		fail("save state: %v", err)
	}
}

// sentinelDelegateToAuditor: respuesta interna que indica handoff al auditor.
const sentinelDelegateToAuditor = "\x00DELEGATE_AUDITOR\x00"

// resetAuditorGreeting fuerza al auditor a re-saludar (re-scan + summary)
// cuando el user pide rescan desde el contexto del mecánico.
func resetAuditorGreeting() error {
	auditorState := "../framework-auditor/temp/state.json"
	if v := os.Getenv("AUDITOR_STATE_REL"); v != "" {
		auditorState = v
	}
	// Si el archivo no existe, no hay nada que resetear.
	if _, err := os.Stat(auditorState); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(auditorState)
}

// interpretMecanico parsea intenciones simples del usuario.
func interpretMecanico(text string) string {
	low := strings.ToLower(text)
	_, _, pp, _, _ := paths()
	props, _ := fixers.LoadProposals(pp)
	hasProps := len(props) > 0

	switch {
	case strings.Contains(low, "propon") && (strings.Contains(low, "auto") || strings.Contains(low, "todo")):
		return runProposeAllAutoCapture()
	case strings.Contains(low, "ver propuestas") || strings.Contains(low, "lista propuestas") || strings.Contains(low, "listar propuestas"):
		return runListProposalsCapture()
	case isAffirmative(low) && strings.Contains(low, "aplic") && strings.Contains(low, "todo"):
		return runApplyAllCapture()
	case strings.Contains(low, "p-") && (strings.Contains(low, "aplic") || isAffirmative(low)):
		id := extractProposalID(text)
		if id == "" {
			return "No pude identificar el id. Probá con \"aplicá P-001\"."
		}
		return runApplyOneCapture(id)
	case isPureAffirmative(low) && hasProps:
		// "sí" / "dale" / "ok" sin más detalle, y hay propuestas pendientes:
		// asumimos "sí, aplicá todo" porque es el último plan que el user vio.
		return runApplyAllCapture()
	case isAffirmative(low) && (strings.Contains(low, "aplica") || strings.Contains(low, "aplique")):
		return "¿Cuál? Decime el id de la propuesta (ej. \"aplicá P-001\") o \"aplicá todo\"."
	case strings.Contains(low, "p-"):
		id := extractProposalID(text)
		if id == "" {
			return "No pude identificar el id. Probá con \"aplicá P-001\"."
		}
		return runApplyOneCapture(id)
	case strings.Contains(low, "reset"):
		return "Para resetear el dataset, corré 'auditor reset'. Para limpiar mis propuestas, 'mecanico reset'."
	case strings.Contains(low, "rescan") || strings.Contains(low, "re-scan") || strings.Contains(low, "auditor") || strings.Contains(low, "scan"):
		// Delegamos al auditor: quedamos idle para que vuelva a hablar él.
		return sentinelDelegateToAuditor
	case strings.Contains(low, "detalle") || strings.Contains(low, "f-"):
		// Pregunta sobre un finding concreto: tarea del auditor.
		return sentinelDelegateToAuditor
	}
	// Conversación general → LLM con system prompt del mecánico.
	return interpretMecanicoWithLLM(text, hasProps)
}

const mecanicoSystemPrompt = `Eres el Mecánico de datos de Remora. Tu rol es tomar los hallazgos que detectó el Auditor y proponer fixes concretos. Nunca tocás el dataset sin la confirmación explícita del usuario.

Tu personalidad:
- Eres práctico, directo y confiable.
- Hablas en español rioplatense informal ("dale", "decime", "listo").
- Nunca aplicás cambios sin permiso.
- Si no sabés algo, lo decís.

Capacidades que podés ofrecer al usuario:
- Proponer fixes auto-corregibles ("proponé los auto")
- Ver propuestas pendientes ("ver propuestas")
- Aplicar una propuesta puntual ("aplicá P-001")
- Aplicar todas las propuestas ("aplicá todo")
- Generar borrador de email de cobranza ("draft-email")
- Pedir rescan al auditor ("rescan")

Contexto actual:
%s

Respondé de forma breve y útil. Si el usuario pregunta algo fuera de tu alcance, explicá qué podés hacer.`

func interpretMecanicoWithLLM(userText string, hasProps bool) string {
	client, err := llm.NewClient()
	if err != nil {
		return fmt.Sprintf("Error iniciando LLM: %v", err)
	}
	contextStr := buildProposalsContext(hasProps)
	system := fmt.Sprintf(mecanicoSystemPrompt, contextStr)
	reply, err := client.Generate(context.Background(), system, userText)
	if err != nil {
		return fmt.Sprintf("Error del LLM: %v", err)
	}
	return reply
}

func buildProposalsContext(hasProps bool) string {
	_, _, pp, _, _ := paths()
	props, _ := fixers.LoadProposals(pp)
	fp, _, _, _, _ := paths()
	finds, _ := loadFindingsOrJSON("", fp)
	autoCount := 0
	for _, f := range finds {
		if f.AutoFixable {
			autoCount++
		}
	}
	if len(finds) == 0 {
		return "No hay findings cargados. El auditor no ha escaneado todavía."
	}
	if hasProps {
		return fmt.Sprintf("%d findings del auditor. %d propuestas pendientes de aprobación. %d auto-corregibles.",
			len(finds), len(props), autoCount)
	}
	return fmt.Sprintf("%d findings del auditor. Sin propuestas generadas aún. %d auto-corregibles.",
		len(finds), autoCount)
}

// isPureAffirmative detecta afirmativos cortos sin verbo de acción explícito.
func isPureAffirmative(low string) bool {
	trim := strings.TrimSpace(strings.TrimRight(low, ".!,"))
	pure := map[string]bool{
		"si": true, "sí": true, "ok": true, "dale": true,
		"si dale": true, "sí dale": true, "si, dale": true, "sí, dale": true,
		"sí ok": true, "si ok": true,
		"adelante": true, "claro": true, "obvio": true, "perfecto": true,
		"hagamoslo": true, "hagámoslo": true,
	}
	return pure[trim]
}

func isAffirmative(s string) bool {
	return strings.Contains(s, "si") || strings.Contains(s, "sí") || strings.Contains(s, "ok") || strings.Contains(s, "dale") || strings.Contains(s, "aplic")
}

func extractProposalID(text string) string {
	upper := strings.ToUpper(text)
	idx := strings.Index(upper, "P-")
	if idx < 0 {
		return ""
	}
	end := idx + 2
	for end < len(upper) && upper[end] >= '0' && upper[end] <= '9' {
		end++
	}
	if end == idx+2 {
		return ""
	}
	return upper[idx:end]
}

// Helpers que invocan los comandos pero capturan la salida en memoria
// para devolverla como respuesta conversacional.
func runProposeAllAutoCapture() string {
	fp, dp, pp, _, _ := paths()
	finds, err := loadFindingsOrJSON("", fp)
	if err != nil {
		return fmt.Sprintf("No tengo findings: %v. Pedile al auditor un scan primero.", err)
	}
	ds, err := loadDatasetOrJSON("", dp)
	if err != nil {
		return fmt.Sprintf("No pude leer el dataset: %v.", err)
	}
	existing, _ := fixers.LoadProposals(pp)
	already := map[string]bool{}
	for _, p := range existing {
		already[p.FindingID] = true
	}
	idx := nextProposalIdx(existing)
	added := 0
	for _, f := range finds {
		if !f.AutoFixable || already[f.ID] {
			continue
		}
		prop := fixers.ProposeForFinding(f, ds, idx)
		if prop == nil {
			continue
		}
		existing = append(existing, *prop)
		idx++
		added++
	}
	if err := fixers.SaveProposals(pp, existing); err != nil {
		return fmt.Sprintf("Error guardando propuestas: %v", err)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Generé %d propuestas nuevas (total pendientes: %d):\n", added, len(existing))
	for _, p := range fixers.SortedProposals(existing) {
		fmt.Fprintf(&sb, "  %s — %s.%s en %s:%s    %v → %v\n",
			p.ID, p.Endpoint, p.Field, p.Endpoint, p.RecordID,
			displayValue(p.CurrentValue), displayValue(p.ProposedValue))
	}
	sb.WriteString("\n¿Aplico todo? Decime \"aplicá todo\" o \"aplicá P-XXX\" puntual.")
	return sb.String()
}

func runListProposalsCapture() string {
	_, _, pp, _, _ := paths()
	props, err := fixers.LoadProposals(pp)
	if err != nil {
		return fmt.Sprintf("Error cargando propuestas: %v", err)
	}
	if len(props) == 0 {
		return "No hay propuestas pendientes. Decime \"propone los auto\" para generarlas."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Tengo %d propuesta(s) pendientes:\n", len(props))
	for _, p := range fixers.SortedProposals(props) {
		fmt.Fprintf(&sb, "%s  [%s:%s.%s]\n  actual: %v\n  propuesto: %v\n  por qué: %s\n\n",
			p.ID, p.Endpoint, p.RecordID, p.Field,
			displayValue(p.CurrentValue), displayValue(p.ProposedValue), p.Rationale)
	}
	return sb.String()
}

func runApplyAllCapture() string {
	_, dp, pp, ap, _ := paths()
	props, err := fixers.LoadProposals(pp)
	if err != nil {
		return fmt.Sprintf("Error cargando propuestas: %v", err)
	}
	if len(props) == 0 {
		return "No hay propuestas para aplicar."
	}
	var sb strings.Builder
	applied, failed := 0, 0
	for _, p := range props {
		rec, err := fixers.Apply(p, dp, ap)
		if err != nil {
			fmt.Fprintf(&sb, "✗ %s falló: %v\n", p.ID, err)
			failed++
			continue
		}
		fmt.Fprintf(&sb, "✓ %s   %s:%s.%s   %v → %v\n", p.ID, rec.Endpoint, rec.RecordID, rec.Field,
			displayValue(rec.Before), displayValue(rec.After))
		applied++
	}
	_ = fixers.SaveProposals(pp, nil)
	fmt.Fprintf(&sb, "\nApliqué %d fix(es). Fallidas: %d. Pediile al auditor un \"rescan\" para ver el estado nuevo.", applied, failed)
	return sb.String()
}

func runApplyOneCapture(id string) string {
	_, dp, pp, ap, _ := paths()
	props, err := fixers.LoadProposals(pp)
	if err != nil {
		return fmt.Sprintf("Error cargando propuestas: %v", err)
	}
	var target *fixers.Proposal
	for i := range props {
		if props[i].ID == id {
			target = &props[i]
			break
		}
	}
	if target == nil {
		return fmt.Sprintf("No encuentro la propuesta %s.", id)
	}
	rec, err := fixers.Apply(*target, dp, ap)
	if err != nil {
		return fmt.Sprintf("Error aplicando %s: %v", id, err)
	}
	_ = fixers.RemoveProposal(pp, target.ID)
	return fmt.Sprintf("Listo. %s aplicado: %s:%s.%s   %v → %v.\nPedile un rescan al auditor para confirmar.",
		id, rec.Endpoint, rec.RecordID, rec.Field, displayValue(rec.Before), displayValue(rec.After))
}

// ---------- helpers ----------

func emitJSON(v interface{}) {
	data, _ := json.Marshal(v)
	fmt.Println(string(data))
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

// Cobranza mode: generate email drafts for debt collection
func cmdDraftEmail(args []string) {
	fs := flag.NewFlagSet("draft-email", flag.ExitOnError)
	deudor := fs.String("deudor", "", "nombre del deudor")
	tono := fs.String("tono", "formal", "amistoso|formal|carta")
	saldo := fs.Float64("saldo", 0, "monto adeudado")
	dias := fs.Int("dias-mora", 0, "días de mora")
	to := fs.String("to", "", "email del destinatario")
	actionID := fs.String("action-id", "", "id de acción seleccionada (action.selection.v1)")
	save := fs.Bool("save", true, "guardar en state")
	fs.Parse(args)

	if strings.TrimSpace(*actionID) == "skip_case" {
		emitJSON(map[string]interface{}{
			"artifact_type": "action.skipped.v1",
			"artifacts":     []string{"action.skipped.v1"},
			"action_id":     "skip_case",
			"skip_reason":   "El usuario eligió pasar al siguiente caso; no se genera borrador.",
			"entity":        *deudor,
			"generated_at":  time.Now().Format(time.RFC3339),
		})
		return
	}

	if *deudor == "" {
		fail("draft-email requiere --deudor")
	}

	draft := generateEmailDraft(*deudor, *tono, *saldo, *dias, *to)

	if *save {
		stateDir := "temp/mecanico"
		_ = os.MkdirAll(stateDir, 0755)

		draftPath := filepath.Join(stateDir, "last_draft.json")
		data, _ := json.MarshalIndent(draft, "", "  ")
		_ = os.WriteFile(draftPath, data, 0644)

		statePath := filepath.Join(stateDir, "state.json")
		resp := map[string]interface{}{
			"response":   formatDraftForUser(draft),
			"draft":      draft,
			"timestamp":  time.Now().Format(time.RFC3339),
			"pending_id": fmt.Sprintf("mecanico_draft_%d", time.Now().Unix()),
		}
		respData, _ := json.Marshal(resp)
		_ = os.WriteFile(statePath, respData, 0644)
	}

	emitJSON(map[string]interface{}{
		"artifact_type":     "message.draft.v1",
		"artifacts":         []string{"message.draft.v1", "message.draft"},
		"channel":           "email",
		"to":                draft.To,
		"subject":           draft.Subject,
		"body":              draft.Body,
		"body_b64":          base64.StdEncoding.EncodeToString([]byte(draft.Body)),
		"requires_approval": draft.RequiresApproval,
		"draft":             draft,
		"human_preview":     formatDraftForUser(draft),
		"generated_at":      draft.GeneratedAt,
	})
}

type emailDraft struct {
	Type             string `json:"type"`
	Action           string `json:"action"`
	Deudor           string `json:"deudor"`
	Subject          string `json:"subject"`
	Body             string `json:"body"`
	To               string `json:"to"`
	Tono             string `json:"tono"`
	LegalReference   string `json:"legal_reference,omitempty"`
	GmailOpenURL     string `json:"gmail_open_url"`
	RequiresApproval bool   `json:"requires_approval"`
	GeneratedAt      string `json:"generated_at"`
}

func generateEmailDraft(deudor, tono string, saldo float64, dias int, to string) *emailDraft {
	estudio := os.Getenv("ESTUDIO_NOMBRE")
	if estudio == "" {
		estudio = "Estudio Jurídico Remora"
	}
	cobrador := os.Getenv("COBRADOR_NOMBRE")
	if cobrador == "" {
		cobrador = "Departamento de Cobranza"
	}
	fecha := time.Now().Format("02/01/2006")

	// Si no hay --to explícito, usamos un placeholder que el flow runner
	// o el usuario debe reemplazar antes del envío real.
	if to == "" {
		to = "(sin destinatario - debe configurarse antes del envío)"
	}

	var subject, body string
	switch tono {
	case "amistoso":
		subject = fmt.Sprintf("Recordatorio de pago - %s - %s", deudor, estudio)
		body = fmt.Sprintf(`Estimado/a %s,

Por medio de la presente le recordamos que tiene facturas pendientes por un monto total de $%.0f.

Detalle:
- Días de mora: %d días

Le solicitamos gentilmente regularizar esta situación a la brevedad.

Saludos cordiales,
%s
%s`, deudor, saldo, dias, cobrador, estudio)

	case "carta":
		subject = fmt.Sprintf("Carta de requerimiento formal - Art. 37 Ley 21.394")
		body = fmt.Sprintf(`%s, %s

Sr./Sra. %s

REFERENCIA: Requerimiento de pago

Por intermedio de este estudio jurídico, nos dirigimos a Usted para notificarle la existencia de la siguiente deuda:

- Monto total: $%.0f
- Días de mora: %d

Se le otorga un plazo de 10 días hábiles para el pago total de la deuda.

Atentamente,
%s
%s`, estudio, fecha, deudor, saldo, dias, cobrador, estudio)

	default: // formal
		subject = fmt.Sprintf("Requerimiento de pago - Facturas vencidas - %s", estudio)
		body = fmt.Sprintf(`Sr./Sra. %s,

Por intermedio del %s, nos dirigimos a Usted para requerir el pago de las facturas pendientes:

- Monto adeudado: $%.0f
- Días de mora: %d

Le informamos que, de no regularizar esta situación, iniciaremos las acciones legales correspondientes.

Atentamente,
%s
%s`, deudor, estudio, saldo, dias, cobrador, estudio)
	}

	// Gmail compose URL
	gmailURL := fmt.Sprintf("https://mail.google.com/mail/?view=cm&fs=1&to=%s&su=%s&body=%s",
		urlEncode(to), urlEncode(subject), urlEncode(body))

	return &emailDraft{
		Type:             "action_proposal",
		Action:           "email",
		Deudor:           deudor,
		Subject:          subject,
		Body:             body,
		To:               to,
		Tono:             tono,
		GmailOpenURL:     gmailURL,
		RequiresApproval: true,
		GeneratedAt:      time.Now().Format(time.RFC3339),
	}
}

func formatDraftForUser(draft *emailDraft) string {
	var sb strings.Builder
	tonoEmoji := map[string]string{"amistoso": "📧", "formal": "📨", "carta": "📑"}
	emoji := tonoEmoji[draft.Tono]
	if emoji == "" {
		emoji = "📧"
	}

	sb.WriteString(fmt.Sprintf("%s **Borrador de email (%s)**\n\n", emoji, draft.Tono))
	sb.WriteString(fmt.Sprintf("**Para:** %s <%s>\n", draft.Deudor, draft.To))
	sb.WriteString(fmt.Sprintf("**Asunto:** %s\n\n", draft.Subject))
	sb.WriteString(fmt.Sprintf("---\n%s\n---\n\n", draft.Body))
	sb.WriteString(fmt.Sprintf("👉 [Abrir en Gmail](%s)\n", draft.GmailOpenURL))
	sb.WriteString("\n_Revisa el borrador y envíalo desde Gmail cuando estés listo._")
	return sb.String()
}

func urlEncode(s string) string {
	return strings.ReplaceAll(s, " ", "%20")
}
