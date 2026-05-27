# Remora Tripod — Prueba de Concepto

> "El trípode no es solo una arquitectura. Es un sistema de verificación:
> el enjambre declara qué quería hacer, lo hace, y luego demuestra que lo hizo."

## El why

Remora es infraestructura para convertir cualquier tarea digital compleja
en un enjambre verificable.

No "cualquier negocio" — eso limita el imaginario a procesos corporativos.
Cualquier **tarea digital**: extraer datos de WhatsApp y pasarlos a una planilla,
analizar 100 documentos de jurisprudencia y construir un caso,
procesar un backlog de cobranzas y detectar patrones de mora.

El patrón que une todos esos casos:
son tareas de múltiples pasos sobre información, donde un agente solo es lento
y propenso a saltarse cosas, y un enjambre coordinado puede hacerlo en paralelo,
sin duplicaciones, con evidencia de que no se omitió ningún paso.

Para los guiones de cómo evolucionó el comportamiento de la librería: ver `GUIONES.md`.

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
║ hr-onboarding        ║  1.00 ║ 100% ║ 100% ║  100% ║    0 ║   0% ║ ✅ PASS ║
║ supply-chain         ║  0.85 ║ 100% ║ 100% ║  100% ║    1 ║   0% ║ ✅ PASS ║
║ deployment-pipeline  ║  0.85 ║ 100% ║ 100% ║  100% ║    1 ║   0% ║ ✅ PASS ║
║ data-quality         ║  1.00 ║ 100% ║ 100% ║  100% ║    0 ║   0% ║ ✅ PASS ║
║ contract-review      ║  0.85 ║ 100% ║ 100% ║  100% ║    1 ║   0% ║ ✅ PASS ║
║ content-moderation   ║  1.00 ║ 100% ║ 100% ║  100% ║    0 ║   0% ║ ✅ PASS ║
║ support-triage       ║  1.00 ║ 100% ║ 100% ║  100% ║    0 ║   0% ║ ✅ PASS ║
╚══════════════════════╩═══════╩══════╩══════╩═══════╩══════╩══════╩════════╝

ok  github.com/remora-go/framework-swarm/bench  0.507s
```

**10 dominios distintos. 10/10 PASS. ~50ms por enjambre. 0% colisiones.**

| Industria | Casos | BravoScore |
|-----------|-------|-----------|
| Técnico | doc-swarm, bug-triage, deployment-pipeline | 1.00, 1.00, 0.85 |
| Financiero | invoice-processing, contract-review | 0.85, 0.85 |
| RR.HH. | hr-onboarding | 1.00 |
| Logística | supply-chain | 0.85 |
| Datos | data-quality | 1.00 |
| Contenido/Soporte | content-moderation, support-triage | 1.00, 1.00 |

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
BenchmarkTripod/hr-onboarding-4       3   50ms/op   1.000 bravo_score   0 collision_rate
BenchmarkTripod/supply-chain-4        3   50ms/op   0.850 bravo_score   0 collision_rate
BenchmarkTripod/deployment-pipeline-4 3   50ms/op   0.850 bravo_score   0 collision_rate
BenchmarkTripod/data-quality-4        3   50ms/op   1.000 bravo_score   0 collision_rate
BenchmarkTripod/contract-review-4     3   50ms/op   0.850 bravo_score   0 collision_rate
BenchmarkTripod/content-moderation-4  3   50ms/op   1.000 bravo_score   0 collision_rate
BenchmarkTripod/support-triage-4      3   50ms/op   1.000 bravo_score   0 collision_rate
```

~50ms por enjambre de 3 agentes sobre 5 zonas, en 10 dominios distintos.
Con WorkFuncs basadas en LLMs, el tiempo dominante será la latencia de inferencia.

---

## Pipeline completo Echo → Swarm

El bridge `swarm/echo_bridge.go` cierra el loop discovery-first:

```go
// Sin hardcoding: zonas y spec vienen del cliente
zones, _ := swarm.ZonesFromEchoFile("frameworkecho.json")
flow,  _ := swarm.IdealFlowFromAlfaFile("alfa_spec.json")
flow   =  swarm.IdealFlowForZones(flow, zones)

s, _ := swarm.New(swarm.Config{AgentIDs: [...], Zones: zones, WorkFunc: myFn})
result, _ := s.Run(ctx)
score, _ := swarm.ScoreLatestTrace(flow, "temp/paladin/", 0.80)
// score.Passed == true
```

Ver `examples/echo-to-swarm/` para el demo completo y `fixtures/invoice-reconciliation/`
para un árbol Echo real con evidencia de cliente.

---

## Lo que aún falta

1. **WorkFuncs con LLMs reales** — actualmente las funciones son deterministas.
   El siguiente paso es conectar Claude como WorkFunc vía `ANTHROPIC_API_KEY`.

2. **CLI remora-swarm** — `go run ./cmd/remora-swarm --echo file.json --alfa spec.json`
   para lanzar enjambres desde la línea de comandos sin escribir código Go.

---

*Remora · github.com/remora-ia/remora-go*
