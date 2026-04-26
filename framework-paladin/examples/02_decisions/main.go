package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/remora-go/framework-paladin/paladin"
)

// Ejemplo 02: Decisiones Lógicas y Ruteo
// Este ejemplo demuestra cómo Paladin permite a una IA entender
// exactamente qué lógica de negocio se aplicó y por qué.

func main() {
	trace := paladin.NewTrace("routing-engine")
	ctx := trace.Start()
	defer trace.Flush()

	// Simular diferentes tipos de requests
	testCases := []Request{
		{ID: "REQ-001", Type: "user_action", Priority: "high", Source: "mobile"},
		{ID: "REQ-002", Type: "batch_job", Priority: "low", Source: "scheduler"},
		{ID: "REQ-003", Type: "user_action", Priority: "low", Source: "web"},
		{ID: "REQ-004", Type: "system_event", Priority: "critical", Source: "internal"},
	}

	for _, req := range testCases {
		handleRequest(ctx, req)
	}
}

// Request representa un request entrante
type Request struct {
	ID       string
	Type     string // "user_action", "batch_job", "system_event"
	Priority string // "low", "normal", "high", "critical"
	Source   string // "mobile", "web", "internal", "scheduler"
	Payload  map[string]any
}

// handleRequest decide cómo procesar cada request
func handleRequest(ctx *paladin.Context, req Request) {
	child := ctx.Child("handleRequest")
	defer child.End()

	child.Var("request.id", req.ID)
	child.Var("request.type", req.Type)
	child.Var("request.priority", req.Priority)
	child.Var("request.source", req.Source)

	// Determinar cola de procesamiento
	queue := determineQueue(child, req)
	child.Var("routing.target_queue", queue)

	// Determinar timeout
	timeout := determineTimeout(child, req)
	child.Var("routing.timeout_ms", timeout)

	// Determinar si requiere confirmación
	requiresConfirm := determineConfirmation(child, req)
	child.Var("routing.requires_confirmation", requiresConfirm)

	// Ejecutar procesamiento
	execute(child, req, queue, timeout, requiresConfirm)
}

// determineQueue decide la cola según múltiples factores
func determineQueue(ctx *paladin.Context, req Request) string {
	ctx.Child("determineQueue")
	defer ctx.End()

	// Factor 1: tipo de request
	typeToQueue := map[string]string{
		"user_action":  "queue_interactive",
		"batch_job":    "queue_batch",
		"system_event": "queue_critical",
	}

	queue := typeToQueue[req.Type]
	ctx.Decision("cola por tipo", fmt.Sprintf("tipo=%s → cola=%s", req.Type, queue))

	// Factor 2: prioridad
	if req.Priority == "critical" {
		queue = "queue_critical"
		ctx.Decision("cola por prioridad", "priority=critical sobrescribe cola por tipo")
	}

	// Factor 3: fuente
	if req.Source == "mobile" && req.Priority != "critical" {
		queue = "queue_mobile"
		ctx.Decision("cola por fuente", "source=mobile tiene cola dedicada")
	}

	ctx.Var("final.queue", queue)
	return queue
}

// determineTimeout decide el timeout según SLA
func determineTimeout(ctx *paladin.Context, req Request) int {
	ctx.Child("determineTimeout")
	defer ctx.End()

	baseTimeout := 5000 // 5s default

	// Ajustar por tipo
	switch req.Type {
	case "user_action":
		baseTimeout = 3000
		ctx.Decision("timeout base para user_action", "3s para respuesta rápida")
	case "batch_job":
		baseTimeout = 30000
		ctx.Decision("timeout base para batch_job", "30s para jobs largos")
	case "system_event":
		baseTimeout = 1000
		ctx.Decision("timeout base para system_event", "1s para eventos críticos")
	}

	// Ajustar por prioridad
	if req.Priority == "high" {
		baseTimeout = int(float64(baseTimeout) * 0.5)
		ctx.Decision("timeout reducido por prioridad", fmt.Sprintf("high priority → %.0f%% del base", 50))
	}
	if req.Priority == "critical" {
		baseTimeout = 500
		ctx.Decision("timeout mínimo para critical", "500ms max para critical")
	}

	ctx.Var("final.timeout_ms", baseTimeout)
	return baseTimeout
}

// determineConfirmation decide si requiere confirmación
func determineConfirmation(ctx *paladin.Context, req Request) bool {
	ctx.Child("determineConfirmation")
	defer ctx.End()

	// Factores que requieren confirmación
	factors := []string{}

	if req.Type == "user_action" {
		factors = append(factors, "user_action type")
	}

	if req.Priority == "high" || req.Priority == "critical" {
		factors = append(factors, fmt.Sprintf("priority=%s", req.Priority))
	}

	if req.Source == "mobile" {
		factors = append(factors, "mobile source (higher error rate)")
	}

	requiresConfirm := len(factors) > 0

	if requiresConfirm {
		ctx.Decision("confirma requerido", fmt.Sprintf("factores: %v", factors))
	} else {
		ctx.Decision("confirma no requerido", "request de bajo riesgo")
	}

	return requiresConfirm
}

// execute procesa el request en la cola asignada
func execute(ctx *paladin.Context, req Request, queue string, timeout int, requiresConfirm bool) {
	child := ctx.Child("execute")
	defer child.End()

	child.Var("execute.queue", queue)
	child.Var("execute.timeout_ms", timeout)

	// Simular procesamiento
	time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)

	// Simular resultado
	success := rand.Float64() > 0.1

	if success {
		child.Decision("ejecución exitosa", fmt.Sprintf("procesado en %s", queue))
	} else {
		child.ErrorMsg(fmt.Sprintf("timeout o error en queue %s", queue))
		handleRetry(child, req, queue)
	}

	if requiresConfirm {
		markForConfirmation(child, req)
	}
}

// handleRetry maneja reintentos
func handleRetry(ctx *paladin.Context, req Request, originalQueue string) {
	ctx.Child("handleRetry")
	defer ctx.End()

	maxRetries := 3
	retryCount := 0

	for retryCount < maxRetries {
		retryCount++
		ctx.Var("retry.attempt", retryCount)
		ctx.Decision("reintentar", fmt.Sprintf("intento %d/%d en %s", retryCount, maxRetries, originalQueue))

		time.Sleep(time.Duration(retryCount*100) * time.Millisecond)

		if rand.Float64() > 0.3 {
			ctx.Decision("reintento exitoso", fmt.Sprintf("recuperado en intento %d", retryCount))
			return
		}
	}

	ctx.ErrorMsg("todos los reintentos fallidos")
}

// markForConfirmation marca para confirmación
func markForConfirmation(ctx *paladin.Context, req Request) {
	ctx.Decision("marcar para confirmación", fmt.Sprintf("request %s requiere confirmación manual", req.ID))
}
