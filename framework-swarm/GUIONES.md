# Guiones de Comportamiento — framework-swarm

> Cada guión describe un cambio en el comportamiento de la librería:
> cómo se comportaba antes, cómo se comporta después.
> No habla del usuario. Habla de lo que hacen los componentes.

---

## GUIÓN 1: El enjambre que no sabía coordinarse

**Cambio:** `StigmaStore.Claim()` — navegación concurrente sin colisiones  
**Commit:** `7136c1c`

---

### ANTES — Navigate sin Claim atómico

```
[t=0ms] Tres agentes despiertan. Llaman Navigate() simultáneamente.]

Navigate(agent-alpha): lee presiones → "paladin": 0.95, "echo": 0.88, "bravo": 0.80
Navigate(agent-beta):  lee presiones → "paladin": 0.95, "echo": 0.88, "bravo": 0.80
Navigate(agent-gamma): lee presiones → "paladin": 0.95, "echo": 0.88, "bravo": 0.80

[Los tres ven la misma presión. Los tres eligen "paladin".]

agent-alpha: Work("paladin") → feromona "solving"
agent-beta:  Work("paladin") → feromona "solving"  [colisión]
agent-gamma: Work("paladin") → feromona "solving"  [colisión]

StigmaStore: zona "paladin" tiene 3 feromonas "solving"
SwarmResult: SolvedZones=5, CollisionRate=0.60
             [3 zonas resueltas una sola vez, 1 zona resuelta 3 veces]
```

### DESPUÉS — Navigate con Claim atómico

```
[t=0ms] Tres agentes despiertan. Llaman Navigate() simultáneamente.

Navigate(agent-alpha): presión "paladin"=0.95
  → StigmaStore.Claim("paladin", "alpha") → mutex.Lock → vacío → escribe → mutex.Unlock
  → retorna "paladin" ✅

Navigate(agent-beta): presión "paladin"=0.95
  → StigmaStore.Claim("paladin", "beta")  → mutex.Lock → ocupada por "alpha" → mutex.Unlock
  → retorna nil. Intenta siguiente: "echo"=0.88
  → StigmaStore.Claim("echo", "beta")     → mutex.Lock → vacía → escribe → mutex.Unlock
  → retorna "echo" ✅

Navigate(agent-gamma): presión "paladin"=0.95 → ocupada. "echo"=0.88 → ocupada.
  → StigmaStore.Claim("bravo", "gamma")   → mutex.Lock → vacía → escribe → mutex.Unlock
  → retorna "bravo" ✅

[Cada agente trabaja una zona distinta. Sin comunicación directa entre ellos.]

SwarmResult: SolvedZones=5, CollisionRate=0.00
```

**Qué cambió en la librería:** Navigate pasó de ser una función de lectura a ser una operación atómica check-and-set. El StigmaStore ahora es la única fuente de verdad sobre qué está siendo trabajado.

---

## GUIÓN 2: El agente que no dejaba rastro

**Cambio:** `agent.TraceCtx()` — WorkFuncs con acceso al contexto semántico  
**Commit:** `b233dd5`

---

### ANTES — WorkFunc ciega

```
WorkFunc: func(ctx, zone, agent) (*Result, error) {
    // El agente ejecuta código.
    total := calcularTotal(facturas)

    // No hay manera de registrar lo que hizo.
    // Paladin no sabe que se calculó un total.
    // Bravo no puede verificar nada.

    return &Result{Output: "done"}, nil
}

Paladin: [span "zone.calculate_totals" existe pero está vacío]
Bravo:   VarCoverage=0%, RuleCoverage=0%
         "No evidence found for any rule."
```

### DESPUÉS — WorkFunc con TraceCtx()

