# Prompt: Vision completa de Remora — flujos, vistas y operacion real

## 0. Instrucciones para la IA que recibe este prompt

Sos una nueva sesion de Devin trabajando en el repo `remora-go`. Tu trabajo NO es aplicar ciegamente este documento como si fuera una especificacion cerrada. Tu trabajo es:

1. Entender la vision completa del producto.
2. Investigar el estado real del codebase.
3. Separar lo que ya esta implementado de lo que falta.
4. Resolver primero el bloqueo que impide probar flujos reales.
5. Implementar la vision de manera incremental, verificable y sin romper las reglas duras del repo.

Este prompt reemplaza prompts anteriores sobre rediseño de flujo/pipeline. No dependas de documentos viejos si existen en historial, ramas o contexto externo. El estado del proyecto probablemente ya avanzo respecto a esos prompts anteriores.

Antes de modificar codigo:

- Lee `ARCHITECTURE.md`.
- Lee `HANDOFF_PROMPT.md`.
- Lee este archivo completo.
- Inspecciona los archivos clave de `remora-flujo/cmd/api_rest/`.
- Verifica el estado real de los manifests de frameworks.
- Corre busquedas para confirmar si una funcionalidad ya existe antes de reimplementarla.

No hagas promesas grandes sin verificar. Si una parte de la vision requiere una decision de arquitectura que no esta clara, deja una propuesta concreta y pequeña.

### Falla actual observada por el usuario

El usuario probo un flujo desde el **modal de Flujos** usando el boton de prueba. La corrida hizo esto:

```text
Radar configura analisis -> OK
Usuario simulado acepta -> OK
Sabio dataset-export -> FALLA
Radar prioritize -> FALLA
Flujo completo -> FALLA
```

El error repetido fue:

```text
Channel no esta disponible en http://127.0.0.1:59544.
Normalmente se inicia automaticamente. Verifica que el base_dir del repo sea correcto.
```

Esto es critico. El puerto `59544` sugiere que el API intento iniciar Channel en un puerto dinamico mediante `ensureChannel`, pero Channel no quedo disponible. Antes de rediseñar UI, resolver esto o ninguna prueba real funcionara.

## 1. Que es Remora y para quien existe

Remora es un sistema de IAs autonomas (frameworks) que orquesta procesos de negocio. Hay tres capas de usuarios:

1. **Staff de Remora**: construyen flujos (automatizaciones) para los clientes de Remora. Prueban los flujos antes de entregarlos.
2. **Clientes de Remora**: empresas que usan Remora para automatizar procesos de sus negocios. Ejemplo: Panalbit (empresa de cobranza).
3. **Usuarios finales**: personas que trabajan para los clientes de Remora. Ejemplo: un cobrador de Panalbit que usa Remora para gestionar su cartera de deudores.

El Staff construye flujos y los prueba en un **modal web (vista de prueba)**. Los usuarios finales operan los flujos en una **vista de nodos (operacion real)**. Son dos vistas completamente distintas con propositos distintos.

---

## 2. Filosofia (NO negociable)

- **Cada framework es autonomo**. Se comunican por JSON (Channel JSON-RPC stdin/stdout). Nunca imports cruzados ni memoria compartida.
- **Routing es capability-based, no name-based.** El orquestador NO hace `if framework == "sabio"`. Pregunta "quien produce capability X".
- **Cada flujo tiene un resultado esperado.** El flujo no esta listo hasta que puede completar un ciclo de principio a fin.
- **Las pruebas siempre son con datos reales.** Nunca mockups ni respuestas hardcodeadas. Los emails se redirigen al correo de prueba (`TEST_EMAIL_RECIPIENT`).
- **Un ciclo = una tarea completada.** Foco sabe cuantas tareas hay hoy. Cada tarea es un ciclo del flujo.
- **Sin emojis** en codigo ni commits salvo pedido explicito.
- **Codigo en ingles** salvo strings de UI en espanol.
- **`panalbit.db` es READ-ONLY** siempre. Nunca escribir en ella.

---

## 3. Arquitectura de capas de frameworks

