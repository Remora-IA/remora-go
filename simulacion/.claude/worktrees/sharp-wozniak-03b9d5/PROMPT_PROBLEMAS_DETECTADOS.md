# Prompt: Problemas detectados en Remora — auditoria de codigo vs vision

## 0. Contexto para la IA que recibe este prompt

Este documento es una **auditoria detallada** del estado actual del codigo de `remora-go` comparado contra la vision descrita en `PROMPT_VISION_COMPLETA.md`. No propone soluciones. Describe problemas con evidencia precisa (archivos, funciones, lineas) para que vos decidas como resolverlos.

Lee primero:
- `PROMPT_VISION_COMPLETA.md` (la vision completa del producto)
- `ARCHITECTURE.md`
- `HANDOFF_PROMPT.md`
- Este archivo

Cada problema tiene:
- **Vision**: que dice el documento de vision que deberia pasar
- **Realidad**: que hace el codigo actualmente
- **Evidencia**: archivos y lineas exactas donde esta el problema
- **Impacto**: que efecto tiene este gap para el usuario o el Staff

---

## 1. PROBLEMA CRITICO: Auditor escanea sin scope de entidad — Mecanico genera preguntas irrelevantes

### Vision

Seccion 7 de `PROMPT_VISION_COMPLETA.md` dice:

> Si falta el email: Mecanico formula una pregunta natural: "Para enviar el cobro a Thiel-Effertz necesito su email. Cual es?"

La expectativa es que las preguntas de Mecanico sean **relevantes a la entidad actual del flujo**. Si el flujo esta procesando a Thiel-Effertz, las preguntas deben ser sobre Thiel-Effertz y sus datos relacionados. No sobre tablas o registros que no tienen nada que ver.

Seccion 4 dice:

> El usuario final NUNCA ve: nombres tecnicos de artifacts, gaps, tablas de DB, IDs internos, nombres de capabilities. Solo ve preguntas naturales y respuestas.

### Realidad

Cuando se corre un flujo para el cliente "Thiel-Effertz", Mecanico genera preguntas como:
- "Cual es el nombre del septimo grupo de clientes?"
- "Cual es el nombre del segundo proveedor?"

Estas preguntas no tienen relacion con Thiel-Effertz ni con la tarea de cobranza en curso. Son ruido.

### Evidencia: la cadena completa del problema

El problema atraviesa 4 capas del pipeline. Aqui esta cada una:

#### Capa 1: Auditor escanea TODAS las tablas sin filtro

**Archivo**: `framework-auditor/checks/checks.go`

`RunAllWithSchema` (linea 313) ejecuta todos los checks sobre todo el dataset:

```go
func RunAllWithSchema(d *Dataset, tableColumns map[string][]string) []Finding {
    fkRels := InferFKRelations(d)
    reqStr := InferRequiredStringFields(d)      // <-- escanea TODAS las tablas
    reqNonNull := InferRequiredNonNullFields(d)  // <-- escanea TODAS las tablas
    dateFields := InferDateFields(d)

    var all []Finding
    all = append(all, CheckFKOrphans(d, fkRels)...)
    all = append(all, CheckEmptyRequired(d, reqStr)...)
    all = append(all, CheckNullRequired(d, reqNonNull)...)
    // ...
}
```

`InferRequiredStringFields` (linea 209) busca campos `name`, `code`, `title` en **toda tabla del dataset**:

```go
var commonRequiredFieldNames = map[string]bool{
    "name": true, "code": true, "title": true,
}

func InferRequiredStringFields(d *Dataset) []EndpointField {
    for endpoint, records := range d.Endpoints {  // <-- itera TODOS los endpoints
        for field := range fieldSet {
            if !commonRequiredFieldNames[field] {
                continue
            }
            out = append(out, EndpointField{Endpoint: endpoint, Field: field})
        }
    }
}
```

El dataset de panalbit tiene **34 tablas** (ver `framework-sabio/semantic/profile.json`). Si `suppliers[2].name` esta vacio, Auditor genera un finding, aunque el flujo sea sobre Thiel-Effertz y `suppliers` no tenga nada que ver.

No existe ningun parametro `--entity-scope`, `--entity-id`, ni `--tables-filter` en el comando `scan`. El manifest de Auditor confirma que `scan` solo acepta `--source` y `--json`:

```json
"scan": {
    "args": ["scan", "--source", "{params.source}", "--json"],
    "params": ["source"]
}
```

#### Capa 2: dataGapsFromFindings convierte TODOS los findings a gaps

**Archivo**: `framework-auditor/cmd/frameworkauditor/main.go`, linea 237

```go
func dataGapsFromFindings(findings []checks.Finding) []map[string]interface{} {
    for _, f := range findings {
        switch f.Rule {
        case checks.RuleEmptyRequired, checks.RuleNullRequired, checks.RuleMissingContact:
            // Agrupa por (rule, endpoint, field) pero NO filtra por entidad
            key := groupKey{rule: f.Rule, endpoint: f.Endpoint, field: f.Field}
            // ...
        }
    }
}
```

Si hay 130 findings repartidos en 34 tablas, se generan ~20 gaps. Ninguno se filtra por relevancia a la entidad actual.

#### Capa 3: resolveFlowGapsIteratively pasa TODOS los gaps a Mecanico

**Archivo**: `remora-flujo/cmd/api_rest/flow_gap_resolution.go`, linea 14

```go
func (s *server) resolveFlowGapsIteratively(...) {
    for pass := 0; pass < maxResolutionPasses; pass++ {
        gaps := parseDataGaps(result.Artifacts)  // <-- toma TODOS los gaps
        // ...
        // Los pasa a invokeMecanicoResolveGaps sin filtro
    }
}
```

`parseDataGaps` (en `flow_proposals.go`, linea 109) extrae todos los gaps del artifact `data.gaps.v1` sin ningun filtro de scope:

```go
func parseDataGaps(artifacts map[string]flowRunArtifact) []dataGap {
    // Extrae cada gap crudo, sin verificar si el endpoint/tabla
    // esta en scope de la entidad actual (entity.ref.v1)
    for _, g := range gapsRaw {
        gaps = append(gaps, dataGap{Kind: kind, Description: desc, ...})
    }
}
```

El artifact `entity.ref.v1` **ya existe** en este punto del pipeline (contiene `{type: "client", id: "184", name: "Thiel-Effertz"}`), pero nadie lo usa para filtrar gaps.

#### Capa 4: Mecanico genera una pregunta por gap, sin contexto de scope

**Archivo**: `framework-mecanico/cmd/frameworkmecanico/main.go`, linea 498

```go
func cmdResolveGaps(args []string) {
    for idx, gap := range gaps {
        q := questionForGapWithLLM(gap, entityRef, entityName, idx)  // <-- genera 1 pregunta por gap
        questions = append(questions, q)
    }
}
```

`questionForGapWithLLM` (linea 532) le pasa al LLM el gap crudo con un system prompt generico:

```go
func questionForGapWithLLM(gap map[string]interface{}, entityRef map[string]interface{}, entityName string, idx int) map[string]interface{} {
    system := `Eres un asistente operativo... Nunca mencionas terminos tecnicos...`
    payload := map[string]interface{}{
        "gap":         gap,           // <-- gap crudo (ej: {type: "empty_required", endpoint: "suppliers", field: "name"})
        "entity":      entityRef,     // <-- entityRef existe pero...
        "entity_name": entityName,    // <-- ...el LLM no sabe que "suppliers" no tiene relacion con entityName
        "flow_context": map[string]interface{}{
            "purpose": "continuar una tarea operativa del negocio",  // <-- contexto generico
        },
    }
}
```

El LLM recibe un gap sobre `suppliers.name` y el nombre "Thiel-Effertz" pero **no tiene informacion sobre que tablas estan en scope** del cliente. Entonces genera una pregunta que suena natural pero es irrelevante: "Cual es el nombre del segundo proveedor?"

### Informacion de scope que YA EXISTE pero no se usa

**Archivo**: `framework-sabio/businesses/panalbit/sabio.business.json`

