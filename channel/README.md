# Channel

**Executor stateless JSON-RPC 2.0 con seguridad en profundidad.**

## Resumen

Channel es un dumb executor que ejecuta operaciones de validación, sanitización y ejecución. **Nunca piensa, nunca decide, nunca interpreta.** Cada request es procesado de forma 100% independiente (stateless).

## Contrato de Respuesta (Inmutable)

```json
{
  "success": boolean,
  "exit_code": number,
  "stdout": string,
  "stderr": string,
  "error": string,
  "duration_ms": number
}
```

## 5 Métodos Expuestos

| Método | Descripción |
|--------|-------------|
| `execute_command` | Ejecuta comandos de la whitelist |
| `read_file` | Lee archivos dentro de BASE_DIR |
| `write_file` | Escribe archivos dentro de BASE_DIR |
| `list_dir` | Lista directorios dentro de BASE_DIR |
| `http_get` | Realiza GET HTTP |

## Uso

```bash
./channel -addr :8080 -base-dir /tmp/channel -api-keys "key1,key2"
```

## Seguridad

- **Defense in Depth**: 5 capas de seguridad activas simultáneamente
- **Whitelist obligatoria**: Solo comandos hardcodeados
- **Timeout hard**: 30 segundos máximo
- **API Key requerida**: Header `X-API-Key`

## API Keys y Puerto por Variables de Entorno

```bash
export CHANNEL_API_KEYS="key1,key2"
export CHANNEL_PORT="8080"
export CHANNEL_BASE_DIR="/tmp/channel"
./channel
```
