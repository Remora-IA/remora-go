package swarm_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// TestMockLLMCoordination prueba que el enjambre mantiene 0% colisiones
// cuando los agentes tienen latencia variable (simulando inferencia LLM real).
//
// Esta es la prueba que cierra el gap entre el benchmark determinista y
// el comportamiento real: los agentes tardan 200-800ms por zona (como un LLM),
// y la estigmergia debe seguir coordinándolos sin duplicar trabajo.
func TestMockLLMCoordination(t *testing.T) {
	zones := []swarm.Zone{
		{ID: "extract_entities",   Name: "Extraer entidades",      PainWeight: 0.95, Description: "Identificar personas, organizaciones y fechas en los documentos"},
		{ID: "classify_intent",    Name: "Clasificar intención",   PainWeight: 0.88, Description: "Determinar el propósito de cada documento: contrato, solicitud, resolución"},
		{ID: "summarize_content",  Name: "Resumir contenido",      PainWeight: 0.80, Description: "Generar un resumen de 3 oraciones por documento"},
		{ID: "detect_conflicts",   Name: "Detectar conflictos",    PainWeight: 0.72, Description: "Identificar contradicciones entre documentos del mismo caso"},
		{ID: "build_timeline",     Name: "Construir cronología",   PainWeight: 0.65, Description: "Ordenar eventos por fecha y construir línea de tiempo del caso"},
	}

	stigmaDir := t.TempDir()

	s, err := swarm.New(swarm.Config{
		ID:         "mock-llm-coordination-test",
		AgentIDs:   []string{"agent-alpha", "agent-beta", "agent-gamma"},
		Zones:      zones,
		WorkFunc:   swarm.MockLLMWorkFunc(200*time.Millisecond, 800*time.Millisecond),
		StigmaPath: filepath.Join(stigmaDir, "stigma.json"),
	})
	if err != nil {
		t.Fatalf("swarm.New: %v", err)
	}

	start := time.Now()
	result, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("swarm.Run: %v", err)
	}
	elapsed := time.Since(start)

	t.Logf("Swarm completado en %v (3 agentes, 5 zonas, latencia 200-800ms/zona)", elapsed)
	t.Logf("Zonas resueltas: %d/%d", result.SolvedZones, result.TotalZones)
	t.Logf("Tasa de colisión: %.1f%%", result.CollisionRate*100)

	if result.SolvedZones != result.TotalZones {
		t.Errorf("solo %d/%d zonas resueltas", result.SolvedZones, result.TotalZones)
	}

	// La estigmergia debe mantener 0% colisiones incluso con latencia variable
	if result.CollisionRate > 0 {
		t.Errorf("colisiones detectadas: %.1f%% — la estigmergia falló bajo latencia LLM", result.CollisionRate*100)
	}

	// Con 3 agentes y latencia max 800ms, esperamos <3s total (paralelismo real)
	if elapsed > 5*time.Second {
		t.Errorf("tardó %v — más de lo esperado para 3 agentes paralelos", elapsed)
	}
}

