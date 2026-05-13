package autonomia

import (
	"strings"
	"testing"
)

func TestAcceptanceSessionEndToEnd(t *testing.T) {
	boot := Bootstrap()
	assertContainsAll(t, boot.Text, "panorama general", "universo analizado")
	assertNotContainsAny(t, boot.Text, "cliente protagonista", "nicolas")
	if len(boot.Shortlist) < 3 {
		t.Fatalf("bootstrap shortlist len = %d", len(boot.Shortlist))
	}
	if boot.Mode != ModeGeneral {
		t.Fatalf("bootstrap mode = %s", boot.Mode)
	}

	state := boot.NextSessionState

	steps := []struct {
		name  string
		input string
		check func(t *testing.T, got Response)
	}{
		{
			name:  "hola stays in general",
			input: "Hola",
			check: func(t *testing.T, got Response) {
				if got.Mode != ModeGeneral {
					t.Fatalf("mode = %s", got.Mode)
				}
				assertContainsAll(t, got.Text, "panorama general", "listo para ayudarte")
				assertNotContainsAny(t, got.Text, "nicolas", "cliente protagonista")
			},
		},
		{
			name:  "como estas is social and natural",
			input: "¿Cómo estás?",
			check: func(t *testing.T, got Response) {
				if got.Mode != ModeGeneral {
					t.Fatalf("mode = %s", got.Mode)
				}
				if got.TurnType != "social" {
					t.Fatalf("turn type = %q", got.TurnType)
				}
				assertContainsAll(t, got.Text, "bien", "listo para ayudarte", "panorama general")
				assertNotContainsAny(t, got.Text, "sigo en panorama general del universo analizado si quieres puedo describir")
			},
		},
		{
			name:  "que is repair clarification",
			input: "¿Qué?",
			check: func(t *testing.T, got Response) {
				if got.Mode != ModeGeneral {
					t.Fatalf("mode = %s", got.Mode)
				}
				if got.TurnType != "repair" {
					t.Fatalf("turn type = %q", got.TurnType)
				}
				assertContainsAll(t, got.Text, "puedo aclararte", "universo", "patrones generales", "focos")
			},
		},
		{
			name:  "universe question keeps general",
			input: "¿Qué universo estoy viendo?",
			check: func(t *testing.T, got Response) {
				if got.Mode != ModeGeneral {
					t.Fatalf("mode = %s", got.Mode)
				}
				assertContainsAll(t, got.Text, "dataset embebido", "entidad raiz", "due_date")
			},
		},
		{
			name:  "general patterns remain general",
			input: "¿Qué patrones generales ves aquí?",
			check: func(t *testing.T, got Response) {
				if got.Mode != ModeGeneral {
					t.Fatalf("mode = %s", got.Mode)
				}
				assertContainsAll(t, got.Text, "composicion", "temporalidad", "shortlist comparativa")
				assertNotContainsAny(t, got.Text, "analisis individual vuelvo")
			},
		},
		{
			name:  "focus suggests drilldown without switching mode",
			input: "¿Qué foco amerita drill-down?",
			check: func(t *testing.T, got Response) {
				if got.Mode != ModeGeneral {
					t.Fatalf("mode = %s", got.Mode)
				}
				assertContainsAll(t, got.Text, "shortlist", "consultoria regulatoria", "proyectos activos", "vacios de gobernanza")
				if len(got.Shortlist) < 3 {
					t.Fatalf("shortlist len = %d", len(got.Shortlist))
				}
			},
		},
		{
			name:  "operational priority is guarded as shortlist",
			input: "¿A quién le puedo cobrar primero?",
			check: func(t *testing.T, got Response) {
				if got.Mode != ModeGeneral {
					t.Fatalf("mode = %s", got.Mode)
				}
				if got.Guardrail != "priorizacion_operativa" {
					t.Fatalf("guardrail = %q", got.Guardrail)
				}
				assertContainsAll(t, got.Text, "no responderia", "shortlist analitica", "confirmacion de caso")
				if len(got.Shortlist) < 3 {
					t.Fatalf("shortlist len = %d", len(got.Shortlist))
				}
			},
		},
		{
			name:  "open case switches mode",
			input: "Abramos el análisis individual del foco principal",
			check: func(t *testing.T, got Response) {
				if got.Mode != ModeCase {
					t.Fatalf("mode = %s", got.Mode)
				}
				if got.CaseContext == nil || got.CaseContext.Label != "Consultoría Regulatoria" {
					t.Fatalf("case context = %+v", got.CaseContext)
				}
				assertContainsAll(t, got.HandoffReason, "Foco elegido desde el panorama general", "Consultoría Regulatoria")
			},
		},
		{
			name:  "case evidence speaks as case",
			input: "¿Qué evidencia hay y qué acción sugerirías?",
			check: func(t *testing.T, got Response) {
				if got.Mode != ModeCase {
					t.Fatalf("mode = %s", got.Mode)
				}
				assertContainsAll(t, got.Text, "analisis individual", "validar internamente cargo y documento", "consultoria regulatoria")
			},
		},
		{
			name:  "return to general preserves memory",
			input: "Volvamos al panorama general",
			check: func(t *testing.T, got Response) {
				if got.Mode != ModeGeneral {
					t.Fatalf("mode = %s", got.Mode)
				}
				assertContainsAll(t, got.Text, "panorama general", "universo analizado")
				assertContainsAll(t, got.Context.Memory, "Regreso al panorama general con memoria del caso revisado", "Consultoría Regulatoria")
				assertNotContainsAny(t, got.Text, "analisis individual")
			},
		},
		{
			name:  "forecast is blocked",
			input: "¿Cuánto voy a recuperar este mes?",
			check: func(t *testing.T, got Response) {
				if !got.ForecastBlocked {
					t.Fatal("expected forecast blocked")
				}
				assertContainsAll(t, got.Text, "no puedo estimar recuperacion futura con rigor", "cobro historico observado")
			},
		},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			got := HandleMessage(state, step.input)
			step.check(t, normalizeResponse(t, got))
			state = got.NextSessionState
		})
	}
}

func normalizeResponse(t *testing.T, resp Response) Response {
	t.Helper()
	resp.Text = normalize(resp.Text)
	resp.HandoffReason = normalize(resp.HandoffReason)
	resp.Context.Memory = normalize(resp.Context.Memory)
	if resp.CaseContext != nil {
		copy := *resp.CaseContext
		copy.Label = strings.TrimSpace(copy.Label)
		resp.CaseContext = &copy
	}
	return resp
}

func assertContainsAll(t *testing.T, text string, terms ...string) {
	t.Helper()
	for _, term := range terms {
		if !strings.Contains(text, normalize(term)) {
			t.Fatalf("text %q does not contain %q", text, normalize(term))
		}
	}
}

func assertNotContainsAny(t *testing.T, text string, terms ...string) {
	t.Helper()
	for _, term := range terms {
		if strings.Contains(text, normalize(term)) {
			t.Fatalf("text %q unexpectedly contains %q", text, normalize(term))
		}
	}
}
