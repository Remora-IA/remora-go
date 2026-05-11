# framework-deployer

Runbook ejecutable para deployar Remora Flujo a Cloud Run con pasos deterministas, salidas JSON estructuradas y guardrails fuertes contra deploys accidentales.

## Targets oficiales

| Ambiente | Service | URL | Imagen |
|---|---|---|---|
| DEV | `flujo-api-dev` | `https://flujo-api-dev-760602975866.us-central1.run.app/` | `gcr.io/project-ceae5831-a2c9-49aa-b1c/flujo-api-dev:<tag>` |
| PROD | `flujo-api` | `https://flujo-api-760602975866.us-central1.run.app/` | `gcr.io/project-ceae5831-a2c9-49aa-b1c/flujo-api:<tag>` |

GCP project: `project-ceae5831-a2c9-49aa-b1c`
Region: `us-central1`

## Uso por etapas

```bash
go run ./cmd/deployer plan --env dev --intent "deploy dev"
go run ./cmd/deployer preflight --env dev
go run ./cmd/deployer test --env dev
go run ./cmd/deployer build --env dev
go run ./cmd/deployer watch-build --build-id BUILD_ID --interval-seconds 60 --max-minutes 15
go run ./cmd/deployer deploy --env dev --tag SHORTSHA-dev-YYYYMMDDHHMMSS
go run ./cmd/deployer verify --env dev
go run ./cmd/deployer verify-single-session --env dev
go run ./cmd/deployer verify-other-env-untouched --env dev
```

## PROD

PROD requiere confirmación textual exacta:

```text
Confirmo deploy a PROD flujo-api
```

Ejemplos:

```bash
go run ./cmd/deployer plan --env prod
go run ./cmd/deployer build --env prod --confirm "Confirmo deploy a PROD flujo-api"
go run ./cmd/deployer deploy --env prod --tag TAG --confirm "Confirmo deploy a PROD flujo-api"
```

Sin esa frase, `build` y `deploy` a PROD devuelven `status=blocked`.

## Contrato

- No genera commits, tags ni push de git.
- No usa `make deploy`, `make deploy-dev` ni `deploy.sh`.
- DEV solo usa `flujo-api-dev` e imagen `flujo-api-dev:<tag>`.
- PROD solo usa `flujo-api` e imagen `flujo-api:<tag>` con confirmación explícita.
- `deploy` exige `--tag`; no usa `latest`.
- Todos los comandos imprimen JSON estructurado.
- `watch-build` hace polling porque Cloud Build puede estar varios minutos sin salida local.

## Diagnóstico

```bash
go run ./cmd/deployer diagnose "command not allowed: ./frameworkpaladin"
go run ./cmd/deployer diagnose "No new output since last status check"
go run ./cmd/deployer diagnose "/healthz 404"
```

## Tests usados antes de deploy

- `remora-flujo`: `go test ./cmd/api_rest`
- `channel`: `go test ./adapter ./cmd/channel ./cmd/vault ./internal ./manifest ./vault`
- `framework-paladin`: `go test ./paladin`
- Paladin lint: `go run ./cmd/paladin lint .. 2>/dev/null | grep '\[fail\]' || true`

`go test ./...` en `channel` no se usa como bloqueo automático si solo falla por el vet preexistente de `cmd/orchestrator`.
