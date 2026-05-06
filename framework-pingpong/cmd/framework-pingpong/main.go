package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"framework-pingpong/internal/paladin"
	pingpong "framework-pingpong/internal/pingpong"
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
	applyDirFlag(os.Args[2:])

	switch os.Args[1] {
	case "init":
		cmdInit(os.Args[2:])
	case "start":
		cmdStart(os.Args[2:])
	case "configure":
		cmdConfigure(os.Args[2:])
	case "next":
		cmdNext(os.Args[2:])
	case "check":
		cmdCheck(os.Args[2:])
	case "accept":
		cmdAccept(os.Args[2:])
	case "set-steps":
		cmdSetSteps(os.Args[2:])
	case "subdivide":
		cmdSubdivide(os.Args[2:])
	case "scan":
		cmdScan(os.Args[2:])
	case "clean":
		cmdClean(os.Args[2:])
	case "peek":
		cmdPeek(os.Args[2:])
	case "search":
		cmdSearch(os.Args[2:])
	case "symbols":
		cmdSymbols(os.Args[2:])
	case "inspect":
		cmdInspect(os.Args[2:])
	case "verify":
		cmdVerify(os.Args[2:])
	case "run":
		cmdRun(os.Args[2:])
	case "ask":
		cmdAsk(os.Args[2:])
	case "log-qa":
		cmdLogQA(os.Args[2:])
	case "done":
		cmdDone(os.Args[2:])
	case "status":
		cmdStatus(os.Args[2:])
	case "signal":
		cmdSignal(os.Args[2:])
	case "reset":
		cmdReset(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`Framework PingPong - Tutor Iterativo

USO:
  ./pingpong init                         Inicializar proyecto
  ./pingpong start --goal "<objetivo>" [--dir path]
                                          Iniciar sesión
  ./pingpong configure --root path --files a.go,b.go
                                          Fijar scope de archivos del ejercicio
  ./pingpong next                         Mostrar el próximo mensaje autoritativo para el usuario
  ./pingpong check [--lang go|python|javascript]
                                          Revisar el paso actual sin elegir IDs
  ./pingpong accept                       Aceptar el paso actual y avanzar sin elegir IDs
  ./pingpong set-steps --steps "p1;p2;p3" Registrar pasos (la IA los define)
  ./pingpong subdivide --step <id> --substeps "s1;s2;s3"
                                          Subdividir un paso en sub-pasos más granulares
  ./pingpong scan [--auto] [--file path.go] [--lang go|python|javascript]
                                          Escanear archivo existente con compile check
  ./pingpong clean --file path.go --from N --to M [--lang go|python|javascript]
                                          Borrar quirúrgicamente un rango de líneas (solo borra)
  ./pingpong peek --file path.go --line N [--radius 3]
                                          Ver líneas alrededor de una línea específica
  ./pingpong search --query "texto" [--file path.go|--root .] [--max 20]
                                          Buscar evidencia textual en archivo o repo
  ./pingpong symbols [--file path.go|--root .]
                                          Listar símbolos Go: structs, tipos, funcs, métodos
  ./pingpong inspect [--file path.go]
                                          Inspeccionar evidencia del paso actual
  ./pingpong verify --file path.go [--lang go|python|javascript]
                                          Verificar compilación y entregar contexto para juicio IA
  ./pingpong run --file path.go [--lang go|python|javascript] [--stdin "..."] [--expect "..."]
                                          Compilar y ejecutar en sandbox (10s timeout)
  ./pingpong done --step <id>             Marcar paso completado
  ./pingpong status                       Ver progreso actual
  ./pingpong reset                        Reiniciar proyecto

FLUJO 80-20:
  1. start --goal "objetivo" → set-steps → pasos declarativos
  2. next → decir data.say literal
  3. check → la IA juzga solo el paso actual
  4. accept si corresponde; si no, feedback conceptual
  5. Al completar todos los pasos, run con casos de prueba
  6. Solo declarar completado si run pasa con output correcto
`)
}

