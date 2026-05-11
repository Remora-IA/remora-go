package main

import (
	"fmt"
	"os"

	"github.com/user/framework-echo/internal/paladin"
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
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func cmdStatus() {
	fmt.Println("Status: OK")
	fmt.Println("Framework: echo")
}

func usage() {
	fmt.Println("framework-echo - CLI\n\nUSO:\n  framework-echo <comando>\n\nCOMANDOS:\n  status  Muestra el estado\n  help    Muestra esta ayuda")
}
