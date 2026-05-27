# framework-store

Persistencia de snapshots de agentes Remora. Permite que conversaciones pausen, sobrevivan reinicios, y corran multi-tenant sin pisarse.

## Why

Un agente conversacional en producción no puede vivir solo en memoria:
- WhatsApp tarda minutos u horas entre mensajes del deudor; el proceso no puede quedarse abierto esperando.
- Múltiples conversaciones concurrentes necesitan estado independiente, no global.
- Auditoría regulatoria (cobranza, compliance) exige historial recuperable.

Esta librería absorbe esa capa.

## Interfaz

```go
type Store interface {
    Save(ctx, conversationID, snapshot)
    Load(ctx, conversationID) (snapshot, error)
    List(ctx) ([]conversationID, error)
    Delete(ctx, conversationID) error
}
```

## Implementaciones

| Paquete | Estado | Notas |
|---|---|---|
| `store/memory` | ✅ funcional | `sync.RWMutex` + `map`. Para tests y MVPs single-proceso. |
| `store/file` | ✅ funcional | Un JSON por conversación. Write atómico con tmp+rename. Para MVP. |
| `store/sqlite` | — | No existe. Para escala (>1k conversaciones concurrentes). |
| `store/redis` | — | No existe. Para multi-proceso con queue de webhooks. |

## Uso

```go
import (
    "github.com/remora-go/framework-agent/agent"
    "github.com/remora-go/framework-store/store"
    filestore "github.com/remora-go/framework-store/store/file"
)

st, _ := filestore.New("./conversations")

// Cargar o crear
snap, err := st.Load(ctx, "SR-2024-0142")
var a *agent.Agent
if errors.Is(err, store.ErrNotFound) {
    a = agent.New(behavior, llmClient, initialState)
} else {
    a = agent.Restore(behavior, llmClient, snap)
}

// ... correr turnos ...

// Persistir después de cada turno o al cerrar el canal
st.Save(ctx, "SR-2024-0142", a.Snapshot())
```

## Comportamiento ante outcomes terminales

Un snapshot con `Outcome != nil` significa que la conversación cerró (acuerdo, escalación, abandono). El consumidor debería rehusarse a abrir nuevos turnos sobre conversaciones terminales — ver `examples/kobra-carolina/main.go` para el patrón.

## Primer consumidor

[`examples/kobra-carolina`](../examples/kobra-carolina). Demuestra:
- Conversación que arranca, pausa al ENTER vacío (canal cerrado), persiste en `./conversations/<deudor_id>.json`.
- Re-ejecución del binario retoma desde el snapshot.
- Terceira ejecución, viendo `agreed`, no reabre.

## Lo que NO hace todavía

- Bloqueo distribuido (si dos workers procesan el mismo deudor a la vez, se sobrescriben). Para multi-proceso sumar `Lock(conversationID)` o usar Redis con LUA.
- TTL / archivado automático de conversaciones terminales.
- Búsqueda por outcome, status, fecha — solo lookup por ID.
- Migraciones de schema si `agent.Snapshot` cambia.
