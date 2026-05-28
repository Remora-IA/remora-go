# Diseño: MERE - Modelo de Estado de Runtime Ejecutable

## Motivación

Un trace JSON es estático: dice "qué pasó". Pero una IA necesita poder **preguntar** sobre el estado en cualquier momento.

Ejemplo:
- "En el momento que se tomó la decisión X, ¿cuál era el valor de `credit.available`?"
- "¿En qué orden se ejecutaron los 5 hijos del span 'processOrder'?"
- "Si `error.code == 500`, ¿qué valores de `vars` estaban activos?"

## Concepto

MERE convierte un trace en un **modelo navegable** donde cada span tiene:

```
span {
  name, file, line
  timing: { start_ns, duration_ms }
  state: { vars }           // variables en ESTE punto
  decisions: [...]
  errors: [...]
  children: [...]          // spans hijos
}
```

## Interfaz propuesta

```go
mere := mere.Load("temp/paladin/trace_pal_*.json")

// Query: encontrar todos los spans con una decisión
decisions := mere.Query("span.decisions.any()")

// Query: encontrar estado en un punto específico
state := mere.StateAt("processOrder", 2) // 2do span 'processOrder'

// Query: encontrar decisiones de "crédito denegado"
denied := mere.Query(`span.decisions.what == "crédito denegado"`)

// Query: reconstruir flujo completo
flow := mere.Reconstruct()
```

## Implementación en Paladin

### Paso 1: Modelo de datos

```go
type MERE struct {
    TraceID string
    Root    *Span
    Index   map[string][]*Span  // índice por nombre
}

func Load(path string) (*MERE, error)
func (m *MERE) Query(expr string) []*Span
func (m *MERE) StateAt(spanName string, nth int) map[string]any
```

### Paso 2: Indexación automática

Al cargar un trace, construir índices:
- Por nombre de span
- Por tipo de decisión
- Por errores

### Paso 3: Query language simple

Opciones:
1. **DSL simple**: `span.name == "processOrder" && span.errors.len() > 0`
2. **Path queries**: `root.children[0].children[1].vars["order.id"]`
3. **GroveQL**: Lenguaje de query propio (¿para qué complejidad?)

## Preguntas de diseño

1. **¿Persistir MERE o generar on-demand?**
   - On-demand es más simple, pero queries repetidas son lentas
   - Persistir el índice acelera queries pero requiere más espacio

2. **¿Qué nivel de normalización?**
   - Baseline: paths absolutos (`root.processOrder.calculateTotal`)
   - Avanzado: normalizar IDs para comparar traces distintos

3. **¿Integración con la IA?**
   - Una IA recibe un trace JSON
   - Pero también puede recibir respuestas de queries MERE
   - "¿Cuál era el estado cuando falló el pago?"

## Siguiente paso

Implementar `mere.go` con:
- `Load()` - cargar trace y construir índices
- `Query()` - buscar spans por criterios
- `StateAt()` - reconstruir estado en un punto

No necesitamos un query language complejo inicialmente. Con funciones concretas alcanza:

```go
mere.FindByName("processOrder")    // todos los spans 'processOrder'
mere.FindByDecision("crédito denegado")  // todos los que tomaron esa decisión
mere.FindByError()                // todos los spans con errores
mere.Ancestors(childSpan)         // camino desde root hasta childSpan
mere.StateAt(span)                // vars en ese span
```
