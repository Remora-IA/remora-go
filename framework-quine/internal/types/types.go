// Package types define los tipos de frameworks y sus checklists de calidad.
package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ============================================================================
// TIPOS DE FRAMEWORK
// ============================================================================

// FrameworkType representa la clasificación de un framework.
type FrameworkType string

const (
	TypeInquisitivo   FrameworkType = "inquisitivo"   // Basado en preguntas/diálogos
	TypeNodosArbol    FrameworkType = "nodos-arbol"   // Usa nodos y árboles de conocimiento
	TypeProcesador    FrameworkType = "procesador"    // Procesa datos/archivos
	TypeIntegracion   FrameworkType = "integracion"   // Conecta sistemas/APIs
	TypeAutomatizador FrameworkType = "automatizador" // Automatiza tareas
	TypeGenerico      FrameworkType = "generico"      // Propósito general
)

// TypeMetadata contiene información sobre cada tipo.
type TypeMetadata struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Examples    []string `json:"examples"`
	Checklists  []string `json:"checklists"`
}

var TypeMetadataMap = map[FrameworkType]TypeMetadata{
	TypeInquisitivo: {
		Name:        "Inquisitivo",
		Description: "Frameworks que guían mediante preguntas y descubren información",
		Examples:    []string{"framework-echo"},
		Checklists: []string{
			"inquisitivo-base",
			"nodos-arbol",
			"comunicacion",
		},
	},
	TypeNodosArbol: {
		Name:        "Nodos y Árbol",
		Description: "Frameworks que usan estructuras de nodos jerárquicos",
		Examples:    []string{"framework-echo"},
		Checklists: []string{
			"nodos-arbol",
			"persistencia-json",
			"validacion-estado",
		},
	},
	TypeProcesador: {
		Name:        "Procesador",
		Description: "Frameworks que procesan, transforman o analizan datos",
		Examples:    []string{},
		Checklists: []string{
			"procesador-base",
			"manejador-errores",
		},
	},
	TypeIntegracion: {
		Name:        "Integración",
		Description: "Frameworks que conectan sistemas o APIs externas",
		Examples:    []string{},
		Checklists: []string{
			"integracion-base",
			"config-externa",
			"reintentos",
		},
	},
	TypeAutomatizador: {
		Name:        "Automatizador",
		Description: "Frameworks que automatizan tareas repetitivas",
		Examples:    []string{},
		Checklists: []string{
			"automatizador-base",
			"logging-ejecucion",
		},
	},
	TypeGenerico: {
		Name:        "Genérico",
		Description: "Frameworks de propósito general",
		Examples:    []string{},
		Checklists: []string{
			"base-comun",
		},
	},
}

// ============================================================================
// CHECKLISTS DE CALIDAD
// ============================================================================

// ChecklistItem representa un item individual en un checklist.
type ChecklistItem struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // required, recommended, optional
	Category    string `json:"category"`
}

// Checklist representa un checklist completo.
type Checklist struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Items       []ChecklistItem `json:"items"`
}

