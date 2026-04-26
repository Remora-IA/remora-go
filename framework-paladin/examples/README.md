# Examples - Framework Paladin

Ejemplos que demuestran cómo usar Paladin para tracing real de aplicaciones.

## Estructura

```
examples/
├── 01_basic/           # Uso básico de tracing
└── 02_decisions/       # Decisiones lógicas y flujo condicional
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

## Cómo interpretar un trace

Un trace de Paladin permite a una IA entender:

1. **Quién hizo qué**: Cada span tiene nombre, archivo y línea
2. **Con qué datos**: Las variables registradas muestran el estado
3. **Por qué se tomó cada decisión**: Los decisions tienen `what` y `why`
4. **Qué falló**: Los errores incluyen el mensaje y contexto

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
    if req.Type == "urgent" {
        child.Decision("usar cola urgente", "tipo de request es urgent")
        return handleUrgent(ctx, req)
    }
    
    child.Decision("usar cola normal", "tipo de request es standard")
    return handleNormal(ctx, req)
}
```
