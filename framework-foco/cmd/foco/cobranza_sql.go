package main

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// priorityItem representa un deudor priorizado para el día de cobranza.
type priorityItem struct {
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

// Estados de `charges` que indican deuda pendiente de cobro.
var cobrablesStates = []string{
	"FACTURADO",
	"EMITIDO",
	"PAGO PARCIAL",
	"ENVIADO AL CLIENTE",
	"EN REVISION",
}

// queryRealPriorities consulta panalbit.db y devuelve el top de deudores
// ranqueados por saldo y días de mora. Usa charges (estado impago) + milestones
// (monto) + clients (nombre).
func queryRealPriorities(dbPath string) ([]priorityItem, error) {
	// panalbit.db es READ-ONLY por contrato (ARCHITECTURE.md §3-4). Forzamos
	// mode=ro + query_only para que cualquier INSERT/UPDATE/DELETE accidental
	// falle a nivel de SQLite, no a nivel de revisión humana.
	dsn := "file:" + dbPath + "?mode=ro&_pragma=query_only(true)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("abrir db: %w", err)
	}
	defer db.Close()

	placeholders := strings.Repeat("?,", len(cobrablesStates))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf(`
		SELECT
			c.id,
			c.code,
			c.name,
			COALESCE(SUM(CAST(m.amount AS REAL)), 0) AS saldo_total,
			COUNT(DISTINCT ch.id) AS facturas_count,
			MIN(m.date) AS fecha_mas_vieja
		FROM charges ch
		JOIN clients c ON c.id = ch.client_id
		LEFT JOIN milestones m ON m.charge_id = ch.id
		WHERE ch.state IN (%s)
		  AND m.amount IS NOT NULL
		  AND m.amount != ''
		GROUP BY c.id
		HAVING saldo_total > 0
		ORDER BY saldo_total DESC
		LIMIT 5
	`, placeholders)

	args := make([]interface{}, len(cobrablesStates))
	for i, s := range cobrablesStates {
		args[i] = s
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query priorities: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	items := make([]priorityItem, 0, 5)
	for rows.Next() {
		var (
			clientID    string
			clientCode  sql.NullString
			clientName  sql.NullString
			saldoTotal  float64
			facturas    int
			fechaVieja  sql.NullString
		)
		if err := rows.Scan(&clientID, &clientCode, &clientName, &saldoTotal, &facturas, &fechaVieja); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		diasMora := 0
		if fechaVieja.Valid && fechaVieja.String != "" {
			if t, perr := time.Parse("2006-01-02", fechaVieja.String[:10]); perr == nil {
				diasMora = int(now.Sub(t).Hours() / 24)
				if diasMora < 0 {
					diasMora = 0
				}
			}
		}

		razon, accion := classifyDebtor(saldoTotal, diasMora)
		score := computeScore(saldoTotal, diasMora)

		deudor := ""
		if clientName.Valid {
			deudor = clientName.String
		}
		if deudor == "" && clientCode.Valid {
			deudor = clientCode.String
		}
		if deudor == "" {
			deudor = clientID
		}

		items = append(items, priorityItem{
			Deudor:         deudor,
			DeudorID:       firstNonEmptyStr(clientCode.String, clientID),
			SaldoTotal:     saldoTotal,
			DiasMoraMax:    diasMora,
			FacturasCount:  facturas,
			Score:          score,
			Razon:          razon,
			AccionSugerida: accion,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iter: %w", err)
	}

	// Re-sort por score para que el ranking final sea por prioridad real,
	// no solo por saldo bruto.
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
	for i := range items {
		items[i].Rank = i + 1
	}
	return items, nil
}

// classifyDebtor devuelve (razón, acción sugerida) en base a Ley 21.394 (Chile)
// y buenas prácticas de cobranza.
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

// computeScore combina monto + mora en un ranking 0-100.
// - log(saldo) para que saldos grandes no dominen totalmente.
// - mora ponderada con saturación en 90 días.
func computeScore(saldo float64, diasMora int) int {
	if saldo <= 0 {
		return 0
	}
	// 0..60 aportados por días de mora
	moraNorm := math.Min(float64(diasMora)/90.0, 1.0) * 60
	// 0..40 aportados por tamaño de saldo (log-scaled, cap en $10M)
	saldoNorm := math.Min(math.Log10(saldo+1)/math.Log10(1e7), 1.0) * 40
	return int(math.Round(moraNorm + saldoNorm))
}

// formatMoney formatea un monto con separador de miles estilo chileno (punto).
func formatMoney(amount float64) string {
	n := int64(math.Round(amount))
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	// Insertar puntos cada 3 dígitos desde el final
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