// AllChecklists mapa de todos los checklists.
var AllChecklists = map[string]Checklist{
	// ==========================================================================
	// COMÚN - Todos los frameworks deben cumplir
	// ==========================================================================
	"base-comun": {
		ID:          "base-comun",
		Name:        "Base Común",
		Description: "Items obligatorios para cualquier framework",
		Items: []ChecklistItem{
			{
				ID:          "INITIAL_PROMPT.md-exists",
				Description: "Existe INITIAL_PROMPT.md con instrucciones claras",
				Severity:    "required",
				Category:    "documentacion",
			},
			{
				ID:          "INITIAL_PROMPT.md-rol",
				Description: "INITIAL_PROMPT.md define el rol de la IA",
				Severity:    "required",
				Category:    "documentacion",
			},
			{
				ID:          "INITIAL_PROMPT.md-filosofia",
				Description: "INITIAL_PROMPT.md tiene sección de filosofía",
				Severity:    "required",
				Category:    "documentacion",
			},
			{
				ID:          "INITIAL_PROMPT.md-comandos",
				Description: "INITIAL_PROMPT.md lista los comandos disponibles",
				Severity:    "required",
				Category:    "documentacion",
			},
			{
				ID:          "AGENTS.md-exists",
				Description: "Existe AGENTS.md para integración con otros frameworks",
				Severity:    "required",
				Category:    "integracion",
			},
			{
				ID:          "README.md-exists",
				Description: "Existe README.md con documentación básica",
				Severity:    "required",
				Category:    "documentacion",
			},
			{
				ID:          "WHY.md-exists",
				Description: "Existe WHY.md con propósito, tipo y límite operativo del framework",
				Severity:    "required",
				Category:    "documentacion",
			},
			{
				ID:          "cmd-main-exists",
				Description: "Existe cmd/<nombre>/main.go",
				Severity:    "required",
				Category:    "estructura",
			},
			{
				ID:          "paladin-integrado",
				Description: "internal/paladin/ existe y tiene trace.go",
				Severity:    "required",
				Category:    "paladin",
			},
			{
				ID:          "go-mod-exists",
				Description: "Existe go.mod válido",
				Severity:    "required",
				Category:    "estructura",
			},
			{
				ID:          "no-hardcoded-paths",
				Description: "No hay rutas hardcodeadas de usuario",
				Severity:    "recommended",
				Category:    "codigo",
			},
		},
	},

	// ==========================================================================
	// INQUISITIVO - Frameworks basados en preguntas
	// ==========================================================================
	"inquisitivo-base": {
		ID:          "inquisitivo-base",
		Name:        "Base Inquisitivo",
		Description: "Items para frameworks que guían mediante preguntas",
		Items: []ChecklistItem{
			{
				ID:          "preguntas-guia",
				Description: "El CLI tiene comandos para hacer/preguntar y registrar respuestas",
				Severity:    "required",
				Category:    "inquisitivo",
			},
			{
				ID:          "log-qa",
				Description: "Existe comando para registrar Q&A (pregunta-respuesta-propósito)",
				Severity:    "required",
				Category:    "inquisitivo",
			},
			{
				ID:          "signal-fatiga",
				Description: "Existe mecanismo para registrar fatiga/confusión del usuario",
				Severity:    "recommended",
				Category:    "inquisitivo",
			},
			{
				ID:          "semáforo-decisión",
				Description: "Hay un mecanismo tipo 'readiness' para saber cuándo parar de preguntar",
				Severity:    "required",
				Category:    "inquisitivo",
			},
			{
				ID:          "no-ofrecer-temprano",
				Description: "Las reglas impiden ofrecer soluciones antes de confirmar dolor real",
				Severity:    "required",
				Category:    "inquisitivo",
			},
			{
				ID:          "una-pregunta-a-la-vez",
				Description: "Las instrucciones enfatizan hacer UNA pregunta a la vez",
				Severity:    "required",
				Category:    "inquisitivo",
			},
			{
				ID:          "percepciones-internas",
				Description: "Existe mecanismo para registrar percepciones internas (no solo hechos)",
				Severity:    "recommended",
				Category:    "inquisitivo",
			},
		},
	},

	// ==========================================================================
	// NODOS Y ÁRBOL - Frameworks con estructuras jerárquicas
	// ==========================================================================
	"nodos-arbol": {
		ID:          "nodos-arbol",
		Name:        "Nodos y Árbol",
		Description: "Items para frameworks que usan nodos y árboles",
		Items: []ChecklistItem{
			{
				ID:          "estructura-nodos",
				Description: "Existe estructura Node con: ID, Type, Title, Status, Parent",
				Severity:    "required",
				Category:    "nodos",
			},
			{
				ID:          "jerarquia-capas",
				Description: "Los nodos tienen layers/capas definidas (ej: AXIOM=0, THEORY=1...)",
				Severity:    "required",
				Category:    "nodos",
			},
			{
				ID:          "estados-nodo",
				Description: "Los nodos tienen estados: PENDING, VALIDATED, REJECTED",
				Severity:    "required",
				Category:    "nodos",
			},
			{
				ID:          "add-nodo-cmd",
				Description: "CLI tiene comandos add-<tipo> con validación de parent",
				Severity:    "required",
				Category:    "nodos",
			},
			{
				ID:          "validate-nodo-cmd",
				Description: "CLI tiene comando validate para marcar nodos como validados",
				Severity:    "required",
				Category:    "nodos",
			},
			{
				ID:          "show-tree-cmd",
				Description: "CLI tiene comando show-tree para visualizar el árbol",
				Severity:    "required",
				Category:    "nodos",
			},
			{
				ID:          "persistencia-json",
				Description: "El estado se persiste en JSON (no editable manualmente)",
				Severity:    "required",
				Category:    "nodos",
			},
			{
				ID:          "validacion-estado",
				Description: "No se puede crear nodo hijo sin parent validado",
				Severity:    "required",
				Category:    "nodos",
			},
			{
				ID:          "preguntas-pendientes",
				Description: "El sistema sugiere/genera preguntas automáticas por nodo",
				Severity:    "recommended",
				Category:    "nodos",
			},
		},
	},

	// ==========================================================================
	// COMUNICACIÓN
	// ==========================================================================
	"comunicacion": {
		ID:          "comunicacion",
		Name:        "Comunicación",
		Description: "Items para frameworks que se comunican con otros frameworks",
		Items: []ChecklistItem{
			{
				ID:          "loop-con-otro-framework",
				Description: "AGENTS.md indica cómo comunicarse con otros frameworks",
				Severity:    "required",
				Category:    "comunicacion",
			},
			{
				ID:          "comando-consulta",
				Description: "CLI tiene comando para consultar/invocar otro framework",
				Severity:    "required",
				Category:    "comunicacion",
			},
			{
				ID:          "parse-respuesta",
				Description: "Hay lógica para parsear la respuesta del otro framework",
				Severity:    "required",
				Category:    "comunicacion",
			},
			{
				ID:          "feedback-al-humano",
				Description: "El framework sabe devolver feedback al humano basado en respuesta de otro",
				Severity:    "recommended",
				Category:    "comunicacion",
			},
		},
	},

	// ==========================================================================
	// PERSISTENCIA Y JSON
	// ==========================================================================
	"persistencia-json": {
		ID:          "persistencia-json",
		Name:        "Persistencia JSON",
		Description: "Items para frameworks que persisten en JSON",
		Items: []ChecklistItem{
			{
				ID:          "archivo-json-nombrado",
				Description: "El archivo JSON se llama <nombreframework>.json",
				Severity:    "recommended",
				Category:    "persistencia",
			},
			{
				ID:          "no-editar-manual",
				Description: "INITIAL_PROMPT dice explícitamente NO editar JSON manualmente",
				Severity:    "required",
				Category:    "persistencia",
			},
			{
				ID:          "comando-init",
				Description: "CLI tiene comando init para inicializar proyecto",
				Severity:    "required",
				Category:    "persistencia",
			},
			{
				ID:          "LoadOrCreate",
				Description: "El código tiene función LoadOrCreate para no perder datos",
				Severity:    "required",
				Category:    "persistencia",
			},
			{
				ID:          "backup-auto",
				Description: "El sistema hace backup automático antes de sobrescribir",
				Severity:    "optional",
				Category:    "persistencia",
			},
		},
	},

	// ==========================================================================
	// PROCESADOR
	// ==========================================================================
	"procesador-base": {
		ID:          "procesador-base",
		Name:        "Base Procesador",
		Description: "Items para frameworks que procesan datos",
		Items: []ChecklistItem{
			{
				ID:          "comando-proccess",
				Description: "CLI tiene comando process con argumentos claros",
				Severity:    "required",
				Category:    "procesador",
			},
			{
				ID:          "input-validacion",
				Description: "El framework valida inputs antes de procesar",
				Severity:    "required",
				Category:    "procesador",
			},
			{
				ID:          "output-formato",
				Description: "El output tiene formato definido (JSON, texto, archivo)",
				Severity:    "required",
				Category:    "procesador",
			},
			{
				ID:          "dry-run",
				Description: "Existe modo dry-run para probar sin ejecutar efectos",
				Severity:    "recommended",
				Category:    "procesador",
			},
		},
	},

	// ==========================================================================
	// MANEJADOR DE ERRORES
	// ==========================================================================
	"manejador-errores": {
		ID:          "manejador-errores",
		Name:        "Manejo de Errores",
		Description: "Items para manejo robusto de errores",
		Items: []ChecklistItem{
			{
				ID:          "errores-descriptivos",
				Description: "Los errores son descriptivos (no solo 'error')",
				Severity:    "required",
				Category:    "errores",
			},
			{
				ID:          "exit-codes",
				Description: "CLI usa exit codes apropiados (0=ok, 1=error, 2=usage)",
				Severity:    "required",
				Category:    "errores",
			},
			{
				ID:          "panic-recovery",
				Description: "El código hace recover de panics en main",
				Severity:    "recommended",
				Category:    "errores",
			},
		},
	},

	// ==========================================================================
	// INTEGRACIÓN
	// ==========================================================================
	"integracion-base": {
		ID:          "integracion-base",
		Name:        "Base Integración",
		Description: "Items para frameworks de integración",
		Items: []ChecklistItem{
			{
				ID:          "config-externa",
				Description: "La configuración viene de archivo/env y no hardcodeada",
				Severity:    "required",
				Category:    "integracion",
			},
			{
				ID:          "auth-handling",
				Description: "Maneja autenticación de forma segura",
				Severity:    "required",
				Category:    "integracion",
			},
			{
				ID:          "reintentos",
				Description: "Tiene mecanismo de reintentos con backoff",
				Severity:    "recommended",
				Category:    "integracion",
			},
			{
				ID:          "timeout-config",
				Description: "Los timeouts son configurables",
				Severity:    "recommended",
				Category:    "integracion",
			},
		},
	},

	// ==========================================================================
	// AUTOMATIZADOR
	// ==========================================================================
	"automatizador-base": {
		ID:          "automatizador-base",
		Name:        "Base Automatizador",
		Description: "Items para frameworks de automatización",
		Items: []ChecklistItem{
			{
				ID:          "logging-ejecucion",
				Description: "Cada ejecución se loguea con timestamp",
				Severity:    "required",
				Category:    "automatizador",
			},
			{
				ID:          "undo-capability",
				Description: "Hay mecanismo para deshacer/revertir cambios",
				Severity:    "recommended",
				Category:    "automatizador",
			},
			{
				ID:          "idempotencia",
				Description: "Las operaciones son idempotentes",
				Severity:    "recommended",
				Category:    "automatizador",
			},
		},
	},

	// ==========================================================================
	// COMANDOS EJECUTABLES - El prompt debe poder convertirse a comandos
	// ==========================================================================
	"comandos-ejecutables": {
		ID:          "comandos-ejecutables",
		Name:        "Comandos Ejecutables",
		Description: "Verifica que las instrucciones del prompt puedan ejecutarse como comandos",
		Items: []ChecklistItem{
			{
				ID:          "prompt-comandos-lista",
				Description: "INITIAL_PROMPT.md lista explícitamente los comandos disponibles",
				Severity:    "required",
				Category:    "comandos-ejecutables",
			},
			{
				ID:          "prompt-comandos-implementados",
				Description: "Todos los comandos listados en el prompt están implementados en main.go",
				Severity:    "required",
				Category:    "comandos-ejecutables",
			},
			{
				ID:          "prompt-no-narrativa-obligatoria",
				Description: "Las reglas obligatorias no dependen de que la IA 'recuerde' narrativamente",
				Severity:    "required",
				Category:    "comandos-ejecutables",
			},
			{
				ID:          "prompt-sintaxis-ejecutable",
				Description: "Los comandos usan sintaxis ejecutable ./nombrecomando ...",
				Severity:    "required",
				Category:    "comandos-ejecutables",
			},
			{
				ID:          "prompt-no-editar-manual-json",
				Description: "El prompt prohíbe editar archivos JSON manualmente",
				Severity:    "required",
				Category:    "comandos-ejecutables",
			},
			{
				ID:          "prompt-instrucciones-cuantificadas",
				Description: "Las instrucciones tienen condiciones claras (si X, entonces ejecuta Y)",
				Severity:    "recommended",
				Category:    "comandos-ejecutables",
			},
			{
				ID:          "prompt-workflow-descomprimido",
				Description: "El flujo de trabajo está descrito como pasos ejecutables, no como reglas mentales",
				Severity:    "recommended",
				Category:    "comandos-ejecutables",
			},
		},
	},
}

