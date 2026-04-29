# Foco Diario - 2026-04-28

- Version foco: `v0.3.0`
- Resultado: usuarios prueban el sistema en producción

## Resumen Primario

```text
RESULTADO OBSERVABLE AL FINAL DEL DIA:
usuarios prueban el sistema en producción

WHY DE HOY:
Channel es la base para que 2 IAs cooperen en producción

PROXIMA TAREA:
No definida todavia.

TAREAS PARA HOY:
- No hay tareas de hoy definidas.

AXIOMAS RELACIONADOS A HOY:
- No hay axiomas vinculados a las tareas de hoy.
```

## Axiomas

- Echo obtiene contexto en maximo 2 preguntas
- En pregunta 3, Alfa deja preguntas en documento
- Preguntas deben usar contexto del arbol de conocimiento
- Si usuario ofrece alternativa, Echo pregunta sobre lo que tiene
- Paladin usa tracing de bajo nivel via semantica
- Orden se activa cuando Paladin no puede ver el flujo
- Al final debemos comprobar Paladin y Orden en 3 ejemplos
- Paladin debe poder decir si lo esta haciendo o no
- Piezas Paladin: context.go, trace.go, explain.go
- Piezas Orden: model.go, checks.go, diagnose.go
- CTO sin ver codigo debe entender todo. Nada oculto
- Semantic crea dictionary en cada dir que analiza
- FPT detecta esencial vs circunstancial/parche
- FPT pregunta: que pasaria si X no estuviera?
- FPT usa comandos, no depende de prompts
- MERE Creator es inquisitivo, pide recursos
- MERE GCP DB guarda en Google Cloud
- RAG Framework hace Retrieval Augmented Generation
- Vector DB guarda en base de datos vectorial
- Channel establece linea RPC entre 2 IAs
- Channel detecta prompt hacking
- Channel da herramientas de terminal a IAs
- Channel debe investigar pi/tau para replicar en Go
- Channel utiliza handoff
- Framework de eventos crea schema normalizado
- Pre Conflicto se arregla antes de tarea principal

## Decisiones

- migracion codex
- normalizacion de invariantes
- aplicar normalizacion final

## Pre Conflictos

- [OPEN] pc_001: Semantic esta roto
- [OPEN] pc_002: Resolver lo necesario antes de continuar con "Corregir Semantic"
- [OPEN] pc_003: Resolver lo necesario antes de continuar con "Corregir Paladin"
- [OPEN] pc_004: Resolver lo necesario antes de continuar con "Corregir Events"

## Arbol De Foco

- [ax_001] AXIOM: Echo obtiene contexto en maximo 2 preguntas
- [ax_002] AXIOM: En pregunta 3, Alfa deja preguntas en documento
- [ax_003] AXIOM: Preguntas deben usar contexto del arbol de conocimiento
- [ax_004] AXIOM: Si usuario ofrece alternativa, Echo pregunta sobre lo que tiene
- [ax_005] AXIOM: Paladin usa tracing de bajo nivel via semantica
- [ax_006] AXIOM: Orden se activa cuando Paladin no puede ver el flujo
- [ax_007] AXIOM: Al final debemos comprobar Paladin y Orden en 3 ejemplos
- [ax_008] AXIOM: Paladin debe poder decir si lo esta haciendo o no
- [ax_009] AXIOM: Piezas Paladin: context.go, trace.go, explain.go
- [ax_010] AXIOM: Piezas Orden: model.go, checks.go, diagnose.go
- [ax_011] AXIOM: CTO sin ver codigo debe entender todo. Nada oculto
- [ax_012] AXIOM: Semantic crea dictionary en cada dir que analiza
- [ax_013] AXIOM: FPT detecta esencial vs circunstancial/parche
- [ax_014] AXIOM: FPT pregunta: que pasaria si X no estuviera?
- [ax_015] AXIOM: FPT usa comandos, no depende de prompts
- [ax_016] AXIOM: MERE Creator es inquisitivo, pide recursos
- [ax_017] AXIOM: MERE GCP DB guarda en Google Cloud
- [ax_018] AXIOM: RAG Framework hace Retrieval Augmented Generation
- [ax_019] AXIOM: Vector DB guarda en base de datos vectorial
- [ax_020] AXIOM: Channel establece linea RPC entre 2 IAs
- [ax_021] AXIOM: Channel detecta prompt hacking
- [ax_022] AXIOM: Channel da herramientas de terminal a IAs
- [ax_023] AXIOM: Channel debe investigar pi/tau para replicar en Go
- [ax_024] AXIOM: Channel utiliza handoff
- [ax_025] AXIOM: Framework de eventos crea schema normalizado
- [ax_026] AXIOM: Pre Conflicto se arregla antes de tarea principal

## Checklist De Ejecucion

- [done] task_pal_001: Identificar piezas esenciales de Paladin
  event: evt_001
- [done] task_ord_001: Identificar piezas esenciales de Orden
  event: evt_001
- [done] task_evt_001: Crear framework Events
  event: evt_001
- [todo] task_sem_fix_001: Corregir Semantic
  event: evt_001
  pre_conflict: pc_002
- [todo] task_pal_fix_001: Corregir Paladin
  event: evt_001
  pre_conflict: pc_003
- [todo] task_evt_fix_001: Corregir Events
  event: evt_001
  pre_conflict: pc_004
