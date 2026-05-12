// frameworkauditor: agente de auditoría que corre checks deterministas
// sobre un dataset JSON-API y emite findings consumibles por
// framework-mecanico.
//
// Comandos:
//
//	./frameworkauditor reset
//	    Restaura dataset.working.json desde dataset.golden.json y borra findings.json.
//
//	./frameworkauditor scan
//	    Ejecuta checks sobre dataset.working.json y persiste findings.json.
//
//	./frameworkauditor list [--severity critical|warning|info] [--json]
//	    Imprime los findings del último scan.
//
//	./frameworkauditor detail --id F-001 [--json]
//	    Imprime el detalle de un finding.
//
//	./frameworkauditor next-question
//	./frameworkauditor ingest-answer --question-id <id> --answer <text>
//	    Modo conversacional para el orquestador api_rest.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"framework-auditor/checks"
	"framework-auditor/internal/llm"
)

const (
	defaultGolden   = "data/dataset.golden.json"
	defaultWorking  = "data/dataset.working.json"
	defaultFindings = "data/findings.json"
	defaultState    = "temp/state.json"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "reset":
		cmdReset(os.Args[2:])
	case "scan":
		cmdScan(os.Args[2:])
	case "list":
		cmdList(os.Args[2:])
	case "detail":
		cmdDetail(os.Args[2:])
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
	fmt.Print(`frameworkauditor: agente de auditoría sobre dataset JSON-API o SQLite

Uso:
  frameworkauditor reset
  frameworkauditor scan [--source <path>] [--db <sqlite_path>] [--out <path>] [--json]
  frameworkauditor list [--severity critical|warning|info] [--json]
  frameworkauditor detail --id F-001 [--json]
  frameworkauditor next-question
  frameworkauditor ingest-answer --question-id <id> --answer <text>

Si se pasa --db, se escanea la base SQLite en vez del JSON.
Esto permite auditar panalbit.db directamente y detectar brechas
estructurales (ej: tablas sin columna de email).

Variables de entorno:
  AUDITOR_GOLDEN    override dataset.golden.json
  AUDITOR_WORKING   override dataset.working.json
  AUDITOR_FINDINGS  override findings.json
  AUDITOR_STATE     override state.json
  AUDITOR_DB        override para --db (path a SQLite)
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

// ---------- reset ----------

func cmdReset(args []string) {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	golden := fs.String("golden", "", "path golden")
	working := fs.String("working", "", "path working")
	findings := fs.String("findings", "", "path findings.json")
	state := fs.String("state", "", "path state.json")
	fs.Parse(args)

	gp := resolvePath(*golden, "AUDITOR_GOLDEN", defaultGolden)
	wp := resolvePath(*working, "AUDITOR_WORKING", defaultWorking)
	fp := resolvePath(*findings, "AUDITOR_FINDINGS", defaultFindings)
	sp := resolvePath(*state, "AUDITOR_STATE", defaultState)

	if err := copyFile(gp, wp); err != nil {
		fail("reset copy: %v", err)
	}
	_ = os.Remove(fp)
	_ = os.Remove(sp)
	fmt.Printf("reset OK: %s ← %s (findings y state limpiados)\n", wp, gp)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// ---------- scan ----------

func cmdScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	source := fs.String("source", "", "path del dataset JSON")
	dbPath := fs.String("db", "", "path a SQLite (panalbit.db); si se pasa, se escanea la DB en vez del JSON")
	out := fs.String("out", "", "path findings.json")
	asJSON := fs.Bool("json", false, "salida JSON con artifacts para orquestador")
	fs.Parse(args)

	fp := resolvePath(*out, "AUDITOR_FINDINGS", defaultFindings)

	// Resolve db path from flag, then env, then empty.
	resolvedDB := *dbPath
	if resolvedDB == "" {
		resolvedDB = os.Getenv("AUDITOR_DB")
	}

	var d *checks.Dataset
	var tableColumns map[string][]string
	var err error
	var sourceLabel string

	if resolvedDB != "" {
		// SQLite mode: read directly from panalbit.db (or any SQLite).
		d, err = checks.LoadDatasetFromSQLite(resolvedDB)
		if err != nil {
			fail("load sqlite %s: %v", resolvedDB, err)
		}
		tableColumns, err = checks.TableColumnsFromDB(resolvedDB)
		if err != nil {
			fail("read schema %s: %v", resolvedDB, err)
		}
		sourceLabel = "sqlite:" + resolvedDB
	} else {
		// JSON mode (legacy): read from dataset.working.json.
		wp := resolvePath(*source, "AUDITOR_WORKING", defaultWorking)
		d, err = checks.LoadDataset(wp)
		if err != nil {
			fail("load: %v", err)
		}
		sourceLabel = "json:" + wp
	}

	findings := checks.RunAllWithSchema(d, tableColumns)
	if err := checks.SaveFindings(fp, findings); err != nil {
		fail("save findings: %v", err)
	}

	totalRecords := 0
	for _, recs := range d.Endpoints {
		totalRecords += len(recs)
	}
	bySev := map[string]int{}
	for _, f := range findings {
		bySev[f.Severity]++
	}
	dataGaps, bulkGaps := dataGapsFromFindings(findings)
	artifacts := []string{"auditor.findings.v1", "data.gaps.v1"}
	if len(bulkGaps) > 0 {
		artifacts = append(artifacts, "data.quality.bulk.v1")
	}
	if *asJSON {
		emitJSON(map[string]interface{}{
			"artifact_type":     "auditor.findings.v1",
			"artifacts":         artifacts,
			"generated_at":      time.Now().UTC().Format(time.RFC3339),
			"source":            sourceLabel,
			"findings_path":     fp,
			"findings":          findings,
			"data_gaps":         dataGaps,
			"data_quality_bulk": bulkGaps,
			"summary": map[string]interface{}{
				"total_records": totalRecords,
				"total":         len(findings),
				"critical":      bySev[checks.SeverityCritical],
				"warning":       bySev[checks.SeverityWarning],
				"info":          bySev[checks.SeverityInfo],
				"bulk_gaps":     len(bulkGaps),
			},
		})
		return
	}
	fmt.Printf("Scan completo (%s): %d registros revisados, %d hallazgos.\n", sourceLabel, totalRecords, len(findings))
	fmt.Printf("  críticos: %d   advertencias: %d   informativos: %d\n",
		bySev[checks.SeverityCritical], bySev[checks.SeverityWarning], bySev[checks.SeverityInfo])
	fmt.Printf("Findings persistidos en %s\n", fp)
}

// dataGapsFromFindings converts Auditor findings into data.gaps.v1 entries
// and data.quality.bulk.v1 entries. Bulk findings (actionability=bulk_migration)
// are separated from user-completable/structural gaps.
// Row-level findings are grouped by (rule, endpoint, field) to avoid flooding.
func dataGapsFromFindings(findings []checks.Finding) (gaps []map[string]interface{}, bulk []map[string]interface{}) {
	// Group noisy row-level rules by (rule, endpoint, field).
	type groupKey struct{ rule, endpoint, field, actionability string }
	type groupData struct {
		severity      string
		fixHint       map[string]interface{}
		recordIDs     []string
		affectedCount int
		totalCount    int
		affectedPct   float64
	}
	groups := map[groupKey]*groupData{}
	var groupOrder []groupKey

	for _, f := range findings {
		switch f.Rule {
		case checks.RuleSchemaContactGap:
			gaps = append(gaps, map[string]interface{}{
				"artifact_type":  "data.gap.v1",
				"rule":           f.Rule,
				"type":           "schema_contact_gap",
				"severity":       f.Severity,
				"endpoint":       f.Endpoint,
				"record_id":      f.RecordID,
				"field":          f.Field,
				"message":        f.Message,
				"fix_hint":       f.FixHint,
				"actionability":  f.Actionability,
			})
		case checks.RuleEmptyRequired, checks.RuleNullRequired, checks.RuleMissingContact:
			key := groupKey{rule: f.Rule, endpoint: f.Endpoint, field: f.Field, actionability: f.Actionability}
			g, exists := groups[key]
			if !exists {
				g = &groupData{
					severity:      f.Severity,
					fixHint:       f.FixHint,
					affectedCount: f.AffectedCount,
					totalCount:    f.TotalCount,
					affectedPct:   f.AffectedPct,
				}
				groups[key] = g
				groupOrder = append(groupOrder, key)
			}
			if f.RecordID != "" {
				g.recordIDs = append(g.recordIDs, f.RecordID)
			}
		default:
			// Other rules are not data gaps.
		}
	}

	// Emit grouped gaps, separating bulk from user-completable/structural.
	for _, key := range groupOrder {
		g := groups[key]
		label := "vacio"
		gapType := key.rule
		if key.rule == checks.RuleNullRequired {
			label = "nulo"
		} else if key.rule == checks.RuleMissingContact {
			label = "sin email/contacto operativo"
			gapType = "missing_contact"
		}
		msg := fmt.Sprintf("%d registros en %s con campo %s %s", len(g.recordIDs), key.endpoint, key.field, label)
		if len(g.recordIDs) == 1 {
			msg = fmt.Sprintf("%s[%s].%s %s", key.endpoint, g.recordIDs[0], key.field, label)
		}
		entry := map[string]interface{}{
			"rule":          key.rule,
			"type":          gapType,
			"severity":      g.severity,
			"endpoint":      key.endpoint,
			"field":         key.field,
			"message":       msg,
			"count":         len(g.recordIDs),
			"record_ids":    g.recordIDs,
			"fix_hint":      g.fixHint,
			"actionability": key.actionability,
		}
		if g.totalCount > 0 {
			entry["total_count"] = g.totalCount
			entry["affected_pct"] = g.affectedPct
		}
		if key.actionability == checks.ActionabilityBulkMigration {
			entry["artifact_type"] = "data.quality.bulk.v1"
			entry["recommendation"] = "Ejecutar un script de migracion o importacion masiva para completar este campo en la fuente de datos."
			bulk = append(bulk, entry)
		} else {
			entry["artifact_type"] = "data.gap.v1"
			gaps = append(gaps, entry)
		}
	}
	return gaps, bulk
}

// ---------- list ----------

func cmdList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	severity := fs.String("severity", "", "filtrar por severity")
	asJSON := fs.Bool("json", false, "salida JSON")
	fp := fs.String("findings", "", "path findings.json")
	fs.Parse(args)

	path := resolvePath(*fp, "AUDITOR_FINDINGS", defaultFindings)
	findings, err := checks.LoadFindings(path)
	if err != nil {
		fail("load findings (%s): %v. ¿Corriste 'scan'?", path, err)
	}
	filtered := findings
	if *severity != "" {
		out := make([]checks.Finding, 0, len(findings))
		for _, f := range findings {
			if f.Severity == *severity {
				out = append(out, f)
			}
		}
		filtered = out
	}
	if *asJSON {
		emitJSON(filtered)
		return
	}
	if len(filtered) == 0 {
		fmt.Println("Sin hallazgos.")
		return
	}
	for _, f := range filtered {
		auto := ""
		if f.AutoFixable {
			auto = " [auto-fix]"
		}
		fmt.Printf("%s [%s] %-20s %s\n", f.ID, f.Severity, f.Endpoint+":"+f.RecordID, f.Message+auto)
	}
}

// ---------- detail ----------

func cmdDetail(args []string) {
	fs := flag.NewFlagSet("detail", flag.ExitOnError)
	id := fs.String("id", "", "id del finding (F-001)")
	asJSON := fs.Bool("json", false, "salida JSON")
	fp := fs.String("findings", "", "path findings.json")
	fs.Parse(args)
	if *id == "" {
		fail("detail: --id requerido")
	}
	path := resolvePath(*fp, "AUDITOR_FINDINGS", defaultFindings)
	findings, err := checks.LoadFindings(path)
	if err != nil {
		fail("load findings: %v", err)
	}
	for _, f := range findings {
		if f.ID == *id {
			if *asJSON {
				emitJSON(f)
				return
			}
			printFinding(f)
			return
		}
	}
	fail("finding %s no encontrado", *id)
}

func printFinding(f checks.Finding) {
	fmt.Printf("%s  [%s]  %s:%s\n", f.ID, f.Severity, f.Endpoint, f.RecordID)
	if f.Field != "" {
		fmt.Printf("  campo: %s\n", f.Field)
	}
	fmt.Printf("  regla: %s\n", f.Rule)
	fmt.Printf("  mensaje: %s\n", f.Message)
	if f.Suggestion != "" {
		fmt.Printf("  sugerencia: %s\n", f.Suggestion)
	}
	if len(f.Evidence) > 0 {
		fmt.Println("  evidencia:")
		keys := make([]string, 0, len(f.Evidence))
		for k := range f.Evidence {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("    - %s: %v\n", k, f.Evidence[k])
		}
	}
	if f.AutoFixable {
		fmt.Println("  auto-fix: sí (framework-mecanico puede proponer fix)")
	} else {
		fmt.Println("  auto-fix: no (requiere revisión humana)")
	}
}

// ---------- modo conversacional ----------

type state struct {
	GreetingSent bool      `json:"greeting_sent"`
	PendingText  string    `json:"pending_text"`
	PendingID    string    `json:"pending_id"`
	LastUserText string    `json:"last_user_text"`
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

// cmdNextQuestion: comportamiento proactivo.
//   - Primera vez: corre scan automáticamente y reporta resumen.
//   - Si hay PendingText: lo entrega y limpia.
//   - Si no: {} (nada pendiente, espera respuesta del user).
func cmdNextQuestion(args []string) {
	fs := flag.NewFlagSet("next-question", flag.ExitOnError)
	statePath := fs.String("state", "", "path state.json")
	fs.Parse(args)
	sp := resolvePath(*statePath, "AUDITOR_STATE", defaultState)
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
		// Auto-scan + saludo proactivo.
		summary, err := autoScanSummary()
		if err != nil {
			summary = fmt.Sprintf("No pude correr el scan automático: %v", err)
		}
		s.GreetingSent = true
		_ = saveState(sp, s)
		emitJSON(map[string]string{
			"id":   fmt.Sprintf("auditor_intro_%d", time.Now().Unix()),
			"text": summary,
		})
		return
	}
	fmt.Println("{}")
}

func autoScanSummary() (string, error) {
	wp := resolvePath("", "AUDITOR_WORKING", defaultWorking)
	fp := resolvePath("", "AUDITOR_FINDINGS", defaultFindings)
	d, err := checks.LoadDataset(wp)
	if err != nil {
		return "", err
	}
	findings := checks.RunAll(d)
	if err := checks.SaveFindings(fp, findings); err != nil {
		return "", err
	}
	totalRecords := 0
	for _, recs := range d.Endpoints {
		totalRecords += len(recs)
	}
	bySev := map[string]int{}
	for _, f := range findings {
		bySev[f.Severity]++
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Hola, soy el auditor. Acabo de revisar %d registros del ERP y encontré %d anomalías.\n",
		totalRecords, len(findings))
	if len(findings) > 0 {
		autoCount := 0
		for _, f := range findings {
			if f.AutoFixable {
				autoCount++
			}
		}
		fmt.Fprintf(&sb, "Desglose: %d críticas, %d advertencias, %d informativas. Auto-corregibles: %d.\n",
			bySev[checks.SeverityCritical], bySev[checks.SeverityWarning], bySev[checks.SeverityInfo], autoCount)
		// Mostramos top-3 como ejemplo.
		shown := 0
		for _, f := range findings {
			if shown >= 3 {
				break
			}
			fmt.Fprintf(&sb, "  • %s — %s\n", f.ID, f.Message)
			shown++
		}
		if len(findings) > 3 {
			fmt.Fprintf(&sb, "...y %d más.\n", len(findings)-3)
		}
		if autoCount > 0 {
			fmt.Fprintf(&sb, "\nDe los %d hallazgos, %d son auto-corregibles. ¿Querés que el mecánico se encargue? Decime \"sí, arreglá\" o pediime el detalle de uno (ej. \"detalle F-001\").", len(findings), autoCount)
		} else {
			sb.WriteString("\nNinguno es auto-corregible: necesitan revisión humana. Pediime el detalle de uno (ej. \"detalle F-001\").")
		}
	} else {
		sb.WriteString("El dataset pasó todos los checks sin observaciones.")
	}
	return sb.String(), nil
}

// cmdIngestAnswer interpreta comandos del user en lenguaje natural mínimo:
//   - "detalle F-XXX" / "ver F-XXX" / "F-XXX" → detalle de finding
//   - "lista" / "listar" / "todos" → listado completo
//   - "scan" / "rescan" / "revisar" → re-ejecuta scan
//   - "reset" → reset
//
// Cualquier otra cosa: respuesta conversacional simple.
func cmdIngestAnswer(args []string) {
	fs := flag.NewFlagSet("ingest-answer", flag.ExitOnError)
	questionID := fs.String("question-id", "", "id de la pregunta")
	answer := fs.String("answer", "", "respuesta del user")
	statePath := fs.String("state", "", "path state.json")
	fs.Parse(args)
	_ = *questionID
	if *answer == "" {
		fail("ingest-answer: --answer requerido")
	}
	sp := resolvePath(*statePath, "AUDITOR_STATE", defaultState)
	s := loadState(sp)
	s.LastUserText = *answer

	reply := interpret(strings.TrimSpace(*answer))
	if reply == sentinelDelegateToMecanico {
		// Quedamos idle: no seteamos PendingText. El orquestador hará polling
		// del siguiente driver (mecánico) y será quien hable a continuación.
		s.PendingText = ""
		s.PendingID = ""
	} else {
		s.PendingText = reply
		s.PendingID = fmt.Sprintf("auditor_reply_%d", time.Now().Unix())
	}
	if err := saveState(sp, s); err != nil {
		fail("save state: %v", err)
	}
}

// sentinelDelegateToMecanico es la marca interna que devuelve interpret
// cuando la intención del user debe ser manejada por el mecánico. El
// caller (cmdIngestAnswer) la traduce en "no setear PendingText" para
// que el orquestador pase el turno al siguiente driver.
const sentinelDelegateToMecanico = "\x00DELEGATE_MECANICO\x00"

func interpret(text string) string {
	low := strings.ToLower(text)
	// Intent de fix → delegar al mecánico (señal idle).
	if looksLikeFixIntent(low) {
		return sentinelDelegateToMecanico
	}
	// Detectar "F-NNN" en cualquier parte del texto.
	if id := extractFindingID(text); id != "" {
		fp := resolvePath("", "AUDITOR_FINDINGS", defaultFindings)
		findings, err := checks.LoadFindings(fp)
		if err != nil {
			return "No tengo findings cargados todavía. Pediime un scan primero."
		}
		for _, f := range findings {
			if f.ID == id {
				return formatFindingForUser(f)
			}
		}
		return fmt.Sprintf("No encontré el hallazgo %s.", id)
	}
	switch {
	case strings.Contains(low, "lista") || strings.Contains(low, "todos") || strings.Contains(low, "listar"):
		return formatListForUser()
	case strings.Contains(low, "rescan") || strings.Contains(low, "re-scan") || strings.Contains(low, "revisar de nuevo") || strings.Contains(low, "scan"):
		summary, err := autoScanSummary()
		if err != nil {
			return fmt.Sprintf("Error al rescanear: %v", err)
		}
		return summary
	case strings.Contains(low, "reset"):
		return "Para resetear, corré 'auditor reset' por CLI o pediselo al mecánico."
	}
	// Conversación general → LLM con system prompt del auditor.
	return interpretWithLLM(text)
}

const auditorSystemPrompt = `Eres el Auditor de datos de Remora. Tu rol es revisar datasets JSON de ERPs, detectar anomalías de integridad referencial, campos requeridos faltantes, fechas inválidas, valores fuera de rango y datos huérfanos.

Tu personalidad:
- Eres directo, técnico y preciso.
- Hablas en español rioplatense informal ("pediime", "fijate", "decime").
- Nunca inventas datos ni hallazgos.
- Si no sabés algo, lo decís.

Capacidades que podés ofrecer al usuario:
- Mostrar detalle de un hallazgo ("detalle F-001")
- Listar todos los hallazgos ("lista")
- Volver a escanear ("rescan")
- Delegar al mecánico para arreglar ("arreglá los auto")

Contexto actual:
%s

Respondé de forma breve y útil. Si el usuario pregunta algo fuera de tu alcance, explicá qué podés hacer.`

func interpretWithLLM(userText string) string {
	client, err := llm.NewClient()
	if err != nil {
		return fmt.Sprintf("Error iniciando LLM: %v", err)
	}
	// Construir contexto de findings actuales.
	contextStr := buildFindingsContext()
	system := fmt.Sprintf(auditorSystemPrompt, contextStr)
	reply, err := client.Generate(context.Background(), system, userText)
	if err != nil {
		return fmt.Sprintf("Error del LLM: %v", err)
	}
	return reply
}

func buildFindingsContext() string {
	fp := resolvePath("", "AUDITOR_FINDINGS", defaultFindings)
	findings, err := checks.LoadFindings(fp)
	if err != nil {
		return "No hay findings cargados (no se corrió scan)."
	}
	if len(findings) == 0 {
		return "Último scan: 0 hallazgos. Dataset limpio."
	}
	bySev := map[string]int{}
	autoCount := 0
	for _, f := range findings {
		bySev[f.Severity]++
		if f.AutoFixable {
			autoCount++
		}
	}
	return fmt.Sprintf("Último scan: %d hallazgos (%d críticos, %d advertencias, %d informativos). Auto-corregibles: %d.",
		len(findings), bySev[checks.SeverityCritical], bySev[checks.SeverityWarning], bySev[checks.SeverityInfo], autoCount)
}

// looksLikeFixIntent detecta intenciones afirmativas o de remediación.
func looksLikeFixIntent(low string) bool {
	fixVerbs := []string{"arregl", "corregi", "corrigi", "corregí", "aplic", "fix", "reparar", "solucion", "mecanic", "propon", "delega"}
	for _, v := range fixVerbs {
		if strings.Contains(low, v) {
			return true
		}
	}
	// Afirmativos cortos sólo si vienen sin otra intención que ya hayamos parseado.
	trim := strings.TrimSpace(low)
	affirmatives := map[string]bool{
		"si": true, "sí": true, "sí.": true, "si.": true,
		"ok": true, "dale": true, "dale!": true,
		"sí dale": true, "si dale": true, "si, dale": true, "sí, dale": true,
		"sí arreglá": true, "si arregla": true, "sí, arreglá": true, "si, arregla": true,
	}
	return affirmatives[trim]
}

func extractFindingID(text string) string {
	upper := strings.ToUpper(text)
	idx := strings.Index(upper, "F-")
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

func formatFindingForUser(f checks.Finding) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s — %s\n", f.ID, f.Message)
	if f.Suggestion != "" {
		fmt.Fprintf(&sb, "Sugerencia: %s\n", f.Suggestion)
	}
	if len(f.Evidence) > 0 {
		sb.WriteString("Evidencia: ")
		parts := make([]string, 0, len(f.Evidence))
		keys := make([]string, 0, len(f.Evidence))
		for k := range f.Evidence {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", k, f.Evidence[k]))
		}
		sb.WriteString(strings.Join(parts, ", "))
		sb.WriteString("\n")
	}
	if f.AutoFixable {
		sb.WriteString("Este hallazgo es auto-corregible: el mecánico puede proponer un fix.")
	} else {
		sb.WriteString("Este hallazgo necesita revisión humana antes de tocarlo.")
	}
	return sb.String()
}

func formatListForUser() string {
	fp := resolvePath("", "AUDITOR_FINDINGS", defaultFindings)
	findings, err := checks.LoadFindings(fp)
	if err != nil {
		return "No tengo findings. Corré scan primero."
	}
	if len(findings) == 0 {
		return "Sin hallazgos."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Tengo %d hallazgos:\n", len(findings))
	for _, f := range findings {
		auto := ""
		if f.AutoFixable {
			auto = " (auto-corregible)"
		}
		fmt.Fprintf(&sb, "• %s [%s] %s%s\n", f.ID, f.Severity, f.Message, auto)
	}
	return sb.String()
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
