# Remora Tripod — Prueba de Concepto

> "El trípode no es solo una arquitectura. Es un sistema de verificación:
> el enjambre declara qué quería hacer, lo hace, y luego demuestra que lo hizo."

---

## Resultados del Benchmark

```
go test ./bench/ -v -run TestTripodTable
```

```
╔══════════════════════╦═══════╦══════╦══════╦═══════╦══════╦══════╦════════╗
║ Case                 ║ Score ║ Path ║ Vars ║ Rules ║ Viol ║ Coll ║ Status ║
╠══════════════════════╬═══════╬══════╬══════╬═══════╬══════╬══════╬════════╣
║ doc-swarm            ║  1.00 ║ 100% ║ 100% ║  100% ║    0 ║   0% ║ ✅ PASS ║
║ invoice-processing   ║  0.85 ║ 100% ║ 100% ║  100% ║    1 ║   0% ║ ✅ PASS ║
║ bug-triage           ║  1.00 ║ 100% ║ 100% ║  100% ║    0 ║   0% ║ ✅ PASS ║
╚══════════════════════╩═══════╩══════╩══════╩═══════╩══════╩══════╩════════╝

ok  github.com/remora-go/framework-swarm/bench  0.160s
```

**3 dominios distintos. 3/3 PASS. ~50ms por enjambre. 0% colisiones.**

---

## Qué prueba esto

### 1. El enjambre se coordina sin coordinador central (0% colisiones)

Tres agentes trabajan 5 zonas en paralelo. Ninguno duplica trabajo.
La coordinación emerge de las feromonas (`StigmaStore.Claim` atómico):

```
agent-alpha → [presión 0.95] validate_invoices  ← claim exitoso
agent-beta  → [presión 0.85] extract_data       ← claim exitoso (paladin tomado)
agent-gamma → [presión 0.80] match_vendors      ← claim exitoso
```

### 2. El enjambre sigue el camino crítico (Path 100%)

El `CriticalPath` definido por Alfa aparece en las trazas de Paladin.
El scorer busca los nombres de pasos en los span names de la traza:

```
"validate_invoices" → span "zone.validate_invoices" ✅
"extract_data"      → span "zone.extract_data"      ✅
...
```

### 3. El enjambre registra las variables críticas (Var 100%)

Cada `WorkFunc` retorna `Result.Vars` con las variables definidas en `IdealFlow.CriticalVars`.
El agente las registra en Paladin automáticamente vía `agentCtx.Var(k, v)`.

### 4. El enjambre evidencia las reglas de negocio (Rule 100%)

Cada zona llama a `agent.TraceCtx().Rule(...)` y `.Check(...)`, dejando
eventos semánticos que el scorer mapea a las reglas del `IdealFlow`.

### 5. El enjambre detecta problemas reales (invoice: 1 violation)

El caso `invoice-processing` tiene un vendor desconocido (`V-UNKNOWN`).
El agente llama `.Violation(...)` → Bravo penaliza el score (-15%).
**Score 0.85 en vez de 1.00 = el sistema detectó un problema real.** ✅

### 6. El patrón es agnóstico al dominio

| Dominio | Zonas | Reglas | BravoScore |
|---------|-------|--------|-----------|
| Documentación técnica | 5 paquetes Go | completeness, no-duplication, coverage | 1.00 |
| Procesamiento financiero | 5 pasos de factura | completeness, vendor-registry, balance, approval | 0.85 |
| Gestión de incidencias | 5 pasos de triage | severity, critical-first, dedup, taxonomy, component | 1.00 |

---

## Cómo funciona el loop completo

```
Echo         → define el problema (pain weights → zonas)
     ↓
Alfa         → compila la especificación (IdealFlow)
     ↓
Swarm        → N agentes trabajan las zonas en paralelo
              (coordinación via feromonas, sin coordinador central)
     ↓
Paladin      → registra traza semántica colectiva
     ↓
Bravo scorer → compara traza vs IdealFlow → BravoScore 0.0–1.0
     ↓
go test      → PASS si Score ≥ threshold (default 0.80)
```

---

## Cómo agregar un nuevo caso

```go
// bench/cases_midominio.go
func MiDominioCase() SwarmCase {
    return SwarmCase{
        Name: "mi-dominio",
        Zones: []swarm.Zone{
            {ID: "paso_uno", Name: "Paso Uno", PainWeight: 0.9},
            // ...
        },
        IdealFlow: &swarm.IdealFlow{
            CriticalPath: []string{"paso_uno", "paso_dos", ...},
            CriticalVars: []string{"var_clave_1", "var_clave_2", ...},
            Rules: []swarm.VerifyRule{
                {Name: "mi-regla", When: "...", Then: "..."},
            },
        },
        WorkFn: func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
            tc := agent.TraceCtx()
            // 1. Hacer el trabajo
            // 2. Declarar reglas:   tc.Rule("mi-regla", ...)
            // 3. Verificar:         tc.Check("mi-regla", expected, actual, passed)
            // 4. Retornar vars:     return &swarm.Result{Vars: map[string]any{"var_clave_1": valor}}
        },
        Threshold: 0.80,
    }
}
```

Luego en `bench_test.go`:
```go
var allCases = []SwarmCase{
    DocCase(),
    InvoiceCase(),
    TriageCase(),
    MiDominioCase(), // ← agregar aquí
}
```

---

## Performance

```
BenchmarkTripod/doc-swarm-4           3   51ms/op   1.000 bravo_score   0 collision_rate
BenchmarkTripod/invoice-processing-4  3   49ms/op   0.850 bravo_score   0 collision_rate
BenchmarkTripod/bug-triage-4          3   50ms/op   1.000 bravo_score   0 collision_rate
```

~50ms por enjambre de 3 agentes sobre 5 zonas.
Con WorkFuncs basadas en LLMs, el tiempo dominante será la latencia de inferencia.

---

## Lo que aún falta

1. **WorkFuncs con LLMs reales** — actualmente las funciones son deterministas.
   El siguiente paso es conectar `nativeagent` (Groq/Claude) como WorkFunc.

2. **10 casos distintos** — con 3 casos probados, la arquitectura está validada.
   Agregar 7 más siguiendo el patrón de arriba completa el proof of concept original.

3. **Integración completa Echo → Alfa** — las zonas y el IdealFlow se construyen
   manualmente en los casos de test. El pipeline real los genera desde el árbol Echo.

---

*Remora · github.com/remora-ia/remora-go*
