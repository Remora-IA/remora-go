package main

import (
	"fmt"
	"math"
	"strings"
)

// priorityItem representa una prioridad operativa ya calculada por un ledger o artifact externo.
type priorityItem struct {
	TaskID         string  `json:"task_id,omitempty"`
	Rank           int     `json:"rank"`
	Deudor         string  `json:"deudor"`
	DeudorID       string  `json:"deudor_id"`
	SaldoTotal     float64 `json:"saldo_total"`
	DiasMoraMax    int     `json:"dias_mora_max"`
	FacturasCount  int     `json:"facturas_count"`
	Score          int     `json:"score"`
	Razon          string  `json:"razon"`
	AccionSugerida string  `json:"accion_sugerida"`
}

func classifyDebtor(saldo float64, diasMora int) (string, string) {
	switch {
	case diasMora >= 90:
		return fmt.Sprintf("deuda antigua (%d días), riesgo legal alto", diasMora),
			"Evaluar escalamiento judicial con abogado. Confirmar si hay carta de requerimiento previa."
	case diasMora >= 60:
		return fmt.Sprintf("cerca de DICOM (%d días), pre-requisito carta certificada", diasMora),
			"Enviar carta certificada de requerimiento hoy. Luego llamar para acordar pago."
	case diasMora >= 30:
		return fmt.Sprintf("mora activa (%d días), zona óptima de negociación", diasMora),
			"Llamada personalizada ofreciendo plan de pago en cuotas."
	default:
		return fmt.Sprintf("mora reciente (%d días)", diasMora),
			"Enviar recordatorio amistoso por email. Confirmar recepción de factura."
	}
}

func computeScore(saldo float64, diasMora int) int {
	if saldo <= 0 {
		return 0
	}
	moraNorm := math.Min(float64(diasMora)/90.0, 1.0) * 60
	saldoNorm := math.Min(math.Log10(saldo+1)/math.Log10(1e7), 1.0) * 40
	return int(math.Round(moraNorm + saldoNorm))
}

func formatMoney(amount float64) string {
	n := int64(math.Round(amount))
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return sign + s
	}
	var b strings.Builder
	first := len(s) % 3
	if first > 0 {
		b.WriteString(s[:first])
		if len(s) > first {
			b.WriteByte('.')
		}
	}
	for i := first; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte('.')
		}
	}
	return sign + b.String()
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
