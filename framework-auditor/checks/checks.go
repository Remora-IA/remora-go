// Package checks implementa las reglas de auditoría sobre un dataset
// con shape de dump JSON-API (timebilling-like). Cada Rule es una función
// pura que recibe el dataset y devuelve los findings detectados.
//
// Diseño:
//   - Sin LLM ni heurística: reglas determinísticas, reproducibles.
//   - Cada Finding describe endpoint/record/campo + evidencia + sugerencia.
//   - AutoFixable indica si framework-mecanico puede proponer un fix sin
//     intervención humana (decisión final siempre del usuario al apply).
package checks

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Severity de un finding.
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// Reglas (rule_id estable, usadas por framework-mecanico para mapear fixers).
const (
	RuleFKOrphan         = "fk_orphan"
	RuleEmptyRequired    = "empty_required"
	RuleNullRequired     = "null_required"
	RuleInvalidValue     = "invalid_value"
	RuleInvalidDate      = "invalid_date"
	RuleStaleAdvance     = "stale_advance"
	RuleDuplicateRecord  = "duplicate_record"
)

// Finding es un hallazgo del auditor.
type Finding struct {
	ID            string                 `json:"id"`
	Rule          string                 `json:"rule"`
	Severity      string                 `json:"severity"`
	Endpoint      string                 `json:"endpoint"`
	RecordID      string                 `json:"record_id"`
	Field         string                 `json:"field,omitempty"`
	Message       string                 `json:"message"`
	Evidence      map[string]interface{} `json:"evidence,omitempty"`
	Suggestion    string                 `json:"suggestion,omitempty"`
	AutoFixable   bool                   `json:"auto_fixable"`
	FixHint       map[string]interface{} `json:"fix_hint,omitempty"`
}

// Dataset representa el dump JSON-API en memoria.
type Dataset struct {
	Endpoints map[string][]map[string]interface{} `json:"-"`
	Raw       map[string]interface{}              `json:"-"`
}

// LoadDataset lee un JSON con shape {endpoints:{...}} o {clients:[...],...}.
func LoadDataset(path string) (*Dataset, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dataset: %w", err)
	}
	var top map[string]interface{}
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("parse dataset: %w", err)
	}
	endpoints := top
	if ep, ok := top["endpoints"].(map[string]interface{}); ok {
		endpoints = ep
	}
	out := &Dataset{
		Endpoints: map[string][]map[string]interface{}{},
		Raw:       top,
	}
	for name, v := range endpoints {
		arr, ok := v.([]interface{})
		if !ok {
			continue
		}
		records := make([]map[string]interface{}, 0, len(arr))
		for _, e := range arr {
			if rec, ok := e.(map[string]interface{}); ok {
				records = append(records, rec)
			}
		}
		out.Endpoints[name] = records
	}
	return out, nil
}

