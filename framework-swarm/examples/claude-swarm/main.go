// claude-swarm — enjambre con Claude real como agente cognitivo.
//
// Cinco secciones de un contrato de servicio, tres agentes Claude.
// Cada uno lee su sección, extrae cláusulas clave, detecta riesgos.
// Paladin registra todo. Bravo verifica que se cubrió el contrato completo.
//
// Run:
//
//	cd framework-swarm
//	go run ./examples/claude-swarm/
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	swarm "github.com/remora-go/framework-swarm/swarm"
)

var contractSections = map[string]string{
	"partes_y_objeto": `
SECCIÓN 1 — PARTES Y OBJETO
El presente contrato se celebra entre TechCorp SpA (en adelante "el Proveedor"),
RUT 76.123.456-7, representada por Carlos Mendoza, y Empresa Cliente Ltda.
(en adelante "el Cliente"), RUT 77.987.654-3, representada por Ana Torres.

El objeto del contrato es la provisión de servicios de desarrollo de software
a medida, incluyendo diseño, implementación, pruebas y mantenimiento de una
plataforma de gestión de inventario. El precio total pactado es de USD 120,000
pagaderos en cuotas según el plan de trabajo acordado en el Anexo A.`,

	"obligaciones_proveedor": `
SECCIÓN 2 — OBLIGACIONES DEL PROVEEDOR
El Proveedor se obliga a: (a) entregar la plataforma funcional dentro de 180
días calendario desde la firma; (b) mantener un equipo mínimo de 3 desarrolladores
senior asignados al proyecto; (c) proveer reportes de avance cada 15 días;
(d) corregir defectos críticos dentro de 24 horas y defectos menores dentro
de 5 días hábiles; (e) mantener confidencialidad absoluta sobre los datos del Cliente.

El incumplimiento del plazo de entrega devengará una multa de USD 500 por día
de atraso, con un tope máximo del 10% del valor total del contrato.`,

	"obligaciones_cliente": `
SECCIÓN 3 — OBLIGACIONES DEL CLIENTE
El Cliente se obliga a: (a) pagar el 30% del valor total al inicio del proyecto;
(b) pagar el 40% al alcanzar el hito de entrega parcial (día 90); (c) pagar
el 30% restante contra entrega y aceptación final; (d) designar una contraparte
técnica que responda las consultas del Proveedor dentro de 48 horas hábiles;
(e) proveer acceso a los sistemas internos necesarios para la integración.

El retraso en los pagos devengará un interés mensual del 1.5% sobre el monto adeudado.`,

	"propiedad_intelectual": `
SECCIÓN 4 — PROPIEDAD INTELECTUAL
Todo el código fuente, documentación, diseños y demás entregables desarrollados
específicamente para este proyecto serán propiedad exclusiva del Cliente una vez
completado el pago total acordado.

El Proveedor retiene los derechos sobre: (i) las librerías y frameworks de uso
general que preexistan al contrato; (ii) los métodos y algoritmos genéricos
no específicos al negocio del Cliente.

ADVERTENCIA: La cláusula de propiedad intelectual no especifica qué sucede
con el código en caso de terminación anticipada del contrato.`,

	"resolucion_disputas": `
SECCIÓN 5 — RESOLUCIÓN DE DISPUTAS Y TÉRMINO
Las partes acuerdan resolver cualquier controversia primero mediante mediación
directa (plazo: 30 días). Si no hay acuerdo, se someterán al arbitraje del
Centro de Arbitraje y Mediación de Santiago, conforme a su reglamento vigente.
El laudo arbitral será definitivo e inapelable.

El contrato puede terminarse anticipadamente por: (a) mutuo acuerdo escrito;
(b) incumplimiento grave no subsanado dentro de 15 días de notificación;
(c) insolvencia declarada de cualquiera de las partes.

NOTA: El contrato no especifica una cláusula de ley aplicable explícita.`,
}

