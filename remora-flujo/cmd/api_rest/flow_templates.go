package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func materializeTemplateInstantiation(manifest *flowManifest) *flowManifest {
	if manifest == nil {
		return &flowManifest{Nodes: []flowNode{}}
	}
	out := cloneFlowManifest(*manifest)
	if out.Provenance.Template {
		templateID := strings.TrimSpace(out.Provenance.TemplateID)
		if templateID == "" {
			templateID = strings.TrimSpace(out.ID)
		}
		out.Provenance.Source = "template_instantiation"
		out.Provenance.Template = false
		out.Provenance.TemplateID = templateID
	}
	return &out
}

func (fs *flowStore) createFlowTemplate(name, description, businessID string, manifest *flowManifest) (*storedFlowTemplate, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name es requerido")
	}
	businessID = strings.TrimSpace(businessID)
	if businessID == "" {
		return nil, fmt.Errorf("business_id es requerido")
	}
	authored := cloneFlowManifest(*manifest)
	authored.BusinessID = businessID
	if strings.TrimSpace(authored.ID) == "" {
		authored.ID = defaultBusinessFlowManifestID()
	}
	stripFlowDerivedState(&authored)
	manifestRaw, err := json.Marshal(&authored)
	if err != nil {
		return nil, fmt.Errorf("no se pudo serializar template: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	id := fmt.Sprintf("flt_%d", time.Now().UnixNano())
	_, err = fs.db.Exec(
		`INSERT INTO flow_templates (id, business_id, name, description, manifest_json, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 'available', ?, ?)`,
		id, businessID, name, description, string(manifestRaw), now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear flow template: %w", err)
	}
	return &storedFlowTemplate{
		ID:           id,
		BusinessID:   businessID,
		Name:         name,
		Description:  description,
		ManifestJSON: string(manifestRaw),
		Status:       "available",
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (fs *flowStore) listFlowTemplatesByBusiness(businessID string) ([]flowTemplateWithManifest, error) {
	rows, err := fs.db.Query(
		`SELECT id, business_id, name, description, manifest_json, status, created_at, updated_at
		 FROM flow_templates WHERE business_id = ? ORDER BY updated_at DESC`, businessID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []flowTemplateWithManifest
	for rows.Next() {
		var tpl storedFlowTemplate
		if err := rows.Scan(&tpl.ID, &tpl.BusinessID, &tpl.Name, &tpl.Description, &tpl.ManifestJSON, &tpl.Status, &tpl.CreatedAt, &tpl.UpdatedAt); err != nil {
			return nil, err
		}
		item := flowTemplateWithManifest{storedFlowTemplate: tpl}
		var manifest flowManifest
		if json.Unmarshal([]byte(tpl.ManifestJSON), &manifest) == nil {
			item.Manifest = &manifest
		}
		out = append(out, item)
	}
	if out == nil {
		out = []flowTemplateWithManifest{}
	}
	return out, nil
}

func (s *server) handleListFlowTemplates(w http.ResponseWriter, r *http.Request) {
	businessID := muxVar(r, "business_id")
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}
	templates, err := s.flows.listFlowTemplatesByBusiness(businessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "no se pudieron listar flow templates: "+err.Error())
		return
	}
	writeOK(w, templates)
}