// ============================================================================
// REPOSITORIO DE FRAMEWORKS
// ============================================================================

// FrameworkRegistry es el repositorio de frameworks conocidos.
type FrameworkRegistry struct {
	Version    string           `json:"version"`
	Updated    string           `json:"updated"`
	Frameworks []FrameworkEntry `json:"frameworks"`
}

// FrameworkEntry representa un framework registrado.
type FrameworkEntry struct {
	Name         string        `json:"name"`
	Type         FrameworkType `json:"type"`
	Path         string        `json:"path"`
	Role         string        `json:"role"`
	Description  string        `json:"description"`
	Created      string        `json:"created"`
	LastReview   string        `json:"last_review,omitempty"`
	QualityScore float64       `json:"quality_score,omitempty"`
}

// RegistryFilePath retorna la ruta del archivo de registro.
func RegistryFilePath() string {
	return filepath.Join(getQuineDir(), "frameworks.json")
}

// LoadRegistry carga el registro de frameworks.
func LoadRegistry() (*FrameworkRegistry, error) {
	path := RegistryFilePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FrameworkRegistry{
				Version:    "1.0",
				Updated:    "",
				Frameworks: []FrameworkEntry{},
			}, nil
		}
		return nil, err
	}

	var registry FrameworkRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, err
	}

	return &registry, nil
}