```
WorkFunc: func(ctx, zone, agent) (*Result, error) {
    tc := agent.TraceCtx()  // el agente accede al span activo de Paladin

    total := calcularTotal(facturas)

    tc.Rule("amounts-balance-rule", "line items must sum to total", nil)
    tc.Check("amounts-balance", "balanced", "balanced", true)
    tc.Event("totals-verified", "total=32250 all_balanced=true", nil)

    return &Result{
        Vars: map[string]any{"line_items_sum": total, "amounts_balanced": true},
    }, nil
}

[Agent.Work() recibe el Result.Vars y los escribe en Paladin automáticamente]

Paladin: span "zone.calculate_totals" {
    semantic: rule "amounts-balance-rule" evidenciada
    semantic: check "amounts-balance" passed
    semantic: event "totals-verified"
    vars: { line_items_sum: 32250, amounts_balanced: true }
}

Bravo:   VarCoverage=100%, RuleCoverage=100%
         "All critical variables recorded. All rules evidenced."
```

**Qué cambió en la librería:** Agent ahora expone su span activo de Paladin durante Work(). El WorkFunc puede escribir en la traza semántica. Agent.Work() escribe automáticamente Result.Vars al terminar.

---

## GUIÓN 3: Las zonas que venían del programador

**Cambio:** `echo_bridge.go` — el problema lo define la sesión de descubrimiento  
**Commit:** `dd06dc4`

---

### ANTES — Zonas hardcodeadas en Go

```
// El programador decide qué es urgente.
// La urgencia está en su cabeza, no en los datos del cliente.

zones := []swarm.Zone{
    {ID: "validate_invoices", PainWeight: 0.95},
    {ID: "match_vendors",     PainWeight: 0.80},
    {ID: "calculate_totals",  PainWeight: 0.75},
}

// Si el cliente dice que el problema más urgente es la trazabilidad,
// el programador tiene que editar el código, recompilar, redesplegar.

Navigate(agent-alpha): presión "validate_invoices"=0.95
→ trabaja "validate_invoices"  [urgencia definida por el programador]
```

### DESPUÉS — Zonas desde el árbol Echo

```
// frameworkecho.json tiene 5 nodos OPPORTUNITY con confidence del cliente.
// ZonesFromEchoFile los lee y convierte:
//   confidence=95 → PainWeight=0.95
//   confidence=83 → PainWeight=0.83

zones, _ := swarm.ZonesFromEchoFile("frameworkecho.json")

// ZonesFromEchoFile:
//   lee nodes → filtra type="OPPORTUNITY", status≠"REJECTED"
//   ordena por confidence desc
//   normaliza title → snake_case ID
//   retorna []Zone ordenada por urgencia real del cliente

zones[0]: {ID: "validar_campos_obligatorios_...", PainWeight: 0.95}  ← el cliente dijo 95%
zones[4]: {ID: "registrar_trazabilidad_...",      PainWeight: 0.83}  ← el cliente dijo 83%

Navigate(agent-alpha): presión "validar_campos..."=0.95
→ trabaja lo que el cliente marcó como más urgente
```

**Qué cambió en la librería:** ZonesFromEchoFile es la única función nueva. Pero cambia quién define el campo de presión: antes era el programador, ahora es el JSON de la sesión Echo. El enjambre trabaja según las prioridades del cliente, no las del código.

---

## GUIÓN 4: El score que no sabía qué verificar

**Cambio:** `swarm.IdealFlowForZones()` + scorer de Bravo — del "¿funcionó?" al "¿hizo lo que debía?"  
**Commit:** `b233dd5` + `dd06dc4`

---

### ANTES — Sin IdealFlow

```
s.Run(ctx)
// El enjambre trabajó. ¿Cómo sé si lo hizo bien?

SwarmResult: SolvedZones=5, CollisionRate=0.00
// "5 zonas resueltas, sin colisiones."
// ¿Pero verificó las reglas de negocio? ¿Registró las variables críticas?
// ¿Siguió el camino correcto? No hay forma de saberlo.
```

### DESPUÉS — Con IdealFlow y BravoScore

