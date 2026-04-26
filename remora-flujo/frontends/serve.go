package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// Servir archivos estáticos del frontend
	frontendDir := "frontends/frontend-chat"

	// API ya está corriendo en 8084, servir frontend en 8085
	fs := http.FileServer(http.Dir(frontendDir))
	http.Handle("/", fs)

	fmt.Println("🌐 Frontend en: http://localhost:8085")
	fmt.Println("📡 API en:      http://localhost:8084")
	fmt.Println("")
	fmt.Println("Abre: http://localhost:8085")
	
	log.Fatal(http.ListenAndServe(":8085", nil))
}
