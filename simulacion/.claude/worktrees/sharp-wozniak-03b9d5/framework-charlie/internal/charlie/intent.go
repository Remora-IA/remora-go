package charlie

import (
	"fmt"
	"sort"
	"strings"
)

// IntentPlan is a deterministic pipeline of Charlie commands that satisfies
// a high-level intent. The router exists so that the AI operator does not
// need to reason about policy trees in INITIAL_PROMPT.md - it asks for an
// intent and executes the returned steps.
type IntentPlan struct {
	Intent   string
	Steps    []IntentStep
	Notes    []string
	Ambiguity []string
}

type IntentStep struct {
	Command     string // e.g. "go run ./cmd/charlie doctor --apply"
	Reason      string
	Optional    bool
}

// intentKeywords is a tiny rule-based classifier. It is intentionally not
// powered by an LLM: the AI calling Charlie already is the LLM. Charlie's
// job is to be predictable.
var intentKeywords = []struct {
	Match []string
	ID    string
}{
	{[]string{"doctor", "diagn", "health", "salud", "chequeo"}, "doctor"},
	{[]string{"commit", "commitea", "commitear", "push", "version nueva", "nueva version", "nueva release", "apply-propose"}, "commit-and-push"},
	{[]string{"reparar", "repair", "arregl", "recover", "recupera", "fix"}, "recover"},
	{[]string{"publicar draft", "publish draft", "push draft"}, "publish-draft"},
	{[]string{"actualiza main", "update main", "publish main", "push main"}, "publish-main"},
	{[]string{"amend", "agregar a version existente", "meter en v", "agregar cambios a v"}, "amend"},
	{[]string{"reconcil", "divergencia", "diverged"}, "reconcile"},
	{[]string{"backup", "respaldar", "resguardar"}, "backup"},
	{[]string{"estado", "status", "que hay"}, "status"},
}

// BuildIntentPlan maps a natural-language intent to a deterministic pipeline.
func BuildIntentPlan(intent string) *IntentPlan {
	lc := strings.ToLower(strings.TrimSpace(intent))
	plan := &IntentPlan{Intent: intent}
	if lc == "" {
		plan.Ambiguity = append(plan.Ambiguity, "intent vacio; usa --intent \"commit and push\" por ejemplo")
		return plan
	}

	ids := matchIntents(lc)
	if len(ids) == 0 {
		plan.Ambiguity = append(plan.Ambiguity, "no reconoci el intent; usa uno de los ejemplos en la ayuda")
		return plan
	}

	// Primary intent wins; secondary ones are logged as ambiguity.
	primary := ids[0]
	if len(ids) > 1 {
		plan.Ambiguity = append(plan.Ambiguity, fmt.Sprintf("multiples intents posibles: %s; elegi %q", strings.Join(ids, ", "), primary))
	}

	switch primary {
	case "doctor":
		plan.Steps = []IntentStep{
			{Command: "go run ./cmd/charlie doctor", Reason: "diagnostico inicial (solo lectura)"},
			{Command: "go run ./cmd/charlie doctor --apply", Reason: "aplicar recetas seguras si hay bloqueos", Optional: true},
		}
	case "commit-and-push":
		plan.Steps = []IntentStep{
			{Command: "go run ./cmd/charlie doctor", Reason: "verificar integridad antes de commitear"},
			{Command: "go run ./cmd/charlie preflight", Reason: "backup liviano y chequeos de branch/upstream"},
			{Command: "go run ./cmd/charlie propose", Reason: "ver el changelog y commit propuesto (dry-run)"},
			{Command: "go run ./cmd/charlie apply-propose --apply --push", Reason: "aplicar commit, tag y push en un solo paso controlado"},
		}
	case "recover":
		plan.Steps = []IntentStep{
			{Command: "go run ./cmd/charlie doctor", Reason: "clasificar el tipo de corrupcion"},
			{Command: "go run ./cmd/charlie doctor --apply", Reason: "auto-recover recetas seguras (fetch-missing-objects, disable-gc-auto)"},
		}
		plan.Notes = append(plan.Notes, "si doctor no recupera, usa repair-release vX.Y.Z --apply")
	case "publish-draft":
		plan.Steps = []IntentStep{
			{Command: "go run ./cmd/charlie preflight", Reason: "validar branch/upstream"},
			{Command: "go run ./cmd/charlie publish-draft --apply", Reason: "push seguro (force-with-lease si hace falta)"},
		}
	case "publish-main":
		plan.Steps = []IntentStep{
			{Command: "go run ./cmd/charlie preflight", Reason: "validar estado"},
			{Command: "go run ./cmd/charlie publish-main --apply", Reason: "actualizar main a una copia exacta de draft"},
		}
	case "amend":
		plan.Steps = []IntentStep{
			{Command: "go run ./cmd/charlie amend-plan <version>", Reason: "diagnosticar si es seguro amendar", Optional: false},
			{Command: "go run ./cmd/charlie repair-release <version> --apply", Reason: "aplicar si no hay bloqueos"},
		}
		plan.Notes = append(plan.Notes, "reemplaza <version> por el tag real (ej. v0.1.4)")
	case "reconcile":
		plan.Steps = []IntentStep{
			{Command: "go run ./cmd/charlie reconcile-draft", Reason: "obtener la politica segura de reconciliacion"},
		}
	case "backup":
		plan.Steps = []IntentStep{
			{Command: "go run ./cmd/charlie backup", Reason: "backup liviano fuera del repo"},
		}
	case "status":
		plan.Steps = []IntentStep{
			{Command: "go run ./cmd/charlie status", Reason: "lista cambios y proxima version lineal"},
		}
	}
	return plan
}

func matchIntents(lc string) []string {
	scores := map[string]int{}
	for _, rule := range intentKeywords {
		for _, kw := range rule.Match {
			if strings.Contains(lc, kw) {
				scores[rule.ID]++
			}
		}
	}
	if len(scores) == 0 {
		return nil
	}
	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if scores[ids[i]] != scores[ids[j]] {
			return scores[ids[i]] > scores[ids[j]]
		}
		return ids[i] < ids[j]
	})
	return ids
}

func FormatIntentPlan(plan *IntentPlan) string {
	var b strings.Builder
	b.WriteString("=== CHARLIE PLAN ===\n\n")
	fmt.Fprintf(&b, "intent: %s\n\n", plan.Intent)
	if len(plan.Steps) == 0 {
		b.WriteString("Sin pasos. Ambiguedades:\n")
		for _, a := range plan.Ambiguity {
			fmt.Fprintf(&b, "  - %s\n", a)
		}
		return b.String()
	}
	b.WriteString("pasos en orden:\n")
	for i, s := range plan.Steps {
		opt := ""
		if s.Optional {
			opt = " [opcional]"
		}
		fmt.Fprintf(&b, "  %d. %s%s\n     -> %s\n", i+1, s.Command, opt, s.Reason)
	}
	if len(plan.Notes) > 0 {
		b.WriteString("\nnotas:\n")
		for _, n := range plan.Notes {
			fmt.Fprintf(&b, "  - %s\n", n)
		}
	}
	if len(plan.Ambiguity) > 0 {
		b.WriteString("\nambiguedades detectadas:\n")
		for _, a := range plan.Ambiguity {
			fmt.Fprintf(&b, "  - %s\n", a)
		}
	}
	return b.String()
}