```
// Alfa compiló la spec del cliente en un IdealFlow:
//   CriticalPath: ["validate_invoices", "match_vendors", ...]
//   CriticalVars: ["invoice_id", "total_amount", "vendor_match", ...]
//   Rules:        [{name: "vendor-registry-rule", when: "...", then: "..."}]

flow := swarm.IdealFlowForZones(base, zones)
// IdealFlowForZones reemplaza CriticalPath con zone IDs reales
// para que el scorer los encuentre en los span names de Paladin

s.Run(ctx)

ScoreLatestTrace(flow, "temp/paladin/", 0.80):

  // Lee la traza de Paladin (todos los spans del enjambre)
  spanNames := collectSpanNames(trace.Root)
  // → ["invoice-swarm", "agent.alpha", "zone.validate_invoices", ...]

  // PathCoverage: ¿están todos los pasos del camino crítico en los spans?
  "validate_invoices" → fuzzyContains(spanNames) → ✅
  "match_vendors"     → fuzzyContains(spanNames) → ✅
  PathCoverage = 100%

  // VarCoverage: ¿están todas las variables críticas en los vars de la traza?
  "invoice_id"    → varKeys contiene "invoice_id"    → ✅
  "total_amount"  → varKeys contiene "total_amount"  → ✅
  VarCoverage = 100%

  // RuleCoverage: ¿hay eventos semánticos que evidencien cada regla?
  "vendor-registry-rule" → semantic events contienen "vendor-registry" → ✅
  RuleCoverage = 100%

  // Violations: ¿hubo eventos de tipo "violation"?
  "violation: vendor_registry" → encontrado → Violations=1

  Score = mean(1.0, 1.0, 1.0) × (1 - 0.15) = 0.85
  Passed = true (0.85 ≥ 0.80)

BravoScore: 0.85 ✅
```

**Qué cambió en la librería:** Antes el enjambre podía "terminar" sin dejar evidencia de nada. Ahora cada run produce un score determinista que responde preguntas concretas: ¿cubrió el camino? ¿registró las variables? ¿evidenció las reglas? ¿detectó problemas?

---

## GUIÓN 5: El gap que aún existe

**Estado actual:** WorkFuncs deterministas  
**Lo que falta:** WorkFuncs con LLMs reales

---

### Lo que el enjambre hace HOY

```
WorkFunc: func(ctx, zone, agent) (*Result, error) {
    // Esta función sabe de antemano qué va a devolver.
    // Simula el resultado de procesar una factura.
    // No lee la factura. No razona. No falla de maneras inesperadas.

    vars["invoice_id"] = "INV-001,INV-002,INV-003,INV-004"  // hardcodeado
    vars["total_amount"] = 32250.0                            // hardcodeado

    tc.Rule("invoice-completeness-rule", "...", nil)  // siempre pasa
    return &Result{Vars: vars}, nil
}

BravoScore: 0.85 ✅
// El score es real. La coordinación es real. La verificación es real.
// El trabajo cognitivo no es real.
```

### Lo que el enjambre debería hacer

```
WorkFunc: func(ctx, zone, agent) (*Result, error) {
    // El agente recibe el texto real de las facturas.
    // Llama a Claude con un prompt construido desde zone.Description.
    // Claude lee, razona, extrae.

    resp := claude.Messages.New(ctx, anthropic.MessageNewParams{
        Model: "claude-sonnet-4-6",
        Messages: buildZonePrompt(zone, invoicesText),
    })

    // Parsea el output de Claude → vars
    vars := parseClaudeOutput(resp.Content[0].Text)

    // vars["invoice_id"] ahora viene de leer el texto real
    // vars["total_amount"] ahora viene de sumar líneas reales
    // tc.Violation() se llama si Claude detecta algo real

    return &Result{Vars: vars}, nil
}

// El mismo BravoScore. La misma coordinación. El mismo 0% colisiones.
// Pero ahora el trabajo es real: Claude leyó las facturas.
```

**Qué falta en la librería:** Una función `ClaudeWorkFunc(client, systemPrompt)` que envuelva una llamada al API de Claude como WorkFunc. La coordinación, la traza y el scoring ya existen. Solo falta el agente cognitivo real.

---

## Cómo leer estos guiones

Cada guión responde la pregunta: **¿qué hace la librería que antes no hacía?**

No responde: ¿qué valor le da al usuario? (eso es consecuencia, no comportamiento)  
No responde: ¿qué features tiene? (eso es marketing, no guión)

Responde: ¿qué pasa adentro cuando llamas a esta función?

---

*Remora · github.com/remora-ia/remora-go*