// SaveRegistry guarda el registro.
func SaveRegistry(r *FrameworkRegistry) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	dir := getQuineDir()
	os.MkdirAll(dir, 0755)

	return os.WriteFile(RegistryFilePath(), data, 0644)
}

// AddFramework agrega un framework al registro.
func (r *FrameworkRegistry) AddFramework(entry FrameworkEntry) error {
	// Verificar si ya existe
	for i, f := range r.Frameworks {
		if f.Name == entry.Name {
			r.Frameworks[i] = entry
			return nil
		}
	}

	r.Frameworks = append(r.Frameworks, entry)
	return nil
}

// GetFramework busca un framework por nombre.
func (r *FrameworkRegistry) GetFramework(name string) *FrameworkEntry {
	for _, f := range r.Frameworks {
		if f.Name == name {
			return &f
		}
	}
	return nil
}

// ============================================================================
// DETECCIÓN DE TIPO
// ============================================================================

// DetectFrameworkType detecta el tipo de un framework basándose en su estructura.
func DetectFrameworkType(basePath string) (FrameworkType, []string) {
	var indicators []string

	whyPath := filepath.Join(basePath, "WHY.md")
	if data, err := os.ReadFile(whyPath); err == nil {
		content := strings.ToLower(string(data))
		for _, fwType := range []FrameworkType{TypeInquisitivo, TypeNodosArbol, TypeProcesador, TypeIntegracion, TypeAutomatizador, TypeGenerico} {
			if strings.Contains(content, "tipo\n\n"+string(fwType)) || strings.Contains(content, "tipo: "+string(fwType)) {
				indicators = append(indicators, "why-type:"+string(fwType))
				return fwType, indicators
			}
		}
	}

	// Leer INITIAL_PROMPT para detectar tipo inquisitivo
	initialPromptPath := filepath.Join(basePath, "INITIAL_PROMPT.md")
	if data, err := os.ReadFile(initialPromptPath); err == nil {
		content := strings.ToLower(string(data))

		// Inquisitivo
		if strings.Contains(content, "pregunta") ||
			strings.Contains(content, "descubrir") ||
			strings.Contains(content, "entrevista") ||
			strings.Contains(content, "guía") ||
			strings.Contains(content, "validar") {
			indicators = append(indicators, "inquisitivo-keywords")
		}

		// Una pregunta a la vez
		if strings.Contains(content, "una pregunta") || strings.Contains(content, "una pregunta a la vez") {
			indicators = append(indicators, "una-pregunta-a-la-vez")
		}
	}

	// Verificar estructura de nodos
	internalPath := filepath.Join(basePath, "internal")
	if entries, err := os.ReadDir(internalPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && (entry.Name() == "tree" || entry.Name() == "nodes" || entry.Name() == "graph") {
				indicators = append(indicators, "nodos-dir:"+entry.Name())

				// Verificar archivo de nodos
				nodeFile := filepath.Join(internalPath, entry.Name(), "node.go")
				if _, err := os.Stat(nodeFile); err == nil {
					indicators = append(indicators, "nodos-estructura")
				}
			}
		}
	}

	// Verificar cmd/main.go para ver qué comandos hay
	mainPath := filepath.Join(basePath, "cmd")
	if entries, err := os.ReadDir(mainPath); err == nil {
		for _, entry := range entries {
			mainFile := filepath.Join(mainPath, entry.Name(), "main.go")
			if data, err := os.ReadFile(mainFile); err == nil {
				content := string(data)

				// Detectar comandos inquisitivos
				if strings.Contains(content, "add-axiom") ||
					strings.Contains(content, "add-theory") ||
					strings.Contains(content, "validate") ||
					strings.Contains(content, "show-tree") {
					indicators = append(indicators, "nodos-comandos")
				}

				// Detectar comandos de preguntas
				if strings.Contains(content, "log-qa") ||
					strings.Contains(content, "next-questions") ||
					strings.Contains(content, "readiness") {
					indicators = append(indicators, "inquisitivo-comandos")
				}
			}
		}
	}

	// Verificar JSON de estado (archivo JSON existe)
	jsonFiles, _ := filepath.Glob(basePath + "/*.json")
	if len(jsonFiles) > 0 {
		indicators = append(indicators, "persistencia-json")
	}

	// Decidir tipo basado en indicadores
	return decideType(indicators), indicators
}

