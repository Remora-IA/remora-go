# AGENTS

## Backend API local

- El backend local correcto escucha en `http://localhost:8084`.
- Para reiniciar el backend **siempre** usar `make restart-api`.
- `make restart-api` recompila `remora-flujo/api_rest`, mata el listener previo en `:8084`, levanta el backend nuevo y fuerza `REMORA_DEV_STATIC=1` para servir `cmd/api_rest/static/` desde disco.
- No reiniciar con `./remora-flujo/api_rest` a secas sin recompilar antes: eso puede dejar un binario viejo y frontend embebido stale.

## Verificación rápida tras restart

```bash
make restart-api
curl -fsS http://localhost:8084/health
stat -f '%Sm %N' remora-flujo/api_rest
```