- [todo] task_ch_001: Crear Channel
  event: evt_001
- [todo] task_fpt_001: Crear FPT
  event: evt_001
- [todo] task_sem_001: Crear Semantic
  event: evt_001
- [todo] task_pal_002: Demo Snake Game
  event: evt_001
- [todo] task_pal_003: Demo API Ecommerce
  event: evt_001
- [todo] task_pal_004: Demo IAs via RPC
  event: evt_001
- [todo] task_ord_002: Demo Orden (fallback)
  event: evt_001
- [todo] task_005: Cerrar con evidencia para Charlie
  event: evt_001
- [todo] task_mere_001: Crear MERE Creator
  event: evt_001
- [todo] task_gcp_001: Crear MERE GCP DB
  event: evt_001
- [todo] task_rag_001: Crear RAG
  event: evt_001
- [todo] task_vec_001: Crear Vector DB
  event: evt_001
- [todo] task_auth: Implementar módulo de autenticación
  event: evt_001
- [todo] task_tests: Escribir tests de integración
  event: evt_001
- [todo] task_docs: Documentar API
  event: evt_001
- [todo] task_022: Resolver pre-conflicto: Resolver lo necesario antes de continuar con "Corregir Semantic"
  event: evt_001
- [todo] task_023: Resolver pre-conflicto: Resolver lo necesario antes de continuar con "Corregir Paladin"
  event: evt_001
- [todo] task_024: Resolver pre-conflicto: Resolver lo necesario antes de continuar con "Corregir Events"
  event: evt_001

## Eventos

- [evt_001] Demo general para usuarios (2026-04-29 10:00): usuarios prueban el sistema en producción
- [evt_002] Channel con minimo 2 frameworks funcionando. Cumpliendo un resultado (2026-04-29 10:00): Channel es la base para que 2 IAs cooperen en producción
- [evt_003] Demo general para usuarios (2026-04-29 18:00): Channel es la base para que 2 IAs cooperen en producción

## Axiomas Vinculados

- [ax_001] La tarea "Identificar piezas esenciales de Paladin" debe cumplir el resultado y why de "Demo general para usuarios" -> task_pal_001
- [ax_002] La tarea "Identificar piezas esenciales de Orden" debe cumplir el resultado y why de "Demo general para usuarios" -> task_ord_001
- [ax_003] La tarea "Crear framework Events" debe cumplir el resultado y why de "Demo general para usuarios" -> task_evt_001
- [ax_004] La tarea "Corregir Semantic" debe cumplir el resultado y why de "Demo general para usuarios" -> task_sem_fix_001
- [ax_005] La tarea "Corregir Paladin" debe cumplir el resultado y why de "Demo general para usuarios" -> task_pal_fix_001
- [ax_006] La tarea "Corregir Events" debe cumplir el resultado y why de "Demo general para usuarios" -> task_evt_fix_001
- [ax_007] La tarea "Crear Channel" debe cumplir el resultado y why de "Demo general para usuarios" -> task_ch_001
- [ax_008] La tarea "Crear FPT" debe cumplir el resultado y why de "Demo general para usuarios" -> task_fpt_001
- [ax_009] La tarea "Crear Semantic" debe cumplir el resultado y why de "Demo general para usuarios" -> task_sem_001
- [ax_010] La tarea "Demo Snake Game" debe cumplir el resultado y why de "Demo general para usuarios" -> task_pal_002
- [ax_011] La tarea "Demo API Ecommerce" debe cumplir el resultado y why de "Demo general para usuarios" -> task_pal_003
- [ax_012] La tarea "Demo IAs via RPC" debe cumplir el resultado y why de "Demo general para usuarios" -> task_pal_004
- [ax_013] La tarea "Demo Orden (fallback)" debe cumplir el resultado y why de "Demo general para usuarios" -> task_ord_002
- [ax_014] La tarea "Cerrar con evidencia para Charlie" debe cumplir el resultado y why de "Demo general para usuarios" -> task_005
- [ax_015] La tarea "Crear MERE Creator" debe cumplir el resultado y why de "Demo general para usuarios" -> task_mere_001
- [ax_016] La tarea "Crear MERE GCP DB" debe cumplir el resultado y why de "Demo general para usuarios" -> task_gcp_001
- [ax_017] La tarea "Crear RAG" debe cumplir el resultado y why de "Demo general para usuarios" -> task_rag_001
- [ax_018] La tarea "Crear Vector DB" debe cumplir el resultado y why de "Demo general para usuarios" -> task_vec_001
- [ax_019] La tarea "Implementar módulo de autenticación" debe cumplir el resultado y why de "Demo general para usuarios" -> task_auth
- [ax_020] La tarea "Escribir tests de integración" debe cumplir el resultado y why de "Demo general para usuarios" -> task_tests
- [ax_021] La tarea "Documentar API" debe cumplir el resultado y why de "Demo general para usuarios" -> task_docs
- [ax_022] El pre-conflicto "Resolver lo necesario antes de continuar con "Corregir Semantic"" debe resolverse antes de "Corregir Semantic" -> task_022
- [ax_023] El pre-conflicto "Resolver lo necesario antes de continuar con "Corregir Paladin"" debe resolverse antes de "Corregir Paladin" -> task_023
- [ax_024] El pre-conflicto "Resolver lo necesario antes de continuar con "Corregir Events"" debe resolverse antes de "Corregir Events" -> task_024