// TestLLMWorkFuncAdapter prueba que LLMWorkFunc conecta correctamente
// cualquier función texto→texto como agente del enjambre.
func TestLLMWorkFuncAdapter(t *testing.T) {
	// LLM simulado: cuenta palabras en el prompt y devuelve estadísticas
	callCount := atomic.Int64{}
	mockLLM := swarm.LLMFunc(func(ctx context.Context, prompt string) (string, error) {
		callCount.Add(1)
		words := len(prompt) / 5 // estimación burda de palabras
		return fmt.Sprintf(
			"zona procesada\npalabras_analizadas: %d\nestado: completado\nconfianza: 0.92",
			words,
		), nil
	})

	zones := []swarm.Zone{
		{ID: "zona_uno", Name: "Zona Uno", PainWeight: 0.90, Description: "Primera zona de prueba"},
		{ID: "zona_dos", Name: "Zona Dos", PainWeight: 0.75, Description: "Segunda zona de prueba"},
	}

	stigmaDir := t.TempDir()

	s, err := swarm.New(swarm.Config{
		ID:       "llm-adapter-test",
		AgentIDs: []string{"agent-a", "agent-b"},
		Zones:    zones,
		WorkFunc: swarm.LLMWorkFunc(mockLLM, swarm.LLMWorkFuncConfig{
			SystemPrompt: "Eres un agente de análisis de documentos legales.",
		}),
		StigmaPath: filepath.Join(stigmaDir, "stigma.json"),
	})
	if err != nil {
		t.Fatalf("swarm.New: %v", err)
	}

	result, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("swarm.Run: %v", err)
	}

	if result.SolvedZones != 2 {
		t.Errorf("esperaba 2 zonas resueltas, got %d", result.SolvedZones)
	}
	if result.CollisionRate > 0 {
		t.Errorf("colisión detectada con LLMWorkFunc")
	}

	// El LLM debió ser llamado exactamente una vez por zona
	if calls := callCount.Load(); calls != 2 {
		t.Errorf("LLMFunc llamada %d veces, esperaba 2 (una por zona)", calls)
	}

	t.Logf("LLMFunc llamada %d veces, 0%% colisiones, %d/%d zonas",
		callCount.Load(), result.SolvedZones, result.TotalZones)
}

// TestMockLLMBravoScore verifica que el BravoScore sigue funcionando
// cuando el trabajo lo hace un LLM (simulado) en lugar de lógica determinista.
func TestMockLLMBravoScore(t *testing.T) {
	zones := []swarm.Zone{
		{ID: "read_documents",  Name: "Leer documentos",   PainWeight: 0.95},
		{ID: "extract_facts",   Name: "Extraer hechos",    PainWeight: 0.88},
		{ID: "build_arguments", Name: "Construir argumentos", PainWeight: 0.80},
	}

	flow := &swarm.IdealFlow{
		Description:  "Análisis de jurisprudencia",
		Intent:       "Leer documentos, extraer hechos, construir argumentos",
		CriticalPath: []string{"read_documents", "extract_facts", "build_arguments"},
		CriticalVars: []string{
			"read_documents_processed",
			"extract_facts_processed",
			"build_arguments_processed",
		},
		Rules: []swarm.VerifyRule{
			{Name: "read_documents-rule",   Then: "document read",     Importance: 1},
			{Name: "extract_facts-rule",    Then: "facts extracted",   Importance: 1},
			{Name: "build_arguments-rule",  Then: "arguments built",   Importance: 1},
		},
	}

	traceDir := filepath.Join(t.TempDir(), "paladin")
	if err := os.MkdirAll(traceDir, 0755); err != nil {
		t.Fatalf("mkdir trace dir: %v", err)
	}

	stigmaDir := t.TempDir()

	// Cambia el directorio de trabajo para que Paladin escriba en traceDir
	origDir, _ := os.Getwd()
	os.Chdir(filepath.Dir(traceDir))
	defer os.Chdir(origDir)

	s, err := swarm.New(swarm.Config{
		ID:         "mock-llm-bravo-test",
		AgentIDs:   []string{"agent-alpha", "agent-beta"},
		Zones:      zones,
		WorkFunc:   swarm.MockLLMWorkFunc(50*time.Millisecond, 150*time.Millisecond),
		StigmaPath: filepath.Join(stigmaDir, "stigma.json"),
	})
	if err != nil {
		t.Fatalf("swarm.New: %v", err)
	}

	if _, err := s.Run(context.Background()); err != nil {
		t.Fatalf("swarm.Run: %v", err)
	}

	score, err := swarm.ScoreLatestTrace(flow, filepath.Join(filepath.Dir(traceDir), "temp", "paladin"), 0.70)
	if err != nil {
		t.Fatalf("ScoreLatestTrace: %v", err)
	}

	t.Logf("BravoScore con MockLLM: %.2f (path=%.0f%% var=%.0f%% rules=%.0f%%)",
		score.Score, score.PathCoverage*100, score.VarCoverage*100, score.RuleCoverage*100)

	if !score.Passed {
		t.Errorf("BravoScore %.2f < threshold — el scorer no funciona con MockLLM", score.Score)
	}
}
