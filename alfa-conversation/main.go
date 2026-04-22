package main

import (
	"fmt"
	"os"

	"alfa-conversation/flowguard"
	"alfa-conversation/minimax"
)

// ============================================================================
// CONSTANTES
// ============================================================================

const (
	APIKey = "REDACTED_MINIMAX_API_KEY"
)

// ============================================================================
// RESULTADO DEL PROTOCOLO
// ============================================================================

type ResultadoProtocolo struct {
	ganadora    string // La simulación que ganó
	debates     int    // Cantidad de debates ejecutados
	alfa1Sim    string // Simulación inicial de Alfa 1
	alfa2Contra string // Contra-simulación de Alfa 2
	bravoPlan   string // Plan ejecutado por Bravo
}

// ============================================================================
// MAIN - Entry Point del Protocolo Dual Gamma
// ============================================================================

func main() {
	trace := flowguard.NewTrace("ProtocoloDualGamma")
	defer trace.Flush()

	ctx := trace.Start()
	defer ctx.End()

	// === REGISTRO INICIAL ===
	ctx.Var("API_KEY_PROVIDED", len(APIKey) > 0)
	ctx.Var("PROJECT_NAME", "Protocolo Dual Gamma v1.0")
	ctx.Var("MAX_DEBATES", 3)

	// === CARGAR PROMPTS ===
	fmt.Printf("\n📁 Cargando prompts...\n")

	alfa1Prompt := loadFile(ctx, "prompts/alfa1.md")
	alfa2Prompt := loadFile(ctx, "prompts/alfa2.md")
	bravoPrompt := loadFile(ctx, "prompts/bravo.md")

	if alfa1Prompt == "" || alfa2Prompt == "" || bravoPrompt == "" {
		ctx.ErrorMsg("Faltan archivos de prompt necesarios")
		fmt.Printf("❌ Error: Faltan archivos de prompt (alfa1.md, alfa2.md, bravo.md)\n")
		return
	}

	// === INICIALIZAR CLIENTE ===
	client := minimax.NewClient(APIKey)
	ctx.Var("client_initialized", true)

	// === PROBLEMA A RESOLVER ===
	problema := "Explica qué es un cuello de botella en programación y cómo detectarlo."

	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  PROTOCOLO DUAL GAMMA - Inicio\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Problema: %s\n", problema)
	fmt.Printf("  Debates: 3\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// === EJECUTAR PROTOCOLO ===
	result := ejecutarProtocoloDualGamma(ctx, client, alfa1Prompt, alfa2Prompt, bravoPrompt, problema)

	// === RESUMEN FINAL ===
	ctx.Var("protocol_completed", true)
	ctx.Var("ganadora", result.ganadora)
	ctx.Var("total_debates", result.debates)

	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  PROTOCOLO DUAL GAMMA - Completado\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Simulación Ganadora: %s\n", result.ganadora)
	fmt.Printf("  Debates Realizados: %d\n", result.debates)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Mostrar resultado de Bravo
	fmt.Printf("📋 Plan de Bravo:\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("%s\n", result.bravoPlan)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━\n\n")

	fmt.Printf("✅ Protocolo Dual Gamma completado!\n")
}

// ============================================================================
// FLUJO PRINCIPAL DEL PROTOCOLO DUAL GAMMA
// ============================================================================

/*
Flujo:
  LEAD → Echo: "Problema"
           ↓
  Echo → Alfa 1: "Simula Candidata"
           ↓ (Crea sesión Alfa 1)
  Alfa 1 → Simula comportamiento de Bravo
           ↓
  Alfa 1 → Echo: "Brief" (conclusiones: axiomas, cuello de botella)
           ↓
  Echo → Alfa 2: "Contra-simula el brief de Alfa 1"
           ↓ (Crea sesión Alfa 2)
  Alfa 2 → Contra-simula axioma y cuello de botella
           ↓
  Alfa 2 → Echo: "Contra-simulación"
           ↓
  Echo → Alfa 1: "Contra-simulación para revisar"
           ↓
  Alfa 1 → Responde a contra-simulación
           ↓
  (Debate: repetir Alfa 1 ↔ Alfa 2)

  CONTADOR: Echo cuenta simulaciones/contra-simulaciones
           ↓ (3 debates = 3 simulaciones + 3 contra-simulaciones)
  Echo → Consenso: "Simulación ganadora"
           ↓
  Ganadora → Alfa que la creó → Alfa entrega a Bravo
           ↓
  Bravo → Ejecuta plan
           ↓
  Echo → LEAD: "Resultado"
*/

func ejecutarProtocoloDualGamma(parent *flowguard.Context, client *minimax.Client, alfa1Prompt, alfa2Prompt, bravoPrompt, problema string) ResultadoProtocolo {
	ctx := parent.Child("ejecutarProtocoloDualGamma")
	defer ctx.End()

	ctx.Var("problema", problema)
	ctx.Var("problema_length", len(problema))

	result := ResultadoProtocolo{}

	// ═══════════════════════════════════════════════════════════════
	// FASE 1: Echo → Alfa 1: "Simula Candidata"
	// ═══════════════════════════════════════════════════════════════
	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  FASE 1: Echo → Alfa 1: Simula Candidata\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	ctx.Decision("fase_1_inicio", "Echo solicita simulación a Alfa 1")

	alfa1Sim, err := callMiniMaxChat(ctx, client, alfa1Prompt,
		"Simula el comportamiento de un candidato ideal para resolver el siguiente problema:\n\n"+
			problema+"\n\n"+
			"Tu respuesta debe incluir:\n"+
			"- AXIOMAS: Principios fundamentales que guían la solución\n"+
			"- SOLUCIÓN: Pasos concretos para resolver el problema\n"+
			"- PUNTOS CLAVE: Aspectos críticos a tener en cuenta",
		"ALFA1_SIMULACION")

	if err != nil {
		ctx.Error(err)
		ctx.Decision("fase_1_error", fmt.Sprintf("Error en simulación Alfa 1: %v", err))
		fmt.Printf("❌ Error Alfa 1: %v\n", err)
		return ResultadoProtocolo{}
	}

	result.alfa1Sim = alfa1Sim
	ctx.Var("alfa1_sim_length", len(alfa1Sim))
	ctx.Decision("fase_1_completa", fmt.Sprintf("Simulación Alfa 1 completada: %d caracteres", len(alfa1Sim)))

	fmt.Printf("\n🔵 ALFA 1 - Simulación:\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("%s\n", alfa1Sim)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━\n\n")

	// ═══════════════════════════════════════════════════════════════
	// FASE 2: Echo → Alfa 2: "Contra-simula"
	// ═══════════════════════════════════════════════════════════════
	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  FASE 2: Echo → Alfa 2: Contra-simula\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	ctx.Decision("fase_2_inicio", "Echo solicita contra-simulación a Alfa 2")

	contraMsg := fmt.Sprintf("Analiza críticamente la siguiente simulación de Alfa 1:\n\n%s\n\n"+
		"Tu contra-simulación debe:\n"+
		"- Identificar errores y inconsistencias\n"+
		"- Cuestionar axiomas débiles\n"+
		"- Proponer mejoras o correcciones\n"+
		"- Evaluar la viabilidad de la solución", alfa1Sim)

	alfa2Contra, err := callMiniMaxChat(ctx, client, alfa2Prompt, contraMsg, "ALFA2_CONTRASIM")

	if err != nil {
		ctx.Error(err)
		ctx.Decision("fase_2_error", fmt.Sprintf("Error en contra-simulación Alfa 2: %v", err))
		fmt.Printf("❌ Error Alfa 2: %v\n", err)
		return ResultadoProtocolo{}
	}

	result.alfa2Contra = alfa2Contra
	ctx.Var("alfa2_contra_length", len(alfa2Contra))
	ctx.Decision("fase_2_completa", fmt.Sprintf("Contra-simulación Alfa 2 completada: %d caracteres", len(alfa2Contra)))

	fmt.Printf("\n🟢 ALFA 2 - Contra-simulación:\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("%s\n", alfa2Contra)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━\n\n")

	// ═══════════════════════════════════════════════════════════════
	// FASE 3-5: DEBATES (3 rondas Alfa 1 ↔ Alfa 2)
	// ═══════════════════════════════════════════════════════════════
	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  FASE 3-5: DEBATES (3 rondas)\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	ctx.Decision("debates_inicio", "Iniciando 3 rondas de debate Alfa 1 ↔ Alfa 2")

	maxDebates := 3
	currentContra := alfa2Contra

	for debate := 1; debate <= maxDebates; debate++ {
		ctx.Var("debate_numero", debate)
		fmt.Printf("\n--- Debate %d/%d ---\n", debate, maxDebates)

		// Alfa 1 responde/defiende contra la contra-simulación
		alfa1Debate, err := callMiniMaxChat(ctx, client, alfa1Prompt,
			fmt.Sprintf("Alfa 2 ha contra-simulado tu trabajo:\n\n%s\n\n"+
				"Debate %d/%d:\n"+
				"- Defiende o refina tu posición\n"+
				"- Address las críticas válidas\n"+
				"- Actualiza tu solución si es necesario", currentContra, debate, maxDebates),
			fmt.Sprintf("ALFA1_DEBATE_%d", debate))

		if err != nil {
			ctx.Error(err)
			ctx.Decision(fmt.Sprintf("debate_%d_alfa1_error", debate), fmt.Sprintf("Error en debate %d Alfa 1", debate))
			break
		}

		ctx.Var(fmt.Sprintf("alfa1_debate_%d_length", debate), len(alfa1Debate))
		fmt.Printf("\n🔵 Alfa 1 (Debate %d): %d chars\n", debate, len(alfa1Debate))

		// Alfa 2 contra-simula de nuevo
		alfa2Debate, err := callMiniMaxChat(ctx, client, alfa2Prompt,
			fmt.Sprintf("Respuesta de Alfa 1 en debate %d:\n\n%s\n\n"+
				"Contra-simula de nuevo. ¿La solución es sólida? ¿Qué más falta?", debate, alfa1Debate),
			fmt.Sprintf("ALFA2_DEBATE_%d", debate))

		if err != nil {
			ctx.Error(err)
			ctx.Decision(fmt.Sprintf("debate_%d_alfa2_error", debate), fmt.Sprintf("Error en debate %d Alfa 2", debate))
			break
		}

		ctx.Var(fmt.Sprintf("alfa2_debate_%d_length", debate), len(alfa2Debate))
		fmt.Printf("🟢 Alfa 2 (Debate %d): %d chars\n", debate, len(alfa2Debate))

		// Preparar para siguiente ronda
		currentContra = alfa2Debate
		result.debates = debate
	}

	ctx.Var("debates_completados", result.debates)
	ctx.Decision("debates_completos", fmt.Sprintf("Debates completados: %d", result.debates))

	// ═══════════════════════════════════════════════════════════════
	// FASE 6: Echo → Consenso: "Simulación Ganadora"
	// ═══════════════════════════════════════════════════════════════
	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  FASE 6: Echo → Consenso\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	ctx.Decision("consenso_inicio", "Determinando simulación ganadora")

	// Usar Alfa 1 como juez (neutral) para determinar ganador
	consenso, err := callMiniMaxChat(ctx, client, alfa1Prompt,
		fmt.Sprintf("Eres ECHO. Analiza TODOS los resultados del protocolo:\n\n"+
			"=== SIMULACIÓN INICIAL DE ALFA 1 ===\n%s\n\n"+
			"=== CONTRA-SIMULACIÓN DE ALFA 2 ===\n%s\n\n"+
			"=== DEBATES ===\n%s\n\n"+
			"Tu tarea:\n"+
			"1. Determina cuál simulación es más sólida yWhy\n"+
			"2. Declare WINNER: ALFA1 o ALFA2\n"+
			"3. Explica brevemente por qué\n\n"+
			"Responde en formato:\nWINNER: [ALFA1/ALFA2]\nRAZÓN: [explicación]", alfa1Sim, alfa2Contra, currentContra),
		"ECHO_CONSENSO")

	if err != nil {
		ctx.Error(err)
		ctx.Decision("consenso_error", fmt.Sprintf("Error en consenso: %v", err))
	}

	ctx.Var("consenso_result", consenso)

	// Extraer winner (lógica simple)
	if len(consenso) > 0 {
		if contains(consenso, "ALFA1") && !contains(consenso, "ALFA2") {
			result.ganadora = "ALFA1"
		} else if contains(consenso, "ALFA2") {
			result.ganadora = "ALFA2"
		} else {
			result.ganadora = "ALFA1" // default
		}
	} else {
		result.ganadora = "ALFA1"
	}

	ctx.Var("ganadora", result.ganadora)
	ctx.Decision("consenso_completo", fmt.Sprintf("Ganadora determinada: %s", result.ganadora))

	fmt.Printf("\n📊 CONSENSO:\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("%s\n", consenso)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("🏆 GANADORA: %s\n\n", result.ganadora)

	// ═══════════════════════════════════════════════════════════════
	// FASE 7: Bravo → Ejecuta plan
	// ═══════════════════════════════════════════════════════════════
	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  FASE 7: Bravo → Ejecuta Plan\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	ctx.Decision("bravo_inicio", "Bravo ejecuta el plan ganador")

	bravoMsg := fmt.Sprintf("Eres BRAVO. Ejecutor de planes.\n\n"+
		"La simulación GANADORA es: %s\n\n"+
		"Contenido de la simulación:\n%s\n\n"+
		"Tu tarea:\n"+
		"1. Ejecuta los pasos del plan\n"+
		"2. Describe las acciones concretas\n"+
		"3. Muestra el resultado esperado\n\n"+
		"EXECUTION:", result.ganadora, alfa1Sim)

	bravoPlan, err := callMiniMaxChat(ctx, client, bravoPrompt, bravoMsg, "BRAVO_EJECUCION")

	if err != nil {
		ctx.Error(err)
		ctx.Decision("bravo_error", fmt.Sprintf("Error en ejecución de Bravo: %v", err))
	}

	result.bravoPlan = bravoPlan
	ctx.Var("bravo_plan_length", len(bravoPlan))
	ctx.Decision("bravo_completo", fmt.Sprintf("Bravo ejecutó plan: %d caracteres", len(bravoPlan)))

	fmt.Printf("\n📋 Plan de Bravo:\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("%s\n", bravoPlan)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━\n\n")

	return result
}

// ============================================================================
// UTILIDADES
// ============================================================================

// loadFile carga un archivo de prompt con instrumentación FlowGuard
func loadFile(parent *flowguard.Context, path string) string {
	ctx := parent.Child("loadFile")
	defer ctx.End()

	ctx.Var("filepath", path)
	ctx.Var("filepath_exists", fileExists(path))

	content, err := os.ReadFile(path)
	if err != nil {
		ctx.Error(err)
		ctx.Var("load_success", false)
		ctx.Decision("file_load_failed", fmt.Sprintf("No se pudo leer %s: %v", path, err))
		return ""
	}

	ctx.Var("content_length", len(content))
	ctx.Var("load_success", true)
	ctx.Decision("file_loaded", fmt.Sprintf("Cargados %d bytes desde %s", len(content), path))

	return string(content)
}

// callMiniMaxChat envuelve la llamada al cliente MiniMax con instrumentación completa
func callMiniMaxChat(parent *flowguard.Context, client *minimax.Client, systemPrompt, userMessage, callName string) (string, error) {
	ctx := parent.Child("callMiniMaxChat")
	defer ctx.End()

	// Registrar inputs COMPLETOS
	ctx.Var("call_name", callName)
	ctx.Var("system_prompt_length", len(systemPrompt))
	ctx.Var("system_prompt", systemPrompt) // COMPLETO
	ctx.Var("user_message_length", len(userMessage))
	ctx.Var("user_message", userMessage) // COMPLETO

	// Ejecutar llamada
	response, err := client.ChatWithFullResponse(ctx, systemPrompt, userMessage)
	if err != nil {
		ctx.Error(err)
		ctx.Var("error", err.Error())
		ctx.Decision("call_failed", fmt.Sprintf("%s falló: %v", callName, err))
		return "", err
	}

	// Registrar outputs COMPLETOS
	ctx.Var("response_length", len(response))
	ctx.Var("full_response", response) // COMPLETO
	ctx.Var("attempts_used", 1)
	ctx.Decision("call_success", fmt.Sprintf("%s completada exitosamente", callName))

	return response, nil
}

// fileExists verifica si un archivo existe
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// contains verifica si un string contiene otro
func contains(s, substr string) bool {
	if len(s) == 0 || len(substr) == 0 {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}