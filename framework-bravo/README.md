# FrameworkBravo v5.1 — Golden Flow

FrameworkBravo es un framework de tracing y empaquetado de contexto diseñado para trabajar junto a una IA agentica con terminal.

Su objetivo no es "pensar por la IA", sino darle dos recursos fundamentales para que no opere a ciegas:

- el **flujo ideal** que el humano espera
- el **flujo real** que el programa ejecutó

La comparación, el diagnóstico y las preguntas faltantes las hace la IA. FrameworkBravo prepara el terreno para que esa comparación sea posible y barata.

## Qué es

- Una capa de instrumentación para capturar el flujo real del programa.
- Un formato para expresar el flujo ideal, reglas de negocio, variables críticas y path crítico.
- Un empaquetador de artefactos para que una IA compare ideal vs real con suficiente contexto.

## Qué NO es

- No es un motor de reglas embebido.
- No es un comparador semántico automático en Go.
- No intenta reemplazar la parte agentica del razonamiento.

Esa decisión es intencional: meter la comparación completa dentro del framework volvería el sistema más frágil, más complejo y mucho menos general.

## Principios

1. **Contexto explícito**: Se pasa de padre a hijo. No hay variables globales.
2. **Árbol jerárquico**: La IA ve qué función llamó a cuál.
3. **Variables por función**: La IA ve el valor exacto de cada variable en cada punto.
4. **Detección de bottlenecks**: Se marcan automáticamente las funciones lentas.
5. **Errores registrados**: La IA prioriza funciones con errores.

## Modelo Mental Correcto

FrameworkBravo tiene tres piezas:

1. **IdealFlow**
   describe cómo debería comportarse el flujo según el humano.
2. **Trace**
   describe cómo se comportó realmente el programa.
3. **IA agentica**
   compara ambos, detecta gaps, pregunta lo que falta y propone diagnóstico.

La simplicidad del framework está en mantener esas responsabilidades separadas.

## Reglas de uso

### Regla 1: Crear trace en main

```go
trace := frameworkbravo.NewTrace("MiApp")
defer trace.Flush()
ctx := trace.Start()
defer ctx.End()
```

### Regla 2: Guardar contexto como primera línea de cada función

```go
func miFuncion(parent *frameworkbravo.Context) {
    ctx := parent.Child("miFuncion")
    defer ctx.End()
    // ... tu código ...
}
```

### Regla 3: Registrar variables importantes

```go
ctx.Var("nombre", valor)
```

### Regla 4: Registrar decisiones lógicas

```go
ctx.Decision("qué se decidió", "por qué")
```

### Regla 5: Registrar errores

```go
if err != nil {
    ctx.Error(err)
}
```

## Workflow esperado con IA

1. Instrumenta el código con FrameworkBravo.
2. Declara el `IdealFlow` con reglas, verbalización, intención y variables críticas.
3. Ejecuta el programa para generar:
   - `temp/ideal_flow.json`
   - `temp/IDEAL_FLOW.md`
   - `temp/trace_*.json`
4. Entrega esos artefactos a una IA agentica usando `prompts/VERIFICATION_PROMPT.md`.
5. La IA compara ideal vs real, detecta desviaciones y pregunta contexto faltante si es necesario.

FrameworkBravo no hace el paso 5 por sí solo. Ese es el trabajo de la IA.

## Archivos temporales

Los archivos generados (`ideal_flow.json`, `IDEAL_FLOW.md`, `trace_*.json`) se guardan en la carpeta `temp/` del directorio donde se ejecuta el programa.

```
ejemplos/
└── ecommerce-pedidos/
    ├── main.go
    └── temp/           # Archivos generados automáticamente
        ├── ideal_flow.json
        ├── IDEAL_FLOW.md
        └── trace_*.json
```

## Ejemplo completo (con bugs intencionales)

El ejemplo `examples/ecommerce-pedidos/` demuestra cómo FrameworkBravo puede ayudarte a encontrar 3 bugs sutiles.

```bash
cd examples/ecommerce-pedidos
go run .
```

## Verificación automática del ejemplo

Para comprobar que el ejemplo compila, ejecuta y genera sus artefactos `temp/`:

```bash
go test ./...
```

La suite ejecuta `examples/ecommerce-pedidos` y valida que se creen:

- `temp/ideal_flow.json`
- `temp/IDEAL_FLOW.md`
- `temp/trace_*.json`

## IdealFlow en tus ejemplos

Define el flujo esperado al inicio del main:

```go
ideal := frameworkbravo.NewIdealFlow("Descripción del flujo")
ideal.SetVerbalization("Explicación completa...")
ideal.AddRule("Nombre", "Descripción", "Entonces...")
ideal.AddCriticalVar("variable_importante")
ideal.Save(".")  // Guarda en temp/
trace.ReloadIdealFlow()  // Recargar para el trace
```

## Análisis con IA

Para que una IA analice el trace:

1. Copia el contenido de `temp/ideal_flow.json`
2. Copia el contenido de `temp/trace_*.json` (el último)
3. Usa el prompt de verificación en `prompts/VERIFICATION_PROMPT.md`
4. Si necesitas guiar a la IA durante desarrollo e instrumentación, usa `prompts/SYSTEM_PROMPT.md`

## Por qué este diseño

Los bugs de compilación o de sintaxis suelen ser fáciles de detectar para una IA. Lo difícil son los bugs de flujo y reglas de negocio: ahí el problema no es solo el código, sino la diferencia entre lo que el usuario quería y lo que el sistema hizo.

FrameworkBravo existe para cerrar justamente esa brecha:

- el humano explicita el flujo ideal
- el runtime deja evidencia del flujo real
- la IA compara ambos con contexto suficiente

Ese es el "why" del framework.
