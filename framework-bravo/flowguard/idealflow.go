package flowguard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type IdealFlow struct {
	TraceID       string `json:"trace_id"`
	Generated     string `json:"generated"`
	Description   string `json:"description"`
	Verbalization string `json:"verbalization"` // Texto completo tal como lo explica el humano
	Intent        string `json:"intent,omitempty"`

	Rules        []Rule   `json:"rules,omitempty"`
	CriticalVars []string `json:"critical_vars,omitempty"`
	CriticalPath []string `json:"critical_path,omitempty"`
}

type Rule struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	When        string `json:"when,omitempty"`
	Then        string `json:"then"`
	Importance  int    `json:"importance,omitempty"` // 1=crítico, 2=importante, 3=deseable
}

func NewIdealFlow(description string) *IdealFlow {
	return &IdealFlow{
		TraceID:       fmt.Sprintf("ideal_%d", time.Now().UnixNano()),
		Generated:     time.Now().Format(time.RFC3339),
		Description:   description,
		Verbalization: "",
		Intent:        "",
		Rules:         []Rule{},
		CriticalVars:  []string{},
		CriticalPath:  []string{},
	}
}

func (i *IdealFlow) SetVerbalization(text string) *IdealFlow {
	i.Verbalization = text
	return i
}

func (i *IdealFlow) SetIntent(intent string) *IdealFlow {
	i.Intent = intent
	return i
}

func (i *IdealFlow) AddRule(name, description, then string) *IdealFlow {
	i.Rules = append(i.Rules, Rule{
		Name:        name,
		Description: description,
		Then:        then,
		Importance:  1,
	})
	return i
}

func (i *IdealFlow) AddRuleWithWhen(name, description, when, then string) *IdealFlow {
	i.Rules = append(i.Rules, Rule{
		Name:        name,
		Description: description,
		When:        when,
		Then:        then,
		Importance:  1,
	})
	return i
}

func (i *IdealFlow) AddCriticalVar(varName string) *IdealFlow {
	i.CriticalVars = append(i.CriticalVars, varName)
	return i
}

func (i *IdealFlow) SetCriticalPath(path ...string) *IdealFlow {
	i.CriticalPath = path
	return i
}

// Save guarda el JSON y también genera IDEAL_FLOW.md para lectura humana
// Los archivos se guardan en baseDir/temp/ (crea la carpeta si no existe)
func (i *IdealFlow) Save(baseDir string) error {
	// Determinar directorio para archivos temporales
	if baseDir == "" {
		baseDir, _ = os.Getwd()
	}
	tempDir := filepath.Join(baseDir, "temp")

	// Crear carpeta temp si no existe
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("no se pudo crear carpeta temp: %w", err)
	}

	// Guardar JSON
	jsonPath := filepath.Join(tempDir, "ideal_flow.json")
	data, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return err
	}

	// Generar markdown legible
	mdPath := filepath.Join(tempDir, "IDEAL_FLOW.md")
	if err := i.generateMarkdownToFile(mdPath); err != nil {
		fmt.Printf("[FLOWGUARD] Warning: no se pudo generar IDEAL_FLOW.md: %v\n", err)
	}

	fmt.Printf("[FLOWGUARD] IdealFlow guardado → %s\n", jsonPath)
	return nil
}

// generateMarkdownToFile genera el markdown y lo guarda en el path especificado
func (i *IdealFlow) generateMarkdownToFile(mdPath string) error {
	md := fmt.Sprintf("# IDEAL FLOW - %s\n\n", i.Description)
	md += fmt.Sprintf("**Generated:** %s\n\n", i.Generated)
	md += "## Verbalización del CTO\n\n"
	md += i.Verbalization + "\n\n"

	if i.Intent != "" {
		md += "## Intent\n\n" + i.Intent + "\n\n"
	}

	md += "## Reglas de Negocio\n\n"
	for _, r := range i.Rules {
		md += fmt.Sprintf("### %s (Importancia: %d)\n", r.Name, r.Importance)
		md += r.Description + "\n\n"
		if r.When != "" {
			md += "**Cuando:** " + r.When + "\n"
		}
		md += "**Entonces:** " + r.Then + "\n\n"
	}

	if len(i.CriticalVars) > 0 {
		md += "## Variables Críticas\n\n"
		for _, v := range i.CriticalVars {
			md += "- `" + v + "`\n"
		}
		md += "\n"
	}

	if len(i.CriticalPath) > 0 {
		md += "## Critical Path Esperado\n\n"
		for idx, step := range i.CriticalPath {
			md += fmt.Sprintf("%d. `%s`\n", idx+1, step)
		}
	}

	return os.WriteFile(mdPath, []byte(md), 0644)
}

// generateMarkdown genera el markdown y lo guarda en "IDEAL_FLOW.md" (compatibilidad hacia atrás)
func (i *IdealFlow) generateMarkdown() error {
	baseDir, _ := os.Getwd()
	return i.generateMarkdownToFile(filepath.Join(baseDir, "IDEAL_FLOW.md"))
}

// LoadIdealFlow carga el ideal_flow.json si existe
// Busca primero en temp/ del directorio actual, luego en padres
func LoadIdealFlow() (*IdealFlow, error) {
	// Buscar desde cwd hacia arriba
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	path := findIdealFlowPath(cwd)
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var flow IdealFlow
	if err := json.Unmarshal(data, &flow); err != nil {
		return nil, err
	}

	fmt.Printf("[FLOWGUARD] IdealFlow cargado (%d reglas, %d vars críticas) → %s\n",
		len(flow.Rules), len(flow.CriticalVars), path)
	return &flow, nil
}

// findIdealFlowPath busca ideal_flow.json en temp/ desde startDir hacia arriba
func findIdealFlowPath(startDir string) string {
	dir := startDir
	for i := 0; i < 5; i++ {
		path := filepath.Join(dir, "temp", "ideal_flow.json")
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
