# Nuevo Mapa: Framework Echo, Framework Alfa y Framework Bravo

Este documento es contexto para una IA agentica que puede leer archivos y usar terminal dentro de `/Users/alcless_a1234_cursor/remora-go`.

La tarea general es seguir mejorando el flujo entre tres frameworks simples en Go, diseñados para que una IA los use como apoyo operativo.

## Objetivo Global

Construir un flujo donde una IA pueda:

1. descubrir el dolor real del usuario,
2. traducir ese dolor en un flujo ideal de automatización,
3. implementar una automatización concreta,
4. verificar con evidencia si el código hizo el flujo esperado.

El fin último es crear automatizaciones útiles. No necesariamente con IA. Pueden ser scripts, cálculos locales, bases SQLite, lectura de Excel/CSV, generación de reportes, dashboards locales o procesos reproducibles desde terminal.

## Framework Echo

Ruta:

```text
/Users/alcless_a1234_cursor/remora-go/framework-echo
```

### Why

Framework Echo descubre el mundo del usuario.

Su objetivo no es preguntar "qué quieres automatizar". El usuario muchas veces no sabe qué necesita, solo conoce su dolor o una solución superficial que imagina.

Echo guía una conversación para construir un árbol validado:

```text
AXIOM -> THEORY -> TASK -> PAIN -> OPPORTUNITY
```

También guarda `perceptions`: notas internas sobre comportamiento, contradicciones o dolores no verbalizados.

Echo responde:

> ¿Cuál es el dolor real y qué oportunidad de automatización parece resolverlo?

### Rol De Echo

Echo debe:

- hacer preguntas naturales y necesarias;
- crear `AXIOM` solo con hechos confirmados;
- inferir `THEORY` y validarla;
- encontrar `TASK` repetitiva;
- confirmar `PAIN` real;
- anotar `OPPORTUNITY` candidata;
- guardar percepciones cuando una respuesta revele algo no obvio.

Echo no debe:

- preguntar "qué automatizar";
- pedir al cliente elegir entre opciones técnicas;
- ofrecer solución antes de confirmar pain;
- hacer preguntas de relleno;
- repetir preguntas que ya fueron respondidas.

### Balance Deseado

Echo debe balancear dos cosas:

1. Entender el dolor real, no solo lo que el usuario dice que quiere.
2. Llegar a una oportunidad concreta de automatización, porque el objetivo final sí es automatizar.

La condición para pasar a Alfa debería ser:

```text
Pain real confirmado
+ tarea repetitiva confirmada
+ oportunidad validada por el usuario
+ suficiente detalle para que Alfa compile o devuelva preguntas precisas
= listo para Framework Alfa
```

Echo no necesita definir toda la automatización. Eso lo hace Alfa. Echo necesita dejar claro qué duele, cuándo ocurre, a quién afecta, qué tarea lo causa, qué oportunidad fue aceptada y qué restricciones/percepciones importan.

También debe capturar el transporte de datos:

```text
dónde vive hoy la información
+ cómo puede llegar a la automatización
+ cuánta intervención humana requiere
+ si el camino es realista y confirmado
= input usable para Alfa/Bravo
```

Echo no debe asumir Excel, WhatsApp, CRM, correo, API ni scraping. Debe preguntarlo.

### Implementado: Log Opcional De Preguntas Y Respuestas

Sería valioso que Echo tenga una configuración para anotar exactamente las preguntas realizadas y las respuestas del usuario.

Motivo: poder revisar sesiones y mejorar el balance entre preguntar suficiente y preguntar demasiado.

Uso:

```bash
./frameworkecho config --qa-log on
./frameworkecho config --qa-log off
```

Cuando esté activo:

```bash
./frameworkecho log-qa \
  --question "¿Quién usa el dashboard diario?" \
  --answer "Lo usa el jefe de cobranza cada mañana" \
  --purpose "Aclarar actor y acción del dashboard"
```

