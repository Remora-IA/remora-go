// Package quine es el generador de frameworks.
package quine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"framework-quine/internal/paladin"
)

// ============================================================================
// TIPOS
// ============================================================================

// FrameworkSpec define qué tipo de framework quiere crear el usuario.
type FrameworkSpec struct {
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Purpose     string   `json:"purpose"`
	Methods     []Method `json:"methods"`
	CLICommands []string `json:"cli_commands"`
}

// Method describe una función que el framework expone.
type Method struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Example     string `json:"example"`
	Returns     string `json:"returns"`
}

// GeneratedFramework contiene toda la estructura de un framework generado.
type GeneratedFramework struct {
	TraceID   string        `json:"trace_id"`
	Generated string        `json:"generated"`
	Spec      FrameworkSpec `json:"spec"`
	Files     []FileSpec    `json:"files"`
	Status    string        `json:"status"`
}

// FileSpec define un archivo a crear.
type FileSpec struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Type    string `json:"type"`
}

// ============================================================================
// FUNCIÓN PRINCIPAL
// ============================================================================

// Generate crea un framework completo basado en la especificación.
func Generate(spec FrameworkSpec, outputDir string) (*GeneratedFramework, error) {
	trace := paladin.NewTrace("quine-generate")
	ctx := trace.Start()
	defer trace.Flush()

	ctx.Var("frameworkName", spec.Name)
	ctx.Var("outputDir", outputDir)
	ctx.Var("methodsCount", len(spec.Methods))

	if spec.Name == "" {
		return nil, fmt.Errorf("el nombre del framework es requerido")
	}

	if outputDir == "" {
		outputDir = fmt.Sprintf("/Users/alcless_a1234_cursor/remora-go/%s", spec.Name)
	}

	ctx.Var("finalOutputDir", outputDir)

	if _, err := os.Stat(outputDir); err == nil {
		return nil, fmt.Errorf("el framework %s ya existe", spec.Name)
	}

	ctx.Child("createStructure")
	files, err := createFrameworkStructure(spec, outputDir)
	if err != nil {
		ctx.Error(err)
		return nil, err
	}
	ctx.End()

	ctx.Child("writeFiles")
	if err := writeFiles(files); err != nil {
		ctx.Error(err)
		return nil, err
	}
	ctx.End()

	ctx.Child("createBuild")
	if err := createBuildFiles(spec, outputDir); err != nil {
		ctx.Error(err)
		return nil, err
	}
	ctx.End()

	result := &GeneratedFramework{
		TraceID:   trace.GetID(),
		Generated: time.Now().Format(time.RFC3339),
		Spec:      spec,
		Files:     files,
		Status:    "created",
	}

	ctx.Var("filesCreated", len(files))
	ctx.Decision("framework-generado", fmt.Sprintf("se crearon %d archivos", len(files)))

	return result, nil
}

// ============================================================================
// CREAR ESTRUCTURA
// ============================================================================

func createFrameworkStructure(spec FrameworkSpec, baseDir string) ([]FileSpec, error) {
	var files []FileSpec
	pkgDir := packageDir(spec.Name)

	files = append(files, FileSpec{
		Path:    filepath.Join(baseDir, "go.mod"),
		Content: generateGoMod(spec),
		Type:    "go",
	})

	dirs := []string{
		filepath.Join(baseDir, "cmd", spec.Name),
		filepath.Join(baseDir, "internal", pkgDir),
		filepath.Join(baseDir, "internal", "paladin"),
		filepath.Join(baseDir, "temp", "paladin"),
	}
	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
	}

	if err := copyPaladin(filepath.Join(baseDir, "internal", "paladin")); err != nil {
		return nil, err
	}

	files = append(files, FileSpec{
		Path:    filepath.Join(baseDir, "cmd", spec.Name, "main.go"),
		Content: generateMainGo(spec),
		Type:    "go",
	})

	files = append(files, FileSpec{
		Path:    filepath.Join(baseDir, "internal", pkgDir, "client.go"),
		Content: generateClientGo(spec),
		Type:    "go",
	})

	files = append(files, FileSpec{
		Path:    filepath.Join(baseDir, "internal", pkgDir, "types.go"),
		Content: generateTypesGo(spec),
		Type:    "go",
	})

	files = append(files, FileSpec{
		Path:    filepath.Join(baseDir, "internal", pkgDir, "client_test.go"),
		Content: generateClientTestGo(spec),
		Type:    "go",
	})

	files = append(files, FileSpec{
		Path:    filepath.Join(baseDir, "WHY.md"),
		Content: generateWhy(spec),
		Type:    "md",
	})

	files = append(files, FileSpec{
		Path:    filepath.Join(baseDir, "INITIAL_PROMPT.md"),
		Content: generateInitialPrompt(spec),
		Type:    "md",
	})

	files = append(files, FileSpec{
		Path:    filepath.Join(baseDir, "README.md"),
		Content: generateReadme(spec),
		Type:    "md",
	})

	files = append(files, FileSpec{
		Path:    filepath.Join(baseDir, "AGENTS.md"),
		Content: generateAgentsMd(spec),
		Type:    "md",
	})

	return files, nil
}

