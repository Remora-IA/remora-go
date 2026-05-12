package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// flowPrerequisite represents a single data requirement for a flow's terminal
// action, with status indicating whether it's available, missing, or not needed.
type flowPrerequisite struct {
	Field       string `json:"field"`
	Label       string `json:"label"`
	Status      string `json:"status"` // "available", "missing", "not_needed"
	Value       string `json:"value,omitempty"`
	Source      string `json:"source,omitempty"`
	Reason      string `json:"reason"`
	GapEndpoint string `json:"gap_endpoint,omitempty"`
	GapField    string `json:"gap_field,omitempty"`
}

const (
	prereqAvailable = "available"
	prereqMissing   = "missing"
	prereqNotNeeded = "not_needed"
	prereqDerivable = "derivable_unresolved"
)

// flowTerminalRequirements infers what artifacts the flow's terminal action
// needs by walking the node list backwards. The terminal action is the last
// node with a cycle_terminal policy or the node that produces message.sent.v1.
// Returns the set of artifact types required to complete the full cycle.
func flowTerminalRequirements(flow flowManifest) []string {
	// Walk nodes in reverse to find the terminal action
	nodes := flow.Nodes
	var terminalRequires []string
	for i := len(nodes) - 1; i >= 0; i-- {
		n := nodes[i]
		if containsString(n.Policies, "cycle_terminal") ||
			containsString(n.Produces, "message.sent.v1") ||
			n.Framework == "mensajero" {
			terminalRequires = append(terminalRequires, n.Requires...)
			terminalRequires = append(terminalRequires, n.Inputs...)
			break
		}
	}
	if len(terminalRequires) == 0 {
		// Default: common terminal requirements for any outbound communication flow
		terminalRequires = []string{"contact.destination.v1", "message.draft.v1"}
	}
	return uniqueStrings(terminalRequires)
}

