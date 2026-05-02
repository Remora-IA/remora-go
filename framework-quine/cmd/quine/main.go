package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"framework-quine/internal/paladin"
	"framework-quine/internal/quine"
	"framework-quine/internal/review"
	"framework-quine/internal/types"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	trace := paladin.NewTrace("quine-main")
	ctx := trace.Start()
	defer trace.Flush()

	command := os.Args[1]
	ctx.Var("command", command)

	switch command {
	case "create":
		cmdCreate(os.Args[2:])

	case "init":
		cmdInit(os.Args[2:])

	case "spec":
		cmdSpec(os.Args[2:])

	case "list":
		cmdList()

	case "use":
		cmdUse(os.Args[2:])

	case "review":
		cmdReview(os.Args[2:])

	case "register":
		cmdRegister(os.Args[2:])

	case "types":
		cmdTypes()

	case "fix":
		cmdFix(os.Args[2:])

	case "analyze-commands", "analyze":
		cmdAnalyzeCommands(os.Args[2:])

	case "help", "-h", "--help":
		usage()

	default:
		if len(os.Args) > 1 {
			cmdUse([]string{command})
		} else {
			usage()
			os.Exit(1)
		}
	}
}

func cmdCreate(args []string) {
	trace := paladin.NewTrace("create")
	ctx := trace.Start()

	fs := flag.NewFlagSet("create", flag.ExitOnError)
	name := fs.String("name", "", "nombre del framework (ej: framework-alfa)")
	role := fs.String("role", "", "rol de la IA (ej: compilador semántico)")
	description := fs.String("description", "", "descripción del framework")
	fwType := fs.String("type", "generico", "tipo de framework (inquisitivo, nodos-arbol, procesador, integracion, automatizador, generico)")
	output := fs.String("output", "", "directorio de salida (opcional)")

	if err := fs.Parse(args); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	if *name == "" {
		fmt.Println("❌ El nombre del framework es requerido")
		fmt.Println("   usa: quine create --name framework-alfa --role \"compilador\" --description \"compila árboles\"")
		os.Exit(1)
	}

	if !strings.HasPrefix(*name, "framework-") {
		*name = "framework-" + *name
	}

	ctx.Var("frameworkName", *name)
	ctx.Var("role", *role)
	ctx.Var("type", *fwType)

	spec := quine.FrameworkSpec{
		Name:        *name,
		Role:        *role,
		Description: *description,
		Type:        *fwType,
		Purpose:     fmt.Sprintf("Framework %s creado para %s", *name, *role),
		Methods:     defaultMethodsForType(*fwType),
	}
	for _, method := range spec.Methods {
		spec.CLICommands = append(spec.CLICommands, strings.ToLower(method.Name))
	}
	spec.CLICommands = append(spec.CLICommands, "help")

	fmt.Printf("🚀 Creando framework: %s\n", *name)
	fmt.Printf("   Rol: %s\n", *role)
	fmt.Printf("   Descripción: %s\n", *description)
	fmt.Printf("   Tipo: %s\n", *fwType)

	result, err := quine.Generate(spec, *output)
	if err != nil {
		ctx.Error(err)
		fmt.Printf("❌ Error creando framework: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Framework %s creado exitosamente!\n", *name)
	fmt.Printf("   Archivos creados: %d\n", len(result.Files))
	fmt.Printf("   Ubicación: %s\n", filepath.Dir(result.Files[0].Path))

	outputDir := filepath.Dir(result.Files[0].Path)
	binName := strings.TrimPrefix(*name, "framework-")
	if err := runGeneratedFrameworkTests(outputDir); err != nil {
		ctx.Error(err)
		fmt.Printf("\n❌ Framework creado pero no pasa tests: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   Tests: go test ./... OK\n")

	fmt.Printf("\n📦 Para compilar:\n")
	fmt.Printf("   cd %s\n", outputDir)
	fmt.Printf("   go build -o %s ./cmd/%s/\n", binName, *name)

	trace.Flush()
}

func runGeneratedFrameworkTests(outputDir string) error {
	gofmtCmd := exec.Command("gofmt", "-w", ".")
	gofmtCmd.Dir = outputDir
	if output, err := gofmtCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gofmt fallo: %v\n%s", err, strings.TrimSpace(string(output)))
	}

	testCmd := exec.Command("go", "test", "./...")
	testCmd.Dir = outputDir
	if output, err := testCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go test fallo: %v\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	name := fs.String("name", "", "nombre del framework")
	output := fs.String("output", "", "directorio de salida")

	if err := fs.Parse(args); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	if *name == "" {
		fmt.Println("❌ Nombre requerido")
		os.Exit(1)
	}

	if !strings.HasPrefix(*name, "framework-") {
		*name = "framework-" + *name
	}

	spec := quine.CreateDefaultSpec(*name, "operador del framework", "framework base")

	result, err := quine.Generate(spec, *output)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Framework %s inicializado en %s\n", *name, filepath.Dir(result.Files[0].Path))
}

func cmdSpec(args []string) {
	fs := flag.NewFlagSet("spec", flag.ExitOnError)
	create := fs.Bool("create", false, "crear especificación de ejemplo")
	name := fs.String("name", "", "nombre del framework")

	if err := fs.Parse(args); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	if *create {
		spec := quine.CreateDefaultSpec("framework-example", "ejemplo", "framework de ejemplo")
		data, _ := json.MarshalIndent(spec, "", "  ")
		fmt.Println(string(data))
		return
	}

	if *name != "" {
		spec, err := quine.LoadSpec(*name + ".json")
		if err != nil {
			fmt.Printf("❌ No se encontró especificación: %s\n", *name)
			os.Exit(1)
		}
		data, _ := json.MarshalIndent(spec, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Println("❌ Usa --create o --name <archivo>")
}

func cmdList() {
	frameworks, err := quine.ListFrameworks("/Users/alcless_a1234_cursor/remora-go")
	if err != nil {
		fmt.Printf("❌ Error listando frameworks: %v\n", err)
		os.Exit(1)
	}

	// Cargar registro para ver tipos
	registry, _ := types.LoadRegistry()

	if len(frameworks) == 0 {
		fmt.Println("📦 No hay frameworks creados")
		fmt.Println("   Usa: quine create --name framework-ejemplo --role \"mi rol\"")
		return
	}

	fmt.Printf("📦 Frameworks existentes (%d):\n\n", len(frameworks))
	for _, fw := range frameworks {
		fwType := "generico"
		if entry := registry.GetFramework(fw); entry != nil {
			fwType = string(entry.Type)
		}
		fmt.Printf("   • %s [%s]\n", fw, fwType)
	}
}

func cmdReview(args []string) {
	fs := flag.NewFlagSet("review", flag.ExitOnError)
	path := fs.String("path", "", "ruta del framework a revisar")
	jsonOutput := fs.Bool("json", false, "salida en JSON")
	register := fs.Bool("register", false, "registrar después de revisión exitosa")

	if err := fs.Parse(args); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	if *path == "" {
		// Buscar en directorio actual
		cwd, _ := os.Getwd()
		*path = cwd
	}

	fmt.Printf("🔍 Revisando framework: %s\n", *path)

	result, err := review.Review(*path)
	if err != nil {
		fmt.Printf("❌ Error en revisión: %v\n", err)
		os.Exit(1)
	}

	// Mostrar resultado
	if *jsonOutput {
		fmt.Println(result.ToJSON())
	} else {
		fmt.Println(review.FormatResult(result))
	}

	// Registrar si se pidió y está listo
	if *register && result.CanBeRegistered {
		registerFramework(*path, result.DetectedType)
	}
}

func cmdRegister(args []string) {
	fs := flag.NewFlagSet("register", flag.ExitOnError)
	path := fs.String("path", "", "ruta del framework")
	name := fs.String("name", "", "nombre (opcional, usa el directorio)")
	fwType := fs.String("type", "", "tipo (detecta automático si no se especifica)")
	role := fs.String("role", "", "rol del framework")
	description := fs.String("description", "", "descripción")

	if err := fs.Parse(args); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	if *path == "" {
		*path, _ = os.Getwd()
	}

	// Detectar tipo si no se especificó
	detectedType := types.TypeGenerico
	if *fwType != "" {
		detectedType = types.FrameworkType(*fwType)
	} else {
		detectedType, _ = types.DetectFrameworkType(*path)
	}

	fwName := filepath.Base(*path)
	if *name != "" {
		fwName = *name
	}

	// Cargar o crear registro
	registry, err := types.LoadRegistry()
	if err != nil {
		registry = &types.FrameworkRegistry{
			Version:    "1.0",
			Frameworks: []types.FrameworkEntry{},
		}
	}

	entry := types.FrameworkEntry{
		Name:        fwName,
		Type:        detectedType,
		Path:        *path,
		Role:        *role,
		Description: *description,
		Created:     "",
	}

	registry.AddFramework(entry)

	if err := types.SaveRegistry(registry); err != nil {
		fmt.Printf("❌ Error guardando registro: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Framework '%s' registrado como [%s]\n", fwName, detectedType)
}

func cmdTypes() {
	fmt.Printf("📋 TIPOS DE FRAMEWORK\n\n")

	for _, meta := range types.TypeMetadataMap {
		fmt.Printf("━━━ %s ━━━\n", meta.Name)
		fmt.Printf("   %s\n", meta.Description)
		fmt.Printf("   Checklists: %s\n\n", strings.Join(meta.Checklists, ", "))
	}

	fmt.Printf("\n📦 FRAMEWORKS REGISTRADOS\n\n")
	registry, err := types.LoadRegistry()
	if err != nil || len(registry.Frameworks) == 0 {
		fmt.Println("   No hay frameworks registrados")
		fmt.Println("   Usa: quine register --path /ruta/framework-ejemplo")
		return
	}

	for _, fw := range registry.Frameworks {
		fmt.Printf("   • %s [%s] - %s\n", fw.Name, fw.Type, fw.Role)
	}
}

func cmdFix(args []string) {
	fs := flag.NewFlagSet("fix", flag.ExitOnError)
	path := fs.String("path", "", "ruta del framework")
	auto := fs.Bool("auto", false, "aplicar fixes automáticos cuando sea posible")

	if err := fs.Parse(args); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	if *path == "" {
		*path, _ = os.Getwd()
	}

	fmt.Printf("🔧 Analizando fixes para: %s\n", *path)

	result, err := review.Review(*path)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	if len(result.Recommendations) == 0 {
		fmt.Println("✅ No hay fixes necesarios")
		return
	}

	fmt.Printf("\n📋 Fixes recomendados:\n\n")
	for _, rec := range result.Recommendations {
		priorityIcon := "🔴"
		if rec.Priority == "recommended" {
			priorityIcon = "🟡"
		}
		fmt.Printf("%s [%s] %s\n", priorityIcon, rec.ItemID, rec.Problem)
		fmt.Printf("   → %s\n\n", rec.Suggestion)
	}

	// Aplicar fixes automáticos si se pidió
	if *auto {
		applyAutoFixes(*path, result)
	}
}

func registerFramework(path string, fwType string) {
	fwName := filepath.Base(path)

	registry, err := types.LoadRegistry()
	if err != nil {
		registry = &types.FrameworkRegistry{
			Version:    "1.0",
			Frameworks: []types.FrameworkEntry{},
		}
	}

	entry := types.FrameworkEntry{
		Name:    fwName,
		Type:    types.FrameworkType(fwType),
		Path:    path,
		Created: "",
	}

	registry.AddFramework(entry)
	types.SaveRegistry(registry)

	fmt.Printf("✅ Framework registrado en el repositorio\n")
}

func applyAutoFixes(path string, result *review.Result) {
	fmt.Printf("\n🔧 Aplicando fixes automáticos...\n\n")

	frameworkName := filepath.Base(path)
	pkgName := strings.ToLower(frameworkName)

	for _, rec := range result.Recommendations {
		switch rec.ItemID {
		case "cmd-main-exists":
			fmt.Printf("   + Creando cmd/%s/main.go\n", frameworkName)
			mainContent := generateMainGoBasic(frameworkName)
			mainPath := filepath.Join(path, "cmd", frameworkName, "main.go")
			os.MkdirAll(filepath.Dir(mainPath), 0755)
			os.WriteFile(mainPath, []byte(mainContent), 0644)
			fmt.Printf("      ✅ Creado: %s\n", mainPath)

		case "paladin-integrado":
			fmt.Printf("   + Copiando paladin desde framework-paladin\n")
			srcDir := "/Users/alcless_a1234_cursor/remora-go/framework-paladin/paladin"
			destDir := filepath.Join(path, "internal", "paladin")
			os.MkdirAll(destDir, 0755)
			entries, _ := os.ReadDir(srcDir)
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
					continue
				}
				srcFile := filepath.Join(srcDir, entry.Name())
				destFile := filepath.Join(destDir, entry.Name())
				data, _ := os.ReadFile(srcFile)
				os.WriteFile(destFile, data, 0644)
			}
			fmt.Printf("      ✅ Copiado a: %s\n", destDir)

		case "INITIAL_PROMPT.md-exists":
			fmt.Printf("   + Creando INITIAL_PROMPT.md\n")
			promptContent := generateInitialPromptBasic(frameworkName, pkgName)
			promptPath := filepath.Join(path, "INITIAL_PROMPT.md")
			os.WriteFile(promptPath, []byte(promptContent), 0644)
			fmt.Printf("      ✅ Creado: %s\n", promptPath)

		case "AGENTS.md-exists":
			fmt.Printf("   + Creando AGENTS.md\n")
			agentsContent := generateAgentsMdBasic(frameworkName)
			agentsPath := filepath.Join(path, "AGENTS.md")
			os.WriteFile(agentsPath, []byte(agentsContent), 0644)
			fmt.Printf("      ✅ Creado: %s\n", agentsPath)

		case "README.md-exists":
			fmt.Printf("   + Creando README.md\n")
			readmeContent := fmt.Sprintf("# Framework %s\n\nCLI en Go para %s.\n\n## Uso\n\n./%s <comando>\n\n", frameworkName, frameworkName, strings.TrimPrefix(frameworkName, "framework-"))
			readmePath := filepath.Join(path, "README.md")
			os.WriteFile(readmePath, []byte(readmeContent), 0644)
			fmt.Printf("      ✅ Creado: %s\n", readmePath)

		default:
			fmt.Printf("   ~ %s (requiere intervención manual)\n", rec.ItemID)
		}
	}

	fmt.Printf("\n✅ Fixes automáticos aplicados\n")
	fmt.Printf("   Ejecuta './quine review --path %s' para verificar\n", path)
}

func cmdAnalyzeCommands(args []string) {
	fs := flag.NewFlagSet("analyze-commands", flag.ExitOnError)
	path := fs.String("path", "", "ruta del framework")
	jsonOutput := fs.Bool("json", false, "salida en JSON")

	if err := fs.Parse(args); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	if *path == "" {
		*path, _ = os.Getwd()
	}

	fmt.Printf("ANALIZANDO: %s\n\n", *path)

	analysis, err := types.AnalyzeCommands(*path)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		data, _ := json.MarshalIndent(analysis, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("=== ANALISIS SEMANTICO ===\n\n")
	fmt.Printf("Framework: %s\n", analysis.FrameworkName)
	fmt.Printf("Tipo detectado: %s\n\n", analysis.DetectedType)

	fmt.Printf("=== CATEGORIAS DETECTADAS ===\n\n")

	categoryIcons := map[string]string{
		"descubrimiento": "DC",
		"validacion":     "VA",
		"transformacion": "TR",
		"generacion":     "GE",
		"comunicacion":   "CO",
		"estado":         "ES",
		"registro":       "RE",
		"modificacion":   "MO",
	}

	for catID, cat := range types.CommandTaxonomy {
		cmds := analysis.Categories[catID]
		if len(cmds) > 0 {
			icon := categoryIcons[catID]
			if icon == "" {
				icon = "--"
			}
			fmt.Printf("[%s] %s:", icon, cat.Name)
			for _, cmd := range cmds {
				fmt.Printf(" %s", cmd)
			}
			fmt.Println()
		}
	}

	if len(analysis.Unclassified) > 0 {
		fmt.Printf("\n-- Sin categoria:")
		for _, cmd := range analysis.Unclassified {
			fmt.Printf(" %s", cmd)
		}
		fmt.Println()
	}

	fmt.Printf("\n=== COHERENCIA CON TIPO ===\n\n")
	fmt.Printf("Categorias requeridas para %s:\n", analysis.DetectedType)

	for _, req := range analysis.Required {
		cat := types.CommandTaxonomy[req]
		isPresent := false
		for _, p := range analysis.Present {
			if p == req {
				isPresent = true
				break
			}
		}
		status := "NO"
		if isPresent {
			status = "OK"
		}
		fmt.Printf("   [%s] %s\n", status, cat.Name)
	}

	if len(analysis.Missing) > 0 {
		fmt.Println()
		fmt.Printf("Categorias que faltan:\n")
		for _, miss := range analysis.Missing {
			cat := types.CommandTaxonomy[miss]
			fmt.Printf("   -- %s: %s\n", cat.Name, cat.Description)
		}
	}

	fmt.Println()
	fmt.Printf("=================================\n")
	fmt.Printf("Score: %.0f%%\n", analysis.Score)

	if analysis.IsCoherent {
		fmt.Printf("OK: Comandos apropiados para %s\n", analysis.DetectedType)
	} else if analysis.Score >= 50 {
		fmt.Printf("PARCIAL: Faltan categorias importantes\n")
	} else {
		fmt.Printf("NO: Comandos no apropiados para el tipo\n")
		fmt.Printf("   Revisa que comandos deberia tener %s\n", analysis.DetectedType)
	}
}

func analyzeRulesNeedingCommands(content string) []string {
	var issues []string
	lower := strings.ToLower(content)

	// Instrucciones que implican lógica que debería estar en código
	rulePatterns := []struct {
		pattern    string
		suggestion string
	}{
		{"deberías verificar", "Agregar comando validate o check"},
		{"debes verificar", "Agregar comando validate o check"},
		{"verifica si", "Agregar comando readiness o check"},
		{"si hay.*pero no", "El sistema debería auto-detectar esto"},
		{"cuando.*ocurre", "Agregar comando detect o monitor"},
	}

	for _, rp := range rulePatterns {
		if strings.Contains(lower, rp.pattern) {
			issues = append(issues, fmt.Sprintf("'%s' → %s", rp.pattern, rp.suggestion))
		}
	}

	return issues
}

func analyzeNarrativeIssues(content string) []string {
	var issues []string
	lower := strings.ToLower(content)

	narrativePatterns := []string{
		"recuerda", "recuerde",
		"piensa", "piénsalo",
		"deberías pensar",
		"no olvides",
		"ten en cuenta",
	}

	for _, pattern := range narrativePatterns {
		count := strings.Count(lower, pattern)
		if count > 0 {
			issues = append(issues, fmt.Sprintf("'%s' aparece %d veces (carga cognitiva)", pattern, count))
		}
	}

	// Verificar si hay secciones sin comandos concretos
	if strings.Contains(lower, "cómo") && !strings.Contains(content, "./") {
		issues = append(issues, "Se describe 'cómo' sin comandos")
	}

	return issues
}

func cmdUse(args []string) {
	fs := flag.NewFlagSet("use", flag.ExitOnError)
	fs.Usage = func() {}
	fs.Parse(args)

	if len(args) == 0 {
		fmt.Println("❌ Nombre del framework requerido")
		fmt.Println("   usa: quine use excel")
		os.Exit(1)
	}

	frameworkName := args[0]
	if !strings.HasPrefix(frameworkName, "framework-") {
		frameworkName = "framework-" + frameworkName
	}

	frameworkPath := fmt.Sprintf("/Users/alcless_a1234_cursor/remora-go/%s", frameworkName)
	if _, err := os.Stat(frameworkPath); os.IsNotExist(err) {
		fmt.Printf("❌ Framework no encontrado: %s\n", frameworkName)
		fmt.Println("   Usa 'quine list' para ver frameworks existentes")
		os.Exit(1)
	}

	initialPromptPath := filepath.Join(frameworkPath, "INITIAL_PROMPT.md")
	if _, err := os.Stat(initialPromptPath); err != nil {
		fmt.Printf("❌ No se encontró INITIAL_PROMPT.md en %s\n", frameworkName)
		os.Exit(1)
	}

	fmt.Printf("🚀 Abriendo sesión de pi con framework %s...\n\n", frameworkName)
	fmt.Println("   El INITIAL_PROMPT del framework se cargará automáticamente.")
	fmt.Println("   Escribe '/quit' para salir.")
	fmt.Println()

	trace := paladin.NewTrace("use")
	ctx := trace.Start()
	ctx.Var("framework", frameworkName)
	ctx.Var("promptFile", initialPromptPath)

	// Pasar el archivo de prompt con @ como argumento a pi
	// Esto funciona tanto en modo interactivo como --print
	cmd := exec.Command("pi", "--no-session", "@"+initialPromptPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	ctx.End()
	trace.Flush()
}

func defaultMethods() []quine.Method {
	return defaultMethodsForType("generico")
}

func defaultMethodsForType(fwType string) []quine.Method {
	if fwType == "integracion" {
		return []quine.Method{
			{
				Name:        "Register",
				Description: "Registra credenciales o configuracion requerida por la integracion",
				Example:     "./mi-framework register --env .env",
				Returns:     "*Result",
			},
			{
				Name:        "Connect",
				Description: "Conecta contra la API externa usando configuracion existente",
				Example:     "./mi-framework connect",
				Returns:     "*Result",
			},
			{
				Name:        "Validate",
				Description: "Valida credenciales, permisos y conectividad",
				Example:     "./mi-framework validate",
				Returns:     "*Result",
			},
			{
				Name:        "Status",
				Description: "Muestra estado de configuracion y conexion",
				Example:     "./mi-framework status",
				Returns:     "*Result",
			},
		}
	}

	return []quine.Method{
		{
			Name:        "Process",
			Description: "Procesa datos según el flujo del framework",
			Example:     "./mi-framework process --path ./data",
			Returns:     "*Result",
		},
		{
			Name:        "Status",
			Description: "Muestra el estado actual del framework",
			Example:     "./mi-framework status",
			Returns:     "*Result",
		},
		{
			Name:        "Validate",
			Description: "Valida datos de entrada",
			Example:     "./mi-framework validate --spec data.json",
			Returns:     "*Result",
		},
	}
}

func usage() {
	fmt.Print(`Framework Quine - Generador y revisor de frameworks

USO:
  quine create --name <nombre> --role "<rol>" --description "<descripción>" [--type <tipo>]
                  Crear un nuevo framework

  quine init --name <nombre>
                  Inicializar framework con defaults

  quine spec --create
                  Mostrar ejemplo de especificación JSON

  quine list
                  Listar frameworks existentes

  quine use <framework>
                  Abrir sesión de pi con el framework seleccionado

  quine review --path <ruta> [--json] [--register]
                  Revisar calidad de un framework
                  - --json: salida en formato JSON
                  - --register: registrar si pasa el estándar

  quine register --path <ruta> [--type <tipo>] [--role <rol>]
                  Registrar un framework existente

  quine types
                  Mostrar tipos de framework y sus checklists

  quine fix --path <ruta> [--auto]
                  Analizar y sugerir fixes para un framework
                  - --auto: aplicar fixes automáticos cuando sea posible

  quine analyze-commands --path <ruta>
                  Analizar comandos del framework por categorías semánticas
                  - --json: salida en formato JSON

  quine <nombre>
                  Atajo para quine use <nombre>

  quine help
                  Mostrar esta ayuda

TIPOS DE FRAMEWORK:
  inquisitivo    - Guías mediante preguntas y descubrimiento
  nodos-arbol    - Usa nodos jerárquicos y árboles de conocimiento
  procesador      - Procesa, transforma o analiza datos
  integracion    - Conecta sistemas o APIs
  automatizador  - Automatiza tareas repetitivas
  generico       - Propósito general

EJEMPLOS:
  # Crear framework inquisitivo
  quine create --name framework-echo --role "gestor de reuniones" --description "guía reuniones de descubrimiento" --type inquisitivo

  # Revisar un framework
  quine review --path /Users/alcless_a1234_cursor/remora-go/framework-echo

  # Registrar después de revisión
  quine review --path /Users/alcless_a1234_cursor/remora-go/framework-alfa --register

  # Ver tipos disponibles
  quine types

  # Ver frameworks registrados
  quine list

NOTAS:
  - Los frameworks se crean en /Users/alcless_a1234_cursor/remora-go/
  - Cada framework incluye Paladin integrado para tracing
  - Ver INITIAL_PROMPT.md en cada framework para instrucciones de la IA
  - El registro de frameworks se guarda en frameworks.json
`)
}

// ============================================================================
// HELPERS
// ============================================================================

func findMainGo(basePath string) string {
	cmdDir := filepath.Join(basePath, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return ""
	}

	// Ordenar para preferir el nombre sin guiones (frameworkecho vs framework-echo)
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	// Poner primero el que no tiene guiones
	for _, dir := range dirs {
		if !strings.Contains(dir, "-") {
			mainPath := filepath.Join(cmdDir, dir, "main.go")
			if _, err := os.Stat(mainPath); err == nil {
				return mainPath
			}
		}
	}
	// Luego el resto
	for _, dir := range dirs {
		if strings.Contains(dir, "-") {
			mainPath := filepath.Join(cmdDir, dir, "main.go")
			if _, err := os.Stat(mainPath); err == nil {
				return mainPath
			}
		}
	}

	return ""
}

// ============================================================================
// HELPERS PARA AUTO-FIX
// ============================================================================

func generateMainGoBasic(frameworkName string) string {
	pkg := strings.ToLower(frameworkName)
	binName := strings.TrimPrefix(frameworkName, "framework-")
	return fmt.Sprintf(`package main

import (
	"flag"
	"fmt"
	"os"

	"framework-%s/internal/paladin"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	trace := paladin.NewTrace("main")
	ctx := trace.Start()
	defer trace.Flush()

	ctx.Var("command", os.Args[1])

	switch os.Args[1] {
	case "status":
		cmdStatus()
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %%s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func cmdStatus() {
	fmt.Println("Status: OK")
	fmt.Printf("Framework: %s\n", "%s")
}

func usage() {
	fmt.Println("%s - CLI\n\nUSO:\n  %s <comando>\n\nCOMANDOS:\n  status  Muestra el estado\n  help    Muestra esta ayuda")
}
`, pkg, frameworkName, frameworkName, binName, binName)
}

func generateInitialPromptBasic(frameworkName, pkgName string) string {
	binName := strings.TrimPrefix(frameworkName, "framework-")
	return fmt.Sprintf("# Initial Prompt: Framework %s\n\nEres la IA operadora de Framework %s.\n\n## Tu filosofia\n\nEste framework es simple y directo. Solo usas los comandos disponibles.\n\n## Comandos\n\n```bash\ncd /Users/alcless_a1234_cursor/remora-go/%s\n./%s status\n```\n\n## Lo que NO necesitas hacer\n\n- No configures conexiones manualmente\n- No edites archivos JSON manualmente\n- Solo usa los comandos\n", frameworkName, frameworkName, frameworkName, binName)
}

func generateAgentsMdBasic(frameworkName string) string {
	return fmt.Sprintf("# Framework %s - Agentes\n\n## Rol\n\nFramework %s - proposito general.\n\n## Responsabilidades\n\n- Procesar datos segun comandos\n- Mantener estado en JSON\n- Reportar status\n\n## Integracion\n\nEste framework puede integrarse con otros frameworks del ecosistema.\n", frameworkName, frameworkName)
}