func copyPaladin(destDir string) error {
	srcDir := "/Users/alcless_a1234_cursor/remora-go/framework-paladin/paladin"
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		srcFile := filepath.Join(srcDir, entry.Name())
		destFile := filepath.Join(destDir, entry.Name())

		data, err := os.ReadFile(srcFile)
		if err != nil {
			return err
		}

		if err := os.WriteFile(destFile, data, 0644); err != nil {
			return err
		}
	}

	return nil
}

func writeFiles(files []FileSpec) error {
	for _, file := range files {
		dir := filepath.Dir(file.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(file.Path, []byte(file.Content), 0644); err != nil {
			return err
		}
	}
	return nil
}

// ============================================================================
// GENERADORES
// ============================================================================

func generateGoMod(spec FrameworkSpec) string {
	return fmt.Sprintf("module %s\n\ngo 1.24.0\n", moduleName(spec.Name))
}

func generateMainGo(spec FrameworkSpec) string {
	var switchCases string
	var cmdMethods string
	binName := binaryName(spec.Name)
	pkgDir := packageDir(spec.Name)
	pkgIdent := packageIdent(spec.Name)
	module := moduleName(spec.Name)
	var helpLines string = "Framework " + spec.Name + " - CLI\\n\\nUSO:\\n"

	for _, method := range spec.Methods {
		cmdName := strings.ToLower(method.Name)
		switchCases += fmt.Sprintf("\tcase \"%s\":\n\t\tcmd%s(os.Args[2:])\n", cmdName, method.Name)
		helpLines += "  ./" + binName + " " + cmdName + "\\n"
		cmdMethods += fmt.Sprintf(`
func cmd%s(args []string) {
	trace := paladin.NewTrace("%s")
	defer trace.Flush()

	client := %s.New()
	result, err := client.%s()
	if err != nil {
		fmt.Printf("Error: %%v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}
`, method.Name, method.Name, pkgIdent, method.Name)
	}

	return fmt.Sprintf(`package main

import (
	"encoding/json"
	"fmt"
	"os"

	%s "%s/internal/%s"
	"%s/internal/paladin"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	trace := paladin.NewTrace("main")
	ctx := trace.Start()
	defer trace.Flush()

	ctx.Var("command", os.Args[1])

	switch os.Args[1] {
%s
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %%s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print("%s")
}
%s`, pkgIdent, module, pkgDir, module, switchCases, helpLines, cmdMethods)
}

func generateClientGo(spec FrameworkSpec) string {
	pkg := packageIdent(spec.Name)
	module := moduleName(spec.Name)
	imports := `"time"`

	var methods string
	if spec.Type == "integracion" {
		methods = generateIntegrationMethods(spec)
		imports = `"fmt"
	"os"
	"strings"`
	} else {
		for _, method := range spec.Methods {
			methods += fmt.Sprintf(`

// %s %s
func (c *Client) %s() (interface{}, error) {
	childCtx := c.ctx.Child("%s")
	defer childCtx.End()
	childCtx.Var("timestamp", time.Now().Format(time.RFC3339))
	childCtx.Decision("ejecutando-%s", "%s")
	return map[string]interface{}{"status": "ok", "method": "%s"}, nil
}
`, method.Name, method.Description, method.Name, method.Name, method.Name, method.Description, method.Name)
		}
	}

	return fmt.Sprintf(`package %s

import (
	%s

	"%s/internal/paladin"
)

// Client es el cliente principal del framework.
type Client struct {
	trace *paladin.Trace
	ctx   *paladin.Context
}

// New crea un nuevo cliente.
func New() *Client {
	return NewWithTrace("%s")
}

// NewWithTrace crea un cliente con tracing activo.
func NewWithTrace(name string) *Client {
	trace := paladin.NewTrace(name)
	ctx := trace.Start()
	return &Client{trace: trace, ctx: ctx}
}

// Flush guarda el trace actual.
func (c *Client) Flush() {
	if c.trace != nil {
		c.trace.Flush()
	}
}
%s`, pkg, imports, module, spec.Name, methods)
}

func generateIntegrationMethods(spec FrameworkSpec) string {
	envLiteral := fmt.Sprintf("[]string{%s}", quoteList(integrationRequiredEnv(spec.Name)))
	return fmt.Sprintf(`
func requiredEnv() []string {
	return %s
}

func missingEnv() []string {
	var missing []string
	for _, key := range requiredEnv() {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

// Register informa la configuracion requerida para habilitar la integracion.
func (c *Client) Register() (interface{}, error) {
	childCtx := c.ctx.Child("Register")
	defer childCtx.End()
	missing := missingEnv()
	childCtx.Var("missing_env", missing)
	return map[string]interface{}{
		"status": "registration_required",
		"required_env": requiredEnv(),
		"missing_env": missing,
		"next_action": "configurar credenciales y volver a ejecutar validate",
	}, nil
}

// Connect conecta con la API externa solo si la configuracion esta completa.
func (c *Client) Connect() (interface{}, error) {
	if _, err := c.Validate(); err != nil {
		return nil, err
	}
	childCtx := c.ctx.Child("Connect")
	defer childCtx.End()
	childCtx.Decision("conexion-autorizada", "credenciales presentes")
	return map[string]interface{}{"status": "ready_to_connect", "service": "%s"}, nil
}

// Validate valida credenciales locales antes de intentar comunicarse con la API.
func (c *Client) Validate() (interface{}, error) {
	childCtx := c.ctx.Child("Validate")
	defer childCtx.End()
	missing := missingEnv()
	if len(missing) > 0 {
		childCtx.Var("missing_env", missing)
		return nil, fmt.Errorf("credenciales faltantes: %%s; ejecutar register para ver configuracion requerida", strings.Join(missing, ", "))
	}
	return map[string]interface{}{"status": "valid", "required_env": requiredEnv()}, nil
}

// Status muestra si la integracion esta lista sin fingir una conexion.
func (c *Client) Status() (interface{}, error) {
	childCtx := c.ctx.Child("Status")
	defer childCtx.End()
	missing := missingEnv()
	return map[string]interface{}{
		"configured": len(missing) == 0,
		"missing_env": missing,
		"required_env": requiredEnv(),
	}, nil
}
`, envLiteral, spec.Name)
}

func generateTypesGo(spec FrameworkSpec) string {
	pkg := packageIdent(spec.Name)
	return "package " + pkg + "\n\n" +
		"// Spec define la estructura del framework.\n" +
		"type Spec struct {\n" +
		"\tName        string\n" +
		"\tRole        string\n" +
		"\tDescription string\n" +
		"\tCreatedAt   string\n" +
		"}\n\n" +
		"// Result representa el resultado de una operacion.\n" +
		"type Result struct {\n" +
		"\tSuccess   bool\n" +
		"\tData      interface{}\n" +
		"\tError     string\n" +
		"\tTimestamp string\n" +
		"}\n"
}

func generateClientTestGo(spec FrameworkSpec) string {
	pkg := packageIdent(spec.Name)
	var calls []string
	for _, method := range spec.Methods {
		if spec.Type == "integracion" && (method.Name == "Connect" || method.Name == "Validate") {
			continue
		}
		calls = append(calls, fmt.Sprintf(`
	if _, err := client.%s(); err != nil {
		t.Fatalf("%s returned error: %%v", err)
	}
`, method.Name, method.Name))
	}
	return fmt.Sprintf(`package %s

import "testing"

func TestClientMethods(t *testing.T) {
	client := New()
	defer client.Flush()
%s
}
`, pkg, strings.Join(calls, ""))
}

func generateInitialPrompt(spec FrameworkSpec) string {
	var methodsMd string
	binName := binaryName(spec.Name)
	for _, method := range spec.Methods {
		methodsMd += "\n### " + method.Name + "\n\n" + method.Description + "\n\nEjemplo: " + method.Example + "\n\nRetorna: " + method.Returns + "\n"
	}

	return "# Initial Prompt: Framework " + spec.Name + "\n\n" +
		"Eres la IA operadora de Framework " + spec.Name + ".\n\n" +
		"Tu trabajo es " + spec.Role + ". Lee WHY.md para entender el tipo y limite del framework.\n\n" +
		"## Filosofia\n\n" +
		"No razones pasos internos. Ejecuta comandos. Si falta configuracion o credenciales, usa el comando de validacion/status y reporta el bloqueo exacto.\n\n" +
		"## Metodos disponibles\n\n" + methodsMd + "\n\n" +
		"## Comandos CLI\n\n" +
		"```bash\ncd /Users/alcless_a1234_cursor/remora-go/" + spec.Name + "\n./" + binName + " <comando>\n```\n\n" +
		"## Ejemplo de uso\n\n" + spec.Methods[0].Example + "\n\n" +
		"## Lo que NO necesitas hacer\n\n" +
		"- No configures conexiones internamente\n" +
		"- No edites archivos JSON manualmente\n" +
		"- No manejes archivos manualmente\n" +
		"- Solo usa los metodos\n"
}

func generateReadme(spec FrameworkSpec) string {
	binName := binaryName(spec.Name)
	return "# Framework " + spec.Name + "\n\n" +
		spec.Description + "\n\n" +
		"## Instalacion\n\n" +
		"```bash\ncd /Users/alcless_a1234_cursor/remora-go/" + spec.Name + "\ngo test ./...\ngo build -o " + binName + " ./cmd/" + spec.Name + "/\n```\n\n" +
		"## Uso\n\n./" + binName + " <comando>\n\n" +
		"## Comandos\n\n- process: Procesa datos\n- status: Muestra estado\n\n"
}

func generateWhy(spec FrameworkSpec) string {
	return "# WHY: " + spec.Name + "\n\n" +
		"## Proposito\n\n" + spec.Purpose + "\n\n" +
		"## Tipo\n\n" + spec.Type + "\n\n" +
		"## Rol operativo\n\n" + spec.Role + "\n\n" +
		"## Limite de razonamiento\n\n" +
		"La IA operadora no debe inventar pasos internos. Debe ejecutar los comandos del framework y dejar que el codigo aplique las reglas de negocio.\n\n" +
		"## Criterio de funcionamiento\n\n" +
		"El framework funciona si `go test ./...` pasa y sus comandos principales responden sin depender de instrucciones narrativas del prompt.\n"
}

func generateAgentsMd(spec FrameworkSpec) string {
	return "# Framework " + spec.Name + " - Agentes\n\n" +
		"## Rol\n\n" + spec.Role + "\n\n" +
		"## Responsabilidades\n\n" + spec.Purpose + "\n\n"
}

func createBuildFiles(spec FrameworkSpec, outputDir string) error {
	binName := binaryName(spec.Name)
	makefile := ".PHONY: build clean test\n\nbuild:\n\tgo build -o " + binName + " ./cmd/" + spec.Name + "/\n\nclean:\n\trm -f " + binName + "\n\trm -rf temp/\n\ntest:\n\tgo test ./...\n\ndev:\n\tgo run ./cmd/" + spec.Name + "/\n"

	if err := os.WriteFile(filepath.Join(outputDir, "Makefile"), []byte(makefile), 0644); err != nil {
		return err
	}

	return nil
}

func moduleName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if !strings.HasPrefix(name, "framework-") {
		name = "framework-" + name
	}
	return name
}