Los frameworks NO son todos iguales. Operan en capas. Esta clasificacion es fundamental para entender como se organizan las vistas y como fluye la informacion.

### Capa 1: User-facing (interactuan directamente con el usuario)

- **Foco**: priorizador del dia. Sabe que tareas hay, las prioriza, entrega la siguiente tarea. Siempre es el punto de entrada despues del bootstrap. Genera `action_options` para que el usuario elija que hacer con cada tarea.
- **Mensajero**: envia correos. Interactua con el usuario para confirmar el envio.

### Capa Bootstrap (solo primera vez, luego desaparece)

- **Radar**: configura como se van a analizar los datos. Propone una configuracion, el usuario la acepta, el flujo se "instala". Despues de instalado, Radar desaparece y Foco toma el control.

### Capa 2: Auxiliar (no interactuan directamente con el usuario, son invocados por otros frameworks)

- **Auditor**: analiza calidad de datos. Detecta gaps, errores, datos faltantes en la DB segun el contexto.
- **Mecanico**: cuando Auditor encuentra problemas, Mecanico razona sobre el gap y formula preguntas conversacionales naturales para que el usuario complete los datos faltantes. Nunca muestra jerga tecnica.

### Capa 3: Data (nunca interactua con el usuario)

- **Sabio**: extraccion y persistencia de datos. Otros frameworks le piden data y Sabio la busca/guarda en la DB.

### Infraestructura (invisible, servicio interno)

- **Hosting**: gestiona credenciales de hosting/cPanel. NO es un paso visible del flujo. Es invocado internamente para verificar si las credenciales que el usuario dio (a traves de Mecanico) funcionan.

---

## 4. Las dos vistas

### Vista 1: Nodos (operacion real — usuario final)

**Proposito**: operar el flujo real del negocio. Es lo que ve el usuario final (el cobrador, por ejemplo).

**Diseno visual**: un grafo/arbol con nodos circulares.

```
                    [ HUMANO ]                  ← Nivel 1: nodo central (el usuario)
                   /          \
              [FOCO]        [MENSAJERO]         ← Nivel 2: user-facing (izq a der, orden lineal)
             /   |   \       /    \
        [AUDITOR][MEC][SABIO][HOSTING]           ← Nivel 3: auxiliares e infraestructura
```

**Fase bootstrap**: al comenzar por primera vez, el usuario solo ve el nodo HUMANO y RADAR. Los demas no aparecen todavia. Radar le presenta la configuracion, el usuario acepta (se "instala" el flujo), y luego Radar desaparece y aparecen todos los demas nodos.

**Fase operativa** (despues del bootstrap):
- Los nodos user-facing aparecen de izquierda a derecha en orden: Foco, luego Mensajero.
- Los nodos auxiliares aparecen debajo de los nodos que los invocan.
- Cada nodo tiene su **PROPIO speech bubble permanente** (globo de texto). Cuando un framework habla, su respuesta aparece en su bubble al lado del nodo. Todos los bubbles ya completados quedan visibles para dar contexto.
- Cada respuesta incluye **3 botones de accion sugerida** (ver seccion 5).
- En la **esquina superior izquierda** hay un **thought bubble independiente** que muestra el ultimo "pensamiento" (razonamiento interno) del framework activo. Esto permite ver el thinking del LLM sin contaminar la respuesta al usuario.

**El usuario final NUNCA ve**: nombres tecnicos de artifacts, gaps, tablas de DB, IDs internos, nombres de capabilities. Solo ve preguntas naturales y respuestas.

### Vista 2: Chat vertical (prueba del Staff — modal de flujos)

**Proposito**: que el Staff de Remora pueda probar flujos antes de entregarlos a los clientes. Un boton "Probar", y listo.

**Diseno visual**: chat vertical (scroll hacia abajo). Cada mensaje del flujo aparece como un bloque.

