# Handoff Prompt — Remora (Sesión nueva, contexto 0)

Sos una IA pair programmer trabajando en **Remora**, el sistema de IAs autónomas que orquesta frameworks. Este prompt te entrega todo el contexto necesario para retomar el trabajo donde quedó.

---

## 1. Filosofía del sistema (NO negociable)

Remora es un sistema de **frameworks autónomos**. Cada framework es una sesión de IA con prompts + código Go. Los prompts le dicen a la IA qué comandos correr y cuándo; los comandos ejecutan código Go. La autonomía emerge porque cada framework declara qué **capabilities** produce y requiere — no porque alguien escriba un flujo prescriptivo.

Reglas duras:

- **Frameworks se crean con `framework-quine`, no a mano.** Quine es el framework que crea frameworks (`./quine create --name X --type Y`). Si vas a crear uno nuevo, usá Quine.
- **Frameworks se comunican por JSON** (stdin/stdout vía Channel JSON-RPC). NO por compartir memoria, NO por importar código de otro framework.
- **Estado canónico compartido → SQLite**. Estado interno de un framework → JSON file. Comunicación → JSON.
- **Routing entre frameworks es capability-based**, NO name-based. El orquestador NUNCA debe hacer `if framework == "sabio"`. Debe hacer `quien produce la capability X`.
- **`flow.rules.json` solo para overrides finos**, no para "quién habla". El "quién habla" emerge del grafo de capabilities + intent del usuario.

---

## 2. Estado actual del sistema

### Deployment vivo

- **URL dev**: `https://flujo-api-dev-760602975866.us-central1.run.app`
- **Cloud project**: `project-ceae5831-a2c9-49aa-b1c`
- **Cloud Run service**: `flujo-api-dev` (us-central1)
- **Build**: `gcloud builds submit --config=cloudbuild.yaml --substitutions="_CHANNEL_API_KEY=test-key-001,_SHORT_SHA=$(git rev-parse --short HEAD)"`
- **Deploy**: `gcloud run deploy flujo-api-dev --image=gcr.io/project-ceae5831-a2c9-49aa-b1c/flujo-api:<sha> --platform=managed --region=us-central1`

### Lo que funciona

- Modo dev: badge naranja en frontend + redirect de emails a `tom3bs@gmail.com`. Endpoint `/api/v1/config` devuelve `{dev_mode, dev_recipient, profile, runtime}`.
- `framework-contactos` con tabla canónica `contacts(entity_type, entity_ref, channel, value, source, verified_at)`. Capabilities: `contact.lookup`, `contact.store`. CLI: `lookup, store, list-missing, import-csv, init`.
- `framework-tareas` (ledger event-sourced). Tablas `tasks` + `task_events`. CLI: `list, next, create, complete, event, seed-from-foco`.
- Endpoints REST en `flujo_api`: `GET/POST /api/v1/tasks`, `GET /api/v1/tasks/next`, `POST /api/v1/tasks/{id}/event`.
- `/send-email` resuelve `entity_ref → email` vía contactos (412 si falta), redirige en dev, emite `task.event` automáticamente si vino `task_id`.
- Seed automático al boot (`entrypoint.sh`): si `contacts.db` o `tasks.db` están vacías, las siembra desde `profiles/cobranza-chile/contacts.seed.csv` y `tasks.seed.json`.

### Bugs reportados por el usuario (pendientes)

1. **Foco regenera prioridades desde 0** ignorando `tasks.db`. Si recargás, te muestra de nuevo el deudor #1 al que ya le mandaste email.
2. **Frontend no pasa `entity_type/entity_ref/task_id`** al `/send-email`. El email sale con `original_to: (sin destinatario)` aunque el contacto exista.
3. **Drafts del LLM tienen basura**: headers markdown como `**Email de Cobranza para X**` y cierres como `**No requiere acción adicional**` van en el body. Y placeholders `[Tu nombre]` se envían sin reemplazar.
4. **Asunto duplicado**: `Cobranza: Cobranza: Thiel-Effertz`.
5. **No hay inputs editables** del subject/body antes de enviar. Todo es generado y se envía sin posibilidad de corregir.
6. **Nodo "MENSAJERO" se ve como `MBN®ENJERO`** (overlap visual de nodos del flow).
7. **`/healthz` recién agregado por el usuario** (deep readiness probe en `main.go`). Hay que verificar que compile y que los campos `s.allManifests`, `s.rules`, `s.channel`, `s.runtimeInfo` existan en el struct `server`.

---

## 3. Arquitectura — qué hay y dónde

### DBs activas

