// doc-swarm: el primer enjambre de Remora.
//
// Tres agentes documentan en paralelo los packages núcleo de remora-go.
// Se coordinan exclusivamente mediante feromonas (sin coordinador central):
//   - El agente con mayor presión disponible toma la zona
//   - Al terminar deja feromona "solved" → la zona desaparece del campo
//   - Los demás agentes se reparten las zonas restantes
//
// Salida:
//   - output/<zone>.md   — análisis estático del package
//   - output/stigma.json — instantánea del campo de feromonas
//   - output/report.md   — informe del benchmark del enjambre
//   - temp/paladin/      — traza semántica completa (generada por Paladin)
package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// repoRoot es la ruta relativa al directorio raíz de remora-go desde este ejemplo.
const repoRoot = "../../../"

func main() {
	fmt.Println("🐟 Remora Doc-Swarm — Benchmark v0.1")
	fmt.Println("======================================")
	fmt.Println()

	// --- Definir zonas (packages de remora-go) ---
	// PainWeight refleja la urgencia de documentar cada package:
	// los más usados por otros frameworks tienen mayor peso.
	zones := []swarm.Zone{
		{
			ID:          "paladin",
			Name:        "framework-paladin",
			Description: "Sustrato semántico de trazas — el pegamento del enjambre",
			PainWeight:  0.95,
			Tags:        []string{"core", "tracing", "semantics"},
			Meta:        map[string]any{"path": "framework-paladin/paladin"},
		},
		{
			ID:          "echo",
			Name:        "framework-echo (tree)",
			Description: "Árbol de descubrimiento AXIOM→PAIN→OPPORTUNITY",
			PainWeight:  0.88,
			Tags:        []string{"core", "discovery"},
			Meta:        map[string]any{"path": "framework-echo/internal/tree"},
		},
		{
			ID:          "bravo",
			Name:        "framework-bravo",
			Description: "Verificación de flujo ideal vs. traza real",
			PainWeight:  0.80,
			Tags:        []string{"core", "verification"},
			Meta:        map[string]any{"path": "framework-bravo/bravo"},
		},
		{
			ID:          "alfa",
			Name:        "framework-alfa",
			Description: "Compilación de descubrimiento a especificación formal",
			PainWeight:  0.75,
			Tags:        []string{"compilation"},
			Meta:        map[string]any{"path": "framework-alfa/internal/alfa"},
		},
		{
			ID:          "charlie",
			Name:        "framework-charlie",
			Description: "Git-safe versioning — los agentes no pueden romper el repo",
			PainWeight:  0.70,
			Tags:        []string{"versioning", "safety"},
			Meta:        map[string]any{"path": "framework-charlie/internal/charlie"},
		},
	}

	// --- Preparar directorio de salida ---
	outDir := "output"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fatal("mkdir output: %v", err)
	}

	// --- Función de trabajo: análisis estático + markdown ---
	workFn := func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		pkgPath := ""
		if m, ok := zone.Meta["path"]; ok {
			pkgPath = m.(string)
		}
		fullPath := filepath.Join(repoRoot, pkgPath)

		fmt.Printf("  🤖 [%s] → %s\n", agent.ID, zone.Name)

		analysis, err := analyzePackage(fullPath)
		if err != nil {
			fmt.Printf("  ⚠️  [%s] → %s: %v\n", agent.ID, zone.Name, err)
			return nil, fmt.Errorf("analyze %s: %w", fullPath, err)
		}

		doc := renderMarkdown(zone, agent.ID, analysis)
		outPath := filepath.Join(outDir, zone.ID+".md")
		if err := os.WriteFile(outPath, []byte(doc), 0644); err != nil {
			return nil, fmt.Errorf("write doc: %w", err)
		}

		summary := fmt.Sprintf("%d funcs exported, %d types, %d interfaces, %d líneas",
			countExported(analysis.Functions),
			countExportedTypes(analysis.Types),
			countExportedTypes(analysis.Interfaces),
			analysis.LineCount,
		)
		fmt.Printf("  ✅ [%s] → %s (%s)\n", agent.ID, zone.Name, summary)

		return &swarm.Result{
			Output: summary,
			Artifacts: []swarm.Artifact{
				{Name: zone.ID + ".md", Path: outPath, Kind: "markdown"},
			},
		}, nil
	}

	// --- Crear el enjambre ---
	s, err := swarm.New(swarm.Config{
		ID:         fmt.Sprintf("doc-swarm-%d", time.Now().Unix()),
		Zones:      zones,
		AgentIDs:   []string{"agent-alpha", "agent-beta", "agent-gamma"},
		WorkFunc:   workFn,
		StigmaPath: filepath.Join(outDir, "stigma.json"),
	})
	if err != nil {
		fatal("crear swarm: %v", err)
	}

	// Mostrar campo de presión inicial
	fmt.Println()
	fmt.Println("📡 Campo de presión inicial:")
	for _, zp := range s.PressureTable() {
		fmt.Printf("   [%.3f] %s\n", zp.Pressure, zp.Zone.Name)
	}
	fmt.Println()
	fmt.Printf("🌊 Lanzando enjambre: 3 agentes, %d zonas\n\n", len(zones))

	startTime := time.Now()
	result, err := s.Run(context.Background())
	if err != nil {
		fatal("swarm run: %v", err)
	}
	elapsed := time.Since(startTime)

	// --- Imprimir resultados ---
	fmt.Println()
	fmt.Println("📊 Benchmark del Enjambre")
	fmt.Println("=========================")
	fmt.Printf("  ID:              %s\n", result.SwarmID)
	fmt.Printf("  Duración total:  %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  Zonas totales:   %d\n", result.TotalZones)
	fmt.Printf("  Resueltas:       %d/%d (%.0f%%)\n",
		result.SolvedZones, result.TotalZones,
		pct(result.SolvedZones, result.TotalZones))
	fmt.Printf("  Bloqueadas:      %d\n", result.BlockedZones)
	fmt.Printf("  Tasa colisión:   %.1f%% (trabajo duplicado)\n", result.CollisionRate*100)
	fmt.Println()

	// Resultados por zona
	fmt.Println("  Resultados por zona:")
	for _, r := range result.Results {
		icon := "✅"
		if !r.Success {
			icon = "❌"
		}
		fmt.Printf("    %s [%-13s] %-10s %s (%dms)\n",
			icon, r.AgentID, r.ZoneID+":", r.Output, r.Duration.Milliseconds())
	}

	// Feromonas
	pheros := s.StigmaSnapshot()
	fmt.Printf("\n  🧪 Feromonas (%d señales):\n", len(pheros))
	for _, p := range pheros {
		exp := "permanente"
		if !p.ExpiresAt.IsZero() {
			exp = fmt.Sprintf("expira %s", p.ExpiresAt.Format("15:04:05"))
		}
		fmt.Printf("    [%.2f] %-12s zona:%-10s por %-14s (%s)\n",
			p.CurrentStrength(), p.Signal, p.Zone, p.AgentID, exp)
	}

	// Escribir reporte markdown
	report := buildReport(result, s.StigmaSnapshot(), s.PressureTable(), elapsed)
	reportPath := filepath.Join(outDir, "report.md")
	if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
		fmt.Printf("  ⚠️  no se pudo escribir reporte: %v\n", err)
	}

	fmt.Println()
	fmt.Printf("  📁 Docs:      %s/\n", outDir)
	fmt.Printf("  📋 Reporte:   %s\n", reportPath)
	fmt.Printf("  🧬 Estigma:   %s/stigma.json\n", outDir)
	fmt.Printf("  🔍 Paladin:   temp/paladin/\n")
	fmt.Println()
}