```json
"primary_entities": {
    "portfolio_client": {
        "table": "clients",
        "scope_key": "id",
        "display_column": "name"
    },
    "project": {"table": "projects", "scope_column": "client_id"},
    "charge": {"table": "charges", "scope_column": "client_id"},
    "billing_document": {"table": "billing_documents", "scope_column": "client_id"},
    "payment": {"table": "payments", "scope_column": "client_id"},
    "expense": {"table": "expenses", "scope_column": "client_id"}
},
"scope_policies": {
    "scope_entity": "portfolio_client",
    "tables": {
        "clients": {"scope_column": "id"},
        "projects": {"scope_column": "client_id"},
        "agreements": {"scope_column": "client_id"},
        "charges": {"scope_column": "client_id"},
        "billing_documents": {"scope_column": "client_id"},
        "payments": {"scope_column": "client_id"},
        "expenses": {"scope_column": "client_id"},
        "milestones": {"join_to_scope": "milestones.charge_id = charges.id AND charges.client_id IN (:allowed_client_ids)"},
        "time_entries": {"join_to_scope": "time_entries.project_code = projects.code AND projects.client_id IN (:allowed_client_ids)"}
    }
}
```

Este JSON dice exactamente que tablas estan en scope de un `portfolio_client` y como se relacionan. Pero ni Auditor ni el flow pipeline ni Mecanico lo usan para filtrar.

Ademas, Auditor ya infiere relaciones FK dinamicamente (`InferFKRelations`, linea 153 de `checks.go`) — detecta que `projects.client_id -> clients`, `charges.client_id -> clients`, etc. Pero esta informacion solo se usa para `CheckFKOrphans`, no para scope filtering.

### Diagrama del flujo de datos actual (sin filtro)

```
Dataset (34 tablas, ~2000 registros)
        |
        v
Auditor.RunAllWithSchema()  ← escanea TODO
        |
        v
~130 findings (agreements.name vacio, suppliers.name vacio, client_groups.name vacio, etc.)
        |
        v
dataGapsFromFindings()  ← convierte TODO a gaps
        |
        v
~20 data.gaps.v1 entries
        |
        v
resolveFlowGapsIteratively()  ← pasa TODO a Mecanico
        |
        v
invokeMecanicoResolveGaps()
        |
        v
Mecanico.resolve-gaps  ← genera 1 pregunta por gap (sin saber cuales son relevantes)
        |
        v
6 preguntas al usuario  ← varias irrelevantes ("nombre del 7mo grupo de clientes")
```

### Impacto

- El usuario recibe preguntas que no puede (ni debe) responder.
- El Staff no puede distinguir si las preguntas son correctas o excesivas sin estudiar a fondo la estructura de datos del negocio — lo cual contradice la filosofia de que "el Staff no necesita pensar demasiado".
- El flujo se bloquea en `needs_input` esperando respuestas a preguntas sobre tablas que no afectan el ciclo actual.
- Al escalar a nuevos negocios (no solo panalbit), cada negocio con 30+ tablas generaria decenas de preguntas irrelevantes.

---

## 2. PROBLEMA: Vista de nodos — fases bootstrap/operativa no implementadas

### Vision

Seccion 4 de `PROMPT_VISION_COMPLETA.md`:

> Fase bootstrap: al comenzar por primera vez, el usuario solo ve el nodo HUMANO y RADAR. Los demas no aparecen todavia. Radar le presenta la configuracion, el usuario acepta, y luego Radar desaparece y aparecen todos los demas nodos.

### Realidad

La vista de nodos (`openFlowInNodeView` en `static/index.html`) renderiza todos los nodos del flujo de golpe usando `flowNodeCanvasPosition` que asigna posiciones segun roles predefinidos. No hay logica de "fase bootstrap" ni "fase operativa" — no hay nodos que aparezcan/desaparezcan segun el estado del flujo.

### Evidencia

**Archivo**: `remora-flujo/cmd/api_rest/static/index.html`

La funcion `flowNodeCanvasPosition` posiciona nodos por `role` (entry, bootstrap, resolution, action) pero no hay un mecanismo que oculte nodos segun la fase del flujo. Todos los nodos se renderizan desde el inicio.

La logica de instalacion existe en el backend (`flow_installation.go`) y usa `install_once` + `human_acceptance_before_continue`, pero el frontend no consume este estado para transicionar visualmente de bootstrap a operativa.

### Impacto

