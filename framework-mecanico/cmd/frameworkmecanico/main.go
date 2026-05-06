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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"framework-mecanico/fixers"
	"framework-mecanico/internal/auditdata"
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
	case "reset":
		cmdReset(os.Args[2:])
	case "next-question":
		cmdNextQuestion(os.Args[2:])
	case "ingest-answer":
		cmdIngestAnswer(os.Args[2:])
	case "draft-email":
		cmdDraftEmail(os.Args[2:])
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

// ---------- propose ----------

func cmdPropose(args []string) {
	fs := flag.NewFlagSet("propose", flag.ExitOnError)
	findingID := fs.String("finding-id", "", "id del finding")
	fs.Parse(args)
	if *findingID == "" {
		fail("propose: --finding-id requerido")
	}
	fp, dp, pp, _, _ := paths()
	finds, err := auditdata.LoadFindings(fp)
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
	ds, err := auditdata.LoadDataset(dp)
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
	fp, dp, pp, _, _ := paths()
	finds, err := auditdata.LoadFindings(fp)
	if err != nil {
		fail("load findings: %v", err)
	}
	ds, err := auditdata.LoadDataset(dp)
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
	rec, err := fixers.Apply(*target, dp, ap)
	if err != nil {
		fail("apply: %v", err)
	}
	if err := fixers.RemoveProposal(pp, target.ID); err != nil {
		fail("remove proposal: %v", err)
	}
	fmt.Printf("Aplicado %s sobre %s:%s.%s   antes=%v   después=%v\n",
		target.ID, rec.Endpoint, rec.RecordID, rec.Field,
		displayValue(rec.Before), displayValue(rec.After))
}

func cmdApplyAll(args []string) {
	_, dp, pp, ap, _ := paths()
	props, err := fixers.LoadProposals(pp)
	if err != nil {
		fail("load proposals: %v", err)
	}
	if len(props) == 0 {
		fmt.Println("Sin propuestas para aplicar.")
		return
	}
	applied := 0
	failed := 0
	for _, p := range props {
		rec, err := fixers.Apply(p, dp, ap)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s falló: %v\n", p.ID, err)
			failed++
			continue
		}
		fmt.Printf("✓ %s   %s:%s.%s   %v → %v\n", p.ID, rec.Endpoint, rec.RecordID, rec.Field,
			displayValue(rec.Before), displayValue(rec.After))
		applied++
	}
	// Limpiamos las propuestas aplicadas. apply-all aplica todo, así que
	// vaciamos el archivo (las que fallaron quedan registradas en stderr).
	_ = fixers.SaveProposals(pp, nil)
	fmt.Printf("\nAplicadas %d propuestas. Fallidas: %d.\n", applied, failed)
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
	finds, _ := auditdata.LoadFindings(fp)
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
		ds, err := auditdata.LoadDataset(dp)
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
	if hasProps {
		return "Puedo aplicar las propuestas pendientes. Decime \"sí, aplicá todo\" o \"aplicá P-XXX\" para una puntual."
	}
	return "Puedo: \"propone los auto\", \"ver propuestas\", \"aplicá P-001\", \"aplicá todo\"."
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
	finds, err := auditdata.LoadFindings(fp)
	if err != nil {
		return fmt.Sprintf("No tengo findings: %v. Pedile al auditor un scan primero.", err)
	}
	ds, err := auditdata.LoadDataset(dp)
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
	save := fs.Bool("save", true, "guardar en state")
	fs.Parse(args)

	if *deudor == "" {
		fail("draft-email requiere --deudor")
	}

	draft := generateEmailDraft(*deudor, *tono, *saldo, *dias)

	if *save {
		stateDir := "temp/mecanico"
		_ = os.MkdirAll(stateDir, 0755)

		// Guardar draft como JSON
		draftPath := filepath.Join(stateDir, "last_draft.json")
		data, _ := json.MarshalIndent(draft, "", "  ")
		_ = os.WriteFile(draftPath, data, 0644)

		// Guardar respuesta para orquestador
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

	// Imprimir respuesta legible
	fmt.Println(formatDraftForUser(draft))
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

func generateEmailDraft(deudor, tono string, saldo float64, dias int) *emailDraft {
	estudio := os.Getenv("ESTUDIO_NOMBRE")
	if estudio == "" {
		estudio = "Estudio Jurídico Remora"
	}
	cobrador := os.Getenv("COBRADOR_NOMBRE")
	if cobrador == "" {
		cobrador = "Departamento de Cobranza"
	}
	fecha := time.Now().Format("02/01/2006")
	email := strings.ToLower(strings.ReplaceAll(deudor, " ", "")) + "@ejemplo.cl"

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
		urlEncode(email), urlEncode(subject), urlEncode(body))

	return &emailDraft{
		Type:             "action_proposal",
		Action:           "email",
		Deudor:           deudor,
		Subject:          subject,
		Body:             body,
		To:               email,
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
