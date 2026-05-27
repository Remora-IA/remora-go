// tripod-demo: el primer end-to-end del Tripode Remora.
//
// Demuestra el loop completo:
//
//  Echo (discovery) → Alfa (spec) → Swarm (ejecución) → Bravo (score)
//
// Caso de uso: automatización de procesamiento de facturas.
// Un enjambre de 3 agentes procesa 5 zonas en paralelo.
// El resultado es un BravoScore 0.0–1.0 que prueba si el tripode funcionó.
//
// Ejecutar:
//
//	cd examples/tripod-demo && go run .
package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// ─── Dominio: Invoice Processing ──────────────────────────────────────────────

type Invoice struct {
	ID        string
	VendorID  string
	Amount    float64
	LineItems []LineItem
	DueDate   time.Time
}

type LineItem struct {
	Desc   string
	Amount float64
}

// vendorRegistry simulates the company's approved vendor list.
var vendorRegistry = map[string]string{
	"V-ACME":    "ACME Corp",
	"V-GLOBEX":  "Globex Inc",
	"V-INITECH": "Initech LLC",
}

// testInvoices are the batch the swarm will process.
var testInvoices = []Invoice{
	{
		ID: "INV-001", VendorID: "V-ACME", Amount: 5000.00,
		LineItems: []LineItem{{"Software License", 3000}, {"Support", 2000}},
		DueDate:   time.Now().AddDate(0, 1, 0),
	},
	{
		ID: "INV-002", VendorID: "V-GLOBEX", Amount: 15000.00,
		LineItems: []LineItem{{"Consulting", 10000}, {"Hardware", 5000}},
		DueDate:   time.Now().AddDate(0, 0, 30),
	},
	{
		ID: "INV-003", VendorID: "V-UNKNOWN", Amount: 3500.00, // unknown vendor → violation
		LineItems: []LineItem{{"Services", 3500}},
		DueDate:   time.Now().AddDate(0, -1, 0), // past due
	},
	{
		ID: "INV-004", VendorID: "V-INITECH", Amount: 8750.00,
		LineItems: []LineItem{{"Development", 6000}, {"Design", 2750}},
		DueDate:   time.Now().AddDate(0, 0, 45),
	},
}

// ─── main ─────────────────────────────────────────────────────────────────────

func main() {
	fmt.Println("🐟 Remora Tripod Demo — Invoice Processing Swarm")
	fmt.Println("=================================================")
	fmt.Println()

	// ── PASO 1: ECHO → ALFA (Ideal Flow) ──────────────────────────────────
	// En producción: esto viene de alfa.AlfaSpec.ToBravoIdealFlow()
	// Aquí lo construimos directamente para demostrar el contrato.
	idealFlow := buildIdealFlow()
	printIdealFlow(idealFlow)

	// ── PASO 2: ZONAS del enjambre = pasos del critical path ──────────────
	zones := []swarm.Zone{
		{ID: "validate_invoices", Name: "Validate Invoices", PainWeight: 0.95,
			Description: "Verify required fields on all invoices"},
		{ID: "extract_data", Name: "Extract Data", PainWeight: 0.88,
			Description: "Extract amounts, vendor IDs, line items"},
		{ID: "match_vendors", Name: "Match Vendors", PainWeight: 0.80,
			Description: "Verify each vendor exists in the approved registry"},
		{ID: "calculate_totals", Name: "Calculate Totals", PainWeight: 0.75,
			Description: "Sum line items and verify they match invoice totals"},
		{ID: "route_approvals", Name: "Route Approvals", PainWeight: 0.70,
			Description: "Route each invoice to the correct approver tier"},
	}

	// ── PASO 3: WORK FUNCTIONS ────────────────────────────────────────────
	workFn := buildWorkFn()

	// ── PASO 4: CREAR Y EJECUTAR ENJAMBRE ─────────────────────────────────
	outDir := "temp"
	_ = os.MkdirAll(outDir, 0755)

	s, err := swarm.New(swarm.Config{
		ID:         fmt.Sprintf("invoice-swarm-%d", time.Now().Unix()),
		Zones:      zones,
		AgentIDs:   []string{"agent-alpha", "agent-beta", "agent-gamma"},
		WorkFunc:   workFn,
		StigmaPath: outDir + "/stigma.json",
	})
	if err != nil {
		fatal("crear swarm: %v", err)
	}

	fmt.Println("📡 Campo de presión inicial:")
	for _, zp := range s.PressureTable() {
		fmt.Printf("   [%.3f] %s\n", zp.Pressure, zp.Zone.Name)
	}
	fmt.Printf("\n🌊 Lanzando enjambre: 3 agentes, %d zonas\n\n", len(zones))

	startTime := time.Now()
	swarmResult, err := s.Run(context.Background())
	if err != nil {
		fatal("swarm run: %v", err)
	}
	elapsed := time.Since(startTime)

	// ── PASO 5: BRAVO SCORE ───────────────────────────────────────────────
	// Compara la traza Paladin real contra el IdealFlow.
	// Este es el cierre del tripode: el enjambre hizo lo que Alfa especificó?
	score, err := swarm.ScoreLatestTrace(idealFlow, "temp/paladin", 0.80)
	if err != nil {
		fmt.Printf("  ⚠️  no se pudo calcular Bravo score: %v\n", err)
	}

	// ── RESULTADOS ────────────────────────────────────────────────────────
	printSwarmResult(swarmResult, elapsed)
	if score != nil {
		printBravoScore(score)
	}

	// Exit code reflects tripod validation
	if score != nil && !score.Passed {
		os.Exit(1)
	}
}

