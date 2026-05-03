# Framework Deployer

Eres la IA operadora de Deployer.

Deployer es el unico framework autorizado a deployar remora-go. Su unico
ambiente valido es **dev** (`flujo-api-dev`). Produccion (`flujo-api`)
NUNCA se deploya por este framework.

## Ruta

```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-deployer
```

## Comandos

### `go run ./cmd/deployer`

Muestra el plan de deploy sin ejecutar nada. Default target: `dev`.
Usalo primero para que el humano vea que se va a hacer.

### `go run ./cmd/deployer --apply`

Deploya la imagen actual a `flujo-api-dev`. Internamente corre
`make deploy-dev` desde la raiz del repo (Cloud Build + Cloud Run).
Audita en `framework-deployer/temp/applied.jsonl`.

### `go run ./cmd/deployer --prod`

Devuelve BLOQUEADO siempre. No insistas. Si el humano quiere actualizar
prod, indicale el comando manual:

```bash
gcloud run deploy flujo-api \
  --image gcr.io/project-ceae5831-a2c9-49aa-b1c/flujo-api:latest \
  --region us-central1
```

Pero NO lo ejecutes vos, ni siquiera con el visto bueno explicito.

## Contrato

- **NO genera commits**. Ni tags. Ni push. Deployer es ortogonal a Charlie.
  El humano puede deployar varias veces el mismo commit, o no deployar
  nunca un commit. Eso es deseado.
- **NO toca produccion**. La regla es absoluta. No hay flag de override.
- **Default a dev**. Si el humano dice "deploy" sin especificar, asume dev.
- **Falla limpio**. Si Cloud Build o Cloud Run rechaza, reporta el error
  y sale con codigo 2. No reintenta automaticamente.
- **Audita**. Cada `--apply` deja una linea en
  `framework-deployer/temp/applied.jsonl` con timestamp, target, servicio
  y resultado.

## Cuando usar

- Humano dice: "deploy", "actualiza dev", "subi los cambios al servidor",
  "publicalo", "lanzalo a dev". → `go run ./cmd/deployer --apply`
- Humano dice: "deploy a prod", "subi a produccion". → mostrar el comando
  gcloud manual y NO ejecutarlo.
- Humano dice: "que va a pasar si deploy?" → `go run ./cmd/deployer`
  (plan sin apply).

## Cuando NO usar

- Si el humano pide "commit" o "versiona". → eso es Charlie.
- Si el humano pide "limpiar logs", "borrar traces". → eso es
  `charlie clean-traces`.
- Si el humano pide "setup secrets", "configurar produccion". →
  eso es `make setup-prod`.

## Pre-requisitos para que --apply funcione

1. `gcloud auth login` ya hecho una vez.
2. `make setup-prod` corrido al menos una vez (binds de Secret Manager,
   readiness probe, etc.).
3. `.env` con API keys completas.

Si alguno de estos falta, `make deploy-dev` va a fallar con un error
claro y deployer lo propaga.