// flowRequiredDataFields returns the business data fields that the flow's
// terminal action needs, derived from the terminal requirements and the
// business semantic pack. This is business-agnostic: it reads the declarative
// mapping from sabio.business.json's artifact_requirements section.
//
// If the pack declares artifact_requirements, those are used directly.
// If not (legacy packs), a minimal generic fallback is applied:
//   - contact.destination.* -> primary entity table + "email"
//   - message.draft.* -> primary entity table + display_column
//   - anything else -> no fields (all gaps pass through)
func flowRequiredDataFields(terminalArtifacts []string, packPath string) []prerequisiteDataField {
	if packPath == "" {
		return nil
	}
	raw, err := os.ReadFile(packPath)
	if err != nil {
		return nil
	}
	var doc struct {
		ArtifactRequirements map[string]struct {
			Fields []struct {
				Table   string `json:"table"`
				Field   string `json:"field"`
				Label   string `json:"label"`
				Reason  string `json:"reason"`
				Derived bool   `json:"derived,omitempty"`
			} `json:"fields"`
		} `json:"artifact_requirements"`
		PrimaryEntities map[string]struct {
			Table         string `json:"table"`
			Label         string `json:"label"`
			ScopeKey      string `json:"scope_key"`
			DisplayColumn string `json:"display_column"`
		} `json:"primary_entities"`
		ScopePolicies struct {
			ScopeEntity string `json:"scope_entity"`
		} `json:"scope_policies"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}

	// Declarative path: read from artifact_requirements in the semantic pack.
	if len(doc.ArtifactRequirements) > 0 {
		return flowRequiredDataFieldsFromPack(terminalArtifacts, doc.ArtifactRequirements)
	}

	// Fallback: generic minimal mapping for legacy packs without
	// artifact_requirements. Uses primary entity metadata only.
	return flowRequiredDataFieldsFallback(terminalArtifacts, doc.PrimaryEntities, doc.ScopePolicies.ScopeEntity)
}

// flowRequiredDataFieldsFromPack resolves required fields from the declarative
// artifact_requirements section. For each terminal artifact, it looks up a
// matching key using prefix matching (e.g. "contact.destination.v1" matches
// "contact.destination.v1" exactly, or "contact.destination" as prefix).
func flowRequiredDataFieldsFromPack(terminalArtifacts []string, reqs map[string]struct {
	Fields []struct {
		Table   string `json:"table"`
		Field   string `json:"field"`
		Label   string `json:"label"`
		Reason  string `json:"reason"`
		Derived bool   `json:"derived,omitempty"`
	} `json:"fields"`
}) []prerequisiteDataField {
	var fields []prerequisiteDataField
	for _, artifact := range terminalArtifacts {
		req, ok := reqs[artifact]
		if !ok {
			// Try prefix match: "contact.destination.v1" -> "contact.destination"
			for key, r := range reqs {
				if strings.HasPrefix(artifact, key) || strings.HasPrefix(key, artifact) {
					req = r
					ok = true
					break
				}
			}
		}
		if !ok {
			continue
		}
		for _, f := range req.Fields {
			fields = append(fields, prerequisiteDataField{
				Artifact: artifact,
				Table:    f.Table,
				Field:    f.Field,
				Label:    f.Label,
				Reason:   f.Reason,
				Derived:  f.Derived,
			})
		}
	}
	return fields
}

// flowRequiredDataFieldsFallback provides a minimal generic mapping when the
// semantic pack does not declare artifact_requirements. Only covers the two
// most common terminal artifacts using primary entity metadata.
func flowRequiredDataFieldsFallback(terminalArtifacts []string, primaryEntities map[string]struct {
	Table         string `json:"table"`
	Label         string `json:"label"`
	ScopeKey      string `json:"scope_key"`
	DisplayColumn string `json:"display_column"`
}, scopeEntity string) []prerequisiteDataField {
	pe, ok := primaryEntities[scopeEntity]
	if !ok {
		return nil
	}
	var fields []prerequisiteDataField
	for _, artifact := range terminalArtifacts {
		switch {
		case strings.Contains(artifact, "contact.destination"):
			fields = append(fields, prerequisiteDataField{
				Artifact: artifact,
				Table:    pe.Table,
				Field:    "email",
				Label:    "Correo electronico del " + pe.Label,
				Reason:   "Para enviar comunicacion al destinatario",
			})
		case strings.Contains(artifact, "message.draft"):
			if pe.DisplayColumn != "" {
				fields = append(fields, prerequisiteDataField{
					Artifact: artifact,
					Table:    pe.Table,
					Field:    pe.DisplayColumn,
					Label:    "Nombre del " + pe.Label,
					Reason:   "Para personalizar el mensaje",
				})
			}
		}
	}
	return fields
}

type prerequisiteDataField struct {
	Artifact string `json:"artifact"`
	Table    string `json:"table"`
	Field    string `json:"field"`
	Label    string `json:"label"`
	Reason   string `json:"reason"`
	Derived  bool   `json:"derived,omitempty"`
}

// filterGapsByFlowPurpose removes gaps that are not relevant to the flow's
// terminal action requirements. This is the key filter that prevents
// irrelevant questions: if agreements.name is empty but the flow doesn't need
// agreements.name to send an email, the gap is filtered out.
//
// Business-agnostic: it compares gap endpoints/fields against the fields
// required by the flow's terminal artifacts.
func filterGapsByFlowPurpose(gaps []dataGap, requiredFields []prerequisiteDataField) []dataGap {
	if len(requiredFields) == 0 {
		return gaps
	}
	// Build a set of (table, field) pairs that the flow actually needs.
	needed := make(map[string]bool, len(requiredFields))
	neededTables := make(map[string]bool, len(requiredFields))
	for _, f := range requiredFields {
		needed[f.Table+"."+f.Field] = true
		neededTables[f.Table] = true
	}
	var filtered []dataGap
	for _, g := range gaps {
		if g.Endpoint == "" {
			// Structural gaps (no endpoint) are always kept — they represent
			// infrastructure issues like missing SMTP credentials.
			filtered = append(filtered, g)
			continue
		}
		// Keep the gap if it's about a field the flow needs, OR if it's
		// a contact/email gap on a table the flow cares about.
		if needed[g.Endpoint+"."+g.Field] {
			filtered = append(filtered, g)
			continue
		}
		if neededTables[g.Endpoint] && isContactRelatedGap(g) {
			filtered = append(filtered, g)
			continue
		}
	}
	return filtered
}

func isContactRelatedGap(g dataGap) bool {
	k := strings.ToLower(g.Kind + " " + g.Field)
	return strings.Contains(k, "contact") ||
		strings.Contains(k, "email") ||
		strings.Contains(k, "mail") ||
		strings.Contains(k, "correo") ||
		strings.Contains(k, "destination")
}

// filterGapsByExistingEntityData removes gaps for fields that already have
// values in the current entity's data. For example, if the gap is
// "clients.name is empty" but the entity ref already has name="Thiel-Effertz",
// the gap is not relevant for this entity and should be skipped.
//
// Scope: ONLY filters gaps on the PRIMARY entity table where the entity
// already has the data. It does NOT filter secondary tables -- that is the
// job of filterGapsByFlowPurpose, which knows which tables the flow needs.
// Filtering secondary tables here creates a conflict: filterGapsByFlowPurpose
// may declare a secondary table necessary, and this filter would silently
// drop its gaps.
func filterGapsByExistingEntityData(gaps []dataGap, entityRef map[string]interface{}, entityTable string) []dataGap {
	if entityRef == nil || len(gaps) == 0 {
		return gaps
	}
	var filtered []dataGap
	for _, g := range gaps {
		// Schema-level gaps are structural (missing columns) and must
		// always be kept -- no entity ref can satisfy them.
		if g.Kind == "schema_contact_gap" || g.Kind == "missing_contact_destination" || g.Kind == "missing_contact" {
			filtered = append(filtered, g)
			continue
		}
		// Only filter gaps on the entity's own primary table (e.g. clients).
		// If the entity already has a value for this field, skip the gap.
		if g.Endpoint != "" && g.Endpoint == entityTable {
			fieldVal := entityRefFieldValue(entityRef, g.Field)
			if fieldVal != "" {
				continue
			}
		}
		// For secondary tables: never filter here. filterGapsByFlowPurpose
		// already decided whether the table is relevant to the flow.
		filtered = append(filtered, g)
	}
	return filtered
}

// entityRefFieldValue extracts a field value from the entity ref map,
// trying both the exact field name and common aliases.
func entityRefFieldValue(entityRef map[string]interface{}, field string) string {
	if entityRef == nil || field == "" {
		return ""
	}
	// Direct match
	if v, ok := entityRef[field].(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	// Check display_name for "name" field
	if field == "name" {
		if v, ok := entityRef["display_name"].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	// Check code variants
	if field == "code" {
		for _, k := range []string{"code", "client_code", "entity_code"} {
			if v, ok := entityRef[k].(string); ok && strings.TrimSpace(v) != "" {
				return v
			}
		}
	}
	return ""
}

// entityTableFromPack returns the primary entity's table name from the
// business semantic pack.
func entityTableFromPack(packPath string) string {
	if packPath == "" {
		return ""
	}
	raw, err := os.ReadFile(packPath)
	if err != nil {
		return ""
	}
	var doc struct {
		PrimaryEntities map[string]struct {
			Table string `json:"table"`
		} `json:"primary_entities"`
		ScopePolicies struct {
			ScopeEntity string `json:"scope_entity"`
		} `json:"scope_policies"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	if pe, ok := doc.PrimaryEntities[doc.ScopePolicies.ScopeEntity]; ok {
		return pe.Table
	}
	return ""
}

// generateFlowPrerequisites creates the flow.prerequisites.v1 artifact
// which acts as a "semaphore" showing what the flow needs, what's available,
// and what's missing. This is the key UX element that makes gaps observable
// without requiring staff to understand the database.
func (s *server) generateFlowPrerequisites(runID string, flow flowManifest, entityRef map[string]interface{}, artifacts map[string]flowRunArtifact, available map[string]bool, gaps []dataGap, timeline []flowRunStep) map[string]interface{} {
	packPath := s.businessSemanticPackPath(flow.BusinessID)
	terminalReqs := flowTerminalRequirements(flow)
	requiredFields := flowRequiredDataFields(terminalReqs, packPath)
	fieldEvidence := buildFlowFieldEvidence(requiredFields, entityRef, artifacts, available)

	entityName := "la entidad"
	if entityRef != nil {
		if n, ok := entityRef["name"].(string); ok && n != "" {
			entityName = n
		}
	}

	var prerequisites []flowPrerequisite
	for idx, f := range requiredFields {
		evidence := fieldEvidence[idx]
		prereq := flowPrerequisite{
			Field:       f.Field,
			Label:       f.Label,
			GapEndpoint: f.Table,
			GapField:    f.Field,
			Reason:      f.Reason,
		}
		switch evidence.Status {
		case prereqAvailable:
			prereq.Status = prereqAvailable
			prereq.Value = evidence.Value
			prereq.Source = evidence.Source
		case prereqDerivable:
			prereq.Status = prereqDerivable
			prereq.Source = evidence.Reason
		default:
			hasGap := false
			for _, g := range gaps {
				if g.Endpoint == f.Table && (g.Field == f.Field || isContactRelatedGap(g)) {
					hasGap = true
					break
				}
			}
			prereq.Status = prereqMissing
			if hasGap {
				prereq.Source = "se preguntara al usuario"
			} else if available[f.Artifact] {
				prereq.Source = "artifact presente, pero sin evidencia estructurada del campo"
			} else {
				prereq.Source = "no encontrado en datos del negocio"
			}
		}
		prerequisites = append(prerequisites, prereq)
	}

	// Add non-data prerequisites (infrastructure artifacts)
	for _, art := range terminalReqs {
		if strings.Contains(art, "credentials") {
			prereq := flowPrerequisite{
				Field:  art,
				Label:  prerequisiteLabel(art),
				Reason: "Necesario para ejecutar la acción",
			}
			if status, ok := artifacts["credentials.status.v1"].Payload.(map[string]interface{}); ok && jsonFirstString(status, "capability") == art {
				if ready, _ := status["ready"].(bool); ready {
					prereq.Status = prereqAvailable
					prereq.Source = "vault legible y consumer-ready"
				} else {
					prereq.Status = prereqMissing
					prereq.Source = firstNonEmptyPipelineString(jsonFirstString(status, "error"), "requiere configuración")
				}
			} else if available[art] {
				prereq.Status = prereqAvailable
				prereq.Source = "configurado"
			} else {
				prereq.Status = prereqMissing
				prereq.Source = "requiere configuración"
			}
			prerequisites = append(prerequisites, prereq)
		}
	}

	// Mark gaps that are NOT needed by the flow
	notNeededGaps := []map[string]interface{}{}
	for _, g := range parseDataGapsForPrereqs(artifacts) {
		isNeeded := false
		for _, f := range requiredFields {
			if g.Endpoint == f.Table && (g.Field == f.Field || isContactRelatedGap(g)) {
				isNeeded = true
				break
			}
		}
		if !isNeeded && g.Endpoint != "" {
			notNeededGaps = append(notNeededGaps, map[string]interface{}{
				"endpoint": g.Endpoint,
				"field":    g.Field,
				"kind":     g.Kind,
				"reason":   fmt.Sprintf("No es necesario para la acción del flujo sobre %s", entityName),
			})
		}
	}

	availableCount := 0
	missingCount := 0
	unresolvedCount := 0
	for _, p := range prerequisites {
		if p.Status == prereqAvailable {
			availableCount++
		} else if p.Status == prereqDerivable {
			unresolvedCount++
		} else if p.Status == prereqMissing {
			missingCount++
		}
	}
	executionBlockers := flowPrerequisiteExecutionBlockers(timeline)
	readyForTerminal := missingCount == 0 && unresolvedCount == 0 && len(executionBlockers) == 0

	payload := map[string]interface{}{
		"artifact_type":           "flow.prerequisites.v1",
		"entity_name":             entityName,
		"terminal_artifacts":      terminalReqs,
		"field_evidence":          fieldEvidence,
		"prerequisites":           prerequisites,
		"not_needed_gaps":         notNeededGaps,
		"execution_blockers":      executionBlockers,
		"available_count":         availableCount,
		"missing_count":           missingCount,
		"unresolved_count":        unresolvedCount,
		"execution_blocker_count": len(executionBlockers),
		"not_needed_count":        len(notNeededGaps),
		"all_satisfied":           readyForTerminal,
		"ready_for_terminal":      readyForTerminal,
		"generated_at":            time.Now().UTC().Format(time.RFC3339Nano),
		"human_summary":           prerequisitesSummary(entityName, prerequisites, notNeededGaps, executionBlockers),
	}
	return payload
}

func (s *server) refreshFlowPrerequisites(runID string, flow flowManifest, available map[string]bool, result *flowRunResult) string {
	if result == nil {
		return ""
	}
	entityRef := entityRefFromArtifacts(result.Artifacts)
	allGaps := filterGapsByScope(parseDataGaps(result.Artifacts), s.loadBusinessScopeTables(flow.BusinessID))
	payload := s.generateFlowPrerequisites(runID, flow, entityRef, result.Artifacts, available, allGaps, result.Timeline)
	path := s.persistFlowArtifact(runID, "flow_prerequisites", "flow.prerequisites.v1", payload)
	available["flow.prerequisites.v1"] = true
	result.Artifacts["flow.prerequisites.v1"] = flowRunArtifact{
		Type:      "flow.prerequisites.v1",
		Source:    "flow_engine.prerequisites",
		Node:      "flow_prerequisites",
		Path:      path,
		Payload:   payload,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	return "flow.prerequisites.v1"
}

func parseDataGapsForPrereqs(artifacts map[string]flowRunArtifact) []dataGap {
	return parseDataGaps(artifacts)
}

func prerequisiteLabel(artifact string) string {
	switch {
	case strings.Contains(artifact, "smtp"):
		return "Credenciales de email (SMTP)"
	case strings.Contains(artifact, "credential"):
		return "Credenciales de acceso"
	case strings.Contains(artifact, "contact.destination"):
		return "Destinatario de contacto"
	case strings.Contains(artifact, "message.draft"):
		return "Borrador del mensaje"
	default:
		return artifact
	}
}

func prerequisitesSummary(entityName string, prereqs []flowPrerequisite, notNeeded []map[string]interface{}, blockers []map[string]interface{}) string {
	available := 0
	missing := 0
	unresolved := 0
	var missingLabels []string
	var unresolvedLabels []string
	for _, p := range prereqs {
		if p.Status == prereqAvailable {
			available++
		} else if p.Status == prereqDerivable {
			unresolved++
			unresolvedLabels = append(unresolvedLabels, p.Label)
		} else if p.Status == prereqMissing {
			missing++
			missingLabels = append(missingLabels, p.Label)
		}
	}
	if missing == 0 && unresolved == 0 && len(blockers) == 0 {
		return fmt.Sprintf("Todos los datos necesarios para %s están disponibles.", entityName)
	}
	total := available + missing + unresolved
	summary := fmt.Sprintf("Para %s: %d/%d requisitos validados.", entityName, available, total)
	if len(unresolvedLabels) > 0 {
		summary += " Derivable pero aún no validado: " + strings.Join(unresolvedLabels, ", ") + "."
	}
	if len(missingLabels) > 0 {
		summary += " Falta: " + strings.Join(missingLabels, ", ") + "."
	}
	if len(blockers) > 0 {
		summary += fmt.Sprintf(" Además hay %d paso(s) previos del flujo que siguen bloqueando el envío.", len(blockers))
	}
	if len(notNeeded) > 0 {
		summary += fmt.Sprintf(" Se omitieron %d brechas de datos que no afectan esta operación.", len(notNeeded))
	}
	return summary
}

func flowPrerequisiteExecutionBlockers(timeline []flowRunStep) []map[string]interface{} {
	blockers := []map[string]interface{}{}
	seen := map[string]bool{}
	for _, step := range timeline {
		if !stepBlocksTerminalReadiness(step) {
			continue
		}
		key := strings.TrimSpace(step.Node)
		if key == "" {
			key = step.Framework + ":" + step.Capability
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		blocker := map[string]interface{}{
			"node":       step.Node,
			"framework":  step.Framework,
			"capability": step.Capability,
			"status":     step.Status,
		}
		if step.HumanSummary != "" {
			blocker["summary"] = step.HumanSummary
		} else if step.Error != "" {
			blocker["summary"] = step.Error
		}
		blockers = append(blockers, blocker)
	}
	return blockers
}

func stepBlocksTerminalReadiness(step flowRunStep) bool {
	switch step.Status {
	case "failed", "blocked", "needs_input", "awaiting_approval":
	default:
		return false
	}
	switch step.Role {
	case "branch", "cycle", "human":
		return false
	}
	if step.Framework == "simulacion" {
		return false
	}
	return true
}

// loadBusinessScopeColumn returns the scope_column for a given table from
// the business's scope_policies. This is used for entity-scoped filtering.
func loadBusinessScopeColumns(packPath string) map[string]string {
	if packPath == "" {
		return nil
	}
	raw, err := os.ReadFile(packPath)
	if err != nil {
		return nil
	}
	var doc struct {
		ScopePolicies struct {
			Tables map[string]struct {
				ScopeColumn string `json:"scope_column"`
			} `json:"tables"`
		} `json:"scope_policies"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	if len(doc.ScopePolicies.Tables) == 0 {
		return nil
	}
	result := make(map[string]string, len(doc.ScopePolicies.Tables))
	for table, policy := range doc.ScopePolicies.Tables {
		if policy.ScopeColumn != "" {
			result[table] = policy.ScopeColumn
		}
	}
	return result
}

// entityIDFromArtifacts extracts the entity ID from entity.ref.v1 artifact.
func entityIDFromArtifacts(artifacts map[string]flowRunArtifact) string {
	art, ok := artifacts["entity.ref.v1"]
	if !ok {
		return ""
	}
	payload, ok := art.Payload.(map[string]interface{})
	if !ok {
		return ""
	}
	return jsonFirstString(payload, "id", "entity_id", "ref", "code")
}

// entityRefFromArtifacts extracts the full entity ref map.
func entityRefFromArtifacts(artifacts map[string]flowRunArtifact) map[string]interface{} {
	art, ok := artifacts["entity.ref.v1"]
	if !ok {
		return nil
	}
	payload, ok := art.Payload.(map[string]interface{})
	if !ok {
		return nil
	}
	return payload
}
