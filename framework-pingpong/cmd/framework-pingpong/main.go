package main

import (
	"encoding/json"
	"fmt"
	"os"
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

	switch os.Args[1] {
	case "init":
		cmdInit(os.Args[2:])
	case "start":
		cmdStart(os.Args[2:])
	case "set-steps":
		cmdSetSteps(os.Args[2:])
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
  ./pingpong start --goal "<objetivo>"    Iniciar sesión
  ./pingpong set-steps --steps "p1;p2;p3" Registrar pasos (la IA los define)
  ./pingpong verify --file path.go        Verificar paso actual (syntax + type-check + AST)
  ./pingpong run --file path.go [--stdin "..."] [--expect "..."]
                                          Compilar y ejecutar en sandbox (10s timeout)
  ./pingpong done --step <id>             Marcar paso completado
  ./pingpong status                       Ver progreso actual
  ./pingpong reset                        Reiniciar proyecto

FLUJO:
  1. start --goal "objetivo" → set-steps → pasos declarativos
  2. verify paso a paso → mini-tests cada 3 pasos
  3. Al completar todos los pasos, run con casos de prueba
  4. Solo declarar completado si run pasa con output correcto
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

func cmdVerify(args []string) {
	trace := paladin.NewTrace("Verify")
	defer trace.Flush()

	file := extractFlag(args, "--file")

	client := pingpong.New()
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

	client := pingpong.New()
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