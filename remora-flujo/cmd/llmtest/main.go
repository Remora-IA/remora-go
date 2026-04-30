package main

import (
	"context"
	"fmt"
	"os"

	"remora-flujo/internal/llm"
)

func main() {
	c, err := llm.New(llm.Spec{
		Provider:     "groq",
		Name:         "meta-llama/llama-4-scout-17b-16e-instruct",
		EnvKey:       "GROQ_API_KEY",
		Capabilities: []string{"text", "multimodal"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERR:", err)
		os.Exit(1)
	}
	out, err := c.Complete(context.Background(), llm.CompletionRequest{
		System: "Eres un asistente que describe imágenes en español. Responde en JSON con campos: tipo, columnas, filas_aprox, observaciones.",
		User:   "Esta es una captura de una planilla. Dime qué ves.",
		Resources: []llm.Resource{
			{Type: "image", Path: "/Users/alcless_a1234_cursor/remora-go/temp/uploads/excel_1.png", MimeType: "image/png"},
		},
		MaxTokens: 800,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERR:", err)
		os.Exit(1)
	}
	fmt.Println(out)
}
