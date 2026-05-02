// Package fixers implementa generación de propuestas de fix y aplicación
// de las mismas sobre el dataset mutable del auditor.
//
// Nunca aplica un fix sin que el caller llame explícitamente a Apply
// con el ID de la propuesta. Las propuestas se persisten en proposals.json
// para que el orquestador pueda mostrarlas al usuario antes de confirmar.
package fixers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"framework-auditor/checks"
)

// Proposal es un plan de fix concreto, derivado de un Finding.
type Proposal struct {
	ID            string      `json:"id"`              // P-001
	FindingID     string      `json:"finding_id"`      // F-001
	Rule          string      `json:"rule"`            // empty_required, etc.
	Endpoint      string      `json:"endpoint"`
	RecordID      string      `json:"record_id"`
	Field         string      `json:"field"`
	Strategy      string      `json:"strategy"`        // derive_from_related, set_null, next_sequence
	CurrentValue  interface{} `json:"current_value"`
	ProposedValue interface{} `json:"proposed_value"`
	Rationale     string      `json:"rationale"`       // explicación humana de por qué este valor
	RequiresUser  bool        `json:"requires_user"`   // si el fix necesita confirmación adicional
	CreatedAt     time.Time   `json:"created_at"`
}

// AppliedRecord es el snapshot que se appendea a applied.jsonl al aplicar.
type AppliedRecord struct {
	ProposalID    string      `json:"proposal_id"`
	FindingID     string      `json:"finding_id"`
	Endpoint      string      `json:"endpoint"`
	RecordID      string      `json:"record_id"`
	Field         string      `json:"field"`
	Before        interface{} `json:"before"`
	After         interface{} `json:"after"`
	Strategy      string      `json:"strategy"`
	AppliedAt     time.Time   `json:"applied_at"`
}

// ProposeForFinding genera una propuesta de fix a partir de un Finding.
// Devuelve nil si no hay estrategia automatizable para esa regla/finding.
func ProposeForFinding(f checks.Finding, ds *checks.Dataset, idx int) *Proposal {
	if !f.AutoFixable {
		return nil
	}
	strategy, _ := f.FixHint["strategy"].(string)
	switch strategy {
	case "derive_from_related":
		return proposeDeriveFromRelated(f, ds, idx)
	case "set_null":
		return proposeSetNull(f, ds, idx)
	case "next_sequence":
		return proposeNextSequence(f, ds, idx)
	}
	return nil
}

// proposeDeriveFromRelated cubre los casos donde un campo se puede derivar
// de un registro relacionado (ej: agreements.name desde clients.name).
func proposeDeriveFromRelated(f checks.Finding, ds *checks.Dataset, idx int) *Proposal {
	rec := findRecord(ds, f.Endpoint, f.RecordID)
	if rec == nil {
		return nil
	}
	switch f.Endpoint {
	case "agreements":
		clientID := asString(rec["client_id"])
		client := findRecord(ds, "clients", clientID)
		switch f.Field {
		case "name":
			clientName := ""
			if client != nil {
				clientName = asString(client["name"])
			}
			proposed := "Acuerdo sin nombre"
			rationale := "El acuerdo no tiene cliente asociado en clients."
			if clientName != "" {
				proposed = "Acuerdo con " + clientName
				rationale = fmt.Sprintf("Derivado del nombre del cliente vinculado (clients.id=%s, name=%q).", clientID, clientName)
			}
			return newProposal(f, idx, "derive_from_related", rec[f.Field], proposed, rationale, false)
		case "code":
			clientCode := ""
			if client != nil {
				clientCode = asString(client["code"])
			}
			proposed := "AGR-" + asString(rec["id"])
			rationale := "No se pudo derivar desde clients (cliente no encontrado), se usa fallback AGR-<agreement_id>."
			if clientCode != "" {
				proposed = "AGR-" + clientCode
				rationale = fmt.Sprintf("Derivado del código del cliente vinculado (clients.id=%s, code=%q).", clientID, clientCode)
			}
			return newProposal(f, idx, "derive_from_related", rec[f.Field], proposed, rationale, false)
		}
	}
	return nil
}

// proposeSetNull para sentinels de fecha tipo "0000-00-00".
func proposeSetNull(f checks.Finding, ds *checks.Dataset, idx int) *Proposal {
	rec := findRecord(ds, f.Endpoint, f.RecordID)
	if rec == nil {
		return nil
	}
	current := rec[f.Field]
	rationale := fmt.Sprintf("La fecha %q no es válida y se reemplaza por null para no propagar errores en cálculos.", asString(current))
	return newProposal(f, idx, "set_null", current, nil, rationale, false)
}

