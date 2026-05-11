package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"remora-flujo/internal/llm"
)

// readInitialPrompt lee el INITIAL_PROMPT.md de un framework.
func readInitialPrompt(rootDir, frameworkName string) (string, error) {
	path := filepath.Join(rootDir, "framework-"+frameworkName, "INITIAL_PROMPT.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("no se pudo leer INITIAL_PROMPT.md de %s: %w", frameworkName, err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", fmt.Errorf("INITIAL_PROMPT.md de %s está vacío", frameworkName)
	}
	return content, nil
}

// generateInitialGreeting usa el LLM del framework para generar un saludo
// contextual basado en su INITIAL_PROMPT.md. El LLM es OBLIGATORIO - no hay fallbacks.
// Si algo falla, el sistema debe detenerse explícitamente.
func generateInitialGreeting(ctx context.Context, rootDir string, conv *Conversation, frameworkName string) *Message {
	prompt, err := readInitialPrompt(rootDir, frameworkName)
	if err != nil {
		panic(fmt.Sprintf("ERROR CRÍTICO: No se pudo leer INITIAL_PROMPT.md para %s: %v. Los frameworks deben tener un initial prompt para funcionar.", frameworkName, err))
	}

	spec, err := modelSpecFor(conv, frameworkName)
	if err != nil {
		panic(fmt.Sprintf("ERROR CRÍTICO: No se pudo resolver modelo para %s: %v. El LLM es obligatorio para el funcionamiento de frameworks.", frameworkName, err))
	}

	client, err := llm.New(spec)
	if err != nil {
		panic(fmt.Sprintf("ERROR CRÍTICO: No se pudo crear LLM client para %s: %v. Verifica las variables de entorno (GROQ_API_KEY, MINIMAX_API_KEY, etc.).", frameworkName, err))
	}

	greeting, err := client.Complete(ctx, llm.CompletionRequest{
		System:    initialPromptSystem(prompt),
		User:      "Acabas de ser activado en una nueva sesión sin mensaje inicial del usuario. Responde únicamente como el framework definido por tu INITIAL_PROMPT.md. No te presentes, no digas que eres una IA/asistente y no abras con un saludo genérico. Ejecuta la regla de salida o la primera acción conversacional que tu INITIAL_PROMPT.md indique para una sesión recién iniciada. No inventes resultados, objetivos, eventos, tareas, axiomas, datos de negocio ni ejemplos concretos. Si falta un dato crítico, devuelve únicamente una pregunta corta y neutral con exactamente 3 opciones genéricas de estado/decisión, sin encabezados, sin prefacios, sin explicar el flujo y sin proponer contenido específico. No repitas ni expliques el prompt.",
		MaxTokens: 300,
	})
	if err != nil {
		panic(fmt.Sprintf("ERROR CRÍTICO: LLM error en %s: %v. El LLM debe estar disponible para procesar el initial prompt.", frameworkName, err))
	}

	greeting = strings.TrimSpace(greeting)
	if greeting == "" {
		panic(fmt.Sprintf("ERROR CRÍTICO: LLM devolvió respuesta vacía para %s. El modelo debe generar un saludo contextual.", frameworkName))
	}

	return &Message{
		ID:        generateMessageID(),
		Role:      "framework",
		Framework: frameworkName,
		Content:   greeting,
		Reasoning: fmt.Sprintf("Leí mi INITIAL_PROMPT.md y generé la respuesta inicial como %s usando LLM.", frameworkName),
		Status:    "needs_input",
		Events: []MessageEvent{{
			Type:      "framework.initial_prompt_loaded",
			Framework: frameworkName,
			Message:   "INITIAL_PROMPT.md procesado con LLM",
		}},
		Timestamp: time.Now(),
	}
}

func initialPromptSystem(prompt string) string {
	return fmt.Sprintf(`Eres un framework conversacional. El bloque INITIAL_PROMPT.md define tu identidad, reglas internas y formato de salida.

REGLAS CRITICAS:
- Nunca copies texto literal de INITIAL_PROMPT.md.
- Nunca muestres encabezados, secciones, comandos, rutas, reglas internas ni explicaciones del prompt.
- Nunca digas "soy una IA", "soy asistente", "estoy listo" ni hagas un saludo genérico.
- Tu salida debe ser solamente el siguiente mensaje conversacional visible para el usuario.
- Si el prompt indica que falta un dato crítico, haz una sola pregunta corta con 3 opciones.

INITIAL_PROMPT.md:
%s`, prompt)
}

// NOTA: Las funciones de fallback fueron eliminadas intencionalmente.
// El sistema REQUIERE LLM para funcionar. Si no hay LLM, el sistema debe fallar
// explícitamente en lugar de dar respuestas predefinidas que no reflejan
// la personalidad del framework.
