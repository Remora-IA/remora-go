# Prompt: Problemas detectados en Remora — Ronda 2

## 0. Contexto para la IA que recibe este prompt

Este documento es la **segunda ronda de auditoria** del codigo de `remora-go`. Los 10 problemas de `PROMPT_PROBLEMAS_DETECTADOS.md` ya fueron resueltos. Ahora hay problemas nuevos descubiertos al probar el flujo real de cobranzas con la data de panalbit.

Lee primero:
- `PROMPT_VISION_COMPLETA.md`
- `PROMPT_PROBLEMAS_DETECTADOS.md` (los 10 ya resueltos)
- `ARCHITECTURE.md`
- `HANDOFF_PROMPT.md`
- Este archivo

Cada problema tiene:
- **Que pasa**: descripcion del comportamiento actual
- **Que deberia pasar**: la expectativa correcta
- **Evidencia**: archivos, funciones y lineas exactas
- **Impacto**: efecto real sobre el usuario o el staff

---

## 1. PROBLEMA: Vista de nodos (canvas) no renderiza UI de inputs cuando el flujo retorna `needs_input`

### Que pasa

Al abrir un flujo en la vista de nodos y clickear "Iniciar Configuracion" de Radar, el flujo termina con status `needs_input` pero el usuario ve un mensaje generico:

> "Operacion termino con estado: needs_input. Revisa el ultimo framework activo para el detalle."

No aparece ningun boton, formulario, ni campo de texto para dar el input que el flujo necesita. El usuario queda atrapado sin poder continuar.

### Que deberia pasar

La vista de nodos deberia renderizar los mismos controles que la vista de "probar flujo" (chat simulacion) ya implementa:
- Para `analysis_acceptance`: un boton "Aceptar configuracion" con la propuesta de Radar
- Para `conversational_question`: un campo de texto con la pregunta de Mecanico y boton "Responder"
- Para `hosting_connect`: un boton "Conectar hosting"

### Evidencia

#### El handler `flow_complete` en `runCanvasFlowReal()` tiene un bug logico

**Archivo**: `remora-flujo/cmd/api_rest/static/index.html`, lineas 7107-7119

```javascript
let thoughtContent = '';
if (status === 'needs_input' && (data.needs_input || []).length) {
    const prereqs = data.prerequisites || (data.artifacts && data.artifacts['flow.prerequisites.v1']);
    if (prereqs) {
        thoughtContent += renderCanvasPrerequisites(prereqs);
    }
    thoughtContent += renderCanvasNeedsInput(data.needs_input);
} else if (status === 'completed') {
    // ...
} else {
    // AQUI CAE: mensaje generico inutil
    const msg = `Operacion termino con estado: ${status}. Revisa el ultimo framework activo para el detalle.`;
    thoughtContent = `${semantic}${semantic ? '<div style="height:10px;"></div>' : ''}${escapeHtml(msg)}`;
}
```

Hay dos sub-problemas:

**Sub-problema A: `renderCanvasNeedsInput` y `renderCanvasPrerequisites` no existen.**

Estas funciones se llaman en lineas 7112 y 7114 pero nunca se definen en el archivo. Si el codigo llega ahi, da `ReferenceError`. La vista de chat simulacion tiene `renderFlowNeedsInput` (linea 5224) y `renderConversationalFlowQuestion` que SI funcionan, pero la vista de nodos no las reutiliza ni tiene equivalentes propios.

**Sub-problema B: `data.needs_input` puede llegar vacio.**

Si `(data.needs_input || []).length` es 0 (porque el array no viene o viene vacio en el SSE), el codigo cae al `else` generico. El SSE envia el resultado completo en `flow_complete` (archivo `flow_run.go` linea 70: `sendSSE("flow_complete", result)`), y `result.NeedsInput` deberia tener items. Pero si hay algun caso donde `NeedsInput` no se populó correctamente, el usuario queda atrapado.

#### El handler `flow_complete` en `runCanvasFlowWithSelection()` es peor

**Archivo**: `remora-flujo/cmd/api_rest/static/index.html`, lineas 6936-6941

