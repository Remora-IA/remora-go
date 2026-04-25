# Remora Flujo

Orquestador handoff para Echo, Alfa y Bravo.

El runtime agentico es nativo en Go. No depende de Tau/Pi.

La idea nueva es que las IAs no se pasen prompts entre si. Cada una trabaja con su framework y deja artefactos persistidos:

- Echo: `../framework-echo/frameworkecho.json`
- Alfa: `../framework-alfa/temp/alfa_spec.json` y `../framework-alfa/temp/ideal_flow.json`
- Bravo: `../framework-bravo/temp/`

El handoff vive en:

```bash
temp/handoff/state.json
```

## Comandos

Desde este directorio:

```bash
go run ./cmd/flujo status
go run ./cmd/flujo next
go run ./cmd/flujo run --dry-run
go run ./cmd/flujo run
go run ./cmd/flujo chat
go run ./cmd/flujo reply "respuesta del usuario"
```

Requiere una API key disponible en el entorno:

```bash
export MINIMAX_API_KEY="..."
```

Cuando el siguiente rol es Echo, `run` abre un chat local interactivo:

```text
Tu >
```

Escribe ahi tus respuestas. No escribas la respuesta directo en zsh.

Reset rapido:

```bash
go run ./cmd/flujo reset
go run ./cmd/flujo reset --all
```

`reset` borra solo handoff/sesiones del flujo. `reset --all` tambien borra el arbol de Echo y artefactos temporales de Alfa/Bravo.

## Eventos

Cada IA se apaga/pasa el mando con eventos:

```bash
go run ./cmd/flujo done echo --event echo_ready_for_alfa --message "discovery listo"
go run ./cmd/flujo done echo --event echo_waiting_user --message "pregunta hecha"
go run ./cmd/flujo done alfa --event alfa_ready_for_bravo --message "ideal_flow listo"
go run ./cmd/flujo ask-echo --from alfa --question "pregunta concreta"
go run ./cmd/flujo done bravo --event bravo_done --message "resultado listo"
go run ./cmd/flujo ask-echo --from bravo --question "pregunta concreta"
```

## Politica

- Solo hay un Alfa.
- Echo es el unico que conversa con el usuario.
- Alfa y Bravo pueden pedir una pregunta, pero la pregunta vuelve a Echo por handoff.
- La informacion importante debe vivir en archivos de framework, no dentro de prompts transferidos.
- El agente Go tiene herramientas locales minimas: `bash`, `read_file`, `write_file`, `list_files`.

## RPC Nativo

Para uso headless/produccion:

```bash
go run ./cmd/agentrpc
```

Protocolo JSONL por stdin/stdout:

```json
{"id":"1","type":"prompt","role":"echo","message":"hola"}
{"id":"2","type":"shutdown"}
```

## Observabilidad Con Paladin

`flujo run`, `flujo chat` y `flujo reply` generan traces Paladin en:

```bash
temp/paladin/
```

Ver ultimo trace desde `remora-flujo`:

```bash
go run ../framework-paladin/cmd/paladin
```

Ver un trace especifico:

```bash
go run ../framework-paladin/cmd/paladin temp/paladin/trace_pal_....json
```
