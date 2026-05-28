# Directrices Generales de Desarrollo y Debugging Asistido por IA

Tu misión principal es desarrollar código robusto y depurable, facilitando que una IA diagnostique problemas de manera precisa y rápida, sin recurrir al método de "prueba y error".

## Principios Fundamentales

1. **FrameworkBravo como backbone**
   - Este proyecto debe usar el framework FrameworkBravo para registrar detalladamente cada paso de la ejecución.
   - Cada función, incluyendo `main`, debe instrumentarse con el patrón:

```go
func miFuncion(parent *frameworkbravo.Context, ...args) (...) {
    ctx := parent.Child("miFuncion")
    defer ctx.End()

    // ... tu código ...
}
```

2. **Registro detallado de estado y lógica**
   - En cada span debes registrar:
   - Variables de entrada relevantes al inicio.
   - Variables de salida relevantes antes de retornar.
   - Variables intermedias críticas para entender el estado o la lógica.
   - En sistemas con APIs, LLMs o servicios externos: el input enviado, la response recibida, errores, tamaños, ids, tokens, tiempos y estados relevantes.
   - Decisiones lógicas con `ctx.Decision("qué se eligió", "por qué se eligió")`.
   - Cualquier error con `ctx.Error(err)`.

3. **Ciclo de desarrollo centrado en el trace**
   - Escribir código instrumentado con FrameworkBravo.
   - Compilar con `go build ./...`.
   - Ejecutar el programa.
   - Analizar `trace_*.json`.
   - Corregir y repetir.
   - Nunca asumas que algo funciona sin revisar el trace.

4. **Estructura del proyecto**
   - `main.go` debe iniciar el trace y usar `defer trace.Flush()`.
   - La integración de FrameworkBravo debe estar en una ubicación estándar del proyecto.

## Reglas Operativas de Tracing

5. **Trace incremental e interrupciones válidas**
   - FrameworkBravo puede crear y actualizar `trace_*.json` durante la ejecución.
   - El trace puede tener `status: "running"`, `status: "interrupted"` o `status: "completed"`.
   - Si el proceso es interrumpido, no asumas que el trace es inválido.
   - Antes de descartar un trace, revisa `status`, `snapshot_reason`, `generated` y el contenido ya persistido.

6. **Fuente de verdad del diagnóstico**
   - La consola puede mostrar previews truncados para no saturar el output.
   - El archivo `trace_*.json` es la fuente de verdad para análisis.
   - No concluyas que faltan datos solo porque el stdout mostró texto truncado.

7. **Modo rápido para generar traces diagnósticos**
   - Si el objetivo principal es obtener un trace útil para diagnóstico, prioriza un modo rápido cuando exista.
   - Un modo rápido puede reducir debates, budget de reasoning, tamaño de salida o latencia total.
   - Usa el modo completo solo cuando la fidelidad del flujo sea más importante que la velocidad.

8. **Estrategia de ejecución para corridas largas**
   - En tareas largas, no dependas únicamente de esperar la salida completa de `go run`.
   - Verifica si `trace_*.json` ya fue creado o actualizado.
   - Si la corrida es larga, puedes ejecutar el proceso en background y luego inspeccionar el trace.

## Checklist mínimo de calidad del trace

- Cada span relevante tiene `vars`, `decisions` y `errors` cuando corresponde.
- Las llamadas externas tienen request, response y metadata suficiente para diagnóstico.
- El árbol de spans refleja el flujo real y no un trace plano.
- Los cuellos de botella son interpretables y el umbral usado tiene sentido para el proyecto.
- El trace final o parcial permite reconstruir qué pasó sin tener que inventar pasos intermedios.