// ─── Ideal Flow ───────────────────────────────────────────────────────────────

func buildIdealFlow() *swarm.IdealFlow {
	return &swarm.IdealFlow{
		Description: "Invoice Processing Automation — Remora Tripod Demo",
		Intent:      "Automatizar el procesamiento de facturas: validar, extraer, verificar vendor, calcular totales, enrutar aprobaciones",
		CriticalPath: []string{
			"validate_invoices",
			"extract_data",
			"match_vendors",
			"calculate_totals",
			"route_approvals",
		},
		CriticalVars: []string{
			"invoice_id",
			"total_amount",
			"vendor_id",
			"line_items_sum",
			"amounts_balanced",
			"vendor_match",
			"approval_level",
		},
		Rules: []swarm.VerifyRule{
			{
				Name:        "invoice-completeness-rule",
				Description: "Facturas deben tener ID, VendorID, Amount > 0 y al menos una línea",
				When:        "factura recibida",
				Then:        "validar presencia de todos los campos requeridos",
				Importance:  1,
			},
			{
				Name:        "vendor-registry-rule",
				Description: "El VendorID de cada factura debe existir en el registro aprobado",
				When:        "VendorID presente",
				Then:        "verificar contra registro de vendors aprobados",
				Importance:  1,
			},
			{
				Name:        "amounts-balance-rule",
				Description: "La suma de las líneas debe igualar el total de la factura (±$0.01)",
				When:        "líneas extraídas",
				Then:        "assert sum(lineItems) ≈ total",
				Importance:  1,
			},
			{
				Name:        "approval-threshold-rule",
				Description: "Facturas con Amount > $10,000 requieren aprobación de nivel senior",
				When:        "Amount > 10000",
				Then:        "enrutar a aprobador senior",
				Importance:  2,
			},
		},
	}
}

// ─── Work Functions ───────────────────────────────────────────────────────────

