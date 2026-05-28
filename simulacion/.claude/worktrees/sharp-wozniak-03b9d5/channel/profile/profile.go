// Package profile implementa el sistema de "Forma Genérica" + "Forma <nombre>".
//
// Cada framework carga su perfil al inicio via REMORA_PROFILE env var.
// El perfil inyecta vocabulario, reglas de negocio y tono sin modificar
// el código genérico del framework.
//
// Convención de archivos:
//   profiles/
//     generic/              # fallback vacío
//     cobranza-chile/
//       profile.json        # metadata
//       glossary.md         # vocabulario compartido
//       sabio.md            # overlay para Sabio
//       foco.md             # criterios de priorización
//       mecanico.md         # templates de acción
//       flow.rules.json     # reglas de orquestación
//       views/              # hints de UI generativa
//
package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Profile representa un perfil de negocio completo.
type Profile struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Frameworks  []string          `json:"frameworks"` // cuáles frameworks activa
	Glossary    string            // contenido de glossary.md
	Overlays    map[string]string // framework -> contenido overlay
	Views       map[string][]byte // tipo -> JSON schema/hint de UI
	FlowRules   []byte            // contenido de flow.rules.json
}

// Loader carga perfiles desde el filesystem.
type Loader struct {
	BasePath string
}

// NewLoader crea un loader. Si basePath está vacío, usa ./profiles.
func NewLoader(basePath string) *Loader {
	if basePath == "" {
		basePath = "profiles"
	}
	return &Loader{BasePath: basePath}
}

// Load carga el perfil solicitado por la env var REMORA_PROFILE.
// Si no está seteada o el perfil no existe, devuelve el perfil "generic" (vacío).
func (l *Loader) Load() (*Profile, error) {
	name := os.Getenv("REMORA_PROFILE")
	if name == "" {
		name = "generic"
	}
	return l.LoadNamed(name)
}

// LoadNamed carga un perfil específico por nombre.
func (l *Loader) LoadNamed(name string) (*Profile, error) {
	dir := filepath.Join(l.BasePath, name)
	
	// Si no existe, fallback a generic
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if name == "generic" {
			return &Profile{Name: "generic", Overlays: map[string]string{}}, nil
		}
		return l.LoadNamed("generic")
	}

	p := &Profile{
		Name:     name,
		Overlays: map[string]string{},
		Views:    map[string][]byte{},
	}

	// Metadata
	metaPath := filepath.Join(dir, "profile.json")
	if data, err := os.ReadFile(metaPath); err == nil {
		_ = json.Unmarshal(data, p) // best effort
	}

	// Glosario
	glossaryPath := filepath.Join(dir, "glossary.md")
	if data, err := os.ReadFile(glossaryPath); err == nil {
		p.Glossary = string(data)
	}

	// Overlays por framework
	frameworks := []string{"sabio", "foco", "mecanico", "bravo", "auditor", "echo", "paladin", "alfa", "quine", "gmail", "indexa"}
	for _, fw := range frameworks {
		overlayPath := filepath.Join(dir, fw+".md")
		if data, err := os.ReadFile(overlayPath); err == nil {
			p.Overlays[fw] = string(data)
		}
	}

	// Flow rules
	flowPath := filepath.Join(dir, "flow.rules.json")
	if data, err := os.ReadFile(flowPath); err == nil {
		p.FlowRules = data
	}

	// Views (UI hints)
	viewsDir := filepath.Join(dir, "views")
	if entries, err := os.ReadDir(viewsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(viewsDir, e.Name()))
			viewType := strings.TrimSuffix(e.Name(), ".json")
			p.Views[viewType] = data
		}
	}

	return p, nil
}

// OverlayFor devuelve el overlay de un framework, o vacío si no existe.
func (p *Profile) OverlayFor(framework string) string {
	return p.Overlays[framework]
}

// SystemPromptWithOverlay arma el system prompt final:
// base (genérico) + glosario + overlay específico.
func (p *Profile) SystemPromptWithOverlay(baseSystem, framework string) string {
	parts := []string{baseSystem}
	
	if p.Glossary != "" {
		parts = append(parts, "\n--- VOCABULARIO DEL NEGOCIO ---\n"+p.Glossary)
	}
	
	if overlay := p.OverlayFor(framework); overlay != "" {
		parts = append(parts, "\n--- ROL Y REGLAS ESPECÍFICAS ---\n"+overlay)
	}
	
	return strings.Join(parts, "\n")
}

// Active checks if this profile activates a specific framework.
func (p *Profile) Active(framework string) bool {
	for _, f := range p.Frameworks {
		if f == framework {
			return true
		}
	}
	return false
}
