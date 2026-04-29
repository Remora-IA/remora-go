#!/bin/bash
cd /Users/alcless_a1234_cursor/remora-go/framework-foco

# Echo/EAlfa
./foco axiom --text "Echo debe obtener en maximo 2 preguntas el contexto suficiente para que Alfa pueda pedir recursos tangibles." --evidence "Axioma base v0.1.4"
./foco axiom --text "En pregunta 3, Alfa deja preguntas en documento y Echo las formula una por una." --evidence "Axioma base"
./foco axiom --text "Las preguntas de Alfa hacia el usuario via Echo usan contexto del arbol de conocimiento." --evidence "Axioma base"
./foco axiom --text "Si el usuario no puede dar lo que Alfa pidio pero ofrece alternativa, Echo pregunta sobre lo que tiene." --evidence "Axioma base"

# Paladin/Orden
./foco axiom --text "Paladin usa tracing de bajo nivel via semantica. Recita el flujo actual de un programa Go." --evidence "Declarado"
./foco axiom --text "Orden se activa cuando Paladin no puede ver el flujo correcto." --evidence "Declarado"
./foco axiom --text "Las 3 piezas esenciales de Paladin son: context.go, trace.go, explain.go." --evidence "Analisis"
./foco axiom --text "Las 3 piezas esenciales de Orden son: model.go, checks.go, diagnose.go." --evidence "Analisis"
./foco axiom --text "Paladin debe poder decir si lo esta haciendo o no." --evidence "Declarado"
./foco axiom --text "Un CTO sin ver codigo debe entender todo. Nada oculto." --evidence "Declarado"

# FPT
./foco axiom --text "FPT analisa chats completos para detectar esencial vs circunstancial/parche." --evidence "Declarado"
./foco axiom --text "FPT pregunta que pasaria si X no estuviera. Se seguiria cumpliendo el WHY?" --evidence "Declarado"
./foco axiom --text "FPT usa comandos siempre, no depende de prompts." --evidence "Declarado"

# MERE/RAG/Vector
./foco axiom --text "MERE Creator es inquisitivo. Pregunta sobre el negocio, pide recursos, crea MERE normalizado." --evidence "Declarado"
./foco axiom --text "MERE GCP DB guarda MERE en Google Cloud." --evidence "Declarado"
./foco axiom --text "RAG Framework hace Retrieval Augmented Generation." --evidence "Declarado"
./foco axiom --text "Vector DB guarda todo en una base de datos vectorial." --evidence "Declarado"

# GitHub
./foco axiom --text "El framework de GitHub usa MCP de GitHub para analizar repositorios." --evidence "Declarado"

# Channel
./foco axiom --text "Channel detecta prompt hacking y lo frena." --evidence "Declarado"
./foco axiom --text "Channel usa JSON-RPC para que 2 IAs hablen entre si." --evidence "Declarado"
./foco axiom --text "Channel le da herramientas de terminal a cualquier IA que utilize la linea RPC." --evidence "Declarado"

# Channel MVP
./foco axiom --text "Channel es una libreria Go. Se importa como paquete, no es un framework. NO piensa, NO orquesta." --evidence "Arquitectura MVP"
./foco axiom --text "Channel solo recibe peticiones, ejecuta, devuelve resultado." --evidence "Arquitectura MVP"
./foco axiom --text "Adapter es un paquete Go generico en internal/rpc/adapter para todos los frameworks." --evidence "Arquitectura MVP"
./foco axiom --text "Channel centraliza herramientas de terminal. Seguro y auditado." --evidence "Arquitectura MVP"
./foco axiom --text "Channel se despliega en Cloud Run. Serverless, escala a cero." --evidence "Arquitectura MVP"
./foco axiom --text "Eventos semanticos tienen schema: from, to, type, payload, timestamp." --evidence "Arquitectura MVP"
./foco axiom --text "Channel expone metodos RPC: execute_command, read_file, write_file, curl, list_dir." --evidence "Arquitectura MVP"

# Foco
./foco axiom --text "Pre-conflictos son para registrar problemas previos y dependencias, no para detener el flujo." --evidence "Declarado hoy"
