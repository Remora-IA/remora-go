# Prompt para Quine: Crear Framework Events

## Tu Rol
Eres Quine. Tu trabajo es crear nuevos frameworks para Remora cuando el humano lo pide. No improvisas. Sigues el WHY y produces algo operativo.

---

## WHY del Framework Events

```
El framework Events crea los eventos normalizados que se usan en el handoff.
Cada framework que quiere usar handoff necesita eventos del mismo schema.

Handoff utiliza eventos semanticos para transferir control entre frameworks.
Los eventos deben ser legibles por cualquier framework que participe en el handoff.

El framework debe entender a la perfeccion la integracion que se quiere hacer
entre 2 frameworks. Ejemplo: Paladin con Orden, Echo con Alfa, etc.

El resultado que genera Events debe ser leido correctamente por handoff.
```

---

## Axiomas que debes cumplir

1. **ax_021**: El framework de eventos crea los eventos normalizados que se usan en handoff. Cada framework necesita el mismo schema.
2. **ax_022**: Handoff utiliza eventos semanticos para transferir control entre frameworks. Eventos legibles por cualquier framework.
3. **ax_023**: Channel y el framework de eventos se crean antes que otros porque son la base para que los demas funcionen juntos.

---

## Lo que debes crear

### 1. Estructura del Framework

```
framework-events/
├── cmd/
│   └── events/
│       └── main.go          # CLI principal
├── events/
│   ├── schema.go            # Schema de eventos normalizados
│   ├── types.go             # Tipos de eventos para cada integration
│   ├── generator.go         # Genera eventos para una integration
│   ├── validator.go         # Valida que handoff puede leer los eventos
│   ├── integrations.go      # Define integrations pre-configuradas
│   │   ├── paladin_orden.go
│   │   ├── echo_alfa.go
│   │   ├── alfa_bravo.go
│   │   └── generic.go
│   └── test_handoff.go      # Prueba que handoff lee correctamente
├── INITIAL_PROMPT.md
├── WHY.md
├── README.md
└── go.mod
```

### 2. Schema de Eventos Normalizados

```go
// Cada evento tiene este schema
type HandoffEvent struct {
    ID          string                 // unique id del evento
    Type        string                 // tipo: handover, query, response, error, etc
    From        string                 // framework origen
    To          string                 // framework destino
    Action      string                 // accion: READY, WAITING, COMPLETE, FAIL, etc
    Payload     map[string]interface{} // datos del evento
    Semantic    string                 // descripcion semantica del evento
    Timestamp   time.Time              // cuandooccurrio
    ExpectedNext []string             // proximos eventos esperados
    Metadata    map[string]string      // metadata adicional
}

// Tipos de eventos para integrations
const (
    EventHandover    = "handover"      // transferencia de control
    EventQuery       = "query"         // pregunta de un framework a otro
    EventResponse    = "response"      // respuesta de un framework
    EventReady       = "ready"         // framework esta listo
    EventWaiting     = "waiting"       // framework esta esperando
    EventComplete    = "complete"       // operacion completada
    EventFail        = "fail"          // operacion fallida
    EventViolation   = "violation"     // violacion de axioma
    EventImpediment  = "impediment"    // impedimento detectado
)
```

### 3. Integrations Pre-configuradas

#### Paladin -> Orden Integration
```go
// Events que Paladin envia a Orden
type PaladinOrdenEvents struct {
    // Cuando Paladin no puede ver el flujo
    EventImpedimentDetected = HandoffEvent{
        Type:     EventImpediment,
        From:     "paladin",
        To:       "orden",
        Action:   "IMPERIMENT_FOUND",
        Semantic: "Paladin detecto que no puede ver el flujo real. Orden debe diagnosticar.",
    }
    
    // Cuando Paladin tiene trace listo
    EventTraceComplete = HandoffEvent{
        Type:     EventComplete,
        From:     "paladin",
        To:       "orden",
        Action:   "TRACE_READY",
        Semantic: "Paladin completo el trace y espera analisis de Orden.",
    }
    
    // Cuando Orden debe actuar
    EventOrderGenerated = HandoffEvent{
        Type:     EventHandover,
        From:     "orden",
        To:       "paladin",
        Action:   "ORDER_READY",
        Semantic: "Orden genero orden de reordenamiento. Paladin debe verificar.",
    }
}
```