```javascript
if (event === 'flow_complete') {
    deactivateAllNodes();
    nodeUser.classList.add('active');
    const status = data.status || 'completed';
    showCanvasThought('Remora', status === 'completed' ? 'Operacion completada.' : `Estado: ${status}`);
}
```

Este handler NO maneja `needs_input` en absoluto. No renderiza inputs, no guarda `canvasFlowState.needsInput`, nada. Si un flujo re-ejecutado con seleccion termina en `needs_input`, el usuario ve "Estado: needs_input" y punto.

### Impacto

El usuario no puede completar la fase bootstrap del flujo desde la vista de nodos. El flujo de Radar necesita aceptacion humana (`analysis_acceptance`) para instalar la configuracion de analisis, y esa UI no existe en canvas. Esto bloquea completamente el uso de la vista de nodos para flujos reales.

### Archivos involucrados

| Archivo | Que tiene | Que falta |
|---------|-----------|-----------|
| `static/index.html` lineas 7107-7119 | Handler flow_complete en runCanvasFlowReal | Definir renderCanvasNeedsInput y renderCanvasPrerequisites |
| `static/index.html` lineas 6936-6941 | Handler flow_complete en runCanvasFlowWithSelection | Manejar needs_input igual que runCanvasFlowReal |
| `static/index.html` lineas 5224-5290 | renderFlowNeedsInput (funciona en chat simulacion) | Reutilizarla o crear version canvas equivalente |

---

## 2. PROBLEMA: Mecanico genera preguntas innecesarias porque no verifica datos existentes de la entidad actual

### Que pasa

Cuando el flujo procesa a Thiel-Effertz (cliente priorizado con score 100, saldo 7500), Mecanico genera 3 preguntas:

1. "Necesito el nombre de contacto de Thiel-Effertz para poder actualizar sus datos"
2. "Necesito el codigo de cliente de Thiel-Effertz para poder actualizar los acuerdos pendientes"
3. "Necesito el correo electronico de contacto de Thiel-Effertz para poder enviar un correo de cobranza"

**De esas 3, solo la pregunta 3 (email) es necesaria.** Las otras dos son ruido:

- **Pregunta 1 (nombre)**: Thiel-Effertz YA tiene nombre en la base (`clients.name = 'Thiel-Effertz'`). El gap viene de que 599 de 614 registros en `agreements` tienen `name` vacio — pero eso es un problema masivo de agreements, no de este cliente.
- **Pregunta 2 (codigo)**: Thiel-Effertz YA tiene codigo en la base (todos los clientes tienen `code` tipo `000XXX`). El gap viene de que 614 registros en `agreements` tienen `code` nulo — mismo problema masivo irrelevante.
- **Pregunta 3 (email)**: CORRECTA. La tabla `clients` no tiene columna `email` en su schema. Es un gap estructural real (`schema_contact_gap`). Sin email no se puede enviar correo de cobranza.

### Que deberia pasar

Mecanico solo deberia preguntar por datos que:
1. Son necesarios para la accion terminal del flujo (enviar correo de cobranza)
2. NO existen en la base de datos para la entidad actual
3. No pueden resolverse automaticamente desde datos existentes

Para enviar un correo de cobranza a Thiel-Effertz, los datos minimos son:
- Email destino → NO existe en la base (pregunta valida)
- Nombre del deudor → YA existe: "Thiel-Effertz" (no preguntar)
- Monto adeudado → YA calculado: 7500 (no preguntar)
- Facturas/cargos → YA en la base (no preguntar)

### Evidencia: la cadena completa del problema

El problema atraviesa 4 puntos del pipeline:

#### Punto 1: Auditor reporta gaps masivos que no son relevantes al caso actual

**Archivo**: `framework-auditor/checks/checks.go`, lineas 203-205

```go
var commonRequiredFieldNames = map[string]bool{
    "name": true, "code": true, "title": true,
}
```

Auditor marca `name` y `code` como "requeridos universales" en TODAS las tablas. Esto genera cientos de findings para `agreements.name` vacio (599 registros) y `agreements.code` nulo (614 registros). Estos son problemas reales de calidad de datos, pero no son relevantes para enviar un correo de cobranza.

