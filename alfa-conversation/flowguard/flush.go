package flowguard

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Flush genera el archivo JSON final con todo el trace.
// SIEMPRE usar con defer en main():
//
//	trace := flowguard.NewTrace("MiApp")
//	defer trace.Flush()
func (t *Trace) Flush() {
	// Cerrar el span raíz
	t.root.DurationMs = time.Since(t.startTime).Milliseconds()

	// Detectar bottlenecks automáticamente
	markBottlenecks(t.root, t.threshold)

	// Construir resultado
	result := TraceResult{
		TraceID:       t.id,
		Version:       "5.0-golden",
		Generated:     time.Now().Format(time.RFC3339),
		TotalDuration: t.root.DurationMs,
		TotalSpans:    countSpans(t.root),
		TotalErrors:   countErrors(t.root),
		Bottlenecks:   collectBottlenecks(t.root),
		Root:          t.root,
	}

	// Serializar
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("[FLOWGUARD] Error serializando: %v\n", err)
		return
	}

	// Guardar archivo
	filename := fmt.Sprintf("trace_%s.json", t.id)
	if err := os.WriteFile(filename, data, 0644); err != nil {
		fmt.Printf("[FLOWGUARD] Error escribiendo archivo: %v\n", err)
		return
	}

	// Resumen en consola
	fmt.Printf("\n========================================\n")
	fmt.Printf(" TRACE COMPLETADO\n")
	fmt.Printf("========================================\n")
	fmt.Printf(" Archivo:      %s\n", filename)
	fmt.Printf(" Duración:     %d ms\n", result.TotalDuration)
	fmt.Printf(" Funciones:    %d\n", result.TotalSpans)
	fmt.Printf(" Errores:      %d\n", result.TotalErrors)
	fmt.Printf(" Bottlenecks:  %d\n", len(result.Bottlenecks))

	if len(result.Bottlenecks) > 0 {
		fmt.Printf(" ⚠ Cuellos:   %v\n", result.Bottlenecks)
	}
	if result.TotalErrors > 0 {
		fmt.Printf(" ✗ Hay errores. Revisa el trace.\n")
	}

	fmt.Printf("========================================\n")
	fmt.Printf(" Pasa este archivo a tu IA para análisis.\n")
	fmt.Printf("========================================\n\n")
}

// --- Funciones auxiliares ---

// markBottlenecks recorre el árbol y marca spans lentos
func markBottlenecks(span *Span, thresholdMs int64) {
	if span == nil {
		return
	}

	if span.DurationMs > thresholdMs {
		// Solo marcar como bottleneck si NO es porque sus hijos son lentos.
		// Es decir: si el tiempo propio (sin hijos) supera el umbral.
		childTime := int64(0)
		for _, child := range span.Children {
			childTime += child.DurationMs
		}

		selfTime := span.DurationMs - childTime
		if selfTime > thresholdMs {
			span.Bottleneck = true
		}
	}

	for _, child := range span.Children {
		markBottlenecks(child, thresholdMs)
	}
}

// collectBottlenecks recolecta los nombres de spans marcados como bottleneck
func collectBottlenecks(span *Span) []string {
	if span == nil {
		return nil
	}

	var result []string
	if span.Bottleneck {
		result = append(result, fmt.Sprintf("%s (%dms en %s:%d)",
			span.Name, span.DurationMs, span.File, span.Line))
	}

	for _, child := range span.Children {
		result = append(result, collectBottlenecks(child)...)
	}
	return result
}

// countSpans cuenta el total de spans en el árbol
func countSpans(span *Span) int {
	if span == nil {
		return 0
	}
	count := 1
	for _, child := range span.Children {
		count += countSpans(child)
	}
	return count
}

// countErrors cuenta el total de errores en todo el árbol
func countErrors(span *Span) int {
	if span == nil {
		return 0
	}
	count := len(span.Errors)
	for _, child := range span.Children {
		count += countErrors(child)
	}
	return count
}
