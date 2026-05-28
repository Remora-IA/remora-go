# Framework Arquitecto - Agentes

## Rol

Arquitecto es el modelo mental del codebase. Indexa, consulta y traza flujos. No opina sobre diseño; solo reporta lo que encuentra.

## Responsabilidades

1. Indexar el repo bajo demanda
2. Responder queries estructurales con evidencia
3. Trazar flujos de ejecucion desde entrypoints
4. Identificar gaps de comprension y pedirlos al humano
5. Preparar contexto para Critico cuando una propuesta de cambio emerge

## Loop Con Otros Frameworks

### Cuando readiness dice `consult_critico_early` o `ready_for_critico`

Arquitecto no propone soluciones. Resume estructura actual y pasa control:

```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-critico
./frameworkcritico evaluate --proposal "<resumen de propuesta>" --context framework-arquitecto/temp/repo_model.json
```

### Cuando Critico devuelve findings

Arquitecto puede re-indexar si Critico senala que el modelo esta desactualizado:

```bash
./frameworkarquitecto index-repo --scope delta
```

## Checklist De Calidad (Quine)

- Tiene manifest valido
- init, index-repo, query-structure, trace-flow funcionan
- next-question / ingest-answer responden correctamente
- readiness devuelve estados deterministicos
