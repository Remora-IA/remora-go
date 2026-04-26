package main

import (
	"flag"
	"fmt"
	"os"

	"framework-framework-echo/internal/paladin"
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
		fmt.Fprintf(os.Stderr, "comando desconocido: framework-echo\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func cmdStatus() {
	fmt.Println("Status: OK")
	fmt.Printf("Framework: echo\n", "%!s(MISSING)")
}

func usage() {
	fmt.Println("%!s(MISSING) - CLI\n\nUSO:\n  %!s(MISSING) <comando>\n\nCOMANDOS:\n  status  Muestra el estado\n  help    Muestra esta ayuda")
}