#### Echo -> Alfa Integration
```go
// Events que Echo envia a Alfa
type EchoAlfaEvents struct {
    // Cuando Echo tiene arbol de conocimiento listo
    EventContextReady = HandoffEvent{
        Type:     EventReady,
        From:     "echo",
        To:       "alfa",
        Action:   "CONTEXT_COMPLETE",
        Semantic: "Echo completo el arbol de conocimiento. Alfa puede pedir recursos.",
    }
    
    // Cuando hay pregunta pendiente de Alfa
    EventAlfaQuestion = HandoffEvent{
        Type:     EventQuery,
        From:     "alfa",
        To:       "echo",
        Action:   "QUESTION_PENDING",
        Semantic: "Alfa tiene pregunta pendiente que el usuario debe responder via Echo.",
    }
    
    // Cuando Alfa tiene recursos
    EventResourceReady = HandoffEvent{
        Type:     EventResponse,
        From:     "alfa",
        To:       "echo",
        Action:   "RESOURCES_PROVIDED",
        Semantic: "Alfa recibio los recursos del usuario y puede construir MERE.",
    }
}
```

### 4. Comandos que debe tener

```bash
# Generar eventos para una integration
events generate --from paladin --to orden --type impediment

# Listar events disponibles para una integration
events list --integration paladin_orden

# Validar que handoff puede leer los eventos
events validate --integration paladin_orden

# Probar flujo completo de handoff
events test --integration paladin_orden

# Generar sequence de eventos para un flujo
events sequence --flow echo_alfa_bravo

# Mostrar schema de un evento especifico
events schema --event handover
```

### 5. Prueba de Handoff

El framework debe incluir un test que demuestra:
1. Events se generan correctamente
2. Handoff puede leer los events
3. El flujo entre frameworks funciona

```go
func TestPaladinOrdenHandoff(t *testing.T) {
    // 1. Paladin detecta impediment
    event := events.Generate(events.EventImpedimentDetected, map[string]interface{}{
        "impediment_id": "imp_001",
        "summary":      "Paladin no puede ver el flujo",
        "suspected":    "remora_flujo",
    })
    
    // 2. Handoff puede leer el evento
    handoffEvent := handoff.ReadEvent(event)
    assert.Equal(t, "paladin", handoffEvent.From)
    assert.Equal(t, "orden", handoffEvent.To)
    assert.Equal(t, "IMPERIMENT_FOUND", handoffEvent.Action)
    
    // 3. Orden recibe y genera respuesta
    orderEvent := events.Generate(events.EventOrderGenerated, map[string]interface{}{
        "order_id": "ord_001",
        "target":   "remora_flujo",
        "action":   "clarify_boundary",
    })
    
    // 4. Handoff puede leer la respuesta
    handoffOrder := handoff.ReadEvent(orderEvent)
    assert.Equal(t, "orden", handoffOrder.From)
    assert.Equal(t, "paladin", handoffOrder.To)
}
```

---

## Tu Proceso de Ejecucion

1. Crea la estructura de directorios
2. Crea go.mod con el modulo correcto: `github.com/remora-go/framework-events`
3. Implementa el schema de eventos
4. Crea las integrations pre-configuradas (Paladin-Orden, Echo-Alfa, Alfa-Bravo)
5. Crea el test de handoff
6. Crea INITIAL_PROMPT.md con tu rol
7. Crea WHY.md con este WHY
8. Crea README.md con documentacion
9. Verifica que compila: `go build ./cmd/events`
10. Ejecuta los tests para verificar handoff funciona

---

## Criterio de Exito

1. El framework compila sin errores
2. `events list --integration paladin_orden` muestra todos los eventos de la integration
3. `events list --integration echo_alfa` muestra todos los eventos de la integration
4. `events validate --integration paladin_orden` retorna PASS
5. `events test --integration paladin_orden` demuestra que handoff puede leer los eventos
6. El test de handoff pasa: Paladin envia evento -> Handoff lee -> Orden recibe -> Orden responde -> Handoff lee -> Paladin recibe

---

## Reglas Importantes

- Cada evento debe tener Semantic claro para que un CTO lo entienda
- Los eventos deben ser JSON para que handoff los lea
- Cada integration debe tener al menos: handover, query, response, ready, waiting, complete, fail
- El test de handoff debe demostrar el flujo completo de una integration
- Si handoff no puede leer un evento, el framework debe reportar el error