#### Punto 2: filterGapsByFlowPurpose filtra por tabla+campo pero no es suficiente

**Archivo**: `remora-flujo/cmd/api_rest/flow_prerequisites.go`, lineas 134-165

```go
func filterGapsByFlowPurpose(gaps []dataGap, requiredFields []prerequisiteDataField) []dataGap {
    needed := make(map[string]bool, len(requiredFields))
    neededTables := make(map[string]bool, len(requiredFields))
    for _, f := range requiredFields {
        needed[f.Table+"."+f.Field] = true
        neededTables[f.Table] = true
    }
    var filtered []dataGap
    for _, g := range gaps {
        if g.Endpoint == "" {
            filtered = append(filtered, g)
            continue
        }
        if needed[g.Endpoint+"."+g.Field] {
            filtered = append(filtered, g)
            continue
        }
        if neededTables[g.Endpoint] && isContactRelatedGap(g) {
            filtered = append(filtered, g)
            continue
        }
    }
    return filtered
}
```

`requiredFields` se genera en `flowRequiredDataFields` (linea 63) y solo mapea:
- `contact.destination.v1` → `clients.email`
- `message.draft.v1` → `clients.name`

Los gaps de `agreements.name` y `agreements.code` deberian ser filtrados aqui porque `agreements` no esta en `neededTables` (solo `clients` lo esta). **Si estan llegando a Mecanico, algo mas los esta pasando** — posiblemente vienen por otro path (ej: `resolveMissingFlowArtifacts` en `flow_gap_inputs.go` que invoca `invokeMecanicoResolveGaps` con los gaps sin filtrar por purpose).

#### Punto 3: Mecanico recibe gaps sin contexto de datos existentes

**Archivo**: `framework-mecanico/cmd/frameworkmecanico/main.go`, lineas 552-589

```go
func questionForGapWithLLM(gap map[string]interface{}, entityRef map[string]interface{}, entityName string, idx int) map[string]interface{} {
    system := `Eres un asistente operativo que ayuda a completar tareas de negocio...`
    payload := map[string]interface{}{
        "gap":         gap,
        "entity":      entityRef,
        "entity_name": entityName,
    }
    // LLM genera pregunta...
}
```

`questionForGapWithLLM` recibe:
- El gap (ej: `{type: "empty_required", endpoint: "agreements", field: "name"}`)
- La entity ref (ej: `{name: "Thiel-Effertz", id: "..."}`)
- El nombre de la entidad

**NO recibe**:
- Los datos que YA existen en la base para esta entidad (nombre, codigo, etc.)
- La lista de campos minimos necesarios para la accion del flujo
- Indicacion de si este gap es realmente relevante para la tarea en curso

El LLM entonces genera preguntas para TODOS los gaps que le llegan, sin poder discriminar.

#### Punto 4: No hay verificacion pre-pregunta contra datos existentes

No existe una capa que, antes de generar la pregunta, haga:
```
¿clients.name para Thiel-Effertz ya tiene valor? → SI → no preguntar
¿clients.code para Thiel-Effertz ya tiene valor? → SI → no preguntar
¿clients.email para Thiel-Effertz tiene valor? → NO EXISTE COLUMNA → preguntar
```

### Impacto

- El usuario final (cliente de cliente de Remora) recibe preguntas innecesarias que no sabe como responder
- Un cobrador no sabe que es un "codigo de cliente" ni por que se lo piden
- Se rompe la confianza en el sistema: si pide datos que ya tiene, parece tonto
- El staff de Remora no tiene forma de saber si las preguntas son correctas sin mirar la base de datos manualmente

### Archivos involucrados