- El usuario ve todos los nodos desde el principio, incluyendo Radar (que deberia desaparecer despues del bootstrap).
- No hay una transicion visual que comunique "el flujo ya esta instalado, ahora entramos en operacion".
- El usuario ve nodos auxiliares (Auditor, Mecanico, Sabio) que segun la vision solo deberian aparecer cuando son invocados.

---

## 3. PROBLEMA: Vista de nodos — speech bubbles compartidos, no propios por nodo

### Vision

Seccion 4:

> Cada nodo tiene su PROPIO speech bubble permanente. Cuando un framework responde, su bubble se llena. Todos los bubbles ya completados quedan visibles para dar contexto.

Y:

> En la esquina superior izquierda hay un thought bubble independiente que muestra el ultimo pensamiento del framework activo.

### Realidad

El sistema actual tiene:
- `setNodeSpeech(nodeId, text)`: pone texto en una burbuja asociada a un nodo. Esto PARCIALMENTE implementa la vision.
- `showCanvasThought(text, framework)`: muestra un thought bubble global. Esto PARCIALMENTE implementa la vision.
- Pero los bubbles de nodos anteriores NO persisten visualmente cuando el flujo avanza a otro nodo. El contexto se pierde.

### Evidencia

**Archivo**: `remora-flujo/cmd/api_rest/static/index.html`

`setNodeSpeech` existe y renderiza markdown en una burbuja junto al nodo. Pero se llama en el handler de streaming — cuando llega un nuevo step, se actualiza la burbuja del nodo activo. No hay logica que preserve las burbujas de nodos anteriores.

### Impacto

- El usuario pierde contexto: no puede ver "que dijo Foco" mientras lee "lo que dice Mecanico".
- En un flujo con 5-6 steps, solo el ultimo speech es visible. Los anteriores desaparecen.

---

## 4. PROBLEMA: Vista de nodos — botones de accion sin onclick handlers

### Vision

Seccion 5:

> El usuario ve las 3 opciones como botones en el speech bubble del nodo. Selecciona una. La seleccion se convierte en un artifact (action.selection.v1).

### Realidad

Los action buttons se renderizan en el speech bubble pero no tienen onclick handlers que envien la seleccion al backend como `action.selection.v1`. Los botones son visuales pero no funcionales en la vista de nodos.

### Evidencia

**Archivo**: `remora-flujo/cmd/api_rest/static/index.html`

La funcion que renderiza action options en el canvas genera los botones HTML pero la interaccion usuario -> backend para convertir la seleccion en `action.selection.v1` y continuar el flujo no esta conectada.

### Impacto

- En la vista de nodos, el usuario ve botones de accion pero no puede usarlos.
- El flujo se queda detenido esperando una seleccion que no puede llegar desde la UI.

---

## 5. PROBLEMA: Vista de prueba Staff — falta dropdown de observabilidad por step

### Vision

Seccion 4:

> PERO con un dropdown colapsable en cada respuesta que muestra informacion extra: que framework genero la respuesta, que capability uso, que artifacts consumio/produjo, si hubo gaps, cuales y como se resolvieron, duracion, exit code, errores, el "thinking" del framework.

### Realidad

La vista de prueba (`dryRunCurrentFlow()` / `streamFlowRun()`) muestra los steps como bloques de chat via SSE streaming. Cada bloque tiene el `human_summary` del step. Pero no hay un dropdown colapsable con la info detallada de observabilidad.

### Evidencia

**Archivo**: `remora-flujo/cmd/api_rest/static/index.html`

El streaming handler procesa events `step_start`, `step_complete`, `needs_input`, `flow_complete`. Al recibir `step_complete`, renderiza el `human_summary` y opcionalmente el `error`. Pero no hay un `<details>` o accordion que muestre: framework, capability, artifacts consumidos/producidos, gaps, duracion, exit code, thinking.

Los datos EXISTEN en el JSON del step (`flowRunStep` tiene `Framework`, `Capability`, `ArtifactTypes`, `DurationMs`, `ExitCode`, `Error`, `Inputs`, `Outputs`, `Produces`, `Requires`, `Policies`). Solo falta renderizarlos.

### Impacto

- El Staff no puede inspeccionar que paso internamente en cada step.
- No puede diagnosticar problemas como "por que Mecanico pregunto algo irrelevante" sin ir al codigo.
- No puede verificar que artifacts se produjeron ni que gaps se resolvieron.

