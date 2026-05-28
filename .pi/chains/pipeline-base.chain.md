---
name: pipeline-base
package: pi-subagents
description: Pipeline base de análisis — Detective → Pre-Narrador → Indexer. Genera narrativa experiencial del proyecto.
---

## pi-detective
progress: true

Analiza todos los componentes del proyecto usando los CPGs disponibles. Detecta métodos, call graphs, interfaces cross-proceso, sinks, y SUPERFICIES DE USUARIO (CLI, frontend web, API). Guarda inventario completo en .pi/chain-runs/{RUN_ID}/01-pi-detective/output.md

## pi-pre-narrador
progress: true

Lee el inventario del Detective y genera una NARRATIVA EXPERIENCIAL del sistema. Organizada por SUPERFICIE (interfaz de usuario) → FLUJO (caso de uso) → ESCENA (momento atómico). Describe qué ve, decide y hace el usuario en cada punto. Incluye TRACE con código al final de cada escena. Documenta GAPS y CABLES como limitaciones experienciales. Output en .pi/chain-runs/{RUN_ID}/02-pi-pre-narrador/output.md

## pi-indexer
progress: true

Construye semantic-index.yaml consolidando Detective + Pre-Narrador. Incluye sección 'narrativa' con índice de superficies, flujos, escenas, gaps y cables para navegación desde el viewer. Output en .pi/semantic-index.yaml y copia en .pi/chain-runs/{RUN_ID}/03-pi-indexer/