func cmdInit(args []string) {
	trace := paladin.NewTrace("Init")
	defer trace.Flush()

	client := pingpong.New()
	result, err := client.Init()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdStart(args []string) {
	trace := paladin.NewTrace("Start")
	defer trace.Flush()

	goal := extractFlag(args, "--goal")
	steps := extractFlag(args, "--steps")

	if goal == "" {
		fmt.Println("Error: necesitas --goal \"<objetivo>\"")
		os.Exit(1)
	}

	client := pingpong.New()
	result, err := client.Start(goal, steps)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdConfigure(args []string) {
	root := extractFlag(args, "--root")
	files := extractFlag(args, "--files")
	if files == "" {
		fmt.Println("Error: necesitas --files archivo1,archivo2")
		os.Exit(1)
	}

	client := newClient(args)
	result, err := client.Configure(root, files)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdNext(args []string) {
	client := newClient(args)
	result, err := client.Next()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdCheck(args []string) {
	file := extractFlag(args, "--file")

	client := newClient(args)
	result, err := client.Check(file)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdAccept(args []string) {
	client := newClient(args)
	result, err := client.Accept()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdSetSteps(args []string) {
	trace := paladin.NewTrace("SetSteps")
	defer trace.Flush()

	steps := extractFlag(args, "--steps")
	if steps == "" {
		fmt.Println("Error: necesitas --steps \"paso1;paso2;paso3\"")
		os.Exit(1)
	}

	client := pingpong.New()
	result, err := client.SetSteps(steps)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdSubdivide(args []string) {
	trace := paladin.NewTrace("Subdivide")
	defer trace.Flush()

	stepStr := extractFlag(args, "--step")
	substeps := extractFlag(args, "--substeps")

	if stepStr == "" || substeps == "" {
		fmt.Println("Error: necesitas --step <id> --substeps \"sub1;sub2;sub3\"")
		os.Exit(1)
	}

	var stepID int
	if _, err := fmt.Sscanf(stepStr, "%d", &stepID); err != nil {
		fmt.Printf("Error: ID inválido: %s\n", stepStr)
		os.Exit(1)
	}

	client := pingpong.New()
	result, err := client.Subdivide(stepID, substeps)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdClean(args []string) {
	trace := paladin.NewTrace("Clean")
	defer trace.Flush()

	file := extractFlag(args, "--file")
	fromStr := extractFlag(args, "--from")
	toStr := extractFlag(args, "--to")

	from, err := strconv.Atoi(fromStr)
	if err != nil {
		fmt.Println("Error: necesitas --from <línea>")
		os.Exit(1)
	}
	to, err := strconv.Atoi(toStr)
	if err != nil {
		fmt.Println("Error: necesitas --to <línea>")
		os.Exit(1)
	}

	client := newClient(args)
	result, err := client.Clean(file, from, to)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdPeek(args []string) {
	file := extractFlag(args, "--file")
	lineStr := extractFlag(args, "--line")
	radiusStr := extractFlag(args, "--radius")

	line := 1
	if lineStr != "" {
		if v, err := strconv.Atoi(lineStr); err == nil {
			line = v
		}
	}
	radius := 3
	if radiusStr != "" {
		if v, err := strconv.Atoi(radiusStr); err == nil {
			radius = v
		}
	}

	client := pingpong.New()
	result, err := client.Peek(file, line, radius)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdSearch(args []string) {
	query := extractFlag(args, "--query")
	file := extractFlag(args, "--file")
	root := extractFlag(args, "--root")
	maxStr := extractFlag(args, "--max")
	max := 20
	if maxStr != "" {
		if v, err := strconv.Atoi(maxStr); err == nil {
			max = v
		}
	}

	client := newClient(args)
	result, err := client.Search(file, root, query, max)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdSymbols(args []string) {
	file := extractFlag(args, "--file")
	root := extractFlag(args, "--root")

	client := newClient(args)
	result, err := client.Symbols(file, root)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdInspect(args []string) {
	file := extractFlag(args, "--file")

	client := newClient(args)
	result, err := client.Inspect(file)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdScan(args []string) {
	trace := paladin.NewTrace("Scan")
	defer trace.Flush()

	file := extractFlag(args, "--file")
	auto := hasFlag(args, "--auto")

	client := newClient(args)
	result, err := client.Scan(file, auto)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdVerify(args []string) {
	trace := paladin.NewTrace("Verify")
	defer trace.Flush()

	file := extractFlag(args, "--file")

	client := newClient(args)
	result, err := client.Verify(file)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdRun(args []string) {
	trace := paladin.NewTrace("Run")
	defer trace.Flush()

	file := extractFlag(args, "--file")
	stdin := extractFlag(args, "--stdin")
	expected := extractFlag(args, "--expect")

	if file == "" {
		fmt.Println("Error: necesitas --file <archivo>")
		os.Exit(1)
	}

	client := newClient(args)
	result, err := client.Run(file, stdin, expected)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdAsk(args []string) {
	trace := paladin.NewTrace("Ask")
	defer trace.Flush()

	if len(args) == 0 {
		fmt.Println("Error: necesitas escribir una pregunta")
		os.Exit(1)
	}

	question := strings.Join(args, " ")
	client := pingpong.New()
	result, err := client.Ask(question)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdLogQA(args []string) {
	trace := paladin.NewTrace("LogQA")
	defer trace.Flush()

	question := extractFlag(args, "--question")
	answer := extractFlag(args, "--answer")
	purpose := extractFlag(args, "--purpose")

	if question == "" || answer == "" {
		fmt.Println("Error: necesitas --question y --answer")
		os.Exit(1)
	}

	client := pingpong.New()
	result, err := client.LogQA(question, answer, purpose)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdDone(args []string) {
	trace := paladin.NewTrace("Done")
	defer trace.Flush()

	stepID := extractFlag(args, "--step")
	if stepID == "" {
		fmt.Println("Error: necesitas --step <id>")
		os.Exit(1)
	}

	client := pingpong.New()
	result, err := client.Done(stepID)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdStatus(args []string) {
	trace := paladin.NewTrace("Status")
	defer trace.Flush()

	client := pingpong.New()
	result, err := client.Status()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdSignal(args []string) {
	trace := paladin.NewTrace("Signal")
	defer trace.Flush()

	signalType := extractFlag(args, "--type")
	note := extractFlag(args, "--note")

	if signalType == "" {
		fmt.Println("Error: necesitas --type <fatiga|confusion>")
		os.Exit(1)
	}

	client := pingpong.New()
	result, err := client.Signal(signalType, note)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func cmdReset(args []string) {
	trace := paladin.NewTrace("Reset")
	defer trace.Flush()

	client := pingpong.New()
	result, err := client.Reset()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func extractFlag(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return strings.Trim(args[i+1], "\"")
		}
	}
	return ""
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func newClient(args []string) *pingpong.Client {
	langName := extractFlag(args, "--lang")
	if langName == "" {
		langName = "go"
	}
	lang, ok := pingpong.DefaultLangConfigs[langName]
	if !ok {
		fmt.Printf("Error: lenguaje no soportado: %s\n", langName)
		os.Exit(1)
	}
	return pingpong.NewWithTrace("framework-pingpong", lang)
}

func applyDirFlag(args []string) {
	dir := extractFlag(args, "--dir")
	if dir == "" {
		return
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Printf("Error: no se pudo entrar a --dir %s: %v\n", dir, err)
		os.Exit(1)
	}
}
