# Remora Go

Sistema multi-framework de IA orquestado por `remora-flujo/cmd/api_rest`
sobre un canal JSON-RPC (`channel/`). Cada framework encapsula una capability
(echo, alfa, foco, sabio, mecanico, mensajero, hosting, etc.) y se compone
declarativamente desde `flow.rules.json`.

## Onboarding (5 pasos)

```bash
# 1. Clonar
git clone <repo-url> remora-go
cd remora-go

# 2. Variables de entorno
cp .env.example .env
# Editar .env y completar al menos:
#   - GROQ_API_KEY o MINIMAX_API_KEY
#   - REMORA_VAULT_KEY (generar con paso 3)

# 3. Generar clave maestra del vault (si vas a usar hosting/cpanel)
cd framework-hosting && go run ./cmd/frameworkhosting genkey
# Pega el output en REMORA_VAULT_KEY del .env

# 4. Compilar todos los binarios
go build ./...

# 5. Arrancar Channel (broker JSON-RPC)
docker compose up channel
# Alternativa sin Docker, desde el root:
(cd channel && go run ./cmd/channel -addr :8765 -base-dir "$(pwd)/.." -api-keys test-key-001)

# 6. Arrancar el API
cd remora-flujo && go run ./cmd/api_rest
# Abrir http://localhost:8080
```

## Estructura

```
remora-go/
├── channel/                ← JSON-RPC executor + vault + axiomas seguridad
├── framework-<X>/          ← un framework por capability
│   ├── cmd/<X>/            ← entrypoint (binario)
│   ├── internal/           ← logica
│   ├── framework.manifest.json
│   └── temp/               ← runtime state (NO versionado)
├── remora-flujo/           ← orquestador REST + frontend
├── profiles/<cliente>/     ← config + data por cliente
├── sessions/               ← conversaciones (NO versionado)
├── .env.example            ← plantilla env vars
└── .gitignore              ← regla: solo codigo fuente al repo
```

## Frameworks operacionales

Dos frameworks dedicados que NO se llaman como capabilities del chat:

- **`framework-charlie/`** → versionado del repo (commits, tags, push).
  Nunca toca infra. Nunca deploya.
- **`framework-deployer/`** → deploy a Cloud Run **dev**. Nunca commitea.
  Nunca toca prod. Podes deployar el mismo commit muchas veces.

Son ortogonales: deployar no implica commit, commitear no implica deploy.

```bash
# Deploy a dev (NO genera commit)
cd framework-deployer && go run ./cmd/deployer --apply

# Versionar codigo (NO genera deploy)
cd framework-charlie && go run ./cmd/charlie apply-propose --apply --push
```

## Versionado: usar Charlie, no git directo

Charlie (`framework-charlie/`) es el unico autorizado a hacer commits, tags
y push. No correr `git commit/tag/push` manual.

```bash
cd framework-charlie

# Plan ante intento natural ("commitear todo", "actualizar main", etc.)
go run ./cmd/charlie plan --intent "<lo que querés hacer>"

# Happy path completo (changelog + commit + tag + push)
go run ./cmd/charlie apply-propose --apply --push
```

Reglas de oro:
- **Producción** (`flujo-api`) queda fija en commit `39f204f`. NUNCA deployar a prod.
- **Desarrollo** (`flujo-api-dev`) recibe todos los deploys.
- Charlie versiona `vX.Y.Z` automaticamente; no calcules versiones a mano.

Ver `framework-charlie/INITIAL_PROMPT.md` para el contrato completo.

## Deploy

```bash
make deploy-dev   # Cloud Build + Cloud Run dev (NUNCA prod)
```

Atajo manual equivalente:

```bash
gcloud builds submit --config cloudbuild.yaml .
gcloud run deploy flujo-api-dev \
  --image gcr.io/project-ceae5831-a2c9-49aa-b1c/flujo-api:latest \
  --region us-central1
```

### Setup PROD (un solo comando)

```bash
make setup-prod
```

Hace todo en orden y es idempotente (corerlo de nuevo no rompe nada):

1. Verifica `gcloud auth` (te avisa si falta `gcloud auth login`).
2. Habilita APIs necesarias (Secret Manager, Cloud Build, Run).
3. Sube los secretos del `.env` a GCP Secret Manager y los bindea al servicio.
4. Activa la readiness probe en `/healthz`.
5. Crea el trigger de CI en Cloud Build.
6. Smoke test del `/healthz` deployado.

### Pasos manuales de UNA SOLA VEZ

Hay 2 cosas que Google no permite hacer a un script (requieren auth humana):

**1. Login en gcloud:**
```bash
gcloud auth login
gcloud config set project project-ceae5831-a2c9-49aa-b1c
```

**2. Conectar el repo de GitHub a Cloud Build** (solo si `make setup-prod`
te avisa que el trigger fallo):
- Abrir https://console.cloud.google.com/cloud-build/triggers/connect
- Click "Connect Repository" → GitHub → autorizar → seleccionar `Remora-IA/remora-go`
- Volver a correr `make setup-prod`

### Secrets en produccion

Los secretos NO van como `--set-env-vars` planos. Se suben a GCP Secret
Manager y se bindean al servicio:

```bash
./scripts/setup-secrets.sh
```

Lee `.env` local, sube cada secreto a Secret Manager (creando o versionando)
y los monta como env vars en `flujo-api-dev`. Es idempotente. Tambien setea
las env vars no-sensibles (`REMORA_PROFILE`, `REMORA_DEV_MODE`, etc.).

### Healthcheck

`api_rest` expone dos endpoints:

- `GET /health` — liveness simple (`{status: ok}`).
- `GET /healthz` — readiness profunda. Devuelve `200` si LLM, frameworks
  y channel estan OK; `503` si algo falta. Cloud Run lo usa como startup
  probe (configurado por `make setup-prod`).

### CI (validacion automatica en cada push)

`cloudbuild-ci.yaml` corre `vet + build + test` sobre todo el repo. Para
activarlo en cada push a `draft`:

```bash
gcloud builds triggers create github \
  --name=remora-go-ci \
  --repo-name=remora-go --repo-owner=Remora-IA \
  --branch-pattern="^draft$" \
  --build-config=cloudbuild-ci.yaml
```

Local equivalente: `make check` (fmt + vet + test).

## Convenciones

- **Codigo fuente**: `cmd/`, `internal/`, `pkg/`. Tracked.
- **Build artifacts** (binarios compilados): se regeneran, NO tracked.
- **Runtime state** (`temp/`, `*.db`, `*.enc`, `sessions/`): por instancia, NO tracked.
- **Secrets** (`.env`, `*.enc`, `vault_data/`): NUNCA tracked. Ver `.env.example`.
- **Logs/traces** (`trace_pal_*.json`, `*.log`): regenerables, NO tracked.

Si `git status` muestra ruido, agregalo a `.gitignore` en lugar de `git add -f`.

## Documentacion adicional

- `framework-charlie/INITIAL_PROMPT.md` — contrato completo de Charlie
- `framework-paladin/README.md` — sistema de tracing
- `framework-bravo/README.md` — IdealFlow tracer
- `docs/CAPABILITIES.md` — capabilities canonicas del sistema
- `nuevo_mapa.md` — mapa actual del sistema
