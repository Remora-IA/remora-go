# framework-deployer

Framework dedicado a deployar remora-go a Cloud Run **dev**. Es ortogonal a
Charlie: no genera commits, tags ni push. Podes deployar el mismo commit
muchas veces, o nunca deployar un commit.

## Uso

```bash
cd framework-deployer

go run ./cmd/deployer            # plan (no hace nada)
go run ./cmd/deployer --apply    # deploy a flujo-api-dev
go run ./cmd/deployer --prod     # SIEMPRE bloqueado
```

## Contrato (no negociable)

- Solo deploya a `flujo-api-dev`.
- `--prod` siempre devuelve BLOQUEADO. No hay override.
- No toca git.
- Audita cada apply en `temp/applied.jsonl`.

## Como produccion se actualiza entonces?

Solo manualmente, por el humano:

```bash
gcloud run deploy flujo-api \
  --image gcr.io/project-ceae5831-a2c9-49aa-b1c/flujo-api:latest \
  --region us-central1
```

Esa es la decision arquitectonica: prod requiere intervencion humana
explicita, sin atajos automatizables. Asi se evita que un agente IA o
script accidental rompa producción.

## Env vars (opcionales)

| Variable | Default | Descripcion |
|---|---|---|
| `PROJECT_ID` | `project-ceae5831-a2c9-49aa-b1c` | GCP project |
| `REGION` | `us-central1` | Cloud Run region |
| `DEV_SERVICE` | `flujo-api-dev` | Nombre del servicio dev |
| `REMORA_ROOT` | git rev-parse | Raiz del repo |