---

## 6. PROBLEMA: Vista de prueba Staff — falta vista comparativa de dimensiones

### Vision

Seccion 6:

> El sistema auto-ejecuta el flujo 3 veces en paralelo, una por cada accion sugerida (cada dimension). Los resultados de las 3 dimensiones se muestran juntos para comparar. El Staff puede ver de un vistazo: "si el usuario elige A, pasa esto; si elige B, pasa esto otro; si elige C, pasa esto".

### Realidad

El backend soporta dimensiones — `runFlowActionBranches` en `flow_dimensions.go` corre branches en paralelo y agrega los resultados al `flowRunResult.Branches`. Pero el frontend no tiene una vista comparativa que muestre los 3 branches lado a lado.

### Evidencia

**Archivo**: `remora-flujo/cmd/api_rest/flow_dimensions.go`

`runFlowActionBranches` existe y funciona: crea hasta 3 branches con `DryRun: true`, las ejecuta, y agrega los resultados a `result.Branches`. Los branches se emiten como steps en el streaming.

**Archivo**: `remora-flujo/cmd/api_rest/static/index.html`

No hay un componente de UI que renderice `result.Branches` como paneles comparativos. Los branches se muestran como steps adicionales en el chat lineal, mezclados con los steps del flujo principal.

### Impacto

- El Staff no puede comparar rapidamente "que pasa si el usuario elige A vs B vs C".
- Las dimensiones se ven como una secuencia confusa de steps en vez de 3 timelines paralelas.

---

## 7. PROBLEMA: Mecanico-Hosting — sin verificacion real de credenciales SMTP

### Vision

Seccion 7:

> Mecanico necesita que Hosting verifique si las credenciales funcionan. Hosting intenta conectar con las credenciales. Si NO funcionan: Mecanico le dice al usuario que no funcionaron y le pide que las ingrese otra vez. Loop hasta que funcionen.

### Realidad

La pipeline de credenciales SMTP (`ensureSMTPCredentialsPipeline` en `flow_data_pipeline.go`) verifica si las credenciales existen en el vault (`has-smtp`). Si existen, marca `credentials.smtp` como disponible. Si no existen, invoca Mecanico para pedir las credenciales al usuario.

Pero **no hay una verificacion real de conexion SMTP**. El check solo es "existen en el vault", no "funcionan". Si el usuario da credenciales invalidas, se guardan y el flujo continua hasta que Mensajero intenta enviar y falla.

### Evidencia

**Archivo**: `remora-flujo/cmd/api_rest/flow_gap_inputs.go`, linea 54-108

```go
// Fallback: consultar vault directamente via provider de credentials.smtp.check
if m, providerName, ok := s.findProviderForCapability("credentials.smtp.check"); ok {
    // ...
    if avail, _ := result["available"].(bool); avail {
        available[artifact] = true  // <-- solo verifica existencia, no validez
    }
}
```

**Archivo**: `framework-hosting/` — No existe un comando `verify-smtp` o `test-connection` en el manifest de Hosting. Solo `has-smtp` (check de existencia) y `save-smtp` (guardar).

### Impacto

- Credenciales invalidas se guardan sin verificar.
- El error aparece recien cuando Mensajero intenta enviar el correo, en vez de inmediatamente despues de que el usuario las ingresa.
- No existe el loop Mecanico -> Hosting -> verificar -> re-preguntar descrito en la vision.

---

## 8. PROBLEMA: action_bounds — acciones semi-hardcodeadas vs dinamicas controladas

### Vision

Seccion 5:

> Cada framework user-facing declara en su manifest un `action_bounds`: los TIPOS de acciones que puede generar. Dentro de esos limites, el LLM genera las acciones concretas adaptadas al contexto.

### Realidad

El sistema de `action_bounds` **esta implementado a nivel de manifest y validacion**. `ActionBoundSpec` existe en `channel/manifest/manifest.go`. Foco tiene bounds declarados (`proceed`, `escalate`, `postpone`). `validateActionOptionsForNode` en `flow_artifacts.go` valida que las opciones generadas por el LLM correspondan a bounds declarados. `flow_dimensions.go` usa las opciones para crear branches.