func binaryName(name string) string {
	return strings.TrimPrefix(moduleName(name), "framework-")
}

func packageDir(name string) string {
	return packageIdent(name)
}

func packageIdent(name string) string {
	base := binaryName(name)
	base = strings.ReplaceAll(base, "-", "_")
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	base = re.ReplaceAllString(base, "_")
	if base == "" {
		return "framework"
	}
	if base[0] >= '0' && base[0] <= '9' {
		base = "fw_" + base
	}
	return base
}

func integrationRequiredEnv(name string) []string {
	if strings.Contains(strings.ToLower(name), "whatsapp") {
		return []string{"WHATSAPP_ACCESS_TOKEN", "WHATSAPP_PHONE_NUMBER_ID"}
	}
	prefix := strings.ToUpper(strings.ReplaceAll(binaryName(name), "-", "_"))
	return []string{prefix + "_API_TOKEN"}
}

func quoteList(items []string) string {
	quoted := make([]string, 0, len(items))
	for _, item := range items {
		quoted = append(quoted, fmt.Sprintf("%q", item))
	}
	return strings.Join(quoted, ", ")
}

// ============================================================================
// UTILIDADES
// ============================================================================

// LoadSpec carga una especificacion desde un archivo JSON.
func LoadSpec(path string) (*FrameworkSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec FrameworkSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// SaveSpec guarda una especificacion en un archivo JSON.
func SaveSpec(spec *FrameworkSpec, path string) error {
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// CreateDefaultSpec crea una especificacion basica.
func CreateDefaultSpec(name, role, description string) FrameworkSpec {
	return FrameworkSpec{
		Name:        name,
		Role:        role,
		Description: description,
		Purpose:     "Framework " + name + " creado por Quine",
		Methods: []Method{
			{
				Name:        "Process",
				Description: "Procesa datos segun el flujo del framework",
				Example:     "./mi-framework process",
				Returns:     "*Result",
			},
			{
				Name:        "Status",
				Description: "Muestra el estado actual del framework",
				Example:     "./mi-framework status",
				Returns:     "*Result",
			},
		},
		CLICommands: []string{"process", "status", "help"},
	}
}

// ListFrameworks retorna los frameworks existentes.
func ListFrameworks(baseDir string) ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	var frameworks []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "framework-") {
			frameworks = append(frameworks, entry.Name())
		}
	}
	return frameworks, nil
}
