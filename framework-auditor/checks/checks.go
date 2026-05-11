// Package checks implementa las reglas de auditoría sobre un dataset
// con shape de dump JSON-API genérico. Cada Rule es una función pura
// que recibe el dataset y devuelve los findings detectados.
//
// Diseño:
//   - Sin LLM ni heurística: reglas determinísticas, reproducibles.
//   - Business-agnostic: FK relationships, required fields, and date fields
//     are inferred dynamically from the dataset structure (no hardcoded
//     table/field names).
//   - Cada Finding describe endpoint/record/campo + evidencia + sugerencia.
//   - AutoFixable indica si framework-mecanico puede proponer un fix sin
//     intervención humana (decisión final siempre del usuario al apply).
package checks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	RuleFKOrphan        = "fk_orphan"
	RuleEmptyRequired   = "empty_required"
	RuleNullRequired    = "null_required"
	RuleInvalidValue    = "invalid_value"
	RuleInvalidDate     = "invalid_date"
	RuleStaleAdvance    = "stale_advance"
	RuleDuplicateRecord = "duplicate_record"
	RuleMissingContact  = "missing_contact_destination"
)

// Finding es un hallazgo del auditor.
type Finding struct {
	ID          string                 `json:"id"`
	Rule        string                 `json:"rule"`
	Severity    string                 `json:"severity"`
	Endpoint    string                 `json:"endpoint"`
	RecordID    string                 `json:"record_id"`
	Field       string                 `json:"field,omitempty"`
	Message     string                 `json:"message"`
	Evidence    map[string]interface{} `json:"evidence,omitempty"`
	Suggestion  string                 `json:"suggestion,omitempty"`
	AutoFixable bool                   `json:"auto_fixable"`
	FixHint     map[string]interface{} `json:"fix_hint,omitempty"`
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
	if tables, ok := top["tables"].(map[string]interface{}); ok {
		endpoints = tables
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

// EndpointField is a generic (endpoint, field) pair used by inferred rules.
type EndpointField struct {
	Endpoint string
	Field    string
}

// FKRelation describes a foreign-key relationship between two endpoints.
type FKRelation struct {
	From      string // source endpoint
	Field     string // FK field in source
	To        string // target endpoint
	TargetKey string // key field in target (usually "id")
}

// InferFKRelations discovers FK relationships by looking for fields that end
// in "_id" and matching them to existing endpoints in the dataset.
// This replaces the hardcoded FKMap and works for any business domain.
func InferFKRelations(d *Dataset) []FKRelation {
	var rels []FKRelation
	seen := map[string]bool{}
	for endpoint, records := range d.Endpoints {
		if len(records) == 0 {
			continue
		}
		// Sample all records to collect field names (some records may have
		// different fields due to sparse JSON).
		fieldSet := map[string]bool{}
		for _, rec := range records {
			for k := range rec {
				fieldSet[k] = true
			}
		}
		for field := range fieldSet {
			if !strings.HasSuffix(field, "_id") {
				continue
			}
			base := strings.TrimSuffix(field, "_id")
			// Try common plural forms.
			for _, candidate := range []string{base + "s", base + "es", base} {
				if _, exists := d.Endpoints[candidate]; exists {
					key := endpoint + "." + field + "->" + candidate
					if !seen[key] {
						seen[key] = true
						rels = append(rels, FKRelation{
							From:      endpoint,
							Field:     field,
							To:        candidate,
							TargetKey: "id",
						})
					}
					break
				}
			}
		}
	}
	// Sort for deterministic output.
	sort.Slice(rels, func(i, j int) bool {
		if rels[i].From != rels[j].From {
			return rels[i].From < rels[j].From
		}
		return rels[i].Field < rels[j].Field
	})
	return rels
}

// commonRequiredFieldNames are field names that are universally expected to
// have non-empty, non-null values when present.  Business-agnostic.
var commonRequiredFieldNames = map[string]bool{
	"name": true, "code": true, "title": true,
}

// InferRequiredStringFields scans the dataset and returns (endpoint, field)
// pairs for fields with commonly-required names that exist in the data.
func InferRequiredStringFields(d *Dataset) []EndpointField {
	var out []EndpointField
	seen := map[string]bool{}
	for endpoint, records := range d.Endpoints {
		if len(records) == 0 {
			continue
		}
		fieldSet := map[string]bool{}
		for _, rec := range records {
			for k := range rec {
				fieldSet[k] = true
			}
		}
		for field := range fieldSet {
			if !commonRequiredFieldNames[field] {
				continue
			}
			key := endpoint + "." + field
			if !seen[key] {
				seen[key] = true
				out = append(out, EndpointField{Endpoint: endpoint, Field: field})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Endpoint != out[j].Endpoint {
			return out[i].Endpoint < out[j].Endpoint
		}
		return out[i].Field < out[j].Field
	})
	return out
}

// InferRequiredNonNullFields returns the same fields as
// InferRequiredStringFields — if a "name" or "code" field exists, it should
// not be null either.
func InferRequiredNonNullFields(d *Dataset) []EndpointField {
	return InferRequiredStringFields(d)
}

// invalidDateSentinels son strings que aparecen como "fechas" pero no lo son.
var invalidDateSentinels = map[string]bool{
	"0000-00-00":          true,
	"0000-00-00 00:00:00": true,
}

// dateFieldNameHints are substrings that indicate a field holds a date value.
var dateFieldNameHints = []string{"date", "_at", "created", "updated", "expired", "starts", "ends"}

// InferDateFields discovers date fields by matching field names against common
// naming patterns and verifying at least one record has a date-like value.
func InferDateFields(d *Dataset) []EndpointField {
	var out []EndpointField
	seen := map[string]bool{}
	for endpoint, records := range d.Endpoints {
		if len(records) == 0 {
			continue
		}
		fieldSet := map[string]bool{}
		for _, rec := range records {
			for k := range rec {
				fieldSet[k] = true
			}
		}
		for field := range fieldSet {
			low := strings.ToLower(field)
			isDate := false
			for _, hint := range dateFieldNameHints {
				if strings.Contains(low, hint) {
					isDate = true
					break
				}
			}
			if !isDate {
				continue
			}
			key := endpoint + "." + field
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, EndpointField{Endpoint: endpoint, Field: field})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Endpoint != out[j].Endpoint {
			return out[i].Endpoint < out[j].Endpoint
		}
		return out[i].Field < out[j].Field
	})
	return out
}

// RunAll ejecuta todos los checks y devuelve los findings ordenados.
func RunAll(d *Dataset) []Finding {
	return RunAllWithSchema(d, nil)
}

// RunAllWithSchema ejecuta todos los checks incluyendo validación de schema
// cuando tableColumns no es nil. tableColumns mapea tabla → lista de columnas
// y se obtiene via TableColumnsFromDB.
//
// All data-quality rules (FK, required fields, dates) are inferred dynamically
// from the dataset structure — no hardcoded business-specific tables or fields.
func RunAllWithSchema(d *Dataset, tableColumns map[string][]string) []Finding {
	// Infer rules dynamically from the actual dataset.
	fkRels := InferFKRelations(d)
	reqStr := InferRequiredStringFields(d)
	reqNonNull := InferRequiredNonNullFields(d)
	dateFields := InferDateFields(d)

	var all []Finding
	all = append(all, CheckFKOrphans(d, fkRels)...)
	all = append(all, CheckEmptyRequired(d, reqStr)...)
	all = append(all, CheckNullRequired(d, reqNonNull)...)
	all = append(all, CheckInvalidDates(d, dateFields)...)
	all = append(all, CheckInvalidValues(d)...)
	all = append(all, CheckStaleAdvances(d)...)
	all = append(all, CheckMissingContactDestination(d)...)
	if tableColumns != nil {
		all = append(all, CheckSchemaContactCapability(d, tableColumns)...)
	}

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

func CheckMissingContactDestination(d *Dataset) []Finding {
	var findings []Finding
	for endpoint, records := range d.Endpoints {
		if !isContactEntityEndpoint(endpoint) {
			continue
		}
		for _, rec := range records {
			if recordHasEmail(rec) {
				continue
			}
			recID := pickID(rec)
			findings = append(findings, Finding{
				Rule:     RuleMissingContact,
				Severity: SeverityWarning,
				Endpoint: endpoint,
				RecordID: recID,
				Field:    "email",
				Message:  fmt.Sprintf("Falta email/contacto operativo: %s[%s].email", endpoint, recID),
				Evidence: map[string]interface{}{
					"checked_fields": []string{"email", "contact_email", "mail", "correo", "to", "destination"},
				},
				Suggestion:  "Solicitar o enriquecer el contacto antes de ejecutar mensajería saliente.",
				AutoFixable: false,
				FixHint: map[string]interface{}{
					"strategy": "request_contact_destination",
					"endpoint": endpoint,
					"field":    "email",
				},
			})
		}
	}
	return findings
}

// CheckFKOrphans valida que cada FK en fkRels exista en el endpoint destino.
func CheckFKOrphans(d *Dataset, fkRels []FKRelation) []Finding {
	var findings []Finding
	// Pre-construimos un set de IDs por endpoint destino.
	targetIDs := map[string]map[string]bool{}
	for _, fk := range fkRels {
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

	for _, fk := range fkRels {
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
func CheckEmptyRequired(d *Dataset, rules []EndpointField) []Finding {
	var findings []Finding
	for _, rule := range rules {
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
					Suggestion:  "Completar el valor o derivarlo de campos relacionados.",
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
func CheckNullRequired(d *Dataset, rules []EndpointField) []Finding {
	var findings []Finding
	for _, rule := range rules {
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
func CheckInvalidDates(d *Dataset, dateFields []EndpointField) []Finding {
	var findings []Finding
	for _, rule := range dateFields {
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

// CheckInvalidValues detecta valores fuera de rango en campos numéricos.
// Checks any field named "number", "amount", or "total" for negative values
// across all endpoints — business-agnostic.
func CheckInvalidValues(d *Dataset) []Finding {
	var findings []Finding
	numericFields := map[string]bool{"number": true, "amount": true, "total": true}
	for endpoint, records := range d.Endpoints {
		for _, rec := range records {
			for field := range numericFields {
				v, ok := rec[field]
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
						Endpoint: endpoint,
						RecordID: recID,
						Field:    field,
						Message:  fmt.Sprintf("Valor negativo: %s[%s].%s=%d", endpoint, recID, field, n),
						Evidence: map[string]interface{}{
							"current_value": n,
						},
						Suggestion:  "Verificar si el valor negativo es correcto o debe corregirse.",
						AutoFixable: true,
						FixHint: map[string]interface{}{
							"strategy": "review_value",
							"endpoint": endpoint,
							"field":    field,
						},
					})
				}
			}
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

func isContactEntityEndpoint(endpoint string) bool {
	e := strings.ToLower(strings.TrimSpace(endpoint))
	switch e {
	case "clients", "customers", "debtors", "deudores", "contacts", "contactos":
		return true
	default:
		return strings.Contains(e, "client") || strings.Contains(e, "customer") || strings.Contains(e, "deudor")
	}
}

func recordHasEmail(rec map[string]interface{}) bool {
	for _, field := range []string{"email", "contact_email", "mail", "correo", "to", "destination"} {
		if v, ok := rec[field]; ok && looksLikeEmail(asString(v)) {
			return true
		}
	}
	return false
}

func looksLikeEmail(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return strings.Contains(s, "@") && strings.Contains(s, ".") && !strings.Contains(s, "@ejemplo.")
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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "findings-*.json.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
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
