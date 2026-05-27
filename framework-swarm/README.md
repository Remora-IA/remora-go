# framework-swarm

> Bio-inspired multi-agent orchestration for Remora.  
> Agents coordinate via **stigmergy** (pheromone fields) — no central coordinator.

---

## El Trípode

```
Echo   → define el espacio de problema (pain weights → campo de presión)
Paladin → sustrato de trazas semánticas colectivas (un trace por enjambre)
Bravo  → verifica el output colectivo contra el ideal_flow
```

`framework-swarm` es la capa que une el trípode en un enjambre funcional.

---

## Cómo funciona

### Estigmergia (Stigmergy)

Los agentes no se comunican directamente. En cambio, modifican el ambiente dejando **feromonas** en el `StigmaStore` compartido, y los demás reaccionan:

```
Feromona "exploring"  → zona ocupada, reducir atracción
Feromona "solved"     → zona resuelta, presión = 0
Feromona "blocked"    → zona con obstáculos
Feromona "promising"  → zona con alta probabilidad de éxito
```

### Campo de Presión

La presión de cada zona determina su atracción:

```
Pressure = PainWeight / (1 + AgentDensity) × (1 − SolvedRatio)
```

- `PainWeight`: urgencia del negocio (viene de Echo)
- `AgentDensity`: agentes ya trabajando ahí (repulsión natural)
- `SolvedRatio`: 1.0 cuando está resuelto → presión 0

El resultado es **orden emergente** sin coordinador central.

---

## Instalación

```bash
go get github.com/remora-go/framework-swarm
```

---

## API

```go
import (
    "context"
    swarm "github.com/remora-go/framework-swarm/swarm"
)

// 1. Definir zonas (vienen del árbol Echo)
zones := []swarm.Zone{
    {ID: "billing",  Name: "Billing Service",  PainWeight: 0.9},
    {ID: "shipping", Name: "Shipping Service", PainWeight: 0.75},
    {ID: "auth",     Name: "Auth Service",     PainWeight: 0.6},
}

// 2. Definir qué hace cada agente en una zona
workFn := func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
    // ... tu lógica aquí (LLM call, análisis, generación de código)
    return &swarm.Result{
        Output: "done",
    }, nil
}

// 3. Crear y ejecutar el enjambre
s, err := swarm.New(swarm.Config{
    ID:       "my-swarm",
    Zones:    zones,
    AgentIDs: []string{"agent-1", "agent-2", "agent-3"},
    WorkFunc: workFn,
    StigmaPath: "temp/stigma.json", // opcional: persistencia
})

result, err := s.Run(context.Background())

fmt.Printf("Solved: %d/%d, Collision rate: %.1f%%\n",
    result.SolvedZones, result.TotalZones, result.CollisionRate*100)
```

---

## Métricas del Benchmark

| Métrica | Descripción | Target |
|---------|-------------|--------|
| `SolvedZones / TotalZones` | Cobertura del problema | > 90% |
| `CollisionRate` | Trabajo duplicado | < 5% |
| `Duration` | Speedup vs 1 agente | > 3× |
| Paladin trace | Trazabilidad colectiva | presente |

---

## Ejemplo: Doc-Swarm

El primer enjambre de Remora documenta los propios packages de remora-go:

```bash
cd examples/doc-swarm
go run .
```

Salida:
- `output/<zone>.md`   — análisis estático del package
- `output/stigma.json` — instantánea del campo de feromonas  
- `output/report.md`   — métricas del benchmark
- `temp/paladin/`      — traza semántica completa (Paladin)

---

## Extender el Enjambre

### Integrar Echo (zonas desde el árbol de dolor)

```go
// Compilar zonas desde frameworkecho.json
echoTree, _ := tree.LoadOrCreate("frameworkecho.json")
opportunities := echoTree.SelectedOpportunities()

zones := make([]swarm.Zone, len(opportunities))
for i, opp := range opportunities {
    zones[i] = swarm.Zone{
        ID:         opp.ID,
        Name:       opp.Title,
        PainWeight: float64(opp.Confidence) / 100.0,
    }
}
```

### Integrar Bravo (verificar output colectivo)

```go
// Después del enjambre, comparar output vs ideal_flow
verifier := bravo.NewVerifier(idealFlow)
for _, result := range swarmResult.Results {
    report := verifier.Compare(result.TraceID)
    // → ¿el enjambre resolvió lo correcto?
}
```

### Agregar LLMs como WorkFunc

```go
workFn := func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
    // Leer feromonas para contexto
    context_pheromones := agent.Stigma().Sense(zone.ID)
    
    // Llamar a tu LLM
    response, err := llmClient.Complete(buildPrompt(zone, context_pheromones))
    
    return &swarm.Result{Output: response}, err
}
```

---

## Arquitectura

```
framework-swarm/
├── swarm/
│   ├── types.go    — Zone, Pheromone, Result, SwarmResult
│   ├── stigma.go   — StigmaStore (thread-safe, file-backed)
│   ├── navigate.go — ComputePressure, Navigate
│   ├── agent.go    — Agent (Work, Navigate, runLoop)
│   └── swarm.go    — Swarm (New, Run, PressureTable)
└── examples/
    └── doc-swarm/  — primer enjambre funcional
```

---

*Remora · github.com/remora-ia/remora-go*
