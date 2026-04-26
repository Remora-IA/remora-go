# Framework Foco

Framework para catastro diario, anotaciones de flujo y definicion del why de la
version.

## Uso

```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-foco
go run ./cmd/foco init --version v0.1.5 --objective "Automatizacion tangible Gmail/WhatsApp hacia Excel usando Alfa-Echo-Bravo"
go run ./cmd/foco ask
go run ./cmd/foco answer --text "Me preocupa no saber que tocar primero y que la demo no pase realmente por Alfa-Echo-Bravo"
go run ./cmd/foco axiom --text "Echo debe dar contexto suficiente en maximo 2 preguntas para que Alfa pueda pedir recursos tangibles" --evidence "Regla declarada por el humano"
go run ./cmd/foco note --kind human --text "Estoy improvisando demasiado que tocar primero"
go run ./cmd/foco note --kind flow --text "El demo debe pasar por Alfa-Echo-Bravo, no solo por comandos sueltos"
go run ./cmd/foco note --kind done --text "Existe comando demo que genera o actualiza Excel con evidencia"
go run ./cmd/foco tree
go run ./cmd/foco readiness
go run ./cmd/foco plan
go run ./cmd/foco next
go run ./cmd/foco done --id task_001 --evidence "Echo produjo arbol y Alfa lo leyo"
go run ./cmd/foco block --id task_001 --reason "Alfa no encuentra el archivo de Echo"
go run ./cmd/foco show
```

## Archivos

Foco escribe en:

- `temp/foco/today.json`
- `temp/foco/today.md`

## Contrato

- No ejecuta commits.
- No cambia codigo de otros frameworks.
- No decide por Charlie.
- No reemplaza bugs por narrativa: si algo no funciona, se anota como bloqueo.
- No ofrece multiples caminos cuando existe una tarea pendiente: dirige con
  `next`.

## Checklist De Ejecucion

Foco mantiene una cola persistente de tareas. No borra tareas completadas ni
bloqueadas.

- `plan`: muestra todo el checklist.
- `next`: dice exactamente que toca ahora.
- `done`: marca una tarea como completada y muestra la siguiente.
- `block`: marca una tarea como bloqueada y muestra la siguiente.

La IA operadora de Foco debe usar `next` cuando el catastro ya esta listo. No
debe cerrar con "que quieres hacer ahora?" si existe una tarea pendiente.

## Conversacion

Foco conversa con una sola pregunta a la vez.

`ask` muestra la pregunta mas importante pendiente. `answer` recibe texto libre y
lo materializa como nodos:

- `AXIOM`: regla no negociable que define el norte.
- `HUMAN_PAIN`: dolor humano o carga mental del dia.
- `FLOW_EXPECTATION`: lo que el flujo debe cumplir.
- `EXPECTED_RESULT`: resultado observable esperado.
- `BLOCKER`: algo que impide o amenaza el resultado.
- `OBSERVATION`: anotacion util que no cae en otra categoria.

## Axiomas

Usa `axiom` para registrar reglas fuertes, por ejemplo frases del humano como
"si o si", "no debe pasar por ningun motivo" o "tiene que funcionar asi".

Un axioma no es una preferencia. Las preferencias pueden cambiar durante el
catastro; los axiomas definen el norte hasta que el humano los revoque
explicitamente.

## Paladin

Paladin puede alimentar a Foco con evidencia, pero no hace falta crear un flujo
formal `paladin-foco` para la version actual.

Regla de hoy:

1. Paladin genera o explica traces.
2. El humano o una IA resume el hallazgo relevante.
3. Foco lo anota como `flow`, `blocker` o `done`.

Un flujo formal `paladin-foco` solo deberia crearse cuando ya existan traces de
la demo y sea repetitivo convertirlos en acciones de version.
