// Package review implementa la revisión de calidad de frameworks.
package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"channel/manifest"
	"framework-quine/internal/paladin"
	"framework-quine/internal/types"
)

// Result contiene el resultado completo de una revisión.
type Result struct {
	FrameworkName   string            `json:"framework_name"`
	FrameworkPath   string            `json:"framework_path"`
	DetectedType    string            `json:"detected_type"`
	Checklists      []ChecklistResult `json:"checklists"`
	TotalItems      int               `json:"total_items"`
	Passed          int               `json:"passed"`
	Failed          int               `json:"failed"`
	Warnings        int               `json:"warnings"`
	Optional        int               `json:"optional"`
	Score           float64           `json:"score"`
	Recommendations []Recommendation  `json:"recommendations,omitempty"`
	CanBeRegistered bool              `json:"can_be_registered"`
}

// ChecklistResult contiene el resultado de un checklist específico.
type ChecklistResult struct {
	ChecklistID   string       `json:"checklist_id"`
	ChecklistName string       `json:"checklist_name"`
	Items         []ItemResult `json:"items"`
	Passed        int          `json:"passed"`
	Failed        int          `json:"failed"`
	Warnings      int          `json:"warnings"`
	Optional      int          `json:"optional"`
}

// ItemResult representa el resultado de un item individual.
type ItemResult struct {
	ItemID      string `json:"item_id"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Status      string `json:"status"` // pass, fail, warning, skip
	Reason      string `json:"reason,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// Recommendation es una recomendación para mejorar el framework.
type Recommendation struct {
	Priority   string `json:"priority"`
	ItemID     string `json:"item_id"`
	Category   string `json:"category"`
	Problem    string `json:"problem"`
	Suggestion string `json:"suggestion"`
}

// Review evalúa un framework y genera un reporte de calidad.
func Review(frameworkPath string) (*Result, error) {
	trace := paladin.NewTrace("review")
	ctx := trace.Start()
	defer trace.Flush()

	ctx.Var("frameworkPath", frameworkPath)

	// Validar que existe
	if _, err := os.Stat(frameworkPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("el directorio no existe: %s", frameworkPath)
	}

	// Obtener nombre del framework
	frameworkName := filepath.Base(frameworkPath)
	ctx.Var("frameworkName", frameworkName)

	// Detectar tipo
	detectedType, indicators := types.DetectFrameworkType(frameworkPath)
	ctx.Var("detectedType", string(detectedType))
	ctx.Var("indicators", indicators)

	// Obtener checklists a aplicar según tipo
	checklistsToApply := getChecklistsForType(detectedType)

	// Ejecutar cada checklist
	var checklistResults []ChecklistResult
	var recommendations []Recommendation

	for _, checklistID := range checklistsToApply {
		checklist, ok := types.AllChecklists[checklistID]
		if !ok {
			continue
		}

		result := executeChecklist(checklist, frameworkPath)
		checklistResults = append(checklistResults, result)

		// Recolectar recomendaciones de items fallidos
		for _, item := range result.Items {
			if item.Status == "fail" {
				recommendations = append(recommendations, Recommendation{
					Priority:   item.Severity,
					ItemID:     item.ItemID,
					Category:   item.Severity, // usar severity como categoría
					Problem:    item.Description,
					Suggestion: item.Suggestion,
				})
			}
		}
	}

	// Calcular scores
	var totalItems, passed, failed, warnings, optional int
	for _, cl := range checklistResults {
		totalItems += len(cl.Items)
		passed += cl.Passed
		failed += cl.Failed
		warnings += cl.Warnings
		optional += cl.Optional
	}

	score := 0.0
	if totalItems > 0 {
		// Score = (required_passed * 2 + recommended_passed) / (required_total * 2 + recommended_total) * 100
		requiredScore := 0.0
		recommendedScore := 0.0
		requiredTotal := 0
		recommendedTotal := 0

		for _, cl := range checklistResults {
			for _, item := range cl.Items {
				if item.Severity == "required" {
					requiredTotal++
					if item.Status == "pass" {
						requiredScore += 2
					}
				} else if item.Severity == "recommended" {
					recommendedTotal++
					if item.Status == "pass" {
						recommendedScore++
					}
				}
			}
		}

		denom := float64(requiredTotal*2 + recommendedTotal)
		if denom > 0 {
			score = ((requiredScore + recommendedScore) / denom) * 100
		}
	}

	// Verificar si puede registrarse (solo requiere que pasen todos los required)
	canRegister := failed == 0 && passed > 0

	result := &Result{
		FrameworkName:   frameworkName,
		FrameworkPath:   frameworkPath,
		DetectedType:    string(detectedType),
		Checklists:      checklistResults,
		TotalItems:      totalItems,
		Passed:          passed,
		Failed:          failed,
		Warnings:        warnings,
		Optional:        optional,
		Score:           score,
		Recommendations: recommendations,
		CanBeRegistered: canRegister,
	}

	ctx.Var("score", score)
	ctx.Var("failed", failed)
	ctx.Decision("review-complete", fmt.Sprintf("score=%.1f, failed=%d, canRegister=%v", score, failed, canRegister))

	return result, nil
}

// getChecklistsForType retorna los checklists aplicables a un tipo de framework.
func getChecklistsForType(fwType types.FrameworkType) []string {
	// Base común + comandos-ejecutables siempre
	checklists := []string{"base-comun", "comandos-ejecutables"}

	// Agregar específicos según tipo
	switch fwType {
	case types.TypeInquisitivo:
		checklists = append(checklists, "inquisitivo-base", "comunicacion", "persistencia-json")
	case types.TypeNodosArbol:
		checklists = append(checklists, "nodos-arbol", "persistencia-json")
	case types.TypeProcesador:
		checklists = append(checklists, "procesador-base", "manejador-errores")
	case types.TypeIntegracion:
		checklists = append(checklists, "integracion-base", "manejador-errores")
	case types.TypeAutomatizador:
		checklists = append(checklists, "automatizador-base", "manejador-errores")
	case types.TypeGenerico:
		// Solo base común + comandos-ejecutables
	}

	return checklists
}

// executeChecklist ejecuta un checklist sobre un framework.
func executeChecklist(checklist types.Checklist, frameworkPath string) ChecklistResult {
	result := ChecklistResult{
		ChecklistID:   checklist.ID,
		ChecklistName: checklist.Name,
		Items:         []ItemResult{},
	}

	for _, item := range checklist.Items {
		itemResult := checkItem(item, frameworkPath)
		result.Items = append(result.Items, itemResult)

		switch itemResult.Status {
		case "pass":
			result.Passed++
		case "fail":
			result.Failed++
		case "warning":
			result.Warnings++
		case "skip":
			result.Optional++
		}
	}

	return result
}

// checkItem evalúa un item individual.
func checkItem(item types.ChecklistItem, frameworkPath string) ItemResult {
	result := ItemResult{
		ItemID:      item.ID,
		Description: item.Description,
		Severity:    item.Severity,
		Status:      "pass", // default
	}

	// Ejecutar el check según el ID del item
	switch item.ID {
	// =========================================================================
	// BASE COMÚN
	// =========================================================================
	case "INITIAL_PROMPT.md-exists":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			result.Status = "fail"
			result.Reason = "No existe INITIAL_PROMPT.md"
			result.Suggestion = "Crear INITIAL_PROMPT.md con las instrucciones para la IA"
		}

	case "cmd-main-exists":
		// Verificar si existe cmd/<nombre>/main.go O un binario compilado
		fwName := filepath.Base(frameworkPath)
		cmdPath := filepath.Join(frameworkPath, "cmd", fwName, "main.go")
		binPath := filepath.Join(frameworkPath, strings.TrimPrefix(fwName, "framework-"))

		if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
			// No existe cmd/main.go, verificar binario
			if _, err := os.Stat(binPath); os.IsNotExist(err) {
				result.Status = "fail"
				result.Reason = "No existe cmd/" + fwName + "/main.go ni binario compilado"
				result.Suggestion = "Crear cmd/" + fwName + "/main.go o compilar el binario"
			} else {
				// Binario existe, OK (se puede mejorar después con estructura cmd/)
				result.Status = "pass"
				result.Reason = "Binario compilado encontrado: " + binPath
			}
		}

	case "INITIAL_PROMPT.md-rol":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if data, err := os.ReadFile(path); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "rol") && !strings.Contains(content, "eres") {
				result.Status = "fail"
				result.Reason = "No define claramente el rol de la IA"
				result.Suggestion = "Agregar sección '## Tu Rol' o similar"
			}
		} else {
			result.Status = "skip"
		}

	case "INITIAL_PROMPT.md-filosofia":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if data, err := os.ReadFile(path); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "filosof") && !strings.Contains(content, "simple") && !strings.Contains(content, "regla") {
				result.Status = "fail"
				result.Reason = "No tiene sección de filosofía"
				result.Suggestion = "Agregar '## Tu filosofía' describiendo el enfoque del framework"
			}
		} else {
			result.Status = "skip"
		}

	case "INITIAL_PROMPT.md-comandos":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if data, err := os.ReadFile(path); err == nil {
			content := string(data)
			if !strings.Contains(content, "./") && !strings.Contains(content, "comando") {
				result.Status = "fail"
				result.Reason = "No lista los comandos disponibles"
				result.Suggestion = "Agregar sección de comandos con ejemplos"
			}
		} else {
			result.Status = "skip"
		}

	case "AGENTS.md-exists":
		path := filepath.Join(frameworkPath, "AGENTS.md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			result.Status = "fail"
			result.Reason = "No existe AGENTS.md"
			result.Suggestion = "Crear AGENTS.md para integración con otros frameworks"
		}

	case "README.md-exists":
		path := filepath.Join(frameworkPath, "README.md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			result.Status = "fail"
			result.Reason = "No existe README.md"
			result.Suggestion = "Crear README.md básico"
		}

	case "WHY.md-exists":
		path := filepath.Join(frameworkPath, "WHY.md")
		if data, err := os.ReadFile(path); err != nil {
			result.Status = "fail"
			result.Reason = "No existe WHY.md"
			result.Suggestion = "Crear WHY.md con propósito, tipo y límite operativo"
		} else {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "proposito") && !strings.Contains(content, "propósito") {
				result.Status = "fail"
				result.Reason = "WHY.md no declara el propósito"
				result.Suggestion = "Agregar sección de propósito"
			}
			if !strings.Contains(content, "tipo") {
				result.Status = "fail"
				result.Reason = "WHY.md no declara el tipo de framework"
				result.Suggestion = "Agregar sección tipo: inquisitivo, integración, procesador, etc."
			}
		}

	case "paladin-integrado":
		path := filepath.Join(frameworkPath, "internal", "paladin")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			result.Status = "fail"
			result.Reason = "No existe internal/paladin/"
			result.Suggestion = "Copiar paladin desde framework-paladin/paladin"
		} else {
			tracePath := filepath.Join(path, "trace.go")
			if _, err := os.Stat(tracePath); os.IsNotExist(err) {
				result.Status = "fail"
				result.Reason = "paladin existe pero falta trace.go"
				result.Suggestion = "Verificar que todos los archivos de paladin estén presentes"
			}
		}

	case "go-mod-exists":
		path := filepath.Join(frameworkPath, "go.mod")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			result.Status = "fail"
			result.Reason = "No existe go.mod"
			result.Suggestion = "Crear go.mod válido"
		}

	case "no-hardcoded-paths":
		// Buscar hardcoded paths en archivos .go
		hasHardcoded := false
		filepath.Walk(frameworkPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			if data, err := os.ReadFile(path); err == nil {
				content := string(data)
				if strings.Contains(content, "/Users/") && strings.Contains(content, "remora") {
					hasHardcoded = true
				}
			}
			return nil
		})
		if hasHardcoded {
			result.Status = "warning"
			result.Reason = "Se encontraron rutas hardcodeadas"
			result.Suggestion = "Usar rutas relativas o variables de entorno"
		}

	// =========================================================================
	// INQUISITIVO
	// =========================================================================
	case "preguntas-guia":
		// Buscar comandos relacionados con preguntas en main.go
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := strings.ToLower(string(data))
			hasQuestionCmds := strings.Contains(content, "add-axiom") ||
				strings.Contains(content, "add-question") ||
				strings.Contains(content, "ask") ||
				strings.Contains(content, "pregunta")
			if !hasQuestionCmds {
				result.Status = "fail"
				result.Reason = "No hay comandos para hacer/preguntar"
				result.Suggestion = "Agregar comandos tipo add-axiom, add-question, etc."
			}
		}

	case "log-qa":
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "log-qa") && !strings.Contains(content, "qa-log") {
				result.Status = "fail"
				result.Reason = "No hay comando para registrar Q&A"
				result.Suggestion = "Agregar comando log-qa --question --answer --purpose"
			}
		}

	case "signal-fatiga":
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "signal") && !strings.Contains(content, "fatiga") && !strings.Contains(content, "fatigue") {
				result.Status = "warning"
				result.Reason = "No hay mecanismo para registrar fatiga"
				result.Suggestion = "Agregar comando signal --type fatigue --note"
			}
		}

	case "semáforo-decisión":
		// Buscar readiness o similar
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "readiness") && !strings.Contains(content, "ready") && !strings.Contains(content, "status") {
				result.Status = "fail"
				result.Reason = "No hay semáforo para saber cuándo parar"
				result.Suggestion = "Agregar comando readiness o similar"
			}
		}

	case "no-ofrecer-temprano":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if data, err := os.ReadFile(path); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "no ofrecer") && !strings.Contains(content, "no ofres") && !strings.Contains(content, "pain") {
				result.Status = "fail"
				result.Reason = "No hay regla que impida ofrecer soluciones antes del dolor"
				result.Suggestion = "Agregar regla: NO ofrecer soluciones antes de confirmar dolor real"
			}
		}

	case "una-pregunta-a-la-vez":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if data, err := os.ReadFile(path); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "una pregunta") && !strings.Contains(content, "1 pregunta") {
				result.Status = "fail"
				result.Reason = "No se enfatiza hacer una pregunta a la vez"
				result.Suggestion = "Agregar regla: haz UNA pregunta a la vez"
			}
		}

	case "percepciones-internas":
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "perception") && !strings.Contains(content, "percib") {
				result.Status = "warning"
				result.Reason = "No hay mecanismo para percepciones internas"
				result.Suggestion = "Agregar comando add-perception --node-id --note"
			}
		}

	// =========================================================================
	// NODOS Y ÁRBOL
	// =========================================================================
	case "estructura-nodos":
		nodePath := findNodeFile(frameworkPath)
		if nodePath == "" {
			result.Status = "fail"
			result.Reason = "No se encontró archivo de nodos"
			result.Suggestion = "Crear internal/tree/node.go con estructura Node"
		} else if data, err := os.ReadFile(nodePath); err == nil {
			content := string(data)
			hasID := strings.Contains(content, "ID") || strings.Contains(content, "Id")
			hasType := strings.Contains(content, "Type")
			hasTitle := strings.Contains(content, "Title")
			hasStatus := strings.Contains(content, "Status")
			hasParent := strings.Contains(content, "Parent")

			if !hasID || !hasType || !hasTitle || !hasStatus || !hasParent {
				result.Status = "fail"
				result.Reason = "Estructura Node incompleta (faltan: ID, Type, Title, Status, Parent)"
				result.Suggestion = "Verificar que Node tenga todos los campos requeridos"
			}
		}

	case "jerarquia-capas":
		nodePath := findNodeFile(frameworkPath)
		if nodePath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(nodePath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "layer") && !strings.Contains(content, "capa") {
				result.Status = "fail"
				result.Reason = "No hay concepto de layers/capas"
				result.Suggestion = "Agregar campo Layer a Node para jerarquía"
			}
		}

	case "estados-nodo":
		nodePath := findNodeFile(frameworkPath)
		if nodePath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(nodePath); err == nil {
			content := strings.ToLower(string(data))
			hasPending := strings.Contains(content, "pending")
			hasValidated := strings.Contains(content, "validated")

			if !hasPending || !hasValidated {
				result.Status = "fail"
				result.Reason = "No hay estados PENDING, VALIDATED"
				result.Suggestion = "Agregar constantes de estado a Node"
			}
		}

	case "add-nodo-cmd":
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "add-") {
				result.Status = "fail"
				result.Reason = "No hay comandos add-<tipo>"
				result.Suggestion = "Agregar comandos: add-axiom, add-task, etc."
			}
		}

	case "validate-nodo-cmd":
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "validate") {
				result.Status = "fail"
				result.Reason = "No hay comando validate"
				result.Suggestion = "Agregar comando validate <node-id>"
			}
		}

	case "show-tree-cmd":
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "show-tree") && !strings.Contains(content, "tree") {
				result.Status = "fail"
				result.Reason = "No hay comando para ver el árbol"
				result.Suggestion = "Agregar comando show-tree"
			}
		}

	case "persistencia-json":
		jsonFiles, _ := filepath.Glob(filepath.Join(frameworkPath, "*.json"))
		if len(jsonFiles) == 0 {
			result.Status = "fail"
			result.Reason = "No hay archivo JSON de persistencia"
			result.Suggestion = "El framework debe persistir su estado en JSON"
		}

	case "validacion-estado":
		// Verificar que el código valide parent antes de crear hijo
		treePath := findTreeFile(frameworkPath)
		if treePath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(treePath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "validated") && !strings.Contains(content, "validat") {
				result.Status = "warning"
				result.Reason = "No se encontró validación de estado de parent"
				result.Suggestion = "Agregar validación: parent debe estar validado para crear hijo"
			}
		}

	case "preguntas-pendientes":
		treePath := findTreeFile(frameworkPath)
		if treePath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(treePath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "question") && !strings.Contains(content, "next") {
				result.Status = "warning"
				result.Reason = "No hay mecanismo de preguntas pendientes"
				result.Suggestion = "Agregar comando next-questions"
			}
		}

	// =========================================================================
	// COMUNICACIÓN
	// =========================================================================
	case "loop-con-otro-framework":
		path := filepath.Join(frameworkPath, "AGENTS.md")
		if data, err := os.ReadFile(path); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "alfa") && !strings.Contains(content, "framework") {
				result.Status = "fail"
				result.Reason = "AGENTS.md no indica comunicación con otros frameworks"
				result.Suggestion = "Agregar sección sobre cómo comunicarse con otros frameworks"
			}
		}

	case "comando-consulta":
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "compile") && !strings.Contains(content, "consult") && !strings.Contains(content, "invoke") {
				result.Status = "warning"
				result.Reason = "No hay comando para consultar otro framework"
				result.Suggestion = "Agregar comando para invocar otro framework"
			}
		}

	// =========================================================================
	// PERSISTENCIA JSON
	// =========================================================================
	case "no-editar-manual":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if data, err := os.ReadFile(path); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "no editar") && !strings.Contains(content, "no edite") && !strings.Contains(content, "never edit") {
				result.Status = "fail"
				result.Reason = "No prohíbe editar JSON manualmente"
				result.Suggestion = "Agregar regla: NUNCA edites <archivo>.json manualmente"
			}
		}

	case "comando-init":
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "init") {
				result.Status = "fail"
				result.Reason = "No hay comando init"
				result.Suggestion = "Agregar comando init para inicializar proyecto"
			}
		}

	case "LoadOrCreate":
		treePath := findTreeFile(frameworkPath)
		if treePath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(treePath); err == nil {
			content := string(data)
			if !strings.Contains(content, "LoadOrCreate") {
				result.Status = "fail"
				result.Reason = "No hay función LoadOrCreate"
				result.Suggestion = "Agregar función LoadOrCreate para no perder datos"
			}
		}

	// =========================================================================
	// PROCESADOR
	// =========================================================================
	case "comando-proccess":
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "process") {
				result.Status = "fail"
				result.Reason = "No hay comando process"
				result.Suggestion = "Agregar comando process"
			}
		}

	case "input-validacion":
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "flag.new") && !strings.Contains(content, "validate") && !strings.Contains(content, "required") {
				result.Status = "warning"
				result.Reason = "No se encontró validación de inputs"
				result.Suggestion = "Agregar validación de argumentos requeridos"
			}
		}

	case "config-externa":
		if frameworkContains(frameworkPath, []string{"os.Getenv", ".env", "requiredEnv", "missingEnv"}) {
			result.Status = "pass"
		} else {
			result.Status = "fail"
			result.Reason = "No se encontró configuración externa por env/archivo"
			result.Suggestion = "Leer credenciales desde variables de entorno o archivo de configuración, no hardcodear"
		}

	case "auth-handling":
		if frameworkContains(frameworkPath, []string{"token", "credential", "credencial", "requiredEnv", "missingEnv"}) {
			result.Status = "pass"
		} else {
			result.Status = "fail"
			result.Reason = "No se encontró manejo explícito de autenticación"
			result.Suggestion = "Agregar validación de credenciales antes de conectar"
		}

	// =========================================================================
	// ERRORES
	// =========================================================================
	case "errores-descriptivos":
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := string(data)
			if !strings.Contains(content, "fmt.Printf") && !strings.Contains(content, "Error:") {
				result.Status = "warning"
				result.Reason = "No se encontró manejo de errores"
				result.Suggestion = "Agregar errores descriptivos"
			}
		}

	case "exit-codes":
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(mainPath); err == nil {
			content := string(data)
			if !strings.Contains(content, "os.Exit(1)") || !strings.Contains(content, "os.Exit(2)") {
				result.Status = "fail"
				result.Reason = "No se encontraron exit codes diferenciados para error y usage"
				result.Suggestion = "Usar os.Exit(1) para errores de ejecución y os.Exit(2) para uso inválido"
			}
		}

		// =========================================================================
		// COMANDOS EJECUTABLES
		// =========================================================================
	case "prompt-comandos-lista":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if data, err := os.ReadFile(path); err == nil {
			content := string(data)
			// Busca que liste comandos con sintaxis ./nombrecomando
			if !strings.Contains(content, "./") || !strings.Contains(content, "comando") {
				result.Status = "fail"
				result.Reason = "INITIAL_PROMPT.md no lista los comandos disponibles"
				result.Suggestion = "Agregar sección '## Comandos' con sintaxis ./nombrecomando --flag valor"
			}
		} else {
			result.Status = "skip"
		}

	case "prompt-comandos-implementados":
		// Verificar que los comandos listados en prompt existen en main.go
		// Soporta tanto comandos directos (./comando) como subcomandos (./ejecutable subcomando)
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		mainPath := findMainGo(frameworkPath)
		if mainPath == "" {
			result.Status = "skip"
		} else if data, err := os.ReadFile(path); err == nil {
			promptContent := string(data)
			mainData, _ := os.ReadFile(mainPath)
			mainContent := string(mainData)

			// Extraer ejecutable principal del framework
			fwName := strings.ToLower(filepath.Base(frameworkPath))
			fwBin := strings.TrimPrefix(fwName, "framework-")

			// Extraer subcomandos del prompt (formato: ./ejecutable subcomando)
			subCmdPattern := regexp.MustCompile(`\./` + fwBin + `[\s]+([a-z][a-z0-9-]+)`)
			subCmdsInPrompt := subCmdPattern.FindAllStringSubmatch(promptContent, -1)

			// También capturar comandos directos
			directCmdPattern := regexp.MustCompile(`\.\/([a-z][a-z0-9-]+)(?:[\s\n]+--|\s\n|\.$|$)`)
			directCmds := directCmdPattern.FindAllStringSubmatch(promptContent, -1)

			var missing []string
			var found []string

			// Verificar subcomandos
			for _, match := range subCmdsInPrompt {
				subCmd := match[1]
				if !strings.Contains(mainContent, "case \""+subCmd+"\"") {
					missing = append(missing, subCmd)
				} else {
					found = append(found, subCmd)
				}
			}

			// Verificar comandos directos (pero ignorar ejecutables con "framework")
			for _, match := range directCmds {
				cmd := match[1]
				if strings.HasPrefix(cmd, "framework") || strings.Contains(cmd, "-") {
					continue
				}
				if !strings.Contains(mainContent, "case \""+cmd+"\"") {
					missing = append(missing, cmd)
				} else {
					found = append(found, cmd)
				}
			}

			if len(found) > 0 && len(missing) == 0 {
				result.Status = "pass"
				result.Reason = fmt.Sprintf("%d comandos verificados y todos implementados", len(found))
			} else if len(missing) > 0 {
				result.Status = "fail"
				result.Reason = "Subcomandos listados en prompt pero no implementados: " + strings.Join(missing, ", ")
				result.Suggestion = "Verificar que todos los subcomandos en el prompt estén en main.go como 'case \"" + missing[0] + "\"'"
			}
		}

	case "prompt-no-narrativa-obligatoria":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if data, err := os.ReadFile(path); err == nil {
			content := strings.ToLower(string(data))
			// Verificar que las reglas importantes tienen respaldo en comandos
			// No debe depender SOLO de "recordar" reglas narrativas
			// El semáforo de readiness debería existir como comando
			mainPath := findMainGo(frameworkPath)
			if mainPath != "" {
				mainData, _ := os.ReadFile(mainPath)
				mainContent := string(mainData)

				// Si hay reglas sobre cuándo actuar, debe haber comando que lo determine
				if strings.Contains(content, "debería") || strings.Contains(content, "debe") || strings.Contains(content, "tiene que") {
					// Verificar que hay comando que lo ejecuta
					if !strings.Contains(mainContent, "readiness") {
						result.Status = "warning"
						result.Reason = "Reglas narrativas sin comando que las ejecute automáticamente"
						result.Suggestion = "Considerar agregar comando que evalúe automáticamente (ej: readiness)"
					}
				}
			}
		} else {
			result.Status = "skip"
		}

	case "prompt-sintaxis-ejecutable":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if data, err := os.ReadFile(path); err == nil {
			content := string(data)
			// Verificar que usa sintaxis ./comando ...
			if !strings.Contains(content, "./") {
				result.Status = "fail"
				result.Reason = "No usa sintaxis ejecutable ./nombrecomando"
				result.Suggestion = "Usar formato: ./nombrecomando --flag valor"
			}
		} else {
			result.Status = "skip"
		}

	case "prompt-no-editar-manual-json":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if data, err := os.ReadFile(path); err == nil {
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "no editar") && !strings.Contains(content, "no edite") && !strings.Contains(content, "never edit") {
				result.Status = "fail"
				result.Reason = "No prohíbe editar JSON manualmente"
				result.Suggestion = "Agregar: NO edites <archivo>.json manualmente"
			}
		} else {
			result.Status = "skip"
		}

	case "prompt-instrucciones-cuantificadas":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if data, err := os.ReadFile(path); err == nil {
			content := string(data)
			// Verificar que hay condiciones tipo "si X, entonces Y"
			if !strings.Contains(content, "si") && !strings.Contains(content, "when") && !strings.Contains(content, "when") {
				result.Status = "warning"
				result.Reason = "Instrucciones no tienen condiciones claras (si X, entonces Y)"
				result.Suggestion = "Agregar condiciones: 'Si hay AXIOMS pero no THEORIES, haz X'"
			}
		} else {
			result.Status = "skip"
		}

	// =========================================================================
	// MANIFEST (contrato declarativo con api_rest)
	// =========================================================================
	case "manifest-exists":
		path := filepath.Join(frameworkPath, "framework.manifest.json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			result.Status = "fail"
			result.Reason = "No existe framework.manifest.json"
			result.Suggestion = "Crear framework.manifest.json siguiendo el schema de channel/manifest. Sin él, el orquestador api_rest no puede integrar este framework automáticamente."
		}

	case "manifest-valid":
		path := filepath.Join(frameworkPath, "framework.manifest.json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Sin manifest no podemos validar; no es fail por sí mismo,
			// el item manifest-exists ya señala la falta.
			result.Status = "skip"
			break
		}
		m, err := manifest.Load(path)
		if err != nil {
			result.Status = "fail"
			result.Reason = fmt.Sprintf("framework.manifest.json no parsea: %v", err)
			result.Suggestion = "Revisar JSON y schema en channel/manifest/manifest.go"
			break
		}
		if err := m.Validate(); err != nil {
			result.Status = "fail"
			result.Reason = err.Error()
			result.Suggestion = "Corregir invariantes mínimos: name, version, binary.command, execution_mode válido, y si user_input.supported=true, declarar next_question_cmd e ingest_answer_cmd"
		}

	case "prompt-workflow-descomprimido":
		path := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
		if data, err := os.ReadFile(path); err == nil {
			content := strings.ToLower(string(data))
			// No debe tener reglas mentales como "debes recordar", "piénsalo bien"
			if strings.Contains(content, "recuerda") || strings.Contains(content, "piensa") || strings.Contains(content, "deberías pensar") {
				result.Status = "warning"
				result.Reason = "El prompt usa instrucciones mentales en lugar de pasos ejecutables"
				result.Suggestion = "Convertir a: 'Ejecuta ./comando X' en lugar de 'Recuerda hacer X'"
			}
		} else {
			result.Status = "skip"
		}

	default:
		// Si no conocemos el item, lo saltamos
		result.Status = "skip"
	}

	if result.Status == "skip" && item.Severity == "required" {
		result.Status = "fail"
		if result.Reason == "" {
			result.Reason = "Check requerido no pudo evaluarse"
		}
		if result.Suggestion == "" {
			result.Suggestion = "Implementar evidencia verificable para este requisito"
		}
	}

	return result
}

