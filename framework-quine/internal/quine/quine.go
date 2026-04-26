// Package quine es el generador de frameworks.
package quine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	files = append(files, FileSpec{
		Path:    filepath.Join(baseDir, "go.mod"),
		Content: generateGoMod(spec),
		Type:    "go",
	})

	dirs := []string{
		filepath.Join(baseDir, "cmd", spec.Name),
		filepath.Join(baseDir, "internal", strings.ToLower(spec.Name)),
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
		Path:    filepath.Join(baseDir, "internal", strings.ToLower(spec.Name), "client.go"),
		Content: generateClientGo(spec),
		Type:    "go",
	})

	files = append(files, FileSpec{
		Path:    filepath.Join(baseDir, "internal", strings.ToLower(spec.Name), "types.go"),
		Content: generateTypesGo(spec),
		Type:    "go",
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
	pkg := strings.ToLower(spec.Name)
	return fmt.Sprintf("module framework-%s\n\ngo 1.24.0\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.12.0\n)\n", pkg)
}

func generateMainGo(spec FrameworkSpec) string {
	var switchCases string
	var cmdMethods string
	var helpLines string = "Framework " + spec.Name + " - CLI\n\nUSO:\n"

	pkg := strings.ToLower(spec.Name)

	for _, method := range spec.Methods {
		cmdName := strings.ToLower(method.Name)
		switchCases += fmt.Sprintf("\tcase \"%s\":\n\t\tcmd%s(args)\n", cmdName, method.Name)
		helpLines += "  " + spec.Name + " " + cmdName + "\n"
		cmdMethods += fmt.Sprintf(`
func cmd%s(args []string) {
	trace := paladin.NewTrace("%s")
	ctx := trace.Start()
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
`, method.Name, method.Name, strings.Title(spec.Name), method.Name)
	}

	return fmt.Sprintf(`package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"framework-%s/internal/%s"
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
%s
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %%s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("%s")
}
%s`, pkg, pkg, pkg, switchCases, helpLines, cmdMethods)
}

func generateClientGo(spec FrameworkSpec) string {
	pkg := strings.ToLower(spec.Name)

	var methods string
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

	return fmt.Sprintf(`package %s

import (
	"time"

	"framework-%s/internal/paladin"
)

// Client es el cliente principal del framework.
type Client struct {
	trace *paladin.Trace
	ctx   *paladin.Context
}

// New crea un nuevo cliente.
func New() *Client {
	return &Client{}
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
%s`, pkg, pkg, methods)
}

func generateTypesGo(spec FrameworkSpec) string {
	pkg := strings.ToLower(spec.Name)
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

func generateInitialPrompt(spec FrameworkSpec) string {
	var methodsMd string
	for _, method := range spec.Methods {
		methodsMd += "\n### " + method.Name + "\n\n" + method.Description + "\n\nEjemplo: " + method.Example + "\n\nRetorna: " + method.Returns + "\n"
	}

	return "# Initial Prompt: Framework " + spec.Name + "\n\n" +
		"Eres la IA operadora de Framework " + spec.Name + ".\n\n" +
		"Tu trabajo es " + spec.Role + ".\n\n" +
		"## Tu filosofia\n\n" +
		"Este framework hace " + spec.Description + ". Solo usas los metodos disponibles.\n\n" +
		"## Metodos disponibles\n\n" + methodsMd + "\n\n" +
		"## Comandos CLI\n\n" +
		"```bash\ncd /Users/alcless_a1234_cursor/remora-go/" + spec.Name + "\n./" + spec.Name + " <comando>\n```\n\n" +
		"## Ejemplo de uso\n\n" + spec.Methods[0].Example + "\n\n" +
		"## Lo que NO necesitas hacer\n\n" +
		"- No configures conexiones internamente\n" +
		"- No manejes archivos manualmente\n" +
		"- Solo usa los metodos\n"
}

func generateReadme(spec FrameworkSpec) string {
	return "# Framework " + spec.Name + "\n\n" +
		spec.Description + "\n\n" +
		"## Instalacion\n\n" +
		"```bash\ncd /Users/alcless_a1234_cursor/remora-go/" + spec.Name + "\ngo mod tidy\ngo build -o " + strings.TrimPrefix(spec.Name, "framework-") + " ./cmd/" + spec.Name + "/\n```\n\n" +
		"## Uso\n\n./" + strings.TrimPrefix(spec.Name, "framework-") + " <comando>\n\n" +
		"## Comandos\n\n- process: Procesa datos\n- status: Muestra estado\n\n"
}

func generateAgentsMd(spec FrameworkSpec) string {
	return "# Framework " + spec.Name + " - Agentes\n\n" +
		"## Rol\n\n" + spec.Role + "\n\n" +
		"## Responsabilidades\n\n" + spec.Purpose + "\n\n"
}

func createBuildFiles(spec FrameworkSpec, outputDir string) error {
	binName := strings.TrimPrefix(spec.Name, "framework-")
	makefile := ".PHONY: build clean test\n\nbuild:\n\tgo build -o " + binName + " ./cmd/" + spec.Name + "/\n\nclean:\n\trm -f " + binName + "\n\trm -rf temp/\n\ntest:\n\tgo test ./...\n\ndev:\n\tgo run ./cmd/" + spec.Name + "/\n"

	if err := os.WriteFile(filepath.Join(outputDir, "Makefile"), []byte(makefile), 0644); err != nil {
		return err
	}

	return nil
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