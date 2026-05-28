# Deploy — remora-flujo

Guía para desplegar el backend Go en Google Cloud Run.

---

## Prerrequisitos

| Herramienta | Versión mínima | Instalación |
|-------------|---------------|-------------|
| gcloud CLI  | 400+          | https://cloud.google.com/sdk/docs/install |
| Docker      | 20+           | https://docs.docker.com/get-docker/ (opcional — ver alternativa abajo) |
| Go          | 1.24+         | https://go.dev/dl/ (solo si compilas local) |

Proyecto GCP: `project-ceae5831-a2c9-49aa-b1c`
Servicio Cloud Run: `flujo-api`
Region: `us-central1`

---

## Pasos — deploy estándar (con Docker local)

```bash
# 1. Autenticar gcloud
gcloud auth login
gcloud auth configure-docker   # para poder hacer push a gcr.io

# 2. Seleccionar proyecto
gcloud config set project project-ceae5831-a2c9-49aa-b1c

# 3. Asegurarse de tener las variables sensibles en remora-flujo/.env
#    (REMORA_VAULT_KEY, GROQ_API_KEY, SMTP_PASS, etc.)

# 4. Ejecutar deploy desde la carpeta remora-flujo/
cd remora-flujo/
./deploy.sh
```

El script:
1. Lee `remora-flujo/.env` y `remora-flujo/.env.local` automáticamente.
2. Compila todos los binarios Go del mono-repo.
3. Construye la imagen Docker y la sube a GCR.
4. Despliega en Cloud Run con todas las variables de entorno.
5. Imprime la URL final y el paso de Auth0.

---

## Alternativa sin Docker local (Cloud Build)

Si no tenés Docker instalado, el script cae automáticamente en modo Cloud Build.
También podés lanzarlo directamente desde la raíz del repo:

```bash
# Desde la raíz del mono-repo (remora-go-lite/)
gcloud builds submit \
  --config=remora-flujo/cloudbuild.yaml \
  --substitutions="_SHORT_SHA=$(git rev-parse --short HEAD)" \
  --project=project-ceae5831-a2c9-49aa-b1c
```

O el modo más simple, sin compilar binarios ni Dockerfile (Cloud Run compila desde fuente):

```bash
cd remora-flujo/cmd/api_rest/
gcloud run deploy flujo-api \
  --source . \
  --region us-central1 \
  --allow-unauthenticated \
  --port 8080 \
  --project project-ceae5831-a2c9-49aa-b1c
```

> Nota: `--source .` usa Buildpacks. El Dockerfile propio da más control sobre
> binarios de frameworks y Channel — se recomienda para producción.

---

## Post-deploy: actualizar Auth0

Después de cada deploy la URL de Cloud Run puede cambiar (si es la primera vez o
si se recrea el servicio). Hay que actualizar en el panel de Auth0:

1. Entrar a https://manage.auth0.com → Applications → remora-flujo
2. En **Allowed Callback URLs** agregar:
   ```
   https://flujo-api-XXXX-uc.a.run.app/callback
   ```
3. En **Allowed Logout URLs** agregar:
   ```
   https://flujo-api-XXXX-uc.a.run.app
   ```
4. En **Allowed Web Origins** agregar:
   ```
   https://flujo-api-XXXX-uc.a.run.app
   ```

La URL exacta aparece al final de `./deploy.sh`. También podés obtenerla con:

```bash
gcloud run services describe flujo-api \
  --region us-central1 \
  --format='value(status.url)'
```

---

## Variables de entorno en producción

Las variables **no secretas** se pasan directamente en `./deploy.sh` (Auth0, puertos,
URLs internas). Las **secretas** se configuran por separado con Secret Manager o
directamente en Cloud Run:

```bash
# Agregar o actualizar una variable sensible sin redeployar la imagen
gcloud run services update flujo-api \
  --region us-central1 \
  --update-env-vars "REMORA_VAULT_KEY=tu_valor_aqui"

# Agregar varias a la vez
gcloud run services update flujo-api \
  --region us-central1 \
  --update-env-vars "GROQ_API_KEY=xxx,SMTP_PASS=yyy,MINIMAX_API_KEY=zzz"
```

### Variables requeridas en producción

| Variable | Descripción |
|----------|-------------|
| `AUTH0_DOMAIN` | `remora-ia.us.auth0.com` |
| `AUTH0_CLIENT_ID` | `p6m3MHqoklOZkpCFGXF2owxIxtDwSJO6` |
| `AUTH0_AUDIENCE` | `https://remora-ia.us.auth0.com/api/v2/` |
| `REMORA_VAULT_KEY` | Clave de cifrado del vault (no commitear) |
| `CHANNEL_API_KEY` | API key interna entre api_rest y Channel |
| `GROQ_API_KEY` | API key de Groq (LLM) |
| `MINIMAX_API_KEY` | API key de Minimax (LLM alternativo) |
| `SMTP_HOST` / `SMTP_USER` / `SMTP_PASS` | Config de email |

> Las API keys sensibles NO están en el repositorio. Deben setearse via
> `gcloud run services update` o mediante Secret Manager.

---

## Recomendación: Secret Manager (opcional pero recomendado)

Para no pasar secrets como variables de entorno en texto plano:

```bash
# Crear un secret
echo -n "tu_vault_key" | gcloud secrets create REMORA_VAULT_KEY --data-file=-

# Referenciar el secret en Cloud Run
gcloud run services update flujo-api \
  --region us-central1 \
  --set-secrets "REMORA_VAULT_KEY=REMORA_VAULT_KEY:latest"
```

---

## Estructura de archivos de deploy

```
remora-go-lite/
├── cloudbuild.yaml                        ← Cloud Build raíz (legacy)
├── remora-flujo/
│   ├── cloudbuild.yaml                    ← Cloud Build con Auth0 (este repo)
│   ├── deploy.sh                          ← Script de deploy principal
│   ├── README-deploy.md                   ← Esta guía
│   ├── .env                               ← Variables locales (NO commitear)
│   └── cmd/api_rest/
│       ├── Dockerfile                     ← Imagen de producción
│       ├── Dockerfile.dev                 ← Imagen de desarrollo (hot-reload)
│       └── entrypoint.sh                  ← Startup: Channel + api_rest
```
