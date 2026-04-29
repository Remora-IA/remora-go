# Framework Foco

Framework para priorizar el dia con foco en `resultado -> evento -> tarea -> axioma`.

## Uso

```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-foco
go run ./cmd/foco init --version v0.1.5 --result "Automatizacion tangible Gmail/WhatsApp hacia Excel usando Alfa-Echo-Bravo"
go run ./cmd/foco event --title "Demo con usuario" --date 2026-04-28 --time 14:00 --why "usuarios prueban el sistema"
go run ./cmd/foco task --title "Preparar demo" --event evt_001 --expected "demo funcional con usuario real"
go run ./cmd/foco axiom --text "La demo debe correr en HTTPS" --task task_001 --evidence "regla del producto"
go run ./cmd/foco today
go run ./cmd/foco next
go run ./cmd/foco done --id task_001 --evidence "demo lista"
go run ./cmd/foco show
```

## Archivos

Foco escribe en:

- `foco_state.json`
- `foco_state.md`

`foco_state.json` es la unica fuente de verdad del estado actual de Foco.

## Contrato

- No ejecuta commits.
- No cambia codigo de otros frameworks.
- No decide por Charlie.
- No reemplaza bugs por narrativa: si algo no funciona, se anota como pre-conflicto.
- No ofrece multiples caminos cuando existe una tarea pendiente: dirige con
  `next`.

## Flujo Primario

Foco opera con este contrato:

- primero define el resultado del dia
- cada tarea debe vincularse a un evento
- cada evento debe tener fecha
- cada axioma debe vincularse a una tarea
- `today` y `next` son la salida principal

La IA operadora no debe cerrar con preguntas abiertas si existe una tarea
pendiente para hoy.

## Conversacion

Foco conversa con una sola pregunta a la vez.

`ask` muestra la pregunta mas importante pendiente. `answer` recibe texto libre y
lo materializa primero en las capas primarias faltantes (`resultado`, `evento`,
`tarea`, `axioma`) y, si no falta ninguna, lo registra como nodos auxiliares:

- `AXIOM`: regla no negociable que define el norte.
- `HUMAN_PAIN`: dolor humano o carga mental del dia.
- `FLOW_EXPECTATION`: lo que el flujo debe cumplir.
- `EXPECTED_RESULT`: resultado observable esperado.
- `PRE_CONFLICT`: algo que impide o amenaza el resultado.
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
3. Foco lo anota como `flow`, `pre_conflict` o `done`.

Un flujo formal `paladin-foco` solo deberia crearse cuando ya existan traces de
la demo y sea repetitivo convertirlos en acciones de version.