| DB | Quién la toca | Operaciones | Ubicación | Persistencia |
|---|---|---|---|---|
| `framework-indexa/data/panalbit.db` | Sabio, Foco, Contactos (`list-missing`) | **SOLO LECTURA** | committeada al repo | persistente (parte del repo) |
| `profiles/<perfil>/contacts.db` | Contactos | RW | filesystem del container | **efímera** entre revisiones de Cloud Run |
| `profiles/<perfil>/tasks.db` | Tareas | RW | filesystem del container | **efímera** entre revisiones |

**CRÍTICO**: `panalbit.db` simula la DB del cliente real. **Nunca debe escribirse en modo dev**. Verifiqué que ningún `INSERT/UPDATE/DELETE` la toca hoy. Hay que agregar un guardrail explícito (ítem 5 del plan abajo) para que abortemos si alguien algún día agrega una escritura.

### JSONs activos

- **Manifests**: `framework-*/framework.manifest.json` declaran capabilities, comandos, params, etc.
- **Sesiones**: `sessions/conv_*.jsonl` — historial de cada conversación.
- **Estado interno por framework**: `framework-*/temp/state.json` o similar.
- **Reglas de composición**: `remora-flujo/cmd/flujo_api/flow.rules.json` (a depurar — ver gap abajo).
- **Seeds**: `profiles/<perfil>/contacts.seed.csv`, `profiles/<perfil>/tasks.seed.json`.

### Manifests — formato canónico

Cada framework tiene `framework.manifest.json` con:

```json
{
  "name": "tareas",
  "version": "0.1.0",
  "description": "...",
  "build": {"command": "go", "args": ["build", "-o", "frameworktareas", "./cmd/frameworktareas"]},
  "binary": {"command": "./frameworktareas"},
  "cwd": "framework-tareas",
  "execution_mode": "async_trigger",
  "commands": {
    "next-question": {"args": [...], "params": [...]},
    "ingest-answer":  {"args": [...], "params": [...]},
    "<comando-de-dominio>": {...}
  },
  "capabilities_semantic": {
    "tags": ["tasks", "ledger"],
    "intent_examples": ["qué tareas tengo", "marcá esto como hecho"],
    "produces": ["task.list", "task.next", "task.create"],
    "requires": []
  }
}
```

Todos los frameworks deben implementar **`next-question`** y **`ingest-answer`** como comandos CLI (contrato conversacional). El primero devuelve `{"id":"...","text":"..."}` o `{}`. El segundo recibe `--question-id` y `--answer`.

### Routing actual (el problema)

`@/Users/alcless_a1234_cursor/remora-go/remora-flujo/cmd/flujo_api/orchestrator.go` ejecuta:

1. Carga lista de drivers para la conversación (`conv.Frameworks`).
2. Aplica reglas de `flow.rules.json` que pueden hacer `prepend_speaker`. **Estas reglas mencionan frameworks por nombre** ("sabio", "alfa") — esto es prescriptivo y hay que matar.
3. Pollea drivers en orden, toma el primero que tenga `next-question` no vacío.

Esto **no es autónomo**. Ningún componente razona sobre capabilities salvo `ensure_credentials_smtp_before_send` (única regla bien hecha que usa `capability_missing` + `delegate_to_provider_of`).

### Discovery — esto SÍ es autónomo

`@/Users/alcless_a1234_cursor/remora-go/remora-flujo/cmd/flujo_api/drivers.go` → `initDriverRegistry` escanea `framework-*/framework.manifest.json` al boot, valida cada uno y construye el registry. Frameworks nuevos con manifest válido entran automáticamente vía `genericDriver`. NO hay que registrarlos a mano.

---

## 4. Plan que dejé acordado con el usuario (7 commits)

El usuario pidió **ordenar la base hoy** (no posponer). Plan acordado, en este orden:

### Commit 1: `ARCHITECTURE.md` corto (1 página, no 10)

Fija las reglas duras como contrato del repo. Ubicación: raíz del repo (`/Users/alcless_a1234_cursor/remora-go/ARCHITECTURE.md`). Debe contener:

- Reglas de DBs y JSONs (tabla del bloque "Arquitectura" arriba).
- Contrato de manifest (capabilities_semantic, contrato CLI mínimo).
- Política de routing: capability-based, NO name-based.
- Aislamiento dev/prod: `panalbit.db` read-only.
- Cómo crear un framework nuevo: usar `./quine create`, no a mano.

### Commit 2: Verificar build con `/healthz` del usuario

El usuario agregó `/healthz` en `main.go` que referencia `s.allManifests`, `s.rules`, `s.channel`, `s.runtimeInfo`. Ejecutar:

```bash
cd /Users/alcless_a1234_cursor/remora-go/remora-flujo
go build -buildvcs=false -o cmd/flujo_api/flujo_api ./cmd/flujo_api
```

