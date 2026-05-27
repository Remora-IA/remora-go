package swarm

// llm.go — adaptadores para conectar LLMs como WorkFuncs.
//
// El trípode no sabe qué modelo está dentro de cada WorkFunc.
// Estos adaptadores permiten que cualquier función que recibe texto
// y devuelve texto funcione como agente cognitivo real.
//
// Uso con Claude (requiere ANTHROPIC_API_KEY):
//
//	client := anthropic.NewClient()
//	workFn := ClaudeWorkFunc(client, "Eres un agente de análisis financiero...")
//	s, _ := swarm.New(swarm.Config{WorkFunc: workFn, ...})
//
// Uso para pruebas sin API key:
//
//	workFn := MockLLMWorkFunc(500*time.Millisecond, 2*time.Second)
//	// simula latencia real de inferencia, coordina con el mismo 0% de colisiones

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// LLMFunc es la firma de cualquier función que puede actuar como agente cognitivo.
// Recibe el prompt construido desde la zona y devuelve la respuesta en texto plano.
// Conectar Claude, GPT, Groq, o cualquier LLM es implementar esta función.
type LLMFunc func(ctx context.Context, prompt string) (string, error)

// LLMWorkFuncConfig configura cómo se convierte la respuesta del LLM en Result.
type LLMWorkFuncConfig struct {
	// SystemPrompt es el prompt de sistema del agente.
	// Describe el rol del agente y el formato de salida esperado.
	SystemPrompt string

	// VarExtractor extrae variables críticas del texto devuelto por el LLM.
	// Si es nil, se usa el extractor por defecto (busca "KEY: value" en el texto).
	VarExtractor func(zone Zone, llmOutput string) map[string]any

	// ViolationDetector detecta si el LLM encontró un problema en la zona.
	// Devuelve ("", "") si no hay violación.
	// Devuelve (rule, detail) si hay una violación que Bravo debe penalizar.
	ViolationDetector func(zone Zone, llmOutput string) (rule string, detail string)
}

// LLMWorkFunc adapta cualquier LLMFunc como WorkFunc del enjambre.
// El prompt que recibe el LLM se construye desde zone.Name y zone.Description.
// La respuesta se convierte en vars, reglas y posibles violaciones vía cfg.
func LLMWorkFunc(llm LLMFunc, cfg LLMWorkFuncConfig) WorkFunc {
	return func(ctx context.Context, zone Zone, agent *Agent) (*Result, error) {
		tc := agent.TraceCtx()

		// Construir prompt desde la zona
		prompt := buildZonePrompt(zone, cfg.SystemPrompt)

		// Llamar al LLM
		output, err := llm(ctx, prompt)
		if err != nil {
			return nil, fmt.Errorf("llm call for zone %s: %w", zone.ID, err)
		}

		// Extraer variables
		vars := map[string]any{}
		if cfg.VarExtractor != nil {
			vars = cfg.VarExtractor(zone, output)
		} else {
			vars = defaultVarExtractor(zone, output)
		}

		// Registrar en Paladin
		if tc != nil {
			tc.Event("llm.response", fmt.Sprintf("zone=%s len=%d", zone.ID, len(output)), nil)
			tc.Rule(zone.ID+"-rule", "LLM processed zone: "+zone.Name, nil)
			tc.Check(zone.ID+"-complete", "output present", fmt.Sprintf("%d chars", len(output)), len(output) > 0)
		}

		// Detectar violaciones
		if cfg.ViolationDetector != nil {
			if rule, detail := cfg.ViolationDetector(zone, output); rule != "" {
				if tc != nil {
					tc.Violation(rule, "no issues", detail)
				}
			}
		}

		return &Result{
			Output: output,
			Vars:   vars,
		}, nil
	}
}

// MockLLMWorkFunc simula un LLM con latencia realista.
// Útil para probar que la coordinación por estigmergia aguanta bajo
// los mismos tiempos que tendría un LLM real (0.5-3s por zona).
//
// No requiere API key. No genera output inteligente.
// Lo que prueba: 0% colisiones bajo latencia variable de inferencia.
func MockLLMWorkFunc(minLatency, maxLatency time.Duration) WorkFunc {
	return func(ctx context.Context, zone Zone, agent *Agent) (*Result, error) {
		tc := agent.TraceCtx()

		// Simular latencia de inferencia (variable, como un LLM real)
		jitter := time.Duration(rand.Int63n(int64(maxLatency - minLatency)))
		latency := minLatency + jitter

		select {
		case <-time.After(latency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		// Output simulado — estructura que un LLM real devolvería
		output := fmt.Sprintf(
			"Zona: %s\nDescripción: %s\nResultado: procesado correctamente\nLatencia simulada: %dms",
			zone.Name, zone.Description, latency.Milliseconds(),
		)

		vars := map[string]any{
			zone.ID + "_processed": true,
			zone.ID + "_latency_ms": latency.Milliseconds(),
			zone.ID + "_agent":     agent.ID,
		}

		if tc != nil {
			tc.Rule(zone.ID+"-rule", "mock LLM processed zone: "+zone.Name, nil)
			tc.Check(zone.ID+"-complete", "processed", "processed", true)
			tc.Event("mock.llm.response",
				fmt.Sprintf("zone=%s latency=%dms agent=%s", zone.ID, latency.Milliseconds(), agent.ID),
				nil)
		}

		return &Result{
			Output: output,
			Vars:   vars,
		}, nil
	}
}

// buildZonePrompt construye el prompt que recibe el LLM para trabajar una zona.
// El prompt incluye el rol del sistema, la zona asignada y su descripción.
func buildZonePrompt(zone Zone, systemPrompt string) string {
	var sb strings.Builder
	if systemPrompt != "" {
		sb.WriteString(systemPrompt)
		sb.WriteString("\n\n")
	}
	sb.WriteString("## Zona asignada\n")
	sb.WriteString(fmt.Sprintf("ID: %s\n", zone.ID))
	sb.WriteString(fmt.Sprintf("Nombre: %s\n", zone.Name))
	if zone.Description != "" {
		sb.WriteString(fmt.Sprintf("Descripción: %s\n", zone.Description))
	}
	sb.WriteString(fmt.Sprintf("Urgencia (pain weight): %.2f\n", zone.PainWeight))
	sb.WriteString("\n## Tu tarea\n")
	sb.WriteString("Procesa esta zona. Devuelve:\n")
	sb.WriteString("- Un resumen de lo que hiciste\n")
	sb.WriteString("- Las variables clave que extrajiste (formato KEY: valor)\n")
	sb.WriteString("- Cualquier problema o violación detectada\n")
	return sb.String()
}

// defaultVarExtractor parsea el output del LLM buscando líneas "KEY: value".
// Es el extractor básico cuando no se provee uno personalizado.
func defaultVarExtractor(zone Zone, output string) map[string]any {
	vars := map[string]any{
		zone.ID + "_output_len": len(output),
	}
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(strings.ToLower(parts[0]))
			key = strings.ReplaceAll(key, " ", "_")
			val := strings.TrimSpace(parts[1])
			if key != "" && val != "" {
				vars[key] = val
			}
		}
	}
	return vars
}