// ============================================================================
// UTILIDADES
// ============================================================================

func findMainGo(basePath string) string {
	// Buscar en cmd/*/main.go
	cmdDir := filepath.Join(basePath, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return ""
	}

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

func frameworkContains(basePath string, needles []string) bool {
	found := false
	filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := strings.ToLower(string(data))
		for _, needle := range needles {
			if strings.Contains(content, strings.ToLower(needle)) {
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}

func findNodeFile(basePath string) string {
	internalDir := filepath.Join(basePath, "internal")
	entries, err := os.ReadDir(internalDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			nodePath := filepath.Join(internalDir, entry.Name(), "node.go")
			if _, err := os.Stat(nodePath); err == nil {
				return nodePath
			}
		}
	}

	return ""
}

func findTreeFile(basePath string) string {
	internalDir := filepath.Join(basePath, "internal")
	entries, err := os.ReadDir(internalDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			treePath := filepath.Join(internalDir, entry.Name(), "tree.go")
			if _, err := os.Stat(treePath); err == nil {
				return treePath
			}
		}
	}

	return ""
}

// FormatResult genera un reporte legible del resultado.
func FormatResult(r *Result) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\n"))
	sb.WriteString(fmt.Sprintf("╔══════════════════════════════════════════════════════════════════╗\n"))
	sb.WriteString(fmt.Sprintf("║  REPORTE DE CALIDAD - %-44s ║\n", r.FrameworkName))
	sb.WriteString(fmt.Sprintf("╠══════════════════════════════════════════════════════════════════╣\n"))
	sb.WriteString(fmt.Sprintf("║  Tipo detectado: %-48s ║\n", r.DetectedType))
	sb.WriteString(fmt.Sprintf("║  Score: %.1f%% | Pass: %d | Fail: %d | Warnings: %d        ║\n",
		r.Score, r.Passed, r.Failed, r.Warnings))
	sb.WriteString(fmt.Sprintf("╚══════════════════════════════════════════════════════════════════╝\n"))

	// Detalle por checklist
	for _, cl := range r.Checklists {
		status := "✅"
		if cl.Failed > 0 {
			status = "❌"
		} else if cl.Warnings > 0 {
			status = "⚠️"
		}

		sb.WriteString(fmt.Sprintf("\n%s %s (%d/%d passed)\n", status, cl.ChecklistName, cl.Passed, len(cl.Items)))

		for _, item := range cl.Items {
			icon := "  ✅"
			switch item.Status {
			case "fail":
				icon = "  ❌"
			case "warning":
				icon = "  ⚠️"
			case "skip":
				icon = "  ➖"
			}

			severityTag := "[" + item.Severity + "]"
			sb.WriteString(fmt.Sprintf("%s %s %s\n", icon, severityTag, item.Description))

			if item.Status == "fail" && item.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("     💡 Suggestion: %s\n", item.Suggestion))
			}
		}
	}

	// Recomendaciones si hay fallas
	if len(r.Recommendations) > 0 {
		sb.WriteString(fmt.Sprintf("\n────────────────────────────────────────────────────────\n"))
		sb.WriteString(fmt.Sprintf("📋 RECOMENDACIONES DE MEJORA\n"))
		sb.WriteString(fmt.Sprintf("────────────────────────────────────────────────────────\n"))

		// Ordenar por prioridad
		sort.Slice(r.Recommendations, func(i, j int) bool {
			order := map[string]int{"required": 0, "recommended": 1, "optional": 2}
			return order[r.Recommendations[i].Priority] < order[r.Recommendations[j].Priority]
		})

		for _, rec := range r.Recommendations {
			if rec.Priority == "required" {
				sb.WriteString(fmt.Sprintf("\n🔴 [%s] %s\n", rec.Priority, rec.Problem))
				sb.WriteString(fmt.Sprintf("   ➡️ %s\n", rec.Suggestion))
			}
		}

		for _, rec := range r.Recommendations {
			if rec.Priority == "recommended" {
				sb.WriteString(fmt.Sprintf("\n🟡 [%s] %s\n", rec.Priority, rec.Problem))
				sb.WriteString(fmt.Sprintf("   ➡️ %s\n", rec.Suggestion))
			}
		}
	}

	// Resumen final
	sb.WriteString(fmt.Sprintf("\n────────────────────────────────────────────────────────\n"))
	if r.CanBeRegistered {
		sb.WriteString(fmt.Sprintf("✅ El framework PUEDE ser registrado\n"))
	} else {
		sb.WriteString(fmt.Sprintf("❌ El framework NO puede ser registrado aún\n"))
		sb.WriteString(fmt.Sprintf("   Corrige los items [required] antes de registrar\n"))
	}

	return sb.String()
}

// ToJSON retorna el resultado en formato JSON.
func (r *Result) ToJSON() string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