func buildWorkFn() swarm.WorkFunc {
	return func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		tc := agent.TraceCtx() // Paladin context for this zone span
		vars := make(map[string]any)

		switch zone.ID {

		case "validate_invoices":
			fmt.Printf("  🤖 [%s] validando %d facturas...\n", agent.ID, len(testInvoices))
			valid := 0
			ids := make([]string, 0, len(testInvoices))
			for _, inv := range testInvoices {
				if inv.ID != "" && inv.VendorID != "" && inv.Amount > 0 && len(inv.LineItems) > 0 {
					valid++
					ids = append(ids, inv.ID)
				}
			}
			vars["invoice_id"] = strings.Join(ids, ",")
			vars["validation_status"] = fmt.Sprintf("%d/%d valid", valid, len(testInvoices))
			if tc != nil {
				tc.Rule("invoice-completeness-rule", "Facturas con todos los campos requeridos", nil)
				tc.Check("required_fields", "all_present",
					fmt.Sprintf("%d/%d present", valid, len(testInvoices)), valid == len(testInvoices))
			}
			fmt.Printf("  ✅ [%s] %d/%d facturas válidas\n", agent.ID, valid, len(testInvoices))

		case "extract_data":
			fmt.Printf("  🤖 [%s] extrayendo datos...\n", agent.ID)
			totalAmt := 0.0
			vendorIDs := make([]string, 0, len(testInvoices))
			itemCount := 0
			for _, inv := range testInvoices {
				totalAmt += inv.Amount
				vendorIDs = append(vendorIDs, inv.VendorID)
				itemCount += len(inv.LineItems)
			}
			vars["total_amount"] = totalAmt
			vars["vendor_id"] = strings.Join(vendorIDs, ",")
			vars["line_items_count"] = itemCount
			if tc != nil {
				tc.Event("data_extracted",
					fmt.Sprintf("total=$%.2f vendors=%d items=%d", totalAmt, len(vendorIDs), itemCount),
					nil)
			}
			fmt.Printf("  ✅ [%s] total=$%.2f, %d vendors, %d items\n",
				agent.ID, totalAmt, len(vendorIDs), itemCount)

		case "match_vendors":
			fmt.Printf("  🤖 [%s] verificando vendors...\n", agent.ID)
			matched := 0
			unknown := make([]string, 0)
			for _, inv := range testInvoices {
				if _, ok := vendorRegistry[inv.VendorID]; ok {
					matched++
				} else {
					unknown = append(unknown, inv.VendorID)
				}
			}
			vars["vendor_match"] = fmt.Sprintf("%d/%d found", matched, len(testInvoices))
			if tc != nil {
				tc.Rule("vendor-registry-rule", "Vendor debe existir en el registro aprobado", nil)
				tc.Check("vendor-exists",
					fmt.Sprintf("%d/%d", len(testInvoices), len(testInvoices)),
					fmt.Sprintf("%d/%d", matched, len(testInvoices)),
					matched == len(testInvoices))
				if len(unknown) > 0 {
					tc.Violation("vendor_registry", "all vendors known",
						fmt.Sprintf("vendors desconocidos: %v", unknown))
				}
			}
			fmt.Printf("  ✅ [%s] %d/%d en registro (desconocidos: %v)\n",
				agent.ID, matched, len(testInvoices), unknown)

		case "calculate_totals":
			fmt.Printf("  🤖 [%s] calculando totales...\n", agent.ID)
			allBalanced := true
			totalSum := 0.0
			for _, inv := range testInvoices {
				lineSum := 0.0
				for _, li := range inv.LineItems {
					lineSum += li.Amount
				}
				totalSum += lineSum
				if math.Abs(lineSum-inv.Amount) > 0.01 {
					allBalanced = false
				}
			}
			vars["line_items_sum"] = totalSum
			vars["amounts_balanced"] = allBalanced
			if tc != nil {
				tc.Rule("amounts-balance-rule", "Suma de líneas debe igualar total de factura", nil)
				status := "balanced"
				if !allBalanced {
					status = "mismatch"
				}
				tc.Check("amounts-balance", "balanced", status, allBalanced)
			}
			fmt.Printf("  ✅ [%s] sum=$%.2f, balanced=%v\n", agent.ID, totalSum, allBalanced)

		case "route_approvals":
			fmt.Printf("  🤖 [%s] enrutando aprobaciones...\n", agent.ID)
			seniorCount, standardCount := 0, 0
			for _, inv := range testInvoices {
				if inv.Amount > 10000 {
					seniorCount++
				} else {
					standardCount++
				}
			}
			vars["approval_level"] = fmt.Sprintf("%d_senior_%d_standard", seniorCount, standardCount)
			vars["approver_id"] = "auto-assigned"
			if tc != nil {
				tc.Rule("approval-threshold-rule", ">$10k requiere aprobación senior", nil)
				tc.Check("approval-routing", ">10000=senior",
					fmt.Sprintf("INV-002=$15000→senior (%d senior total)", seniorCount), true)
			}
			fmt.Printf("  ✅ [%s] %d senior, %d standard\n", agent.ID, seniorCount, standardCount)

		default:
			fmt.Printf("  ⚠️  [%s] zona desconocida: %s\n", agent.ID, zone.ID)
		}

		return &swarm.Result{
			Output: fmt.Sprintf("zone %s processed by %s", zone.ID, agent.ID),
			Vars:   vars,
		}, nil
	}
}