**Lo que el Staff ve**:
- La misma experiencia que veria el usuario final (las respuestas de cada framework en lenguaje natural).
- PERO con un **dropdown colapsable** en cada respuesta que muestra informacion extra:
  - Que framework genero la respuesta
  - Que capability uso
  - Que artifacts consumio/produjo
  - Si hubo gaps, cuales y como se resolvieron
  - Duracion, exit code, errores
  - El "thinking" del framework

**Un solo boton "Probar"**. No "dry run", no "simulacion", no "revisar preparacion". Un boton. El Staff le da click y el flujo corre completo con datos reales (emails redirigidos a `TEST_EMAIL_RECIPIENT`). Las dimensiones se auto-ejecutan (ver seccion 6).

**IMPORTANTE**: la prueba y la operacion real ejecutan exactamente el mismo codigo (`flow_runner.go`). La diferencia es solo de presentacion en el frontend.

---

## 5. Acciones sugeridas (NO hardcodeadas)

Cada framework user-facing, cuando le toca hablar, debe generar su respuesta + 3 opciones de accion para el usuario. Estas acciones NO pueden estar hardcodeadas. Queremos comportamiento dinamico pero controlado.

### Diseno: `action_bounds` en el manifest

Cada framework declara en su `framework.manifest.json` un `action_bounds`: los TIPOS de acciones que puede generar. Esto define los limites (boundaries). Dentro de esos limites, el LLM genera las acciones concretas adaptadas al contexto del caso actual.

Ejemplo para Foco en un flujo de cobranza:
```json
{
  "action_bounds": [
    {
      "type": "proceed",
      "description": "Avanzar con la tarea propuesta",
      "examples": ["Enviar correo de cobranza", "Contactar al deudor"]
    },
    {
      "type": "postpone",
      "description": "Postergar la tarea para despues",
      "examples": ["Dejar para manana", "Aplazar una semana"]
    },
    {
      "type": "escalate",
      "description": "Escalar o cambiar de estrategia",
      "examples": ["Marcar como incobrable", "Escalar a gerencia"]
    }
  ]
}
```

Cuando Foco asigna una tarea (ej: "Cobrar a Thiel-Effertz, $5,000, 45 dias de mora"), el LLM genera 3 acciones concretas DENTRO de esos bounds:
1. "Enviar correo de cobranza a Thiel-Effertz" (proceed)
2. "Postergar — esperemos una semana mas" (postpone)
3. "Escalar a revision legal por antiguedad" (escalate)

### Como funciona el flujo despues de la seleccion

1. El usuario ve las 3 opciones como botones en el speech bubble del nodo.
2. Selecciona una.
3. La seleccion se convierte en un artifact (`action.selection.v1`) con el `type` del bound seleccionado.
4. Los frameworks siguientes leen ese artifact para saber que camino tomar.
5. El flow runner NO necesita saber que boton se apreto. Solo sabe que `action.selection.v1` esta disponible y continua al siguiente nodo, que lo lee y actua en consecuencia.

### Validacion

El flow runner valida que la accion generada por el LLM corresponda a uno de los `action_bounds` declarados en el manifest. Si el LLM genera algo fuera de los bounds, se rechaza y se usa el fallback (la primera opcion).

### Observabilidad

Cada accion generada es un artifact rastreable. El Staff ve en su dropdown: que bounds estaban declarados, que acciones genero el LLM, cual eligio el usuario, y por que el framework siguiente tomo el camino que tomo.

---

## 6. Dimensiones (auto-prueba del Staff)

Las dimensiones son las posibles respuestas del usuario ante las acciones sugeridas. En la prueba del Staff, el sistema auto-ejecuta TODAS las dimensiones de una vez para verificar que el flujo responde correctamente a cualquier eleccion.

### Como funciona

1. El Staff da click en "Probar".
2. El flujo corre hasta que Foco genera las 3 acciones sugeridas.
3. El sistema auto-ejecuta el flujo 3 veces en paralelo, una por cada accion sugerida (cada dimension).
4. Los resultados de las 3 dimensiones se muestran juntos para comparar.
5. Cada dimension muestra su timeline completa independiente.
6. El Staff puede ver de un vistazo: "si el usuario elige A, pasa esto; si elige B, pasa esto otro; si elige C, pasa esto".

