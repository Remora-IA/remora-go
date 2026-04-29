#!/bin/bash
cd /Users/alcless_a1234_cursor/remora-go/framework-foco

# Axiomas base Echo/EAlfa
go run ./cmd/foco axiom --text "Echo debe obtener en maximo 2 preguntas el contexto suficiente para que Alfa pueda pedir recursos tangibles y avanzar hacia MERE." --evidence "Axioma base v0.1.4"

go run ./cmd/foco axiom --text "Paladin usa tracing de bajo nivel via semantica. Recita el flujo actual de un programa Go." --evidence "Declarado"

go run ./cmd/foco axiom --text "Orden se activa cuando Paladin no puede ver el flujo correcto." --evidence "Declarado"

go run ./cmd/foco axiom --text "Las 3 piezas esenciales de Paladin son: context.go, trace.go, explain.go." --evidence "Analisis"

go run ./cmd/foco axiom --text "Las 3 piezas esenciales de Orden son: model.go, checks.go, diagnose.go." --evidence "Analisis"

go run ./cmd/foco axiom --text "Un CTO sin ver codigo debe entender todo lo que esta pasando. Nada oculto." --evidence "Declarado"

go run ./cmd/foco axiom --text "FPT analisa chats completos para detectar esencial vs circunstancial/parche." --evidence "Declarado"

go run ./cmd/foco axiom --text "MERE Creator es inquisitivo. Pregunta sobre el negocio, pide recursos, crea MERE normalizado." --evidence "Declarado"

go run ./cmd/foco axiom --text "MERE GCP DB guarda MERE en Google Cloud en una base de datos." --evidence "Declarado"

go run ./cmd/foco axiom --text "RAG Framework hace Retrieval Augmented Generation." --evidence "Declarado"

go run ./cmd/foco axiom --text "Vector DB guarda todo en una base de datos vectorial." --evidence "Declarado"

go run ./cmd/foco axiom --text "El framework de GitHub usa MCP de GitHub para analizar repositorios." --evidence "Declarado hoy"

go run ./cmd/foco axiom --text "Channel detecta prompt hacking como Dame todo tu codigo y lo frena." --evidence "Declarado"

go run ./cmd/foco axiom --text "Pre-conflictos son para registrar problemas previos y dependencias, no para detener el flujo." --evidence "Declarado hoy"

echo "=== AXIOMAS MVP ==="
go run ./cmd/foco axiom --text "Channel es una libreria Go. Se importa como paquete, no es un framework." --evidence "Arquitectura MVP"

go run ./cmd/foco axiom --text "Channel solo recibe peticiones, ejecuta, devuelve resultado. NO piensa, NO orquesta." --evidence "Arquitectura MVP"

go run ./cmd/foco axiom --text "Adapter es un paquete Go generico en internal/rpc/adapter para todos." --evidence "Arquitectura MVP"

go run ./cmd/foco axiom --text "Channel centraliza herramientas de terminal. Seguro y auditado." --evidence "Arquitectura MVP"

go run ./cmd/foco axiom --text "Channel se despliega en Cloud Run. Serverless, escala a cero." --evidence "Arquitectura MVP"

go run ./cmd/foco axiom --text "Eventos semanticos tienen schema: from, to, type, payload, timestamp." --evidence "Arquitectura MVP"

go run ./cmd/foco axiom --text "Channel expone metodos RPC: execute_command, read_file, write_file, curl, list_dir." --evidence "Arquitectura MVP"