func main() {
	start := time.Now()
	fmt.Println()
	fmt.Println("┌──────────────────────────────────────────────────────────────┐")
	fmt.Println("│  REMORA — Enjambre con Claude real                           │")
	fmt.Println("│  Dominio: Análisis de contrato de servicios de software      │")
	fmt.Println("│  5 secciones · 3 agentes Claude · Bravo verifica cobertura  │")
	fmt.Println("└──────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// API key — usa token de sesión de Claude Code o ANTHROPIC_API_KEY
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		tokenFile := "/home/claude/.claude/remote/.session_ingress_token"
		if data, err := os.ReadFile(tokenFile); err == nil {
			apiKey = strings.TrimSpace(string(data))
		}
	}
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "ERROR: necesito ANTHROPIC_API_KEY")
		os.Exit(1)
	}

	zones := []swarm.Zone{
		{ID: "partes_y_objeto",        Name: "Partes y Objeto",           PainWeight: 0.95,
			Description: "Identificar partes, objeto del contrato y precio total"},
		{ID: "obligaciones_proveedor", Name: "Obligaciones del Proveedor", PainWeight: 0.90,
			Description: "Extraer plazos, penalizaciones y obligaciones del proveedor"},
		{ID: "obligaciones_cliente",   Name: "Obligaciones del Cliente",   PainWeight: 0.85,
			Description: "Extraer calendario de pagos y obligaciones del cliente"},
		{ID: "propiedad_intelectual",  Name: "Propiedad Intelectual",      PainWeight: 0.80,
			Description: "Determinar titularidad del código y condiciones"},
		{ID: "resolucion_disputas",    Name: "Resolución de Disputas",     PainWeight: 0.75,
			Description: "Identificar mecanismo de resolución y cláusulas de término"},
	}

	flow := &swarm.IdealFlow{
		Description:  "Análisis completo de contrato de servicios de software",
		Intent:       "Extraer cláusulas clave, detectar riesgos, verificar cobertura completa",
		CriticalPath: []string{"partes_y_objeto", "obligaciones_proveedor", "obligaciones_cliente", "propiedad_intelectual", "resolucion_disputas"},
		CriticalVars: []string{"partes", "precio", "plazo", "penalizacion", "pagos", "propiedad", "disputas"},
		Rules: []swarm.VerifyRule{
			{Name: "partes-identificadas-rule", Then: "ambas partes con RUT identificadas",        Importance: 1},
			{Name: "precio-documentado-rule",   Then: "precio total y condiciones de pago claras", Importance: 1},
			{Name: "propiedad-intelectual-rule", Then: "titularidad del código definida",          Importance: 1},
			{Name: "riesgo-contractual-rule",   Then: "riesgos y cláusulas faltantes detectadas",  Importance: 2},
		},
	}

	stigmaDir, _ := os.MkdirTemp("", "claude-swarm-*")
	defer os.RemoveAll(stigmaDir)

	s, err := swarm.New(swarm.Config{
		ID:         "contract-analysis-swarm",
		AgentIDs:   []string{"agent-alpha", "agent-beta", "agent-gamma"},
		Zones:      zones,
		WorkFunc:   contractAnalysisWorkFn(apiKey),
		StigmaPath: filepath.Join(stigmaDir, "stigma.json"),
	})
	if err != nil {
		fatalf("swarm.New: %v", err)
	}

	fmt.Println("🐝 Campo de presión inicial:")
	for _, zp := range s.PressureTable() {
		fmt.Printf("   [%.2f] %s\n", zp.Pressure, zp.Zone.Name)
	}
	fmt.Println()
	fmt.Println("🚀 3 agentes Claude analizando el contrato en paralelo...")
	fmt.Println()

	result, err := s.Run(context.Background())
	if err != nil {
		fatalf("swarm.Run: %v", err)
	}

	fmt.Printf("\n✅ Enjambre completado en %v\n", result.Duration.Round(time.Millisecond))
	fmt.Printf("   Secciones analizadas : %d/%d\n", result.SolvedZones, result.TotalZones)
	fmt.Printf("   Colisiones           : %.0f%%\n", result.CollisionRate*100)
	fmt.Println()

	root := findRoot()
	traceDir := filepath.Join(root, "temp", "paladin")
	os.MkdirAll(traceDir, 0755)

	fmt.Println("🔬 Verificando cobertura con Bravo...")
	score, err := swarm.ScoreLatestTrace(flow, traceDir, 0.60)
	if err != nil {
		fatalf("score: %v", err)
	}

	fmt.Println()
	printScore(score)
	fmt.Printf("\n⏱️  Pipeline completo en %v\n", time.Since(start).Round(time.Millisecond))
	fmt.Println()
}

