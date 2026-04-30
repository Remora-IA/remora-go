package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"channel/manifest"
	"remora-flujo/internal/llm"
)

// loadFrameworkManifest lee el framework.manifest.json del framework dado.
// Convención: cada framework vive en /Users/alcless_a1234_cursor/remora-go/framework-<name>.
func loadFrameworkManifest(name string) (*manifest.Manifest, error) {
	path := filepath.Join("/Users/alcless_a1234_cursor/remora-go", "framework-"+name, "framework.manifest.json")
	return manifest.Load(path)
}

// modelSpecFor devuelve el spec efectivo del framework para esta conversación.
// Prioridad: conv.Models[fw] (override) > manifest.Model. Si conv.Models[fw]
// solo tiene el nombre, mantiene provider/env del manifest.
func modelSpecFor(conv *Conversation, frameworkName string) (llm.Spec, error) {
	man, err := loadFrameworkManifest(frameworkName)
	if err != nil {
		return llm.Spec{}, err
	}
	spec := llm.Spec{
		Provider:     man.Model.Provider,
		Name:         man.Model.Name,
		EnvKey:       man.Model.EnvKey,
		Capabilities: man.Model.Capabilities,
		BaseURL:      man.Model.BaseURL,
	}
	if override, ok := conv.Models[frameworkName]; ok && override != "" {
		spec.Name = override
	}
	return spec, nil
}

// preprocessVision toma el mensaje del usuario (texto + recursos imagen) y
// lo pasa por el modelo multimodal del framework de destino. Devuelve un
// "answer enriquecido": el texto original + la descripción estructurada de
// las imágenes en formato JSON.
//
// Persistencia (efecto MERE-like): el análisis crudo se guarda en
//   framework-alfa/temp/alfa_spec_api_<conv>.image_analysis.json
// como un append-only log, así queda disponible para Alfa, Bravo y futuras
// queries sin tener que re-llamar al modelo.
func preprocessVision(ctx context.Context, conv *Conversation, targetFramework string, userText string, resources []MessageResource) (string, error) {
	imgs := []llm.Resource{}
	for _, r := range resources {
		if r.Type == "image" {
			imgs = append(imgs, llm.Resource{
				Type:     r.Type,
				Path:     r.Path,
				MimeType: r.MimeType,
				Name:     r.Name,
			})
		}
	}
	if len(imgs) == 0 {
		return userText, nil
	}

	spec, err := modelSpecFor(conv, targetFramework)
	if err != nil {
		return userText, fmt.Errorf("no pude resolver model spec: %w", err)
	}
	if !spec.HasCapability("multimodal") {
		return userText, fmt.Errorf("framework %s no declara capability multimodal en su manifest", targetFramework)
	}

	client, err := llm.New(spec)
	if err != nil {
		return userText, err
	}

	system := strings.Join([]string{
		"Eres un analista visual al servicio de un orquestador de frameworks.",
		"Tu output va a ser consumido por otro framework (",
		targetFramework,
		") como evidencia, NO por un humano.",
		"Devuelve SIEMPRE un único JSON válido con esta forma exacta:",
		`{"resource_kind":"...","entities":[...],"columns":[...],"sample_rows":[...],"observations":"..."}`,
		"resource_kind = qué es (ej: 'planilla_excel', 'pantalla_app', 'foto_documento').",
		"entities = lista de conceptos identificados.",
		"columns = nombres de columna si es una tabla, [] si no.",
		"sample_rows = hasta 3 ejemplos de filas con sus valores.",
		"observations = texto libre con cualquier dato relevante para automatización.",
		"NO incluyas comentarios fuera del JSON. NO uses markdown.",
	}, " ")

	userPrompt := "Contexto del usuario: " + userText + "\n\nAnaliza las imágenes adjuntas."
	out, err := client.Complete(ctx, llm.CompletionRequest{
		System:    system,
		User:      userPrompt,
		Resources: imgs,
		MaxTokens: 1200,
	})
	if err != nil {
		return userText, err
	}

	// Limpiar code fences si el modelo los agregó pese al system.
	cleaned := strings.TrimSpace(out)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	// Persistir análisis crudo (append).
	if err := appendImageAnalysis(conv, targetFramework, userText, imgs, cleaned); err != nil {
		fmt.Fprintf(os.Stderr, "[flujo_api] warn: no pude persistir image_analysis: %v\n", err)
	}

	// Nota: Channel valida args y rechaza newlines por axioma de seguridad
	// (path safety). El answer cruza Channel como argumento, así que
	// aplastamos newlines a espacios. El JSON sobrevive (es válido inline).
	flatCleaned := strings.ReplaceAll(strings.ReplaceAll(cleaned, "\r", " "), "\n", " ")
	flatUser := strings.ReplaceAll(strings.ReplaceAll(userText, "\r", " "), "\n", " ")
	enriched := flatUser + " [análisis_visual_" + targetFramework + "] " + flatCleaned
	return enriched, nil
}

// appendImageAnalysis persiste cada análisis multimodal como append a un
// archivo log por conversación. Esto sobrevive a recompiles del spec.
func appendImageAnalysis(conv *Conversation, framework, userText string, imgs []llm.Resource, raw string) error {
	dir := "/Users/alcless_a1234_cursor/remora-go/framework-" + framework + "/temp"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "alfa_spec_api_"+conv.ID+".image_analysis.json")

	type entry struct {
		Timestamp time.Time     `json:"ts"`
		Framework string        `json:"framework"`
		UserText  string        `json:"user_text"`
		Resources []llm.Resource `json:"resources"`
		RawJSON   json.RawMessage `json:"analysis"`
	}

	all := []entry{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &all)
	}

	var raw_msg json.RawMessage
	if json.Valid([]byte(raw)) {
		raw_msg = json.RawMessage(raw)
	} else {
		raw_msg = json.RawMessage(`{"_raw":` + jsonString(raw) + `}`)
	}
	all = append(all, entry{
		Timestamp: time.Now(),
		Framework: framework,
		UserText:  userText,
		Resources: imgs,
		RawJSON:   raw_msg,
	})

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
