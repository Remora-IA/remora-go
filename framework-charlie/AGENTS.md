# AGENTS.md

## Integración con otros frameworks

### Quine
Charlie es creado y mantenido por **Quine** (framework auto-replicante).

### Paladin
Charlie usa **Paladin** para tracing de operaciones:
- Importado en `internal/paladin/`
- Tracea cada análisis de cambios
- Registra versiones calculadas

### Alfa
Charlie necesita coordinación con **Alfa** para ejecutar commits:
- Charlie propone: `chore: commit vX.Y.Z - descripción`
- Alfa ejecuta: `git commit` y `git tag`

### Echo
Charlie usa **Echo** para preguntas de clarificación cuando:
- Hay cambios ambiguos que necesitan clasificación
- Se requiere confirmar el scope de cambios

### Bravo
Charlie notifica a **Bravo** cuando:
- Hay nuevas versiones creadas
- Se actualiza CHANGELOG.md