func decideType(indicators []string) FrameworkType {
	hasNodos := false
	hasInquisitivo := false

	for _, ind := range indicators {
		if strings.HasPrefix(ind, "nodos") {
			hasNodos = true
		}
		if strings.HasPrefix(ind, "inquisitivo") {
			hasInquisitivo = true
		}
	}

	if hasNodos && hasInquisitivo {
		return TypeNodosArbol // Nodos-arbol incluye inquisitivo en este caso
	}
	if hasInquisitivo {
		return TypeInquisitivo
	}
	if hasNodos {
		return TypeNodosArbol
	}

	return TypeGenerico
}

// ============================================================================
// TAXONOMÍA SEMÁNTICA DE COMANDOS
// ============================================================================

// CommandTaxonomyCategory representa una categoría semántica de comandos.
type CommandTaxonomyCategory struct {
	Name        string
	Description string
	Verbs       []string // Verbos que la identifican
	Patterns    []string // Patrones de nombre de comando
}

// CommandTaxonomy mapa de todas las categorías semánticas.
var CommandTaxonomy = map[string]CommandTaxonomyCategory{
	"descubrimiento": {
		Name:        "Descubrimiento",
		Description: "Comandos para explorar y capturar información del mundo real",
		Verbs:       []string{"ask", "question", "probe", "explore", "discover", "add", "create", "capture"},
		Patterns:    []string{"add-", "ask-", "question", "probe", "explore"},
	},
	"validacion": {
		Name:        "Validación",
		Description: "Comandos para verificar, confirmar o evaluar información",
		Verbs:       []string{"validate", "confirm", "verify", "check", "test", "assess", "evaluate", "ready"},
		Patterns:    []string{"validate", "confirm", "verify", "check", "test", "readiness", "assess"},
	},
	"transformacion": {
		Name:        "Transformación",
		Description: "Comandos para procesar, compilar o transformar datos",
		Verbs:       []string{"compile", "process", "transform", "convert", "parse", "normalize", "extract"},
		Patterns:    []string{"compile", "process", "transform", "convert", "parse", "spec", "normalize"},
	},
	"generacion": {
		Name:        "Generación",
		Description: "Comandos para crear o generar artefactos de código",
		Verbs:       []string{"generate", "create", "build", "make", "scaffold", "init", "setup"},
		Patterns:    []string{"generate", "create", "build", "make", "scaffold", "init", "setup"},
	},
	"comunicacion": {
		Name:        "Comunicación",
		Description: "Comandos para invocar o comunicarse con otros frameworks",
		Verbs:       []string{"invoke", "call", "send", "query", "request", "inspect", "parse", "connect", "login", "auth"},
		Patterns:    []string{"invoke", "call", "send", "query", "inspect", "parse-", "connect", "login", "auth"},
	},
	"estado": {
		Name:        "Estado",
		Description: "Comandos para consultar o mostrar el estado actual",
		Verbs:       []string{"status", "state", "health", "info", "show", "list", "get", "view"},
		Patterns:    []string{"status", "state", "health", "info", "show", "list", "get", "view", "tree"},
	},
	"registro": {
		Name:        "Registro",
		Description: "Comandos para registrar información, señales o percepciones",
		Verbs:       []string{"log", "track", "signal", "note", "register", "append", "configure", "config"},
		Patterns:    []string{"log", "track", "signal", "note", "register", "configure", "config", "credential", "perception", "qa"},
	},
	"modificacion": {
		Name:        "Modificación",
		Description: "Comandos para editar, actualizar o modificar estado",
		Verbs:       []string{"edit", "update", "modify", "set", "config", "reject", "select", "confidence"},
		Patterns:    []string{"edit", "update", "modify", "set", "config", "reject", "select", "confidence"},
	},
}