// ─── Análisis estático ────────────────────────────────────────────────────────

// PackageAnalysis holds the result of parsing a Go package.
type PackageAnalysis struct {
	PackageName string
	Files       []string
	Functions   []FuncInfo
	Types       []TypeInfo
	Interfaces  []TypeInfo
	LineCount   int
}

type FuncInfo struct {
	Name     string
	Exported bool
	Doc      string
	Receiver string // non-empty if method
}

type TypeInfo struct {
	Name     string
	Exported bool
	Kind     string // "struct", "interface", "alias"
	Doc      string
}

func analyzePackage(dir string) (*PackageAnalysis, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no Go packages found in %s", dir)
	}

	analysis := &PackageAnalysis{}
	for pkgName, pkg := range pkgs {
		analysis.PackageName = pkgName
		// sort files for determinism
		fileNames := make([]string, 0, len(pkg.Files))
		for name := range pkg.Files {
			fileNames = append(fileNames, name)
		}
		sort.Strings(fileNames)

		for _, filename := range fileNames {
			file := pkg.Files[filename]
			analysis.Files = append(analysis.Files, filepath.Base(filename))
			analysis.LineCount += fset.File(file.Pos()).LineCount()

			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					doc := ""
					if d.Doc != nil {
						doc = firstLine(d.Doc.Text())
					}
					recv := ""
					if d.Recv != nil && len(d.Recv.List) > 0 {
						recv = fmt.Sprintf("%s", d.Recv.List[0].Type)
					}
					analysis.Functions = append(analysis.Functions, FuncInfo{
						Name:     d.Name.Name,
						Exported: d.Name.IsExported(),
						Doc:      doc,
						Receiver: recv,
					})

				case *ast.GenDecl:
					for _, spec := range d.Specs {
						ts, ok := spec.(*ast.TypeSpec)
						if !ok {
							continue
						}
						doc := ""
						if d.Doc != nil {
							doc = firstLine(d.Doc.Text())
						}
						if ts.Doc != nil {
							doc = firstLine(ts.Doc.Text())
						}
						switch ts.Type.(type) {
						case *ast.InterfaceType:
							analysis.Interfaces = append(analysis.Interfaces, TypeInfo{
								Name: ts.Name.Name, Exported: ts.Name.IsExported(),
								Kind: "interface", Doc: doc,
							})
						case *ast.StructType:
							analysis.Types = append(analysis.Types, TypeInfo{
								Name: ts.Name.Name, Exported: ts.Name.IsExported(),
								Kind: "struct", Doc: doc,
							})
						default:
							analysis.Types = append(analysis.Types, TypeInfo{
								Name: ts.Name.Name, Exported: ts.Name.IsExported(),
								Kind: "type", Doc: doc,
							})
						}
					}
				}
			}
		}
		break // use first package only
	}
	return analysis, nil
}