Este log no debería ser parte del árbol principal. Es una bitácora de aprendizaje para evaluar:

- qué preguntas aportaron;
- cuáles fueron repetidas;
- cuáles aclararon el pain;
- cuáles cambiaron de dirección;
- cuánto tardó Echo en llegar a una oportunidad validada.

Es opcional porque a veces se quiere máxima velocidad: solo siguiente pregunta rápida, sin pasos extra.

### Limpieza Implementada: Nombre Antiguo

Echo todavía puede estar contaminado por el nombre antiguo `processtree`.

Se limpiaron referencias de docs/binarios antiguos a:

```text
processree
processtree
Processree
```

El lenguaje correcto es:

```text
Framework Echo
frameworkecho
frameworkecho.json
```

Si una IA dice "El CLI se llama processtree", probablemente está leyendo un artefacto externo o un backup fuera del framework actual.

### Problema Detectado: Preguntas Repetidas

Ejemplo problemático:

```text
Pregunta: Cuando el equipo ve el dashboard cada mañana, ¿qué pasa si el dashboard les dice que hay 50 clientes que cobrar? ¿Cómo deciden a quién empezar?
Respuesta: ya te dije, simplemente como están anotados se les cobra 1 por 1.
Pregunta siguiente: Confirmo antes de editar: ¿Tu lista es simplemente ordenada por monto de deuda o tiene algún otro orden?
Respuesta: solo por monto de deuda. Pongo un check actualmente si se llamó hoy o no.
```

La segunda pregunta repite parte de lo ya respondido. Mejor sería comprimir lo sabido y preguntar solo lo faltante:

```text
Entonces hoy el orden es monto de deuda, y solo saltan a quien ya tiene check de contacto hoy. Confirmo una cosa: ¿el check solo evita repetir contacto el mismo día?
```

Regla para Echo:

> Antes de preguntar, resume internamente lo ya sabido. Si la nueva pregunta pide información ya respondida, reformúlala para preguntar solo el hueco faltante.

## Framework Alfa

Ruta:

```text
/Users/alcless_a1234_cursor/remora-go/framework-alfa
```

### Why

Framework Alfa es el mediador entre Echo y Bravo.

No descubre dolores desde cero y no debuggea código directamente. Compila intención.

Echo habla en lenguaje de descubrimiento:

```text
AXIOM, THEORY, TASK, PAIN, OPPORTUNITY, PERCEPTION
```

Bravo habla en lenguaje de ejecución/verificación:

```text
IdealFlow, rules, critical_vars, critical_path, trace
```

Alfa traduce entre ambos mundos:

```text
frameworkecho.json -> alfa_spec.json -> ideal_flow.json
```

Alfa responde:

> ¿Cómo traduzco el conocimiento validado del usuario en un flujo ideal verificable por código?

### Rol De Alfa

Alfa debe:

- leer `frameworkecho.json`;
- seleccionar una o más `OPPORTUNITY` validadas;
- recorrer linaje `OPPORTUNITY -> PAIN -> TASK -> THEORY -> AXIOM`;
- generar `alfa_spec.json`;
- generar `ideal_flow.json` compatible con Bravo;
- marcar `export_ready=false` si falta información;
- devolver `open_questions` para Echo si no puede compilar sin inventar.

Alfa no debe inventar reglas de negocio.

Tampoco debe inventar fuentes de datos ni integraciones. Si Echo no confirmó cómo entran los datos a la automatización, Alfa debe marcar `export_ready=false` y devolver preguntas a Echo.

Ejemplo:

```text
Echo dice:
"El cliente necesita priorizar cobranzas por riesgo, no solo por fecha."

Alfa traduce:
1. cargar cartera
2. calcular riesgo por cliente
3. ponderar monto, antigüedad e historial
4. ordenar clientes por prioridad
5. generar lista diaria
6. generar resumen semanal
```

Si falta ponderación de riesgo, Alfa no inventa. Devuelve una pregunta:

```text
Cuando dices riesgo de no pago, ¿qué señales pesan más: antigüedad, monto, comportamiento histórico u otra cosa?
```

### Estado Actual De Alfa

Alfa ya existe como CLI mínimo.

Comandos:

```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-alfa

./frameworkalfa compile \
  --echo-tree ../framework-echo/frameworkecho.json \
  --out temp/alfa_spec.json

./frameworkalfa inspect --spec temp/alfa_spec.json

./frameworkalfa export-bravo \
  --spec temp/alfa_spec.json \
  --out temp/ideal_flow.json
```

Para compilar una oportunidad específica:

```bash
./frameworkalfa compile \
  --echo-tree ../framework-echo/frameworkecho.json \
  --opportunity op_001 \
  --out temp/alfa_spec_op_001.json
```

### Resultado Actual Con El Árbol De Cobranzas

Alfa detectó:

```text
export_ready: false
open_questions: 4
ideal_flow: draft
```

Preguntas abiertas generadas:

1. Antes de usar "Factores de riesgo: tiempo, monto, comportamiento histórico" como base, ¿puedes confirmar o reformular esta tarea?
2. Cuando dices riesgo de no pago, ¿qué señales pesan más: antigüedad, monto, comportamiento histórico u otra cosa?
3. Cuando dices resumen semanal, ¿qué decisión debería poder tomar gerencia al verlo?
4. ¿Quién usa el dashboard diario y qué acción concreta debe tomar después de verlo?

Esto es correcto: Alfa generó un draft para Bravo, pero marcó lo que todavía no debe tratarse como definitivo.

### Implementado: Selección Persistente De Oportunidades

Hoy Echo puede anotar y validar oportunidades. Alfa puede compilar todas las validadas o una específica usando `--opportunity`.

Pero Echo no tiene un mecanismo persistente para marcar "estas oportunidades son las elegidas para trabajar".

Echo permite:

```bash
./frameworkecho select-opportunity op_001
./frameworkecho selected-opportunities
```

Así Alfa podría compilar por defecto solo las seleccionadas, no todas las validadas.
Alfa compila por defecto las seleccionadas cuando `selected_opportunity_ids` existe. Si no hay selección, mantiene compatibilidad y compila todas las oportunidades validadas.

## Framework Bravo

Ruta:

```text
/Users/alcless_a1234_cursor/remora-go/framework-bravo
```

### Why

Framework Bravo verifica el mundo del código.

Sirve especialmente cuando el código parece estar bien programado, no hay bugs obvios ni errores de sintaxis, pero el flujo sigue sin ser lo que el usuario quería.

El problema ahí no es que la IA programe mal una función. El problema es que la IA no entendió bien el flujo deseado.

Bravo compara:

```text
ideal flow -> trace real -> diferencias
```

Bravo responde:

> ¿El sistema ejecutado se comporta como el flujo ideal esperado?

### Rol De Bravo

Bravo debe:

- instrumentar código con spans;
- registrar variables críticas;
- registrar decisiones con `ctx.Decision`;
- registrar errores;
- generar trace real;
- permitir que una IA compare trace real vs IdealFlow.

Bravo no debe inventar el flujo ideal. Ese flujo viene de Alfa.

Bravo puede construir prototipos mínimos para validar la idea con el cliente antes de seguir desarrollando. Después del prototipo debe existir una decisión explícita:

```text
cliente aprueba prototipo -> seguir a versión más completa
cliente no aprueba prototipo -> registrar motivo y devolver el gap a Alfa/Echo
```

Si el cliente no aprueba, Bravo no sigue construyendo a ciegas. El rechazo se trata como evidencia:

- output incorrecto -> Alfa ajusta IdealFlow o pregunta a Echo;
- pain no resuelto -> Echo vuelve al dolor real;
- datos difíciles de obtener -> Echo aclara transporte de datos;
- oportunidad incorrecta -> Echo revisa otra oportunidad validada o descubre una nueva.