Sin embargo, Foco todavia tiene logica de fallback que genera acciones con un set fijo via `normalizeActionOptions` (mencionado en `PROMPT_VISION_COMPLETA.md` seccion 8: "Foco tiene buildActionOptionsFromStrategy que genera opciones desde strategy.recommendation.v1, con fallback a un set fijo"). Esto significa que si el LLM no genera opciones (o genera opciones fuera de bounds), el sistema cae a opciones hardcodeadas en vez de fallar de forma visible.

### Evidencia

**Archivo**: `channel/manifest/manifest.go` — `ActionBoundSpec` esta definido.

**Archivo**: `framework-foco/framework.manifest.json` — `action_bounds` con `proceed`, `escalate`, `postpone` esta declarado.

**Archivo**: `remora-flujo/cmd/api_rest/flow_artifacts.go` — `validateActionOptionsForNode` valida opciones contra bounds.

La parte que falta verificar: la generacion de acciones por parte del LLM dentro de Foco (`framework-foco/cmd/frameworkfoco/main.go`) — si efectivamente genera opciones dentro de bounds o si sigue usando el fallback hardcodeado como caso principal.

### Impacto

- Si el fallback se activa frecuentemente, las acciones no son "dinamicas adaptadas al contexto" sino un set generico fijo.
- El Staff no puede distinguir si las acciones fueron generadas por el LLM (contextualizadas) o son fallback (genericas).

---

## 9. PROBLEMA: Staff necesita entender la estructura de datos para validar preguntas

### Vision

La vision implica que el Staff construye flujos para muchos negocios sin necesidad de estudiar a fondo cada base de datos. El sistema deberia ser lo suficientemente inteligente para hacer las preguntas correctas automaticamente.

### Realidad

Cuando el Staff prueba un flujo y ve que Mecanico pregunta "Cual es el nombre del septimo grupo de clientes?", no puede saber si esa pregunta es correcta o no sin:

1. Conocer la estructura de la base de datos del negocio (34 tablas en panalbit).
2. Entender que tablas estan en scope de la entidad actual.
3. Saber que `client_groups` no tiene relacion directa con el ciclo de cobranza de un cliente especifico.

Esto contradice la premisa de que el Staff "no necesita pensar demasiado".

### Evidencia

El sistema ya tiene toda la informacion necesaria para resolver esto automaticamente:

1. **`sabio.business.json`** tiene `primary_entities` y `scope_policies` que definen exactamente que tablas estan en scope de un `portfolio_client`.
2. **Auditor** ya infiere relaciones FK dinamicamente via `InferFKRelations`.
3. **`entity.ref.v1`** ya existe en el pipeline con el ID y nombre del cliente.

Pero ninguno de estos datos se usa para filtrar gaps ni para limitar las preguntas de Mecanico.

### Impacto

- El Staff tiene que estudiar el MER/modelo logico de cada negocio para validar si las preguntas son correctas.
- Escalar a 10, 50, 100 negocios con estructuras distintas seria inviable con este nivel de intervencion manual.
- La experiencia del Staff contradice la filosofia de "el sistema es inteligente, el Staff solo configura y prueba".

---

## 10. PROBLEMA MENOR: .env tiene lineas que generan error al parsear

### Evidencia

Al ejecutar `make dev`, el shell reporta:

```
.env: line 54: Juridico: command not found
.env: line 55: de: command not found
```

Esto sugiere que las lineas 54-55 del `.env` tienen texto libre (probablemente un comentario sin `#`) que el shell interpreta como comando. No bloquea el startup pero ensucia la salida.

### Impacto

- Ruido en logs de startup.
- Potencial confusion para quien corre `make dev` por primera vez.

---

## 11. Resumen de archivos clave por problema