// ─── Renderizado Markdown ─────────────────────────────────────────────────────

func renderMarkdown(zone swarm.Zone, agentID string, a *PackageAnalysis) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("# %s\n\n", zone.Name)
	w("> %s\n\n", zone.Description)
	w("| Campo | Valor |\n|---|---|\n")
	w("| Package | `%s` |\n", a.PackageName)
	w("| Pain weight | %.2f |\n", zone.PainWeight)
	w("| Agente | `%s` |\n", agentID)
	w("| Archivos | %s |\n", strings.Join(a.Files, ", "))
	w("| Líneas | %d |\n", a.LineCount)
	w("| Tags | %s |\n\n", strings.Join(zone.Tags, ", "))

	// Funciones exportadas
	w("## Funciones exportadas\n\n")
	exported := 0
	for _, f := range a.Functions {
		if !f.Exported {
			continue
		}
		exported++
		if f.Receiver != "" {
			w("### `(%s) %s`\n", f.Receiver, f.Name)
		} else {
			w("### `%s`\n", f.Name)
		}
		if f.Doc != "" {
			w("%s\n", f.Doc)
		}
		w("\n")
	}
	if exported == 0 {
		w("_(ninguna)_\n\n")
	}

	// Tipos
	w("## Tipos\n\n")
	hasType := false
	for _, t := range a.Types {
		if t.Exported {
			hasType = true
			w("- **`%s`** (%s)", t.Name, t.Kind)
			if t.Doc != "" {
				w(": %s", t.Doc)
			}
			w("\n")
		}
	}
	if !hasType {
		w("_(ninguno)_\n")
	}

	// Interfaces
	if len(a.Interfaces) > 0 {
		w("\n## Interfaces\n\n")
		for _, iface := range a.Interfaces {
			if iface.Exported {
				w("- **`%s`**", iface.Name)
				if iface.Doc != "" {
					w(": %s", iface.Doc)
				}
				w("\n")
			}
		}
	}

	w("\n---\n_Generado por Remora Doc-Swarm · agente `%s` · %s_\n",
		agentID, time.Now().Format("2006-01-02 15:04:05"))
	return b.String()
}