// proposeNextSequence: para campos numéricos correlativos (ej. número de factura)
// reasignamos el siguiente número disponible en la misma serie.
func proposeNextSequence(f checks.Finding, ds *checks.Dataset, idx int) *Proposal {
	rec := findRecord(ds, f.Endpoint, f.RecordID)
	if rec == nil {
		return nil
	}
	groupBy, _ := f.FixHint["group_by"].(string)
	groupVal := ""
	if groupBy != "" {
		groupVal = asString(rec[groupBy])
	}
	max := 0
	currentID := asString(rec["id"])
	for _, r := range ds.Endpoints[f.Endpoint] {
		if groupBy != "" && asString(r[groupBy]) != groupVal {
			continue
		}
		if asString(r["id"]) == currentID {
			continue
		}
		n, err := strconv.Atoi(asString(r[f.Field]))
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	proposed := strconv.Itoa(max + 1)
	rationale := fmt.Sprintf("Siguiente número disponible en serie %q: %d (máximo actual=%d).", groupVal, max+1, max)
	return newProposal(f, idx, "next_sequence", rec[f.Field], proposed, rationale, true)
}

func newProposal(f checks.Finding, idx int, strategy string, current, proposed interface{}, rationale string, reqUser bool) *Proposal {
	return &Proposal{
		ID:            fmt.Sprintf("P-%03d", idx),
		FindingID:     f.ID,
		Rule:          f.Rule,
		Endpoint:      f.Endpoint,
		RecordID:      f.RecordID,
		Field:         f.Field,
		Strategy:      strategy,
		CurrentValue:  current,
		ProposedValue: proposed,
		Rationale:     rationale,
		RequiresUser:  reqUser,
		CreatedAt:     time.Now().UTC(),
	}
}

// Apply muta el dataset según la propuesta y appendea audit log.
// Devuelve el AppliedRecord persistido.
func Apply(p Proposal, dsPath, appliedLogPath string) (*AppliedRecord, error) {
	ds, err := checks.LoadDataset(dsPath)
	if err != nil {
		return nil, fmt.Errorf("load dataset: %w", err)
	}
	rec := findRecord(ds, p.Endpoint, p.RecordID)
	if rec == nil {
		return nil, fmt.Errorf("record %s:%s no encontrado", p.Endpoint, p.RecordID)
	}
	before := rec[p.Field]
	rec[p.Field] = p.ProposedValue
	if err := ds.Save(dsPath); err != nil {
		return nil, fmt.Errorf("save dataset: %w", err)
	}
	app := AppliedRecord{
		ProposalID: p.ID,
		FindingID:  p.FindingID,
		Endpoint:   p.Endpoint,
		RecordID:   p.RecordID,
		Field:      p.Field,
		Before:     before,
		After:      p.ProposedValue,
		Strategy:   p.Strategy,
		AppliedAt:  time.Now().UTC(),
	}
	if err := appendAppliedLog(appliedLogPath, app); err != nil {
		return nil, err
	}
	return &app, nil
}

func appendAppliedLog(path string, r AppliedRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, _ := json.Marshal(r)
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

// SaveProposals persiste lista de propuestas.
func SaveProposals(path string, proposals []Proposal) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	wrap := map[string]interface{}{
		"version":    1,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"count":      len(proposals),
		"proposals":  proposals,
	}
	data, err := json.MarshalIndent(wrap, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadProposals lee propuestas persistidas. Si el archivo no existe, devuelve [].
func LoadProposals(path string) ([]Proposal, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var wrap struct {
		Proposals []Proposal `json:"proposals"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	return wrap.Proposals, nil
}

// RemoveProposal saca una proposal de la lista persistida.
func RemoveProposal(path, proposalID string) error {
	all, err := LoadProposals(path)
	if err != nil {
		return err
	}
	out := make([]Proposal, 0, len(all))
	for _, p := range all {
		if p.ID != proposalID {
			out = append(out, p)
		}
	}
	return SaveProposals(path, out)
}

// SortedProposals devuelve copias ordenadas por endpoint+record para output.
func SortedProposals(in []Proposal) []Proposal {
	out := make([]Proposal, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Endpoint != out[j].Endpoint {
			return out[i].Endpoint < out[j].Endpoint
		}
		return out[i].RecordID < out[j].RecordID
	})
	return out
}

// ---------- helpers ----------

func findRecord(ds *checks.Dataset, endpoint, recordID string) map[string]interface{} {
	for _, r := range ds.Endpoints[endpoint] {
		if asString(r["id"]) == recordID {
			return r
		}
	}
	return nil
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		b, _ := json.Marshal(v)
		s := string(b)
		s = strings.Trim(s, "\"")
		return s
	}
}