// SaveDataset persiste el dataset preservando el shape original
// (endpoints anidados si así estaba; reemplazando los endpoints modificados).
func (d *Dataset) Save(path string) error {
	// Reinyectamos los endpoints potencialmente modificados al raw original.
	if epMap, ok := d.Raw["endpoints"].(map[string]interface{}); ok {
		for name, recs := range d.Endpoints {
			arr := make([]interface{}, 0, len(recs))
			for _, r := range recs {
				arr = append(arr, r)
			}
			epMap[name] = arr
		}
		d.Raw["endpoints"] = epMap
	} else {
		for name, recs := range d.Endpoints {
			arr := make([]interface{}, 0, len(recs))
			for _, r := range recs {
				arr = append(arr, r)
			}
			d.Raw[name] = arr
		}
	}
	data, err := json.MarshalIndent(d.Raw, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// FKMap describe relaciones de FK declaradas (endpoint.field -> targetEndpoint).
// Solo declaramos las que son SEMÁNTICAMENTE FKs en el dominio timebilling.
var FKMap = []struct {
	From      string // endpoint origen
	Field     string // campo FK en el origen
	To        string // endpoint destino
	TargetKey string // por defecto "id"
}{
	{"advances", "client_id", "clients", "id"},
	{"advances", "project_id", "projects", "id"},
	{"projects", "client_id", "clients", "id"},
	{"projects", "agreement_id", "agreements", "id"},
	{"billing_documents", "client_id", "clients", "id"},
	{"billing_documents", "charge_id", "charges", "id"},
	{"payments", "client_id", "clients", "id"},
	{"charges", "client_id", "clients", "id"},
	{"charges", "agreement_id", "agreements", "id"},
	{"agreements", "client_id", "clients", "id"},
	{"bank_accounts", "bank_id", "banks", "id"},
	{"bank_accounts", "currency_id", "currencies", "id"},
	{"time_entries", "user_id", "users", "id"},
	{"expenses", "client_id", "clients", "id"},
	{"rates", "user_id", "users", "id"},
}

// RequiredStringFields define campos string que NO deben estar vacíos.
var RequiredStringFields = []struct {
	Endpoint string
	Field    string
}{
	{"agreements", "name"},
	{"clients", "name"},
	{"clients", "code"},
	{"projects", "name"},
	{"projects", "code"},
	{"areas", "name"},
}

// RequiredNonNullFields define campos que no deben ser null.
var RequiredNonNullFields = []struct {
	Endpoint string
	Field    string
}{
	{"agreements", "code"},
	{"agreements", "name"},
}

// invalidDateSentinels son strings que aparecen como "fechas" pero no lo son.
var invalidDateSentinels = map[string]bool{
	"0000-00-00":          true,
	"0000-00-00 00:00:00": true,
}

// dateFields que deberían contener fechas válidas (o ser null).
var DateFields = []struct {
	Endpoint string
	Field    string
}{
	{"projects", "inactive_at"},
	{"clients", "agreement_start_date"},
	{"charges", "date_from"},
	{"charges", "date_to"},
	{"advances", "date"},
	{"payments", "date"},
	{"billing_documents", "date"},
}

// RunAll ejecuta todos los checks y devuelve los findings ordenados.
func RunAll(d *Dataset) []Finding {
	var all []Finding
	all = append(all, CheckFKOrphans(d)...)
	all = append(all, CheckEmptyRequired(d)...)
	all = append(all, CheckNullRequired(d)...)
	all = append(all, CheckInvalidDates(d)...)
	all = append(all, CheckInvalidValues(d)...)
	all = append(all, CheckStaleAdvances(d)...)

	// Ordenamos: critical → warning → info, después por endpoint, después record_id.
	sevRank := map[string]int{SeverityCritical: 0, SeverityWarning: 1, SeverityInfo: 2}
	sort.SliceStable(all, func(i, j int) bool {
		if sevRank[all[i].Severity] != sevRank[all[j].Severity] {
			return sevRank[all[i].Severity] < sevRank[all[j].Severity]
		}
		if all[i].Endpoint != all[j].Endpoint {
			return all[i].Endpoint < all[j].Endpoint
		}
		return all[i].RecordID < all[j].RecordID
	})
	// Asignamos IDs estables F-001, F-002, ...
	for i := range all {
		all[i].ID = fmt.Sprintf("F-%03d", i+1)
	}
	return all
}

// CheckFKOrphans valida que cada FK declarada en FKMap exista en el endpoint destino.
func CheckFKOrphans(d *Dataset) []Finding {
	var findings []Finding
	// Pre-construimos un set de IDs por endpoint destino.
	targetIDs := map[string]map[string]bool{}
	for _, fk := range FKMap {
		if _, done := targetIDs[fk.To]; done {
			continue
		}
		set := map[string]bool{}
		for _, rec := range d.Endpoints[fk.To] {
			if v, ok := rec[fk.TargetKey]; ok && v != nil {
				set[asString(v)] = true
			}
		}
		targetIDs[fk.To] = set
	}

	for _, fk := range FKMap {
		records := d.Endpoints[fk.From]
		if len(records) == 0 {
			continue
		}
		targets := targetIDs[fk.To]
		for _, rec := range records {
			val, ok := rec[fk.Field]
			if !ok || val == nil {
				continue // null FK no es orphan, es ausencia (otro check lo cubre si aplica)
			}
			id := asString(val)
			if id == "" || id == "0" {
				continue
			}
			if !targets[id] {
				recID := pickID(rec)
				findings = append(findings, Finding{
					Rule:     RuleFKOrphan,
					Severity: SeverityCritical,
					Endpoint: fk.From,
					RecordID: recID,
					Field:    fk.Field,
					Message: fmt.Sprintf("Referencia rota: %s[%s].%s=%s no existe en %s",
						fk.From, recID, fk.Field, id, fk.To),
					Evidence: map[string]interface{}{
						"missing_id":      id,
						"target_endpoint": fk.To,
						"target_key":      fk.TargetKey,
					},
					Suggestion:  fmt.Sprintf("Verificar si %s id=%s fue eliminado, o si el registro fuente es huérfano y debe darse de baja.", fk.To, id),
					AutoFixable: false,
				})
			}
		}
	}
	return findings
}

// CheckEmptyRequired detecta campos string requeridos que están vacíos "".
func CheckEmptyRequired(d *Dataset) []Finding {
	var findings []Finding
	for _, rule := range RequiredStringFields {
		for _, rec := range d.Endpoints[rule.Endpoint] {
			v, ok := rec[rule.Field]
			if !ok {
				continue
			}
			s, isString := v.(string)
			if !isString {
				continue
			}
			if strings.TrimSpace(s) == "" {
				recID := pickID(rec)
				findings = append(findings, Finding{
					Rule:     RuleEmptyRequired,
					Severity: SeverityWarning,
					Endpoint: rule.Endpoint,
					RecordID: recID,
					Field:    rule.Field,
					Message: fmt.Sprintf("Campo requerido vacío: %s[%s].%s",
						rule.Endpoint, recID, rule.Field),
					Evidence: map[string]interface{}{
						"current_value": s,
					},
					Suggestion:  "Completar el valor o derivarlo de campos relacionados (ej. nombre del cliente).",
					AutoFixable: true,
					FixHint: map[string]interface{}{
						"strategy": "derive_from_related",
						"endpoint": rule.Endpoint,
						"field":    rule.Field,
					},
				})
			}
		}
	}
	return findings
}

// CheckNullRequired detecta campos requeridos que son null.
func CheckNullRequired(d *Dataset) []Finding {
	var findings []Finding
	for _, rule := range RequiredNonNullFields {
		for _, rec := range d.Endpoints[rule.Endpoint] {
			v, ok := rec[rule.Field]
			if !ok || v == nil {
				recID := pickID(rec)
				findings = append(findings, Finding{
					Rule:     RuleNullRequired,
					Severity: SeverityWarning,
					Endpoint: rule.Endpoint,
					RecordID: recID,
					Field:    rule.Field,
					Message: fmt.Sprintf("Campo requerido null: %s[%s].%s",
						rule.Endpoint, recID, rule.Field),
					Suggestion:  "Asignar valor derivado o solicitar al área dueña del dato.",
					AutoFixable: true,
					FixHint: map[string]interface{}{
						"strategy": "derive_from_related",
						"endpoint": rule.Endpoint,
						"field":    rule.Field,
					},
				})
			}
		}
	}
	return findings
}

// CheckInvalidDates detecta sentinels tipo "0000-00-00" en campos de fecha.
func CheckInvalidDates(d *Dataset) []Finding {
	var findings []Finding
	for _, rule := range DateFields {
		for _, rec := range d.Endpoints[rule.Endpoint] {
			v, ok := rec[rule.Field]
			if !ok || v == nil {
				continue
			}
			s, isString := v.(string)
			if !isString {
				continue
			}
			if invalidDateSentinels[s] {
				recID := pickID(rec)
				findings = append(findings, Finding{
					Rule:     RuleInvalidDate,
					Severity: SeverityWarning,
					Endpoint: rule.Endpoint,
					RecordID: recID,
					Field:    rule.Field,
					Message: fmt.Sprintf("Fecha inválida sentinel en %s[%s].%s = %q",
						rule.Endpoint, recID, rule.Field, s),
					Evidence: map[string]interface{}{
						"current_value": s,
					},
					Suggestion:  "Reemplazar por null (campo no aplicable) o por la fecha real del evento.",
					AutoFixable: true,
					FixHint: map[string]interface{}{
						"strategy": "set_null",
						"endpoint": rule.Endpoint,
						"field":    rule.Field,
					},
				})
			}
		}
	}
	return findings
}

// CheckInvalidValues detecta valores fuera de rango en campos numéricos
// con semántica conocida (ej: número de factura no puede ser negativo).
func CheckInvalidValues(d *Dataset) []Finding {
	var findings []Finding
	// billing_documents.number debe ser positivo.
	for _, rec := range d.Endpoints["billing_documents"] {
		v, ok := rec["number"]
		if !ok || v == nil {
			continue
		}
		nStr := asString(v)
		n, err := strconv.Atoi(nStr)
		if err != nil {
			continue
		}
		if n < 0 {
			recID := pickID(rec)
			findings = append(findings, Finding{
				Rule:     RuleInvalidValue,
				Severity: SeverityCritical,
				Endpoint: "billing_documents",
				RecordID: recID,
				Field:    "number",
				Message:  fmt.Sprintf("Número de factura negativo: %s[%s].number=%d", "billing_documents", recID, n),
				Evidence: map[string]interface{}{
					"current_value": n,
				},
				Suggestion:  "Reasignar el número con el siguiente correlativo válido de la serie.",
				AutoFixable: true,
				FixHint: map[string]interface{}{
					"strategy": "next_sequence",
					"endpoint": "billing_documents",
					"field":    "number",
					"group_by": "series_number",
				},
			})
		}
	}
	return findings
}

// CheckStaleAdvances detecta anticipos donde residue == amount y fecha > 1 año:
// el cliente entregó dinero que nunca se aplicó a ningún consumo.
func CheckStaleAdvances(d *Dataset) []Finding {
	var findings []Finding
	threshold := time.Now().AddDate(-1, 0, 0)
	for _, rec := range d.Endpoints["advances"] {
		amount := toFloat(rec["amount"])
		residue := toFloat(rec["residue"])
		if amount == 0 || residue == 0 {
			continue
		}
		if amount != residue {
			continue
		}
		dateStr, _ := rec["date"].(string)
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if t.After(threshold) {
			continue
		}
		recID := pickID(rec)
		findings = append(findings, Finding{
			Rule:     RuleStaleAdvance,
			Severity: SeverityWarning,
			Endpoint: "advances",
			RecordID: recID,
			Message: fmt.Sprintf("Anticipo nunca consumido desde %s (monto %.2f sin aplicar)",
				dateStr, amount),
			Evidence: map[string]interface{}{
				"date":       dateStr,
				"amount":     amount,
				"residue":    residue,
				"client_id":  asString(rec["client_id"]),
				"project_id": asString(rec["project_id"]),
			},
			Suggestion:  "Confirmar con finanzas si corresponde aplicar a una factura abierta o devolver al cliente.",
			AutoFixable: false,
		})
	}
	return findings
}

// ---------- helpers ----------

func pickID(rec map[string]interface{}) string {
	if v, ok := rec["id"]; ok && v != nil {
		return asString(v)
	}
	if v, ok := rec["code"]; ok && v != nil {
		return asString(v)
	}
	return ""
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// JSON numbers vienen como float64; preservamos enteros sin decimales.
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func toFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	default:
		return 0
	}
}

// SaveFindings persiste findings.json con metadata.
func SaveFindings(path string, findings []Finding) error {
	out := map[string]interface{}{
		"version":    1,
		"scanned_at": time.Now().UTC().Format(time.RFC3339),
		"count":      len(findings),
		"findings":   findings,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadFindings lee un findings.json previo.
func LoadFindings(path string) ([]Finding, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Findings []Finding `json:"findings"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	return wrap.Findings, nil
}