// ─── Reporte del benchmark ────────────────────────────────────────────────────

func buildReport(result *swarm.SwarmResult, pheros []*swarm.Pheromone, pressures []swarm.ZonePressure, elapsed time.Duration) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("# Remora Doc-Swarm — Reporte de Benchmark\n\n")
	w("**Fecha:** %s  \n", time.Now().Format("2006-01-02 15:04:05"))
	w("**Swarm ID:** `%s`\n\n", result.SwarmID)

	w("## Métricas\n\n")
	w("| Métrica | Valor |\n|---|---|\n")
	w("| Duración total | %s |\n", elapsed.Round(time.Millisecond))
	w("| Agentes | %d |\n", result.TotalAgents)
	w("| Zonas totales | %d |\n", result.TotalZones)
	w("| Resueltas | %d (%.0f%%) |\n", result.SolvedZones, pct(result.SolvedZones, result.TotalZones))
	w("| Bloqueadas | %d |\n", result.BlockedZones)
	w("| Tasa de colisión | %.1f%% |\n\n", result.CollisionRate*100)

	w("## Resultados por zona\n\n")
	w("| Zona | Agente | Estado | Output | Tiempo |\n|---|---|---|---|---|\n")
	for _, r := range result.Results {
		status := "✅ solved"
		if !r.Success {
			status = "❌ blocked"
		}
		w("| %s | `%s` | %s | %s | %dms |\n",
			r.ZoneID, r.AgentID, status, r.Output, r.Duration.Milliseconds())
	}

	w("\n## Campo de presión final\n\n")
	w("| Zona | Presión | Densidad | Resuelta |\n|---|---|---|---|\n")
	for _, zp := range pressures {
		solved := "no"
		if zp.SolvedRatio > 0 {
			solved = "sí"
		}
		w("| %s | %.3f | %d | %s |\n", zp.Zone.Name, zp.Pressure, zp.AgentDensity, solved)
	}

	w("\n## Feromonas (%d señales)\n\n", len(pheros))
	w("| Señal | Zona | Agente | Fuerza | Expiración |\n|---|---|---|---|---|\n")
	for _, p := range pheros {
		exp := "permanente"
		if !p.ExpiresAt.IsZero() {
			exp = p.ExpiresAt.Format("15:04:05")
		}
		w("| %s | %s | `%s` | %.2f | %s |\n",
			p.Signal, p.Zone, p.AgentID, p.CurrentStrength(), exp)
	}

	w("\n---\n_Remora Doc-Swarm · github.com/remora-ia/remora-go_\n")
	return b.String()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func countExported(fns []FuncInfo) int {
	n := 0
	for _, f := range fns {
		if f.Exported {
			n++
		}
	}
	return n
}

func countExportedTypes(ts []TypeInfo) int {
	n := 0
	for _, t := range ts {
		if t.Exported {
			n++
		}
	}
	return n
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

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
