# Verificación Genérica de Trace para Diagnóstico sin Pruebas

## Objetivo

Dado un trace JSON generado por FrameworkBravo para un proyecto cualquiera, determina si el trace contiene suficiente información para diagnosticar de forma fiable problemas de contenido y de flujo lógico, sin requerir pruebas adicionales.

## Entrada

- `trace_json`: cadena con el JSON completo generado por FrameworkBravo.
- `descripción_opcional`: breve descripción del flujo objetivo del proyecto.
- `ideal_flow_json_opcional`: JSON que describe el flujo ideal esperado para este proyecto. Puede omitirse si no existe.

## Instrucciones de análisis

0. **Estado del trace**
   - Revisa `status`, `snapshot_reason`, `generated` y `version`.
   - Si `status` es `running` o `interrupted`, evalúa si el trace sigue siendo suficiente para un diagnóstico parcial fiable.
   - No descartes automáticamente un trace interrumpido. Marca solo los gaps causados por la interrupción.

1. **Estructura y jerarquía**
   - ¿El trace presenta una jerarquía razonable con `root` y `children`, o es plano?
   - ¿Cada span tiene un camino claro de padre a hijo y refleja el flujo real?

2. **Interacciones externas y entrada/salida**
   - En spans que representen llamadas a APIs, LLMs o servicios externos, ¿se registran al menos:
   - input o prompts enviados, o si no es viable, un resumen fiel.
   - salida o responses recibidas y posibles errores.
   - longitudes, tokens, ids, estados y tiempos relevantes para cada interacción.

3. **Variables y estado**
   - ¿Cada span relevante contiene variables de entrada y salida suficientes para entender el estado en ese punto?
   - ¿Faltan variables críticas necesarias para reproducir o entender el fallo?

4. **Decisiones y lógica**
   - ¿Se registran decisiones lógicas tomadas entre pasos con `what` y `why`?
   - ¿Hay fases o etapas claramente identificables?

5. **Rendimiento y cuellos de botella**
   - ¿Se detectan y reportan bottlenecks adecuados?
   - ¿Los umbrales usados parecen razonables o deberían adaptarse?

6. **Cobertura del objetivo del proyecto**
   - Si se proporcionó `ideal_flow_json_opcional`, ¿el trace permite comparar el flujo actual con el ideal?
   - ¿Existen discrepancias claras entre lo esperado y lo observado?

7. **Robustez ante tamaño de datos**
   - ¿El trace distingue correctamente entre previews de consola y datos persistidos en el JSON?
   - Si hay textos largos, ¿incluye al menos longitud, tipo, ids, tokens, estados y contexto suficiente para diagnosticar?
   - Si faltan cuerpos completos, ¿existen previews fieles o referencias suficientes para reconstruir el problema?

## Salida solicitada

Responde con un JSON con este formato exacto:

```json
{
  "sufficient": true,
  "gaps": [
    {
      "span_path": "ruta.al.span",
      "missing_fields": ["campo1", "campo2"],
      "rationale": "explicación corta de por qué es necesario"
    }
  ],
  "observations": "resumen de hallazgos relevantes",
  "recommendations": [
    "instrumentación adicional 1",
    "instrumentación adicional 2"
  ],
  "ideal_comparison": {
    "matched": true,
    "differences": [
      {
        "criterion": "nombre_del_criterio",
        "actual": "valor actual",
        "expected": "valor esperado"
      }
    ],
    "notes": "detalle sobre la comparación"
  }
}
```

## Criterio de decisión

- Usa `sufficient: true` solo si el trace permite reconstruir el flujo real y diagnosticar la causa probable del problema sin ejecutar pruebas adicionales.
- Usa `sufficient: false` si faltan datos críticos en spans clave, especialmente en decisiones, estado o llamadas externas.
