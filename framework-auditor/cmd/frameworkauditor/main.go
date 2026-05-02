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
//	    Modo conversacional para el orquestador flujo_api.
package main

import (
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
	fmt.Print(`frameworkauditor: agente de auditoría sobre dataset JSON-API

Uso:
  frameworkauditor reset
  frameworkauditor scan [--source <path>] [--out <path>]
  frameworkauditor list [--severity critical|warning|info] [--json]
  frameworkauditor detail --id F-001 [--json]
  frameworkauditor next-question
  frameworkauditor ingest-answer --question-id <id> --answer <text>

Variables de entorno:
  AUDITOR_GOLDEN    override dataset.golden.json
  AUDITOR_WORKING   override dataset.working.json
  AUDITOR_FINDINGS  override findings.json
  AUDITOR_STATE     override state.json
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
	source := fs.String("source", "", "path del dataset")
	out := fs.String("out", "", "path findings.json")
	fs.Parse(args)

	wp := resolvePath(*source, "AUDITOR_WORKING", defaultWorking)
	fp := resolvePath(*out, "AUDITOR_FINDINGS", defaultFindings)

	d, err := checks.LoadDataset(wp)
	if err != nil {
		fail("load: %v", err)
	}
	findings := checks.RunAll(d)
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
	fmt.Printf("Scan completo: %d registros revisados, %d hallazgos.\n", totalRecords, len(findings))
	fmt.Printf("  críticos: %d   advertencias: %d   informativos: %d\n",
		bySev[checks.SeverityCritical], bySev[checks.SeverityWarning], bySev[checks.SeverityInfo])
	fmt.Printf("Findings persistidos en %s\n", fp)
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
	GreetingSent  bool      `json:"greeting_sent"`
	PendingText   string    `json:"pending_text"`
	PendingID     string    `json:"pending_id"`
	LastUserText  string    `json:"last_user_text"`
	UpdatedAt     time.Time `json:"updated_at"`
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
	return "Puedo darte el detalle de un hallazgo (ej. \"detalle F-001\"), listar todos (\"lista\"), volver a escanear (\"rescan\") o pasarle el turno al mecánico (\"arreglá los auto\")."
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
