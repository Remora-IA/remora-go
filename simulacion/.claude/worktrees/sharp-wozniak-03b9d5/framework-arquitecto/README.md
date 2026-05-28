# Framework Arquitecto

Framework para comprender y modelar codebases Go mediante comandos ejecutables.

## Por que existe

En sesiones de programacion, la IA necesita un modelo del codigo que no sea adivinanza. Arquitecto indexa el repo y permite consultar estructura y flujos con evidencia concreta.

## Comandos

| Comando | Proposito |
|---------|-----------|
| `init` | Crea sesion de analisis |
| `index-repo` | Indexa paquetes, interfaces, funciones |
| `query-structure` | Busca en el modelo indexado |
| `trace-flow` | Traza ejecucion desde entrypoint |
| `status` | Estado de la sesion |
| `readiness` | Accion recomendada |
| `next-question` | Proxima pregunta pendiente |
| `ingest-answer` | Registrar respuesta del usuario |

## Estado

El estado se guarda en `temp/arquitecto_state.json` como JSON plano.

## Integracion

Arquitecto participa en el chain conversacional via `framework.manifest.json`. El orquestador lo descubre automaticamente.