### Criterio de exito

Cada dimension debe completar un ciclo o parar de forma controlada (ej: "postpone" no envia email pero marca la tarea). Si alguna dimension falla de forma inesperada (error, crash, artifact faltante), el flujo no esta listo para produccion.

### Implementacion existente

Ya existe logica de dimensiones en `flow_dimensions.go`. La funcion `runFlowActionBranches` corre hasta 3 branches en paralelo con `DryRun: true`. Reutilizar esta logica como base.

---

## 7. Flujo completo de ejemplo: cobranza Panalbit

Panalbit es un cliente de Remora. Es una empresa de cobranza. Sus usuarios (cobradores) usan Remora para gestionar la cobranza de empresas deudoras. El flujo automatiza el proceso de analisis y cobranza.

### Resultado esperado del flujo

Un correo de cobranza se envia a una empresa deudora. Eso es un ciclo completo.

### Fase bootstrap (primera vez)

1. **Radar** propone configuracion de analisis (ej: materialidad 40%, comportamiento historico 30%, riesgo legal 30%).
2. El usuario acepta.
3. El flujo se "instala". Radar desaparece. Se pasa a fase operativa.

### Fase operativa (cada ciclo)

1. **Foco** prioriza la cartera y asigna la tarea de mayor prioridad: "Tu proxima tarea: cobrar a Thiel-Effertz ($5,000, 45 dias de mora)". Presenta 3 acciones sugeridas.
2. El usuario elige "Enviar correo de cobranza".
3. **Mensajero** necesita un email de destino para enviar el correo.
4. Internamente: **Sabio** busca el email de Thiel-Effertz en la DB.
5. **Auditor** valida lo que Sabio encontro (o no encontro).
6. Si falta el email: **Mecanico** formula una pregunta natural: "Para enviar el cobro a Thiel-Effertz necesito su email. Cual es?"
7. El usuario responde con el email.
8. **Sabio** persiste el email en `contacts.db` para que no se vuelva a pedir.
9. **Auditor** re-valida: email existe, OK.
10. Ahora Mensajero necesita credenciales de correo para enviar.
11. **Auditor** detecta que no hay credenciales SMTP.
12. **Mecanico** pregunta: "Para poder enviar correos necesito acceso a tu email. Cual es tu proveedor de hosting?"
13. El usuario da las credenciales.
14. **Mecanico** necesita que **Hosting** verifique si las credenciales funcionan. (Mecanico no puede verificar esto solo — Hosting es el que sabe conectarse al servidor de correo.)
15. **Hosting** intenta conectar con las credenciales.
16. Si NO funcionan: **Mecanico** le dice al usuario que no funcionaron y le pide que las ingrese otra vez. Vuelve al paso 12.
17. Si funcionan: se guardan las credenciales. Continua.
18. **Mensajero** prepara el draft del correo, el usuario lo confirma, se envia.
19. **Ciclo completado**. Foco vuelve a asignar la siguiente tarea (paso 1).

### Preguntas ya respondidas por el usuario

- La prueba fallida se corrio desde el modal de Flujos, al hacer prueba de un flujo.
- En la operacion real, la vista debe ser de nodos/grafo, no chat vertical.
- En la prueba del Staff, la vista debe ser chat vertical, no nodos.
- En la vista de nodos, cada framework debe tener su propio speech bubble permanente. No basta con un bubble global que se mueve al nodo activo.
- Si las credenciales de hosting fallan, Mecanico debe pedirlas de nuevo conversacionalmente.
- El Staff quiere que las dimensiones se corran todas de una vez para comparar rapidamente.
- El nuevo trabajo debe considerar la vision completa. No limitarse a los prompts antiguos ni asumir que esos problemas siguen igual.

### Patron de datos (generico para cualquier flujo)

Cada vez que un framework de Capa 1 necesita datos:
```
Framework Capa 1 necesita artifact X
  -> Sabio busca X en la DB (Capa 3)
  -> Auditor valida el resultado (Capa 2)
  -> Si hay gap:
       -> Mecanico formula pregunta natural (Capa 2)
       -> Usuario responde
       -> Sabio persiste la respuesta en contacts.db
       -> Auditor re-valida
       -> Si pasa: continua
  -> Si no hay gap: continua
```