// TypeCategoryRequirements define qué categorías debe tener cada tipo de framework.
var TypeCategoryRequirements = map[FrameworkType][]string{
	TypeInquisitivo: {
		"descubrimiento", "validacion", "estado", "registro",
	},
	TypeNodosArbol: {
		"descubrimiento", "validacion", "estado", "modificacion",
	},
	TypeProcesador: {
		"transformacion", "validacion", "estado",
	},
	TypeIntegracion: {
		"comunicacion", "validacion", "estado",
	},
	TypeAutomatizador: {
		"generacion", "estado", "registro",
	},
	TypeGenerico: {
		"estado", // Mínimo obligatorio
	},
}

// CommandAnalysis contiene el resultado del análisis semántico de comandos.
type CommandAnalysis struct {
	FrameworkPath string              `json:"framework_path"`
	FrameworkName string              `json:"framework_name"`
	DetectedType  FrameworkType       `json:"detected_type"`
	Commands      []string            `json:"commands"`
	Categories    map[string][]string `json:"categories"`
	Required      []string            `json:"required_categories"`
	Present       []string            `json:"present_categories"`
	Missing       []string            `json:"missing_categories"`
	Unclassified  []string            `json:"unclassified_commands"`
	Score         float64             `json:"score"`
	IsCoherent    bool                `json:"is_coherent"`
}

