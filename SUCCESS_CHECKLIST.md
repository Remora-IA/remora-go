# Brief para test ciego — Kobra-Carolina

**Para uso en sesión nueva de Claude sin contexto previo. No referencia frameworks específicos de Remora a propósito — la idea es ver si Claude descubre solo qué piezas de la librería sirven.**

---

## Lo que tienes que construir

Un agente conversacional llamado **Carolina** que negocia cobranza de deuda por WhatsApp. Su cliente es Kobra (Growth-as-a-Service), que a su vez sirve a empresas como Somos Rentable (crowdfunding inmobiliario chileno) que necesitan recuperar pagos atrasados.

## Lo que Carolina hace

1. Inicia el contacto con un deudor por mensaje. Saluda, valida emocionalmente, **no menciona números en el primer turno**.
2. En el segundo turno pregunta si el deudor puede hacer esfuerzo único o necesita cuotas.
3. Recién en el tercer turno ofrece 1 o 2 planes de pago del catálogo predefinido.
4. Si el deudor confirma un plan, cierra el acuerdo y agradece.
5. Si el deudor pide algo fuera del catálogo, no inventa — dice que tiene que consultar.
6. Si el deudor se pone hostil, escala a humano inmediatamente sin discutir.
7. Si después de 3 propuestas no hay avance, escala a humano por sin-avance.

## Catálogo de planes (no inventes otros)

Dado una deuda total D:
- 1 cuota de D × 0.92 con 8% de descuento por pronto pago.
- 3 cuotas de D / 3 sin recargo.
- 6 cuotas de D × 1.06 / 6 con recargo por mora.

## Caso de prueba

Patricia Morales. Deuda CLP $847.000. 38 días de atraso. Historial: puntual hasta 2024. Tono preferido: cercano.

## Restricciones técnicas

- En Go.
- Persistente: la conversación debe sobrevivir reinicios del proceso (deudor puede tardar horas o días en responder).
- Multi-tenant: en el futuro habrá N deudores en paralelo, así que el diseño no puede asumir global state.
- Trace observable: debe quedar registro auditable de cada decisión que Carolina toma (por qué propuso ese plan, por qué escaló, etc).
- Modo dev sin gastar tokens: si no hay `ANTHROPIC_API_KEY`, el agente debe correr con respuestas stub determinísticas para poder probar el flujo.
- Por ahora la "WhatsApp" es la consola (stdin/stdout). Producción cambiará a WhatsApp real más adelante.

## Lo que tienes a disposición

El repositorio en `github.com/Remora-IA/remora-go`. Léelo. Si encuentras piezas que sirvan, úsalas. Si no encuentras lo que necesitas, escríbelo a mano y anota qué te faltó.

## Lo que el operador del test mide

1. ¿Encontró el operador (Claude) las piezas adecuadas del repo, o tuvo que escribir todo a mano?
2. ¿Cuántas líneas de código escribió que **deberían** haber sido librería?
3. ¿Llegó a un MVP funcional que cierre acuerdos por consola? ¿Cuánto tardó?
4. ¿Qué documentación buscó y no encontró?
5. ¿Qué decisiones técnicas tomó que sugieren que el diseño del repo no es claro?

## Reporte final esperado

Al terminar (o al estancarte), produce un archivo `REMORA_FEEDBACK.md` con:

- Lo que usaste del repo y por qué.
- Lo que tuviste que escribir a mano y por qué no estaba en el repo.
- Lo que el repo te dificultó (docs confusas, módulos rotos, naming inconsistente).
- Qué sugerirías que se agregue para que el próximo Claude no tenga que escribir lo que tú escribiste.

Eso es lo que de verdad estamos midiendo.