### Regla Local-First Para Automatizaciones

Por ahora, las automatizaciones de Bravo deberían ser local-first.

Esto significa priorizar soluciones que la IA pueda construir y ejecutar localmente con terminal, sin depender de APIs externas.

Ejemplos válidos:

- leer CSV/XLSX locales;
- guardar datos en SQLite local;
- generar reportes HTML/CSV/PDF;
- calcular rankings, scores y métricas;
- limpiar y cruzar datos;
- generar dashboards locales;
- crear scripts reproducibles;
- procesar archivos entregados por el cliente vía Echo.

Evitar por ahora:

- depender de APIs externas;
- requerir credenciales;
- automatizar WhatsApp, email o sistemas externos si no hay acceso explícito;
- construir flujos que solo funcionan con integraciones no confirmadas.

Si Bravo necesita un dato externo, Alfa debe generar una pregunta para Echo.

Ejemplo:

```text
Para calcular este ranking necesito los 3 Excel actualizados diariamente.
¿El cliente puede entregarlos como archivos locales?
```

Regla:

> Antes de usar APIs externas, Bravo debe intentar resolver con archivos locales, SQLite, scripts y cálculos reproducibles. Si falta un dato externo, Alfa devuelve una pregunta a Echo.

## Flujo Completo Esperado

```text
Echo
  descubre dolor real
  valida oportunidad
  opcionalmente registra Q/A si qa-log está activo
        ↓
Alfa
  compila árbol Echo a alfa_spec
  si falta información, genera open_questions para Echo
  si está listo, exporta IdealFlow para Bravo
        ↓
Bravo
  implementa automatización local-first
  instrumenta con trace
  compara trace real vs IdealFlow
        ↓
Alfa
  si Bravo detecta gap semántico, traduce ese gap en pregunta para Echo
        ↓
Echo
  pregunta al usuario lo faltante
```

Resumen simple:

```text
Echo pregunta bien.
Alfa traduce bien.
Bravo verifica bien.
```

## Cómo Evaluar Una Sesión Echo Nueva

Cuando se entregue un chat de Echo, revisar:

1. ¿Qué preguntas fueron buenas?
2. ¿Qué preguntas fueron repetidas?
3. ¿Dónde aclaró dolor real?
4. ¿Dónde se fue demasiado pronto a solución?
5. ¿Qué nodos debió crear, validar, editar o rechazar?
6. ¿Las OPPORTUNITIES aceptadas por el usuario quedaron anotadas?
7. ¿Hay suficiente detalle para Alfa?
8. Si Alfa genera `open_questions`, ¿son buenas preguntas para devolver a Echo?

## Cómo Saber Si Un IdealFlow Está Bien

La prueba no es que exista `ideal_flow.json`.

La prueba es:

> ¿Bravo podría mirar un trace real y decir si el código cumplió o no cumplió este flujo?

Checklist:

- `export_ready` es `true`.
- `open_questions` está vacío.
- Cada `OPPORTUNITY` mapea a un `PAIN` validado.
- Cada regla es verificable en código.
- Cada variable crítica puede registrarse con `ctx.Var`.
- El `critical_path` parece una secuencia real de ejecución.
- No hay reglas vagas tipo "hacer buen análisis".
- El output esperado está claro: ranking, score, resumen, métricas, destinatario, acción siguiente.
- El input esperado y su transporte desde la operación actual están claros.
- Si hay prototipo, queda claro qué validó el cliente y qué rechazó.

Si falta algo, Alfa debe devolver preguntas para Echo, no inventar.

## Próximo Trabajo Sugerido

1. Evaluar el nuevo chat de Echo completo.
2. Identificar preguntas repetidas y preguntas útiles.
3. Recompilar Alfa hasta lograr `export_ready=true`.
4. Usar Bravo para implementar automatización local-first y verificar trace vs IdealFlow.
