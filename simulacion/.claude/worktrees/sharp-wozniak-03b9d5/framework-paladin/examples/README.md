# Examples - Framework Paladin

Ejemplos que demuestran cómo usar Paladin para tracing real de aplicaciones.

## Estructura

```
examples/
├── 01_basic/           # Uso básico de tracing
├── 02_decisions/       # Decisiones lógicas y flujo condicional
└── 03_semantic_flow/   # Reglas, checks, expectations y handoffs
```

## 01_basic - Procesamiento de Órdenes

Flujo completo de una orden de compra con:
- Spans jerárquicos
- Variables de contexto
- Decisiones de negocio
- Manejo de errores
- Timing de operaciones

**Ejecutar:**
```bash
cd examples/01_basic
go run main.go
```

**Ver el trace generado:**
```bash
cat temp/paladin/trace_pal_*.json | jq .
```

## 03_semantic_flow - Flujo Semántico

Demuestra el uso correcto para validar lógica de negocio:
- `Actor` para declarar quién actúa
- `Goal` para declarar intención
- `Rule` y `Check` para reglas evaluables
- `Expect` para el próximo estado esperado
- `Violation` para inconsistencias conocidas

**Ejecutar:**
```bash
cd examples/03_semantic_flow
go run main.go
go run ../../cmd/paladin explain temp/paladin/trace_*.json
```

## Cómo interpretar un trace

Un trace de Paladin permite a una IA entender:

1. **Quién hizo qué**: Cada span tiene nombre, archivo y línea
2. **Con qué datos**: Las variables registradas muestran el estado
3. **Por qué se tomó cada decisión**: Los decisions tienen `what` y `why`
4. **Qué falló**: Los errores incluyen el mensaje y contexto
5. **Qué regla de negocio se aplicó**: Los eventos `semantic` declaran reglas, checks y handoffs

## Agregar tracing a tu aplicación

```go
import "github.com/remora-go/framework-paladin/paladin"

func main() {
    trace := paladin.NewTrace("tu-app")
    ctx := trace.Start()
    defer trace.Flush()
    
    tuFuncionPrincipal(ctx)
}

// Pasá ctx a cada función que necesitás trazar
func tuFuncionPrincipal(ctx *paladin.Context) {
    child := ctx.Child("tuFuncionPrincipal")
    defer child.End()
    
    // Registrá variables importantes
    child.Var("request.id", req.ID)
    child.Var("request.type", req.Type)
    
    // Registrá decisiones
    child.Actor("router", "decide qué cola procesa el request")
    child.Goal("enrutar request respetando prioridad")
    child.Rule("urgent_goes_fast", "requests urgent deben ir a cola urgente", nil)

    if req.Type == "urgent" {
        child.Check("urgent_goes_fast", "queue=urgent", "queue=urgent", true)
        child.Decision("usar cola urgente", "tipo de request es urgent")
        child.Expect("next_queue", "urgent")
        return handleUrgent(ctx, req)
    }
    
    child.Check("urgent_goes_fast", "queue=urgent", "queue=normal", req.Type != "urgent")
    child.Decision("usar cola normal", "tipo de request es standard")
    child.Expect("next_queue", "normal")
    return handleNormal(ctx, req)
}
```
