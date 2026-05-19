package main

import "strings"

const (
	defaultBusinessFlowName        = "Cobranza asistida con análisis"
	defaultBusinessFlowDescription = "Plantilla base para priorizar cobranza, revisar contexto del cliente y preparar el envío del mensaje."
)

func defaultBusinessFlowManifest() *flowManifest {
	return &flowManifest{
		ID: "flow_cobranza_asistida_con_anlisis",
		Provenance: flowProvenance{
			Source:     "system_default_proposal",
			Template:   true,
			TemplateID: "default_business_collection",
		},
		Policies: []string{"trace_required"},
		Nodes: []flowNode{
			{ID: "node_1_analysis_configure", Framework: "radar", Capability: "analysis.configure", Role: flowRoleBootstrap},
			{ID: "node_3_collection_priority_list", Framework: "radar", Capability: "collection.priority_list", Role: flowRoleBootstrap},
			{ID: "node_4_focus_next_collection_task", Framework: "foco", Capability: "focus.next_collection_task", Role: flowRoleEntry},
			{ID: "node_5_data_entity_360", Framework: "sabio", Capability: "data.entity_360", Role: flowRolePipeline},
			{ID: "node_6_message_draft_collection_ema", Framework: "mecanico", Capability: "message.draft.collection_email", Role: flowRolePipeline},
			{ID: "node_7_data_quality_audit", Framework: "auditor", Capability: "data.quality.audit", Role: flowRolePipeline},
			{ID: "node_2_credentials_smtp_check", Framework: "hosting", Capability: "credentials.smtp.check", Role: flowRolePipeline},
			{ID: "node_8_message_send", Framework: "mensajero", Capability: "message.send", Role: flowRolePipeline},
		},
		Edges: []flowEdge{
			{From: "node_1_analysis_configure", To: "node_3_collection_priority_list"},
			{From: "node_3_collection_priority_list", To: "node_4_focus_next_collection_task"},
			{From: "node_4_focus_next_collection_task", To: "node_5_data_entity_360"},
			{From: "node_5_data_entity_360", To: "node_6_message_draft_collection_ema"},
			{From: "node_6_message_draft_collection_ema", To: "node_7_data_quality_audit"},
			{From: "node_7_data_quality_audit", To: "node_2_credentials_smtp_check"},
			{From: "node_2_credentials_smtp_check", To: "node_8_message_send"},
		},
	}
}

func defaultBusinessFlowManifestID() string {
	return "flow_" + flowSafeIDStr(defaultBusinessFlowName)
}

func (s *server) ensureDefaultBusinessAssets(business authBusiness) error {
	if strings.TrimSpace(business.ID) == "" || s == nil || s.flows == nil {
		return nil
	}
	templates, err := s.flows.listFlowTemplatesByBusiness(business.ID)
	if err != nil {
		return err
	}
	for _, template := range templates {
		if template.Name == defaultBusinessFlowName {
			return nil
		}
		if template.Manifest != nil && template.Manifest.ID == defaultBusinessFlowManifestID() {
			return nil
		}
	}
	_, err = s.flows.createFlowTemplate(defaultBusinessFlowName, defaultBusinessFlowDescription, business.ID, defaultBusinessFlowManifest())
	return err
}

func (s *server) ensureDefaultBusinessAssetsForAllBusinesses() error {
	if s == nil || s.auth == nil {
		return nil
	}
	businesses, err := s.auth.listBusinesses()
	if err != nil {
		return err
	}
	for _, business := range businesses {
		if err := s.ensureDefaultBusinessAssets(business); err != nil {
			return err
		}
	}
	return nil
}
