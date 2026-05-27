# Kobra-Carolina MVP

Implementación de referencia de Carolina (el agente conversacional de Kobra para cobranza por WhatsApp) usando Remora como librería base. Cliente real: Somos Rentable (Chile).

Este ejemplo **prueba el why de Remora contra realidad**: ¿puede un founder no-técnico, usando Claude, construir esto con Remora sin reinventar la rueda?

## Cómo correr

```bash
go build .
./kobra-carolina                          # modo stub determinístico
ANTHROPIC_API_KEY=sk-... ./kobra-carolina # modo Claude API real
```

Escribes como el deudor en stdin. Carolina responde. Termina cuando hay acuerdo, escalación, o silencio.

### Códigos de salida
- `0` — acuerdo alcanzado
- `1` — escalado a humano (hostilidad o sin avance)
- `2` — deudor no responde
- `3` — error técnico

## Lo que muestra

- Persona Carolina con tono adaptable (cercano/formal según perfil).
- Catálogo de planes — Carolina **no inventa** planes fuera del catálogo (regla central de Alfa: no inventar reglas de negocio).
- Detección de hostilidad → escalación inmediata.
- Detección de 3 rechazos seguidos → escalación por sin-avance.
- Cada turno deja traza Paladin con actor, goal, decisiones y variables.
- Salida estructurada para que Somos Rentable pueda integrar resultado en su sistema.

## Qué de Remora SÍ ayudó

- **Paladin (trace).** El trace por turno con `actor`, `goal`, `decision`, `event`, `var` es exactamente lo que se necesita para auditar conversaciones de cobranza. Esto sería 200+ líneas de logging custom sin Paladin. Bien hecho.
- **Convenciones semánticas.** Pensar en términos de `actor`/`goal`/`decision` me forzó a estructurar Carolina como agente con responsabilidad clara, no como un montón de funciones sueltas.
- **El concepto de Echo.** Aunque no importé `framework-echo`, internalizar "no preguntar 'qué quieres', descubrir el dolor real" guió el system prompt de Carolina (turnos 1-2 son indagación emocional, no oferta).

## Qué de Remora estorbó o no aplicó

- **Swarm.** Carolina no es un enjambre. Es UN agente persistente con conversación stateful. La primitiva de zonas + estigmergia no se mapea a "una conversación de WhatsApp". El multi-deudor (N conversaciones en paralelo) tampoco encaja: cada deudor es independiente, no compiten por "zonas". Para Kobra-en-producción esto pesa: la promesa "enjambre de agentes" no aplica a este caso de uso aunque suene a "agentes en equipo".
- **Bravo.** No hay un `IdealFlow` claro para una negociación. La conversación es no-determinista por diseño. Bravo asume flujo verificable; este flujo no lo es. Bravo aplicaría a *partes* (ej: "¿se respetó el catálogo de planes?") pero no al todo.
- **Echo/Alfa programáticos.** No los usé como módulos Go porque sus APIs hoy esperan árboles JSON y compilan specs — no son librerías para *runtime* conversacional. Carolina los honra conceptualmente pero no los importa.

## Lo que tuve que escribir yo (= deuda de Remora)

Las siguientes piezas existieron en este MVP como código custom. Si Remora las absorbiera, el próximo founder (Kobra-V2 o cliente nuevo) no las escribiría:

1. **Wrapper de Claude API.** Cliente HTTP + tipos del Messages API + stub determinístico para correr sin API key. ~70 líneas. Debería ser `remora/llm` o similar.

2. **Estado conversacional por agente.** `CarolinaState` con `PropuestasHechas`, `RechazosSeguidos`, `Acuerdo`, `Escalado`. Cada agente conversacional persistente necesita esto. Debería ser un primitivo de Remora (`agent.State` o similar) con persistencia opcional (Redis, SQLite, archivo).

3. **System prompt builder.** Función que arma el system prompt desde perfil + estado + catálogo + reglas. Patrón repetido en cualquier agente. Debería ser `remora/persona` con plantillas.

4. **Detectores de intención simples.** `detectarHostilidad`, `detectarConfirmacion`, `mencionaPlan` — keyword matching primitivo. Funciona para MVP pero un agente serio necesita clasificadores LLM o regex robustos. Debería ser `remora/intents` con detectores reutilizables.

5. **Integración WhatsApp.** Este MVP usa stdin. Carolina real necesita Twilio o Meta Cloud API. No implementado aquí. Debería ser `remora/channels/whatsapp` con send/receive.

6. **Persistencia multi-deudor.** Una sola instancia por proceso. Carolina real maneja N conversaciones concurrentes con estado persistente. Debería ser `remora/conversations` con storage backend.

## Verdaderas brechas del why

Después de construir esto, las 3 brechas de Remora que matan más al why hoy son:

1. **No hay `remora/llm` ni `remora/agent`.** El developer todavía pega LLM calls a mano. Eso es ~30% del código aquí.
2. **No hay `remora/conversations`.** Estado y persistencia multi-tenant son crítico, ausente.
3. **No hay `remora/channels`.** Sin integraciones (WhatsApp, email, voz) Remora es teoría — un agente sin canal no llega al deudor.

Esas 3 piezas convertirían a Remora de "librería con buena observabilidad" en "substrate real para founders no-técnicos". Hoy es lo primero.

## Próximo paso natural

Refactor: extraer `Carolina` a una `Agent` interface en Remora, mover `CallClaude` a `remora/llm`, mover el state pattern a `remora/agent/state`. Ese refactor demuestra que las primitivas son compartibles — no solo para Kobra sino para el siguiente founder que construya su propio Carolina-equivalente.
