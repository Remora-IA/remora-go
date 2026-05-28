# Remora — Arquitectura (contrato del repo)

Una página. Reglas duras. Si un PR las rompe, no entra.

Ver también `docs/AXIOMS.md`. Axioma rector: **un flujo está listo cuando cualquier framework puede entrar, actuar, pedir ayuda, esperar o salir del equipo usando solo capabilities declaradas, contratos verificables y trazas auditables, sin cables por nombre ni conocimiento oculto.**

## 1. Modelo mental

Remora es un sistema de **frameworks autónomos**. Cada framework es una sesión de IA (prompts) + un binario Go que expone comandos CLI. El orquestador (`remora-flujo/cmd/api_rest`) los compone vía Channel (JSON-RPC sobre stdin/stdout). La autonomía emerge porque cada framework declara qué **capabilities** produce y requiere — no porque alguien escriba un flujo prescriptivo.

## 2. Reglas duras

1. **Frameworks nuevos se crean con `framework-quine`**, no a mano. `./quine create --name X --type Y`. Si Quine no alcanza, mejorar Quine antes que crear a mano.
2. **Frameworks se comunican por JSON** (Channel JSON-RPC). Nunca por imports cruzados ni memoria compartida.
3. **Routing es capability-based, no name-based.** El orquestador NUNCA debe hacer `if framework == "sabio"`. Debe preguntar "quién produce capability X". `flow.rules.json` es solo para overrides finos (ej. `capability_missing → delegate_to_provider_of`); no para "quién habla".
4. **Cada manifest declara capabilities.** `capabilities_semantic` describe routing blando (`tags`, `intent_examples`, `produces`, `requires`) y `capabilities` declara contratos typed (`id`, `command`, `inputs`, `outputs`, `execution`, `policies`) para que Paladin valide el trabajo en equipo.
5. **Contrato CLI mínimo de todo framework**: `next-question` (devuelve `{"id","text","ask_via"}` o `{}`) y `ingest-answer` (recibe `--question-id` y `--answer`).
6. **Sin emojis** en código ni commits salvo pedido explícito del usuario.

## 3. Estado: dónde vive cada cosa

| Tipo de estado | Backend | Quién toca | Notas |
|---|---|---|---|
| Estado canónico compartido entre frameworks | **SQLite** en `profiles/<perfil>/*.db` | Frameworks que producen la capability dueña | Efímero entre revisiones de Cloud Run; se reseedea al boot vía `entrypoint.sh` |
| Estado interno de un framework | JSON file (`framework-*/temp/state.json`) | Solo ese framework | No se comparte |
| ERP origen del cliente | `framework-indexa/data/panalbit.db` | Sabio, Foco | **READ-ONLY siempre.** Committeado al repo. Simula la DB del cliente real |
| Comunicación entre componentes | JSON por stdin/stdout | Channel | Nunca compartir memoria |
| Sesiones de conversación | `sessions/conv_*.jsonl` | api_rest | Append-only |
| Reglas de composición | `remora-flujo/cmd/api_rest/flow.rules.json` | api_rest | Solo overrides; ver regla 3 |
| Seeds del perfil | `profiles/<perfil>/{contacts.seed.csv,tasks.seed.json}` | `entrypoint.sh` | Idempotente: solo siembra si la DB está vacía |

## 4. Aislamiento dev/prod

- Modo dev se activa con `REMORA_DEV_MODE=true` o detectado por `TEST_EMAIL_RECIPIENT`.
- Frontend muestra badge naranja vía `GET /api/v1/config`.
- Backend redirige todos los emails a `tom3bs@gmail.com` y prefija subject con `[DEV → original: ...]`.
- **`panalbit.db` es read-only.** Si en algún momento un framework intenta escribir, el wrapper de `framework-indexa/store` debe abortar.

## 5. Manifest canónico

```json
{
  "name": "tareas",
  "version": "0.1.0",
  "build": {"command": "go", "args": ["build", "-o", "frameworktareas", "./cmd/frameworktareas"]},
  "binary": {"command": "./frameworktareas"},
  "cwd": "framework-tareas",
  "execution_mode": "async_trigger",
  "commands": {
    "next-question": {"args": [...], "params": [...]},
    "ingest-answer": {"args": [...], "params": [...]}
  },
  "capabilities_semantic": {
    "tags": ["tasks", "ledger"],
    "intent_examples": ["qué tareas tengo", "marcá esto como hecho"],
    "produces": ["task.list", "task.next", "task.create"],
    "requires": []
  }
}
```

## 6. Deployment

- **Solo dev.** Servicio: `flujo-api-dev` en `project-ceae5831-a2c9-49aa-b1c`, región `us-central1`.
- URL: `https://flujo-api-dev-760602975866.us-central1.run.app`.
- Producción (`flujo-api`) está congelada en commit `39f204f`. No tocar.
- Build: `gcloud builds submit --config=cloudbuild.yaml --substitutions="_CHANNEL_API_KEY=test-key-001,_SHORT_SHA=$(git rev-parse --short HEAD)"`.
- Deploy: `gcloud run deploy flujo-api-dev --image=gcr.io/project-ceae5831-a2c9-49aa-b1c/flujo-api:<sha> --platform=managed --region=us-central1`.

## 7. Cómo agregar un framework nuevo

1. `cd framework-quine && ./quine create --name <X> --type <tipo>`.
2. Editar el `framework.manifest.json` generado: completar `capabilities_semantic` (especialmente `produces` y `intent_examples`).
3. Implementar `next-question` e `ingest-answer` en el binario.
4. `go build` localmente. Verificar que el manifest sea válido (`drivers.go::initDriverRegistry` lo escanea al boot — si lo skipea, hay un error que ver en logs `[boot]`).
5. Agregar el step de build a `cloudbuild.yaml` si el framework genera binario.
6. Si necesita estado canónico nuevo (no cabe en `contacts.db` ni `tasks.db`), discutirlo antes: una DB nueva por capability nueva es válido; una DB nueva por framework no.