// AnalyzeCommands clasifica semánticamente los comandos de un framework.
func AnalyzeCommands(frameworkPath string) (*CommandAnalysis, error) {
	// Detectar tipo
	detectedType, _ := DetectFrameworkType(frameworkPath)

	// Obtener nombre
	fwName := filepath.Base(frameworkPath)

	// Leer main.go
	mainPath := findMainGoInPath(frameworkPath)
	if mainPath == "" {
		return nil, fmt.Errorf("no se encontró main.go")
	}

	data, err := os.ReadFile(mainPath)
	if err != nil {
		return nil, err
	}
	content := string(data)

	// Extraer comandos del código
	casePattern := regexp.MustCompile(`case\s+"([a-z][a-z][a-z0-9-]*)"`)
	commands := []string{}
	seen := make(map[string]bool)
	for _, match := range casePattern.FindAllStringSubmatch(content, -1) {
		cmd := match[1]
		if len(cmd) > 3 && !seen[cmd] {
			seen[cmd] = true
			commands = append(commands, cmd)
		}
	}

	// Clasificar comandos por categoría
	categories := make(map[string][]string)
	unclassified := []string{}
	for _, cmd := range commands {
		cat := ClassifyCommand(cmd)
		if cat != "" {
			categories[cat] = append(categories[cat], cmd)
		} else {
			unclassified = append(unclassified, cmd)
		}
	}

	// Obtener requisitos para el tipo
	required := TypeCategoryRequirements[detectedType]

	// Verificar cuáles están presentes
	present := []string{}
	missing := []string{}
	for _, req := range required {
		if cmds, ok := categories[req]; ok && len(cmds) > 0 {
			present = append(present, req)
		} else {
			missing = append(missing, req)
		}
	}

	// Calcular score
	score := 0.0
	if len(required) > 0 {
		score = float64(len(present)) / float64(len(required)) * 100
	}

	return &CommandAnalysis{
		FrameworkPath: frameworkPath,
		FrameworkName: fwName,
		DetectedType:  detectedType,
		Commands:      commands,
		Categories:    categories,
		Required:      required,
		Present:       present,
		Missing:       missing,
		Unclassified:  unclassified,
		Score:         score,
		IsCoherent:    score >= 80,
	}, nil
}

