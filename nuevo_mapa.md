# Nuevo Mapa: Framework Echo, Framework Alfa y Framework Bravo

Este documento es contexto para una IA agentica que puede leer archivos y usar terminal dentro de `/Users/alcless_a1234_cursor/remora-go`.

La tarea general es seguir mejorando el flujo entre tres frameworks simples en Go, diseñados para que una IA los use como apoyo operativo.

## Objetivo Global

Construir un flujo donde una IA pueda:

1. descubrir el dolor real del usuario,
2. consultar temprano a Alfa para idear una primera automatización candidata,
3. traducir esa automatización en un flujo ideal,
4. implementar una automatización concreta,
5. verificar con evidencia si el código hizo el flujo esperado.

El fin último es crear automatizaciones útiles rápido. No necesariamente con IA. Pueden ser scripts, cálculos locales, bases SQLite, lectura de Excel/CSV, APIs oficiales, generación de reportes, dashboards locales o procesos reproducibles desde terminal.

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
- pedir recursos reales cuando reducen mejor la incertidumbre que una respuesta verbal;
- formar acuerdos operativos mínimos cuando la automatización depende de contexto que hoy no existe;
- consultar a Alfa temprano cuando ya existen tarea repetitiva y dolor real, para aterrizar una primera iteración;
- descubrir reglas de negocio y estructura de datos suficientes para que Alfa compile un MERE sin inventar;
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
- obligar al usuario a describir de memoria algo que puede verse en una captura, archivo, factura, correo o chat de ejemplo;
- asumir que podrá relacionar datos sin contexto cuando hoy llegan sueltos;
- seguir preguntando indefinidamente para evitar el riesgo de construir un prototipo;
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

Pero existe un paso anterior:

```text
tarea repetitiva confirmada
+ pain real confirmado
= consultar Alfa temprano para draft de primera automatización
```

Ese draft no significa solución final. Significa que Alfa propone una primera iteración y devuelve gaps concretos:

```text
automatización candidata
+ fuente de datos necesaria
+ recursos reales que hay que mirar
+ contexto faltante
+ APIs/permisos/credenciales por confirmar
+ acuerdos humanos mínimos
= preguntas de Echo mucho más precisas
```

Antes de automatizar, Alfa debe poder describir:

```text
estructura actual de datos
+ reglas de negocio confirmadas
+ MERE normalizado propuesto
+ relaciones/cardinalidades claras
+ gaps explícitos si falta una regla
= automatización que no alucina ni replica el caos
```

Ejemplo de gap crítico, solo como caso particular de una regla de cardinalidad:

```text
"¿Manejan pagos parciales o cada movimiento de dinero corresponde a una compra/factura pagada en su totalidad en ese momento?"
```

La regla general no es "pagos y facturas". La regla general es: cuando la automatización necesita relacionar elementos, Alfa debe saber si la relación es 1 a 1, 1 a muchos, muchos a muchos, parcial, temporal o con excepciones. Si no lo sabe, Echo pregunta esa regla o pide un recurso real que la muestre.

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

Si no existe fuente estructurada, Echo debe descubrir por qué no existe antes de proponer carga manual. "No hay planilla" puede significar fricción, pereza, baja habilidad con Excel, computador poco confiable, operación basada en WhatsApp, información en la memoria de una persona o falta real de fuente.

Si una oportunidad depende de que el usuario registre datos, Echo debe confirmar:

```text
momento real de captura
+ mínimo dato útil
+ esfuerzo tolerado
+ señal de que el hábito no rompe su flujo actual
= captura manual viable
```

Si la brecha de información puede cerrarse con evidencia, Echo debe pedir un recurso antes de pedir una descripción larga:

```text
 captura/pantallazo/foto/archivo/factura/chat de ejemplo
+ idealmente anonimizado o con datos sensibles tapados
+ mensajes o contexto alrededor si el recurso aislado no basta
= estructura real entendida con menos carga para el usuario
```

Ejemplo:

```text
Mejor:
"¿Tienes un ejemplo anonimizado de cómo llega una transferencia, incluyendo el pantallazo y los mensajes que normalmente vienen antes o después?"

Peor:
"Las capturas que llegan por WhatsApp, ¿el monto, fecha y quién paga ya se entiende de la imagen, o hay que escribirlo a mano?"
```

Si la automatización necesita contexto que hoy no existe, Echo debe convertirlo en un acuerdo explícito, no en una pregunta abstracta:

```text
dato que falta para automatizar
+ momento en que el humano lo puede agregar
+ formato mínimo tolerable
+ compromiso confirmado
= contexto operativo viable
```

Ejemplo:

```text
"Para automatizar esto necesito unir cada recurso con el registro correcto. Si hoy ese contexto no viene escrito, ¿te podrías comprometer a agregar después un mensaje corto con la referencia mínima acordada?"
```

Echo no debe extraer diseño detallado del usuario. Debe extraer verdad operativa suficiente. Si el usuario responde "no sé" en una rama donde ya existen pain, impacto, input mínimo y restricción crítica, Echo debe dejar de profundizar y validar una hipótesis mínima concreta.

Regla anti-entrevista-infinita:

```text
pain confirmado
+ tarea repetitiva
+ impacto real
+ oportunidad aceptada
+ transporte o momento de captura
+ restricción de fricción
= cerrar discovery y pasar a Alfa
```

Esta regla no debe depender solo del prompt. Echo debe exponer un semáforo mecánico:

```bash
./frameworkecho readiness
```

Salida esperada:

```text
ready_for_alfa: true|false
recommended_action: ask_next_missing_fact|validate_minimum_hypothesis|select_opportunity|pass_to_alfa
next_question: ...
risks: ...
checks:
  task_confirmed
  pain_confirmed
  opportunity_validated
  opportunity_selected
  data_transport_confirmed
  manual_capture_viability
```

La IA operadora razona con el usuario, pero no decide sola si seguir cavando o cerrar discovery. Debe consultar este comando y usar su recomendación como riel operativo.

Echo también debe registrar señales conversacionales sin depender de `qa-log`:

```bash
./frameworkecho signal --type fatigue --note "El usuario dijo: estás preguntando muchas cosas"
./frameworkecho signal --type unknown --note "El usuario dijo: no tengo idea"
./frameworkecho signal --type confusion --note "El usuario dijo: no te entiendo"
```

Si hay core discovery suficiente y aparece fatiga, `readiness` puede devolver:

```text
recommended_action: close_discovery_with_risk
risks: manual_capture_viability_unconfirmed
```

Eso significa: no seguir preguntando al cliente. Echo cierra discovery, Alfa compila un draft con riesgo explícito, y Bravo solo puede prototipar para validar esa hipótesis, no declarar solución definitiva.

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
- si Echo consulta temprano, generar un draft desde TASK + PAIN aunque todavía no exista OPPORTUNITY validada;
- recorrer linaje `OPPORTUNITY -> PAIN -> TASK -> THEORY -> AXIOM`;
- generar `alfa_spec.json`;
- generar `ideal_flow.json` compatible con Bravo;
- marcar `export_ready=false` si falta información;
- devolver `open_questions` para Echo si no puede compilar sin inventar.

Alfa no debe inventar reglas de negocio.

Tampoco debe tratar fuentes de datos ni integraciones como confirmadas si Echo no las confirmó. Pero sí debe proponer hipótesis concretas de integración para una primera iteración, por ejemplo API de WhatsApp + API/archivo de Excel + cruce de información, y marcar qué falta confirmar.

Alfa tampoco debe asumir viabilidad operacional. Si la automatización requiere captura, registro, formulario, planilla o cualquier hábito manual nuevo, Echo debe haber confirmado cuándo se hará y cuánto esfuerzo tolera el usuario. Si no, Alfa debe marcar `export_ready=false`.

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

Si Bravo usa datos de ejemplo, el resultado es solo un prototipo no validado. No debe declarar `pain_resolved=true` como conclusión de negocio hasta que el cliente valide la salida. Estados separados:

```text
prototipo_ejecuta=true
trace_verificable=true
cliente_aprueba_prototipo=true|false
pain_resolved=true solo con validación suficiente
```

### Regla De Integraciones Para Automatizaciones

Las automatizaciones no deben limitarse a local-first.

La ruta preferida depende de la fuente real:

Ejemplos válidos:

- leer CSV/XLSX locales;
- conectarse a APIs oficiales con permisos y credenciales confirmadas;
- guardar datos en SQLite local;
- generar reportes HTML/CSV/PDF;
- calcular rankings, scores y métricas;
- limpiar y cruzar datos;
- generar dashboards locales;
- crear scripts reproducibles;
- procesar archivos entregados por el cliente vía Echo.

Evitar por ahora:

- requerir credenciales si el cliente no puede o no quiere entregarlas;
- automatizar WhatsApp, email o sistemas externos si no hay acceso explícito;
- automatizar usando interfaces visuales, clicks, navegación manual o simulación de usuario como si fuera una integración estable;
- construir flujos que solo funcionan con integraciones no confirmadas.

Si Bravo necesita un dato externo, Alfa debe generar una pregunta para Echo.

Ejemplo:

```text
Para calcular este ranking necesito los 3 Excel actualizados diariamente.
¿El cliente puede entregarlos como archivos locales?
```

Regla:

> APIs oficiales son válidas si hay permisos, credenciales y estabilidad suficiente. Lo que debe evitarse es usar interfaces visuales con clicks/navegación como integración principal, salvo workaround temporal explícitamente aceptado.

## Flujo Completo Esperado

```text
Echo
  descubre dolor real
  opcionalmente registra Q/A si qa-log está activo
        ↓
Alfa
  si hay task + pain, puede compilar draft temprano
  propone primera automatización candidata y gaps concretos
        ↓
Echo
  pide recursos reales o acuerdos mínimos para cerrar esos gaps
        ↓
Alfa
  compila árbol Echo a alfa_spec
  si falta información, genera open_questions para Echo
  si está listo, exporta IdealFlow para Bravo
        ↓
Bravo
  implementa automatización por archivo, API o integración confirmada
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
4. Usar Bravo para implementar automatización por archivo, API o integración confirmada y verificar trace vs IdealFlow.
