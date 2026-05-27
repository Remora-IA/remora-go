# WHY - Remora

Remora existe para crear automatizaciones útiles rápido contra dolores reales.
No para construir frameworks.

Los frameworks (Echo, Alfa, Bravo, Foco, Paladin, Swarm, Charlie, Excel, Gmail,
Quine) son medios. El fin es que un usuario fuera del equipo use una
automatización al día siguiente y vuelva.

## El Riesgo Operativo Actual

Cada framework nuevo suma superficie sin demostrar que el ciclo completo
entrega valor. Hoy el repo sabe describir un flujo. Todavía no demostró que
ese flujo, corriendo end to end, le resuelva un dolor a un usuario fuera del
equipo en un plazo corto.

Mientras eso no ocurra, agregar SDK público, contratos de `Framework`
unificados, pipelines tipo LangChain/Mastra o nuevos frameworks es *walking*
— diseñar la interfaz antes de que duela no tenerla.

## El Gate

No se agrega un framework nuevo, no se extrae `pkg/remora/` como librería, no
se introduce abstracción opinada arriba del flujo Echo→Alfa→Bravo, hasta que
al menos el Roce 1 esté corrido y registrado.

Si una abstracción parece urgente antes de eso, el síntoma es que falta
información del mundo real, no que falta arquitectura.

## Tres Roces Contra La Realidad

### Roce 1 — Cliente real, caso real, 48h

Un usuario fuera del equipo. Un dolor que no sea cobranzas (sesgo del árbol
Echo actual). Desde descubrimiento hasta automatización corriendo en su
computador en menos de 48 horas.

Mide:

- ¿Echo pregunta bien cuando el operador no es el autor del framework?
- ¿Alfa compila sin inventar en un dominio nuevo?
- ¿Bravo entrega algo que el cliente usa al día siguiente o lo abandona?

Resultado registrado en `temp/roce_1/` con: árbol Echo final, `alfa_spec`,
`ideal_flow`, trace Paladin, output entregado, y nota del cliente al día +1.

### Roce 2 — Mismo dolor, dos rutas

Un mismo caso corrido en paralelo con el pipeline lineal de `remora-flujo`
(Echo→Alfa→Bravo) y con `framework-swarm` (estigmergia).

Mide:

- ¿El enjambre aporta o es complejidad bonita?
- Si convergen al mismo output, swarm es overhead hoy.
- Si divergen, aprendimos para qué tipo de dolor sirve cada patrón.

Sin esa comparación, swarm es teoría arquitectónica.

### Roce 3 — Otro operador, otro modelo

Cambiar al humano que conduce el flujo y al LLM detrás (no el autor del
repo, no Claude).

Mide:

- ¿Echo, Alfa y Bravo son frameworks o son prompts personales disfrazados?
- Si solo funcionan en una combinación específica de operador + modelo, no
  es librería. Es un workflow personal.

Esa pregunta decide si la conversación sobre "Remora como librería tipo
LangChain" siquiera tiene sentido todavía.

## Qué Es Walking Y Qué Es Running Hoy

Walking ahora:

- Construir `pkg/remora/` con contrato `Framework` y pipeline builder.
- Refactorizar Echo/Alfa/Bravo para cumplir un contrato común.
- Agregar otro framework al árbol.
- Documentar más la arquitectura.

Running ahora:

- Encontrar el primer cliente real para el Roce 1.
- Correrlo end to end con el flujo actual, sin retoques previos.
- Registrar dónde se rompió.
- Recién ahí decidir qué framework, abstracción o cambio responde a ese gap
  específico.

## Cómo Se Lee Esta Decisión

Si una IA agéntica abre este repo y propone construir una librería, un SDK,
un contrato unificado o un nuevo framework antes de que `temp/roce_1/`
exista, está violando este WHY. La respuesta correcta es: primero el roce,
después la abstracción.