// ClassifyCommand clasifica un comando en una categoría semántica.
func ClassifyCommand(cmd string) string {
	cmdLower := strings.ToLower(cmd)

	for catID, cat := range CommandTaxonomy {
		// Verificar patrones primero
		for _, pattern := range cat.Patterns {
			if strings.Contains(cmdLower, pattern) {
				return catID
			}
		}
		// Verificar verbos
		for _, verb := range cat.Verbs {
			if cmdLower == verb || strings.HasPrefix(cmdLower, verb+"-") || strings.HasPrefix(cmdLower, verb+"_") {
				return catID
			}
		}
	}

	return ""
}

// findMainGoInPath encuentra el main.go en un framework.
func findMainGoInPath(basePath string) string {
	cmdDir := filepath.Join(basePath, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return ""
	}

	// Preferir directorio sin guiones (frameworkecho vs framework-echo)
	for _, entry := range entries {
		if entry.IsDir() && !strings.Contains(entry.Name(), "-") {
			mainPath := filepath.Join(cmdDir, entry.Name(), "main.go")
			if _, err := os.Stat(mainPath); err == nil {
				return mainPath
			}
		}
	}
	// Luego el resto
	for _, entry := range entries {
		if entry.IsDir() {
			mainPath := filepath.Join(cmdDir, entry.Name(), "main.go")
			if _, err := os.Stat(mainPath); err == nil {
				return mainPath
			}
		}
	}

	return ""
}

// ============================================================================
// UTILIDADES
// ============================================================================

func getQuineDir() string {
	return "/Users/alcless_a1234_cursor/remora-go/framework-quine"
}