| Archivo | Funcion | Problema |
|---------|---------|----------|
| `framework-auditor/checks/checks.go` | `commonRequiredFieldNames` | Marca name/code como requeridos universales sin contexto de flujo |
| `remora-flujo/cmd/api_rest/flow_prerequisites.go` | `filterGapsByFlowPurpose` | Solo filtra por tabla+campo, no verifica datos existentes |
| `remora-flujo/cmd/api_rest/flow_gap_inputs.go` | `resolveMissingFlowArtifacts` | Invoca Mecanico con gaps que quiza no pasaron por filterGapsByFlowPurpose |
| `remora-flujo/cmd/api_rest/flow_gap_resolution.go` | `invokeMecanicoResolveGaps` | Pasa gaps al LLM sin verificar datos existentes de la entidad |
| `framework-mecanico/cmd/frameworkmecanico/main.go` | `questionForGapWithLLM` | No recibe ni verifica datos existentes de la entidad |

---

## 3. PROBLEMA ESTRATEGICO: El staff de Remora no puede validar preguntas sin entender la data

### Que pasa

El staff de Remora (incluido el fundador) no entiende la estructura de datos de los clientes (panalbit, y futuros negocios). No sabe:
- Que tablas existen
- Como se relacionan
- Que campos estan llenos y cuales vacios
- Si una pregunta de Mecanico es valida o es basura

Cuando ven que el sistema hace 3 preguntas al usuario final, no tienen forma rapida de saber si esas preguntas son las correctas. Tendrian que abrir la base SQLite, entender el MER, correr queries, y comparar manualmente. Eso no escala: van a tener muchos negocios con bases de datos distintas.

### Que deberia pasar

El staff necesita una forma de validar las preguntas SIN tener que pensar ni entender la data. Deberia ser **evidente** — algo visual y automatico que diga:

> "Para enviar correo de cobranza a Thiel-Effertz necesitamos 4 datos. 3 ya estan en la base y 1 falta. Solo vamos a preguntar por el que falta."

### Lo que ya existe (pero incompleto)

#### flow.prerequisites.v1: un semaforo de prerequisitos

**Archivo**: `remora-flujo/cmd/api_rest/flow_prerequisites.go`, lineas 180-293

Ya existe un artifact `flow.prerequisites.v1` que genera una lista de prerequisitos con status `available/missing/not_needed`. Genera un `human_summary` tipo:

> "Para Thiel-Effertz: 1/2 requisitos listos. Falta: Correo electronico del Cliente/deudor. Se omitieron 5 brechas de datos que no afectan esta operacion."

**Problemas del semaforo actual:**

1. **Solo mapea 2 campos**: `contact.destination.v1 → clients.email` y `message.draft.v1 → clients.name` (lineas 88-116). No mapea todos los campos que el correo de cobranza necesita (monto, facturas, dias mora, etc.)

2. **No verifica valores reales**: Dice "Nombre del Cliente/deudor: available" si el artifact existe, pero no verifica que `clients.name` tenga valor real en la base para esta entidad especifica.

3. **No se muestra al staff**: El artifact se genera y se persiste en disco, pero no hay un panel en la UI donde el staff pueda ver "estos son los prerequisitos, estos estan verdes, estos rojos". Solo aparece en el JSON de observabilidad de cada step.

4. **Los gaps que NO son necesarios se cuentan pero no se explican**: Dice "Se omitieron 5 brechas" pero no dice cuales son ni por que se omitieron.

### Lo que falta para que el staff no tenga que pensar

#### A. Un "mapa de datos requeridos" por tipo de flujo

Para cada tipo de flujo (cobranza, notificacion, etc.), deberia existir un manifiesto declarativo que diga:

```
Para un correo de cobranza necesito:
- email del deudor (tabla: clients, campo: email) → CRITICO, sin esto no puedo enviar
- nombre del deudor (tabla: clients, campo: name) → NECESARIO, para personalizar
- monto adeudado (derivado: SUM milestones.amount de charges impagos) → NECESARIO
- numero de factura (tabla: billing_documents, campo: number) → OPCIONAL
- dias de mora (derivado: julianday(now) - julianday(milestones.date)) → NECESARIO
```

Hoy esto no existe como un manifiesto explícito. `flowRequiredDataFields` hace un mapping minimo hardcodeado, no un mapa completo.

#### B. Verificacion de datos existentes contra la entidad actual

Antes de generar preguntas, el sistema deberia consultar la base:

```sql
SELECT name, code FROM clients WHERE id = <entity_id>
-- Resultado: name='Thiel-Effertz', code='000XXX'
-- Conclusion: name y code YA existen, no preguntar
```

Hoy `questionForGapWithLLM` no hace esta verificacion.

#### C. Panel de observabilidad para el staff

Un panel que muestre para cada ejecucion del flujo:
- Entidad actual: Thiel-Effertz
- Datos requeridos: [tabla de semaforo verde/rojo]
- Preguntas que se van a hacer: [lista con razon de cada una]
- Gaps omitidos: [lista con razon de omision]
- Datos ya disponibles: [nombre, codigo, monto, etc. con valor real]

Esto permitiria al staff ver de un vistazo si el sistema esta haciendo las preguntas correctas, sin necesitar entender la base de datos.

### Evidencia de la data real

Para dar contexto de por que esto importa, aqui esta la estructura real de panalbit:

| Tabla | Filas | Observacion |
|-------|-------|-------------|
| clients | 269 | **No tiene columna email.** Tiene: id, name, code, active, group_id, agreement_start_date |
| client_groups | 7 | Tiene: id, name, client_id, client_code, country_id. **Tampoco tiene email.** |
| agreements | 614 | **599 tienen name vacio, 614 tienen code nulo.** Es data sucia masiva pero irrelevante para cobranza |
| charges | 1518 | Cargos con state (PAGADO, FACTURADO, etc.) |
| milestones | 308 | Hitos con amount y date — fuente para saldo pendiente |
| billing_documents | 1579 | Facturas con number y date |
| payments | 1451 | Pagos con amount |
| projects | 521 | Proyectos/casos |
| time_entries | 30050 | Horas trabajadas |
| providers | 1 | **name vacio** — gap detectado pero irrelevante para cobranza |

La relacion clave es: `clients → charges (client_id) → milestones (charge_id)` para calcular saldo pendiente. Y `clients → billing_documents (client_id)` para facturas.

**Lo critico**: la tabla `clients` no tiene columna `email`. Es un gap estructural de la base de datos fuente (panalbit/TimeBilling). El email habria que obtenerlo de otra fuente o pedirlo al usuario. Esta es la UNICA pregunta que Mecanico deberia hacer.

### Impacto

- El staff no puede validar el comportamiento del sistema sin ser experto en datos
- No escala: con cada nuevo negocio, el staff tendria que estudiar una nueva base de datos
- El fundador no entiende la data de su propio primer cliente — imagina un operador nuevo
- Sin observabilidad, el sistema es una caja negra que hace preguntas y nadie sabe si son correctas

### Archivos involucrados

| Archivo | Que tiene | Que falta |
|---------|-----------|-----------|
| `flow_prerequisites.go` | Semaforo basico de prerequisitos | Mapeo completo de campos requeridos por tipo de flujo |
| `flow_prerequisites.go` | `flowRequiredDataFields` | Verificacion de valores reales en la base por entidad |
| `sabio.business.json` | `primary_entities`, `scope_policies` | Mapa declarativo de "campos minimos por tipo de accion" |
| `static/index.html` | Observabilidad por step en dropdown | Panel de prerequisitos visible al staff |
| `framework-mecanico/main.go` | `questionForGapWithLLM` | Recibir datos existentes para no preguntar lo que ya tiene |

---

## Resumen para la IA que va a implementar

| # | Problema | Tipo | Prioridad |
|---|----------|------|-----------|
| 1 | Vista de nodos no renderiza UI de inputs en needs_input | Bug de frontend | Alta — bloquea uso de vista de nodos |
| 2 | Mecanico pregunta por datos que ya existen en la base | Bug de pipeline | Alta — genera preguntas innecesarias al usuario final |
| 3 | Staff no puede validar preguntas sin entender la data | Falta de observabilidad | Alta — bloquea operacion a escala |

Los tres problemas estan conectados: el Problema 2 genera preguntas malas, el Problema 3 hace que nadie pueda detectar que son malas, y el Problema 1 hace que ni siquiera se puedan responder desde la vista de nodos.