Si falla, los campos no existen — agregarlos al struct `server` o ajustar el handler para usar lo que sí hay.

### Commit 3: Capability router mínimo

Modificar `@/Users/alcless_a1234_cursor/remora-go/remora-flujo/cmd/flujo_api/orchestrator.go` y/o `rules.go` para:

1. En `runLoop`, antes del polling lineal, **clasificar la respuesta del usuario contra `capabilities_semantic.intent_examples`** de cada manifest cargado.
2. Si match (heurística simple primero: substring case-insensitive), `prepend` ese framework al frente del orden.
3. **Eliminar nombres de framework de `flow.rules.json`**. Reescribir reglas en términos de capabilities cuando sea posible. Las que no se puedan reescribir, marcarlas `@deprecated` con comentario.

Diseño mínimo (no perfecto):

```go
// classifyIntent matchea userAnswer contra intent_examples de cada manifest.
// Devuelve nombre de framework con mejor match, o "" si no hay.
func classifyIntent(userAnswer string, manifests map[string]*manifest.Manifest) string { ... }
```

NO usar LLM clasificador en este commit. Dejarlo como TODO para iteración 2. Substring match contra `intent_examples` ya es un avance enorme respecto a `prepend_speaker: "sabio"` hardcodeado.

### Commit 4: Foco consume `tasks.db`

Modificar `@/Users/alcless_a1234_cursor/remora-go/framework-foco/cmd/foco/cobranza_sql.go`. La función `queryRealPriorities` hoy hace SELECT sobre `panalbit.db`. Cambio:

1. Primero llamar a `frameworktareas list --status pending --profile <perfil>`.
2. Si hay tareas pendientes → devolver esas como prioridades (con datos enriquecidos desde panalbit.db).
3. Si no hay (primer arranque) → query SQL como hoy + opcionalmente seed.

Variable env: `REMORA_TAREAS_BIN` (path al binario), `REMORA_PROFILE`.

### Commit 5: Guardrail anti-write `panalbit.db` en dev

Wrapper alrededor de `sql.Open` para `panalbit.db`. Si `dev_mode=true` (env `REMORA_DEV_MODE` o detectado por `TEST_EMAIL_RECIPIENT`), envolver el `*sql.DB` en algo que rechace `Exec` con verbos de escritura. Tres líneas — solo paranoia productiva.

Ubicación sugerida: `framework-indexa/store/store.go` o un helper compartido.

### Commit 6: Fixes tácticos del email

Frontend (`remora-flujo/cmd/flujo_api/static/index.html`):

- El handler del botón "Enviar email" debe pasar `entity_type`, `entity_ref`, `task_id` al `/api/v1/send-email`.
- Sanitizar el draft antes del fetch: regex `^\*\*[^\n]*\*\*\s*\n` para sacar headers, `\n\*\*No requiere acción.*\*\*\s*$` para cierres.
- Validar placeholders `\[[^\]]+\]` antes de habilitar "Enviar". Si hay, pintar el botón en rojo y mostrar tooltip "Completá los placeholders".
- Agregar `<input>` editables de subject + `<textarea>` de body antes del botón Enviar.

Backend (`main.go::handleSendEmail`):

- Antes de armar `mensajero.send`, sanitizar `req.Subject` (un solo prefijo `Cobranza:`, no duplicar) y validar que `req.Body` no contenga placeholders sin resolver. Si los tiene, devolver 422 con `error: "placeholders sin resolver: [...]"`.

### Commit 7: Frontend lee `/tasks/next` al cargar

En `static/index.html`, al inicializar el flow `cobranza-chile`:

1. Fetch `GET /api/v1/tasks/next` antes de inicializar Foco.
2. Si `data.found === true`, pasarle el deudor a Foco como contexto inicial: "trabajemos sobre `<task.title>` (#`<task.priority>` de `<task.pending + task.completed>`)".
3. Pintar contador "X de N" en el header del flow.

Esto hace que el reload sea idempotente.

### Commit 8 (al final): Build + deploy + smoke test

```bash
git add -A && git commit -m "..."
SHORT_SHA=$(git rev-parse --short HEAD)
gcloud builds submit --config=cloudbuild.yaml --substitutions="_CHANNEL_API_KEY=test-key-001,_SHORT_SHA=$SHORT_SHA"
gcloud run deploy flujo-api-dev --image=gcr.io/project-ceae5831-a2c9-49aa-b1c/flujo-api:$SHORT_SHA --platform=managed --region=us-central1
```

Smoke test después del deploy:

```bash
BASE=https://flujo-api-dev-760602975866.us-central1.run.app
curl -s "$BASE/api/v1/config" | python3 -m json.tool
curl -s "$BASE/api/v1/tasks/next" | python3 -m json.tool
curl -s "$BASE/healthz" | python3 -m json.tool
```

---

## 5. Layout del repo (relevante)

```
remora-go/
├── ARCHITECTURE.md              ← crear en commit 1
├── HANDOFF_PROMPT.md            ← este archivo
├── cloudbuild.yaml              ← build steps por framework
├── channel/                     ← JSON-RPC entre orquestador y frameworks
├── framework-quine/             ← genera frameworks (cmd/quine)
├── framework-echo/              ← inquisitivo (hardcoded driver)
├── framework-alfa/              ← spec compiler (hardcoded driver)
├── framework-sabio/             ← retrieval sobre panalbit.db
├── framework-foco/              ← prioridades cobranza
├── framework-mensajero/         ← envío email vía SMTP
├── framework-hosting/           ← provisión credentials.smtp
├── framework-indexa/            ← indexer ERP → panalbit.db
├── framework-contactos/         ← creado en sesión anterior, RW contacts.db
├── framework-tareas/            ← creado en sesión anterior, RW tasks.db
├── profiles/cobranza-chile/     ← perfil activo (seeds, flow.rules, etc)
└── remora-flujo/cmd/flujo_api/  ← orquestador HTTP + frontend embebido
    ├── main.go                  ← endpoints, routing
    ├── orchestrator.go          ← runLoop (a refactorizar en commit 3)
    ├── drivers.go               ← discovery + genericDriver
    ├── rules.go                 ← flow rules (a depurar en commit 3)
    ├── contactos.go             ← cliente del binario
    ├── tareas.go                ← cliente del binario
    ├── flow.rules.json          ← reglas de composición (a depurar)
    ├── Dockerfile
    ├── entrypoint.sh            ← seeds al boot
    └── static/index.html        ← frontend (~3000 líneas)
```

---

## 6. Convenciones operativas

- **Idioma**: el usuario habla español. Respondele en español. Código en inglés salvo strings de UI.
- **Sin emojis** en commits ni en respuestas salvo que el usuario pida.
- **Commits chicos y verificables**, uno por ítem del plan.
- **Build local antes de deploy**: `go build -buildvcs=false -o ... ./cmd/...`. Cloud Build usa `golang:1.26-bookworm`.
- **No tocar `panalbit.db`** con escrituras. Read-only siempre.
- **Modo dev** (`TEST_EMAIL_RECIPIENT=tom3bs@gmail.com`): redirige todos los emails a esa dirección y prefija subject con `[DEV → original]`.
- **El usuario puede modificar archivos durante la sesión** (cambia `main.go` mientras codeás). Releelos antes de editarlos.

---

## 7. Comandos útiles

```bash
# Ver qué frameworks tiene Quine registrados
cat /Users/alcless_a1234_cursor/remora-go/framework-quine/frameworks.json

# Registrar todos los frameworks en Quine (commit 5 alternativo)
cd /Users/alcless_a1234_cursor/remora-go/framework-quine
for fw in ../framework-*; do ./quine register --path "$fw"; done

# Test local de un framework
cd /Users/alcless_a1234_cursor/remora-go/framework-tareas
TASKS_DB_PATH=/tmp/test.db ./frameworktareas list --profile cobranza-chile

# Deploy quick
cd /Users/alcless_a1234_cursor/remora-go
SHORT_SHA=$(git rev-parse --short HEAD)
gcloud builds submit --config=cloudbuild.yaml \
  --substitutions="_CHANNEL_API_KEY=test-key-001,_SHORT_SHA=$SHORT_SHA"
gcloud run deploy flujo-api-dev \
  --image=gcr.io/project-ceae5831-a2c9-49aa-b1c/flujo-api:$SHORT_SHA \
  --platform=managed --region=us-central1
```

---

## 8. Lo que el usuario espera de vos

- Honestidad técnica. No vender autonomía donde hay if-else.
- Cuando dudes entre orden y velocidad, ordená — pero en una sola sesión, no en una semana.
- Antes de crear un framework nuevo, **usar Quine**. Si Quine no alcanza, decirlo y proponer mejorarlo.
- Reportar el gap entre la visión (autonomía total) y lo que se puede hacer hoy. El usuario prefiere un avance honesto chico a una promesa grande.
- Cero "tenés razón" o validaciones. Directo al grano.

---

## 9. Pregunta de arranque sugerida para vos

Cuando el usuario te de el OK para empezar, arrancá por el **Commit 1 (`ARCHITECTURE.md`)** porque es el contrato. Después seguí en orden. Si algún commit revela que el plan estaba mal, parás y le decís al usuario antes de improvisar.

Suerte.