| Problema | Archivos principales |
|----------|---------------------|
| 1. Scope de gaps | `framework-auditor/checks/checks.go` (InferRequiredStringFields, RunAllWithSchema), `framework-auditor/cmd/frameworkauditor/main.go` (dataGapsFromFindings, cmdScan), `remora-flujo/cmd/api_rest/flow_gap_resolution.go` (resolveFlowGapsIteratively, invokeMecanicoResolveGaps), `remora-flujo/cmd/api_rest/flow_proposals.go` (parseDataGaps), `framework-mecanico/cmd/frameworkmecanico/main.go` (cmdResolveGaps, questionForGapWithLLM), `framework-sabio/businesses/panalbit/sabio.business.json` (primary_entities, scope_policies) |
| 2. Fases bootstrap/operativa | `remora-flujo/cmd/api_rest/static/index.html` (openFlowInNodeView, flowNodeCanvasPosition), `remora-flujo/cmd/api_rest/flow_installation.go` |
| 3. Speech bubbles propios | `remora-flujo/cmd/api_rest/static/index.html` (setNodeSpeech, showCanvasThought) |
| 4. Botones de accion | `remora-flujo/cmd/api_rest/static/index.html` (render de action options en canvas) |
| 5. Dropdown observabilidad Staff | `remora-flujo/cmd/api_rest/static/index.html` (streaming handler, step rendering) |
| 6. Vista comparativa dimensiones | `remora-flujo/cmd/api_rest/flow_dimensions.go` (runFlowActionBranches), `remora-flujo/cmd/api_rest/static/index.html` |
| 7. Verificacion SMTP real | `remora-flujo/cmd/api_rest/flow_gap_inputs.go` (resolveMissingFlowArtifacts), `framework-hosting/` |
| 8. action_bounds fallback | `framework-foco/cmd/frameworkfoco/main.go`, `remora-flujo/cmd/api_rest/flow_artifacts.go` (validateActionOptionsForNode) |
| 9. Staff y entendimiento de datos | Problema derivado del problema 1 |
| 10. .env parsing | `.env` lineas 54-55 |

---

## 12. Lo que SI funciona bien

Para dar contexto completo, esto es lo que ya esta implementado correctamente:

1. **Channel auto-start** (`flow_channel.go`): `ensureChannel` encuentra/compila el binario, asigna puerto libre, arranca subproceso, hace health check. Funciona despues de compilar.
2. **Flow runner e2e** (`flow_runner.go`): pipeline completo con multi-ciclo, capability-based routing, artifact system. 454 lineas, robusto.
3. **Data pipeline JIT** (`flow_data_pipeline.go`): resolucion just-in-time de contacts, SMTP credentials, dataset mediation. 301 lineas.
4. **action_bounds en manifests** (`channel/manifest/manifest.go`): `ActionBoundSpec` definido, Foco tiene bounds declarados, `validateActionOptionsForNode` valida.
5. **Dimensiones en backend** (`flow_dimensions.go`): `runFlowActionBranches` corre branches en paralelo. 208 lineas, funcional.
6. **Instalacion de flujos** (`flow_installation.go`): Radar analysis install con `install_once` + `human_acceptance_before_continue`.
7. **Ciclos** (`flow_cycles.go`): deteccion de ciclo completado via `message.sent.v1`.
8. **Thought bubble** (`showCanvasThought`): bubble global de thinking implementado.
9. **Scrub tecnico en Mecanico** (`scrubTechnicalQuestion`): reemplaza "gap" por "dato faltante", "tabla" por "registro", etc.
10. **Auditor business-agnostic** (`checks.go`): las reglas son deterministas y no dependen de tablas hardcodeadas — infieren relaciones FK, campos requeridos y fechas dinamicamente.

---

## 13. Prioridad sugerida (informativo, no prescriptivo)

Los problemas estan numerados por criticidad, no por facilidad de resolucion:

1. **Problema 1** (scope de gaps) es el mas critico porque afecta la experiencia del usuario final Y del Staff. Mientras no se resuelva, las pruebas de flujo generan ruido que confunde.
2. **Problemas 2-4** (vista de nodos) son criticos para la operacion real pero no bloquean la prueba del Staff.
3. **Problemas 5-6** (vista Staff) son importantes para la productividad del Staff pero el flujo puede probarse sin ellos.
4. **Problema 7** (verificacion SMTP) afecta la robustez del ciclo completo pero tiene workaround (el error aparece al enviar).
5. **Problema 8** (action_bounds fallback) afecta la calidad de las acciones sugeridas pero no bloquea el flujo.
6. **Problema 9** es consecuencia directa del problema 1.
7. **Problema 10** es cosmetico.
