# Framework Echo

CLI en Go para guiar reuniones de descubrimiento de procesos y construir un árbol validado por respuestas reales.

El objetivo no es preguntarle al cliente "qué automatizar". El objetivo es descubrir tareas repetitivas, dolores reales y oportunidades de automatización que encajen con la forma en que el usuario ya trabaja.

## Conceptos

- `AXIOM`: hecho confirmado por el cliente.
- `THEORY`: hipótesis inferida que debe validarse.
- `TASK`: tarea repetitiva confirmada.
- `PAIN`: dolor o impacto confirmado.
- `OPPORTUNITY`: automatización candidata anotada después de un `PAIN`; no significa que deba ofrecerse todavía.

Los nodos también pueden tener `perceptions`: notas internas sobre comportamiento, contradicciones o dolores no verbalizados.

## Uso

Para iniciar una IA operadora de Echo, entrégale primero:

```text
INITIAL_PROMPT.md
```

```bash
./frameworkecho init --project-id "registro-marcas" --client "Gamma" --date "2026-04-23"

./frameworkecho add-axiom --title "Libreta desordenada" --evidence "El cliente dice que demora recordando qué info va con qué"
./frameworkecho add-perception ax_001 --note "El cliente llama 'desastre' a la libreta: hay dolor emocional además de desorden operativo"

./frameworkecho add-theory --parent ax_001 --title "Ordenar info de clientes reduciría búsqueda" --evidence "La información existe, pero no es recuperable"
./frameworkecho validate th_001 --answer "Sí, eso ayudaría"

./frameworkecho add-task --parent th_001 --title "Registrar marca en INAPI" --evidence "Tarea repetitiva confirmada"
./frameworkecho validate tk_001 --answer "Sí, dos veces por semana"

./frameworkecho add-pain --parent tk_001 --title "Registro toma 30 min por búsqueda y transcripción" --evidence "Cliente confirma 30 min por marca"
./frameworkecho validate pn_001 --answer "Sí, ese es el problema"

./frameworkecho add-opportunity --parent pn_001 --title "Base simple de clientes" --evidence "Candidata para resolver búsqueda en libreta"
./frameworkecho select-opportunity op_001

./frameworkecho config --qa-log on
./frameworkecho log-qa --question "¿Dónde buscas esa información hoy?" --answer "En una libreta" --purpose "Confirmar fuente actual"

./frameworkecho show-tree
./frameworkecho status
./frameworkecho next-questions
```

## Reglas

- No editar `frameworkecho.json` manualmente.
- Crear `AXIOM` solo con respuesta confirmada.
- No preguntar "qué automatizar".
- No pedir al cliente que elija entre opciones técnicas.
- Anotar oportunidades no es ofrecer soluciones.
- Recomendar solo después de confirmar el dolor real y el encaje de la oportunidad.

## Desarrollo

```bash
go test ./...
go build -o frameworkecho ./cmd/frameworkecho
```
