# Remora Go

Sistema multi-framework de IA orquestado por `remora-flujo/cmd/flujo_api`
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

# 5. Arrancar el API
cd remora-flujo && go run ./cmd/flujo_api
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
# Cloud Build compila todos los binarios y arma la imagen.
gcloud builds submit --config cloudbuild.yaml .

# Deploy a DEV (nunca a prod sin autorizacion explicita)
gcloud run deploy flujo-api-dev \
  --image gcr.io/project-ceae5831-a2c9-49aa-b1c/flujo-api:latest \
  --region us-central1
```

Las env vars en Cloud Run se setean via `--set-env-vars` o (preferido)
montando GCP Secret Manager. Ver `cloudbuild.yaml` para detalles.

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