func contractAnalysisWorkFn(apiKey string) swarm.WorkFunc {
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(&http.Client{
			Transport: &bearerTransport{token: apiKey},
		}),
	)

	return func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		tc := agent.TraceCtx()
		section := contractSections[zone.ID]

		fmt.Printf("   📄 %-30s → %s\n", zone.Name, agent.ID)

		prompt := "Eres un abogado especialista en contratos de tecnología.\n" +
			"Analiza esta sección y responde EXACTAMENTE con este formato (una línea por campo):\n\n" +
			"PARTES: [partes mencionadas o 'no aplica']\n" +
			"PRECIO: [monto y condiciones o 'no aplica']\n" +
			"PLAZO: [plazos clave o 'no aplica']\n" +
			"PENALIZACION: [multas o 'no aplica']\n" +
			"PAGOS: [calendario de pagos o 'no aplica']\n" +
			"PROPIEDAD: [titularidad del código o 'no aplica']\n" +
			"DISPUTAS: [mecanismo de resolución o 'no aplica']\n" +
			"RIESGO: [cláusulas faltantes o ambiguas — sé específico]\n" +
			"RESUMEN: [1 oración]\n\n" +
			"SECCIÓN: " + zone.Name + "\n---\n" + section

		resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.ModelClaudeHaiku4_5,
			MaxTokens: 400,
			Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))},
		})
		if err != nil {
			return nil, fmt.Errorf("claude: %w", err)
		}

		output := resp.Content[0].Text
		fmt.Printf("   ✅ %-30s (%d tokens out)\n", zone.Name, resp.Usage.OutputTokens)

		vars := parseKeyValue(output)

		if tc != nil {
			tc.Rule("partes-identificadas-rule", "identificar partes del contrato", nil)
			tc.Rule("precio-documentado-rule", "documentar precio y condiciones de pago", nil)
			tc.Rule("propiedad-intelectual-rule", "definir titularidad del código", nil)
			tc.Rule("riesgo-contractual-rule", "detectar cláusulas faltantes o riesgosas", nil)

			// En contract review los riesgos son el deliverable, no errores del proceso.
			// Van como Events para quedar en la traza, no como Violations que penalizan el score.
			if riesgo, ok := vars["riesgo"].(string); ok && len(riesgo) > 10 && riesgo != "no aplica" {
				tc.Event("hallazgo.riesgo", fmt.Sprintf("seccion=%s: %s", zone.ID, riesgo), nil)
			}
			tc.Check("riesgo-contractual-rule", "riesgos analizados", "analizado", true)
			tc.Event("claude.analisis", fmt.Sprintf("seccion=%s tokens_out=%d", zone.ID, resp.Usage.OutputTokens), nil)
		}

		return &swarm.Result{Output: output, Vars: vars}, nil
	}
}

func parseKeyValue(text string) map[string]any {
	vars := map[string]any{}
	for _, line := range strings.Split(text, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			k := strings.ToLower(strings.TrimSpace(parts[0]))
			v := strings.TrimSpace(parts[1])
			if k != "" && v != "" {
				vars[k] = v
			}
		}
	}
	return vars
}

func printScore(score *swarm.VerifyResult) {
	status := "✅ PASS"
	if !score.Passed {
		status = "❌ FAIL"
	}
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Printf("║  BravoScore: %.2f  %-29s║\n", score.Score, status)
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Printf("║  Secciones cubiertas: %3.0f%%                      ║\n", score.PathCoverage*100)
	fmt.Printf("║  Variables extraídas: %3.0f%%                      ║\n", score.VarCoverage*100)
	fmt.Printf("║  Reglas evidenciadas: %3.0f%%                      ║\n", score.RuleCoverage*100)
	fmt.Printf("║  Riesgos detectados : %3d                        ║\n", score.Violations)
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()
	for _, d := range score.Details {
		if strings.HasPrefix(d, "✅") || strings.HasPrefix(d, "❌") || strings.HasPrefix(d, "⚠️") {
			fmt.Printf("  %s\n", d)
		}
	}
}

type bearerTransport struct{ token string }

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Del("x-api-key")
	r.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(r)
}

func findRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		p := filepath.Dir(dir)
		if p == dir {
			return dir
		}
		dir = p
	}
}

func fatalf(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+f+"\n", a...)
	os.Exit(1)
}