Para credenciales, el patron se extiende:
```
Mecanico obtiene credenciales del usuario
  -> Hosting verifica si funcionan (Infraestructura)
  -> Si no funcionan: Mecanico pide de nuevo (loop)
  -> Si funcionan: se guardan, continua
```

Este patron debe ser **generico**. Si manana se agrega un flujo de facturacion, el mismo pipeline Sabio -> Auditor -> Mecanico funciona sin cambios.

---

## 8. Estado actual del codigo

### Que existe y funciona (parcialmente)

1. **Flow runner** (`flow_runner.go`): motor de ejecucion que recorre nodos, maneja artifacts, emite steps via streaming. Funciona.
2. **Data pipeline** (`flow_data_pipeline.go`): `ensureDataPipeline`, `ensureContactDestinationPipeline`, `ensureSMTPCredentialsPipeline`. Implementado parcialmente.
3. **Gap resolution** (`flow_gap_resolution.go`): `resolveFlowGapsIteratively` + integracion con Mecanico. Existe.
4. **Dimensiones** (`flow_dimensions.go`): `runFlowActionBranches` corre branches en paralelo. Funciona.
5. **DAG topology** en frontend (`index.html` ~linea 4229): `computeFlowTopology`, `renderFlowTopology` con nodos por capa, lineas, labels. Existe.
6. **Thought bubbles** en frontend (`index.html` ~linea 6868): CSS con estilos por framework, posicionamiento junto al nodo activo, contenido markdown. Existe.
7. **Node view** (`openFlowInNodeView` ~linea 6755): crea nodos en canvas, hace streaming de la ejecucion, muestra thought bubbles. Existe pero es basica.
8. **Channel auto-start** (`flow_channel.go`): `ensureChannel` busca/compila el binario de Channel y lo arranca como subproceso en un puerto libre. Existe.

### Que esta roto

1. **Channel no se conecta**. El error actual al probar un flujo es:
```
Channel no esta disponible en http://127.0.0.1:59544. Normalmente se inicia automaticamente. Verifica que el base_dir del repo sea correcto.
```
El puerto 59544 (no es el default 8765) indica que `ensureChannel` SI asigno un puerto libre y SI intento arrancar el subproceso, pero algo fallo. Probablemente el binario de Channel no se encontro o no compilo. Investigar `findChannelBinary` y `buildChannelBinary` en `flow_channel.go`. Sin esto NADA funciona.

2. **La vista de nodos es basica**. `openFlowInNodeView` crea nodos y muestra UN thought bubble compartido que salta al nodo activo. Necesita:
   - Cada nodo con su PROPIO speech bubble permanente
   - Thought bubble separado en esquina superior izquierda para el thinking
   - 3 botones de accion sugerida por respuesta
   - Fase bootstrap vs operativa
   - Los auxiliares debajo de los que los invocan

3. **La vista de prueba del Staff** es confusa. Hay multiples action cards, funciones de dry run, simulacion, etc. Necesita simplificarse a un boton "Probar" con chat vertical y dropdowns de observabilidad.

4. **Las acciones sugeridas estan semi-hardcodeadas**. Foco tiene `buildActionOptionsFromStrategy` que genera opciones desde `strategy.recommendation.v1`, con fallback a un set fijo (`normalizeActionOptions`). Hay que evolucionar esto al sistema de `action_bounds`.

5. **La interaccion Mecanico-Hosting para verificar credenciales no existe**. No hay un mecanismo para que Mecanico delegue a Hosting la verificacion de credenciales y reciba el resultado.

### Archivos clave