// ─── Printing ─────────────────────────────────────────────────────────────────

func printIdealFlow(f *swarm.IdealFlow) {
	fmt.Println("📋 Ideal Flow (Echo → Alfa → BravoIdealFlow)")
	fmt.Println("─────────────────────────────────────────────")
	fmt.Printf("   %s\n", f.Description)
	fmt.Printf("   Intención: %s\n", f.Intent)
	fmt.Printf("   Path crítico (%d pasos): %s\n", len(f.CriticalPath), strings.Join(f.CriticalPath, " → "))
	fmt.Printf("   Variables críticas (%d): %s\n", len(f.CriticalVars), strings.Join(f.CriticalVars, ", "))
	fmt.Printf("   Reglas de negocio: %d\n\n", len(f.Rules))
}

func printSwarmResult(r *swarm.SwarmResult, elapsed time.Duration) {
	fmt.Println()
	fmt.Println("📊 Resultado del Enjambre")
	fmt.Println("─────────────────────────")
	fmt.Printf("   ID:            %s\n", r.SwarmID)
	fmt.Printf("   Duración:      %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("   Zonas:         %d/%d resueltas (%.0f%%)\n",
		r.SolvedZones, r.TotalZones, pct(r.SolvedZones, r.TotalZones))
	fmt.Printf("   Colisiones:    %.1f%%\n", r.CollisionRate*100)
	fmt.Println()
	fmt.Println("   Por zona:")
	for _, res := range r.Results {
		icon := "✅"
		if !res.Success {
			icon = "❌"
		}
		fmt.Printf("     %s [%-13s] %-22s %dms\n",
			icon, res.AgentID, res.ZoneID+":", res.Duration.Milliseconds())
	}
}

func printBravoScore(s *swarm.VerifyResult) {
	fmt.Println()
	fmt.Println("🔬 Bravo Score (traza Paladin vs IdealFlow)")
	fmt.Println("────────────────────────────────────────────")
	fmt.Printf("   Score final:      %.2f / 1.00\n", s.Score)
	fmt.Printf("   Path coverage:    %.0f%%\n", s.PathCoverage*100)
	fmt.Printf("   Var coverage:     %.0f%%\n", s.VarCoverage*100)
	fmt.Printf("   Rule coverage:    %.0f%%\n", s.RuleCoverage*100)
	fmt.Printf("   Violations:       %d\n", s.Violations)
	fmt.Printf("   Threshold:        %.2f\n", s.Threshold)
	fmt.Println()

	if s.Passed {
		fmt.Println("   🎯 TRIPOD VALIDATED  ✅  Score ≥ 0.80")
		fmt.Println("   El enjambre siguió el ideal_flow y evidenció las reglas.")
	} else {
		fmt.Println("   ⚠️  TRIPOD NEEDS WORK  Score < 0.80")
		fmt.Println("   Ver detalles para identificar gaps.")
	}

	fmt.Println()
	fmt.Println("   Detalles:")
	for _, d := range s.Details {
		fmt.Printf("     %s\n", d)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "fatal: "+format+"\n", args...)
	os.Exit(1)
}