```
remora-flujo/cmd/api_rest/
  flow_runner.go             -- Motor de ejecucion principal (runFlowManifest)
  flow_run_types.go          -- Tipos: flowRunRequest, flowRunResult, flowRunStep
  flow_execution.go          -- executeFlowNode: ejecuta un nodo via Channel
  flow_channel.go            -- ensureChannel: arranca Channel como subproceso
  flow_data_pipeline.go      -- Pipeline just-in-time: Sabio -> Auditor -> Mecanico
  flow_gap_resolution.go     -- Resolucion iterativa de gaps
  flow_dimensions.go         -- Branches/dimensiones en paralelo
  flow_preflight.go          -- Audit preflight checks
  flow_installation.go       -- Logica de bootstrap/instalacion
  flow_cycles.go             -- Logica de ciclos
  flow_backend.go            -- Tipos: flowManifest, flowNode, roles, capabilityRegistry
  flow_artifacts.go          -- Gestion de artifacts
  flow_store.go              -- Persistencia de flujos en SQLite
  drivers.go                 -- Discovery de frameworks via manifests
  main.go                    -- Servidor HTTP, endpoints, Channel init (~58K)
  static/index.html          -- Frontend completo (~8000 lineas)

framework-radar/             -- Bootstrap: configura analisis
framework-foco/              -- Entry: prioriza tareas, genera action_options
framework-sabio/             -- Capa 3: extraccion y persistencia de datos
framework-auditor/           -- Capa 2: validacion de datos
framework-mecanico/          -- Capa 2: resolucion conversacional de gaps
framework-hosting/           -- Infraestructura: credenciales hosting
framework-mensajero/         -- Capa 1: envio de correos

channel/                     -- Broker JSON-RPC entre orquestador y frameworks
profiles/                    -- Perfiles con seeds (contacts.seed.csv, tasks.seed.json)
ARCHITECTURE.md              -- Reglas duras del proyecto
HANDOFF_PROMPT.md            -- Contexto completo del sistema
```

---

## 9. Entregables

### 9.1 Resolver Channel (PRIMERO — sin esto nada funciona)

- Diagnosticar por que `ensureChannel` falla al arrancar el subproceso.
- Verificar que `findChannelBinary` encuentra el binario o que `buildChannelBinary` lo compila correctamente.
- Verificar que el `base_dir` que se le pasa al Channel es correcto (deberia ser la raiz del repo).
- Verificar con `go build` en `channel/cmd/channel/` que compila.
- Resultado esperado: al probar un flujo desde el modal, Channel arranca y los frameworks pueden comunicarse.

### 9.2 Flujo end-to-end funcional

- Verificar que el flujo de cobranza de Panalbit puede completar un ciclo completo en la prueba del modal.
- El ciclo: Radar configura -> usuario acepta -> Sabio exporta datos -> Foco asigna tarea -> gap de email -> Mecanico pregunta -> Sabio persiste -> Mensajero envia (redirigido a TEST_EMAIL_RECIPIENT) -> ciclo completo.
- Si alguno de estos pasos falla, investigar y resolver.

### 9.3 Vista de nodos (operacion real — usuario final)

Refactorizar `openFlowInNodeView` y funciones relacionadas para implementar:

- **Layout de grafo**: nodo central HUMANO, nivel 2 user-facing (Foco, Mensajero), nivel 3 auxiliares.
- **Fase bootstrap**: solo Radar visible. Al aceptar config, Radar desaparece y aparecen los demas.
- **Speech bubble PROPIO por nodo**: cada nodo tiene su propio globo de texto permanente. No un bubble compartido que salta. Cuando un framework responde, su bubble se llena. Los bubbles anteriores quedan visibles.
- **Thought bubble**: en la esquina superior izquierda, un bubble separado que muestra el ultimo pensamiento/razonamiento del framework activo.
- **3 botones de accion sugerida**: dentro del speech bubble del nodo activo, 3 botones con las acciones generadas por el LLM dentro de los `action_bounds`.
- **Flujo conversacional sin jerga**: el usuario final nunca ve nombres de frameworks, artifacts, gaps, ni errores tecnicos.

### 9.4 Vista de prueba del Staff (modal de flujos)

Refactorizar el modal de flujos para implementar:

- **Un solo boton "Probar"**. Eliminar "Revisar preparacion", "dry run", "simulacion" como opciones separadas.
- **Chat vertical**: cada paso del flujo aparece como un bloque de chat (scroll down). Formato limpio, legible.
- **Dropdown de observabilidad**: cada bloque tiene un dropdown colapsable con info extra (framework, capability, artifacts, gaps, duracion, thinking).
- **Dimensiones**: al terminar la corrida principal, las 3 dimensiones se auto-ejecutan en paralelo y sus resultados aparecen en paneles comparativos.

### 9.5 Sistema de action_bounds

- Agregar `action_bounds` al schema del manifest (`framework.manifest.json`).
- Modificar Foco (y cualquier framework user-facing) para que genere acciones concretas dentro de los bounds declarados.
- El flow runner valida que las acciones generadas correspondan a los bounds.
- Conectar con el sistema de dimensiones: cada accion generada = una dimension a probar.

### 9.6 Interaccion Mecanico-Hosting para credenciales

- Disenar e implementar el mecanismo para que Mecanico, al obtener credenciales del usuario, pueda delegarle a Hosting la verificacion.
- Hosting intenta conectar. Si falla, devuelve el error a Mecanico.
- Mecanico le dice al usuario que no funcionaron (en lenguaje natural) y pide que las ingrese otra vez.
- Loop hasta que funcionen o el usuario cancele.
- Cuando funcionan, se guardan las credenciales y el flujo continua.

---

## 10. Restricciones

- **Sin emojis** en codigo ni commits.
- **`panalbit.db` es READ-ONLY.** Nunca escribir en ella.
- **Modo dev**: `TEST_EMAIL_RECIPIENT=tom3bs@gmail.com` redirige emails. Badge naranja en frontend.
- **Codigo en ingles** salvo strings de UI en espanol.
- **Frameworks se comunican por JSON** (Channel). Nunca imports cruzados.
- **Routing capability-based**, nunca name-based. Nunca `if framework == "sabio"`.
- **Commits chicos y verificables.** Build local antes de deploy.
- **No usar LLM para clasificar intents en el router** (substring match basta). Pero SI usar LLM para: generar acciones sugeridas (dentro de bounds), formular preguntas de Mecanico, generar thinking de los frameworks.
- **El usuario habla espanol.** UI en espanol.

---

## 11. Orden sugerido de ejecucion

1. **Resolver Channel** — sin esto nada funciona. Diagnosticar y resolver el error de channel.
2. **Verificar flujo e2e** — correr el flujo de Panalbit en el modal y verificar que un ciclo se completa.
3. **Implementar action_bounds** — agregar al manifest, modificar Foco, validar en flow runner.
4. **Refactorizar vista de nodos** — layout de grafo, speech bubbles propios, thought bubble, botones de accion, fases bootstrap/operativa.
5. **Refactorizar vista de prueba** — un boton, chat vertical, dropdowns de observabilidad.
6. **Dimensiones auto-test** — conectar action_bounds con el sistema de branches para auto-probar las 3 acciones.
7. **Interaccion Mecanico-Hosting** — delegacion de verificacion de credenciales.
8. **Build + test manual e2e** — verificar todo junto.

---

## 12. Criterios de verificacion

El trabajo esta completo cuando:

1. Un flujo se puede probar desde el modal con un solo boton y completa un ciclo e2e.
2. Las 3 dimensiones se auto-ejecutan y sus resultados se pueden comparar.
3. La vista de nodos muestra el grafo correcto con speech bubbles propios por nodo, thought bubble, y botones de accion.
4. Las acciones sugeridas son generadas por LLM dentro de los bounds del manifest (no hardcodeadas).
5. Si falta un email, Mecanico pregunta conversacionalmente y Sabio persiste la respuesta.
6. Si faltan credenciales, Mecanico las pide, Hosting las verifica, y si fallan Mecanico pide de nuevo.
7. El usuario final nunca ve jerga tecnica.
8. El Staff ve la experiencia del usuario + observabilidad en dropdown.
9. `go build -buildvcs=false` compila sin errores.
10. Los datos se persisten: un email dado en ciclo 1 no se pide en ciclo 2.
