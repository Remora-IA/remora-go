# Channel — Especificación de Axiomas

> **Propósito**: Channel es un executor stateless que ejecuta operaciones de validación, sanitización y ejecución. **NUNCA piensa, nunca decide, nunca interpreta.**

---

## AXIOMA 1: Stateless Total
**Channel DEBE ser completamente stateless.** Cada petición DEBE ser procesada de forma 100% independiente, sin acceder a sesión alguna, caché, contador, o memoria de peticiones anteriores.

- **Precondition**: Request válido con X-API-Key correcta.
- **Postcondition**: Estado interno del servidor es idéntico antes y después de procesar el request.
- **Invariant**: No existe estado mutable entre requests.
- **Evidence**: Cloud Run escala a cero sin pérdida de funcionalidad. Sin estado, no hay bugs de sesión ni race conditions.

---

## AXIOMA 2: Dumb Executor (Nunca Piensa)
**Channel DEBE ejecutar únicamente operaciones de validación, sanitización y ejecución.** DEBE rechazar cualquier lógica de orquestación, decisión de negocio, enrutamiento semántico, interpretación de resultados, o conocimiento de frameworks externos.

- **Precondition**: Request válido.
- **Postcondition**: El resultado es puramente el output de la ejecución del comando o lectura del recurso.
- **Invariant**: El código fuente de Channel NO contiene lógica condicional basada en el propósito, objetivo, o identidad del llamador.
- **Evidence**: Si una decisión requiere entender el propósito o el flujo del usuario, no pertenece a Channel.

---

## AXIOMA 3: Contrato de Respuesta Fijo
**Channel DEBE devolver siempre exactamente este JSON con los 6 campos en este orden exacto:**
```json
{
  "success": boolean,
  "exit_code": number,
  "stdout": string,
  "stderr": string,
  "error": string,
  "duration_ms": number
}
```
NO DEBE añadir, modificar, ni eliminar ningún campo bajo ninguna circunstancia.

- **Precondition**: Ninguna (este axioma aplica incluso en errores).
- **Postcondition**: La respuesta siempre contiene los 6 campos especificados y ningún otro.
- **Invariant**: La estructura de respuesta es inmutable durante toda la vida del sistema.
- **Evidence**: El FrameworkAdapter y todos los frameworks dependen de este contrato exacto.

---

## AXIOMA 4: Seguridad Defense in Depth
**Channel DEBE aplicar validación en capas:**
1. X-API-Key obligatoria en header
2. Whitelist estricta de comandos
3. Sanitización de paths rechazando `..`, symlinks y escapes fuera de BASE_DIR
4. Uso exclusivo de `exec.Command` con argumentos separados (NUNCA `sh -c`)
5. Rechazo obligatorio de comandos destructivos (`rm`, `sudo`, `chmod`, `rm -rf`, `chmod -R`)

- **Precondition**: Request entrante.
- **Postcondition**: Si cualquier capa de seguridad falla, la ejecución es abortada y se devuelve `success:false` con error descriptivo.
- **Invariant**: Las 5 capas de seguridad están activas simultáneamente y no pueden ser deshabilitadas.
- **Evidence**: Cada capa es redundante. Si una falla, las demás protegen. Es Defense in Depth.

---

## AXIOMA 5: Timeout Hard de 30 Segundos
**Channel DEBE terminar cualquier ejecución que exceda 30 segundos.** El proceso DEBE ser terminado con `kill`, Y se DEBE devolver `success:false` con `error="timeout exceeded"`.

- **Precondition**: Comando en ejecución.
- **Postcondition**: El proceso es terminado. La respuesta contiene `success:false`, `error:"timeout exceeded"`.
- **Invariant**: El timeout máximo es exactamente 30 segundos. No puede ser mayor.
- **Evidence**: Cloud Run tiene límites de tiempo de respuesta. 30s es el máximo inquebrantable.

---

## AXIOMA 6: Formato de Entrada JSON-RPC 2.0
**Channel DEBE aceptar únicamente requests que cumplan el estándar JSON-RPC 2.0** (method name en string, params como objeto). DEBE rechazar inmediatamente cualquier JSON malformado o que no siga el estándar con HTTP 400 y sin escribir en stdout.

- **Precondition**: Request entrante.
- **Postcondition**: Si el JSON no es válido o no cumple JSON-RPC 2.0, se responde con HTTP 400 y log de error.
- **Invariant**: No se intenta parsear, corregir, ni aceptar JSON no estándar.
- **Evidence**: JSON-RPC 2.0 es el contrato de entrada. Sin estándar no hay interoperabilidad.

---

## AXIOMA 7: Whitelist Obligatoria de Comandos
**Channel DEBE mantener una whitelist hardcodeada de comandos permitidos.** CUALQUIER comando que no esté explícitamente en la whitelist DEBE ser rechazado con `success:false`.

- **Precondition**: Request con método `execute_command` y argumentos.
- **Postcondition**: Si el comando no está en whitelist, se devuelve `success:false` y error descriptivo.
- **Invariant**: No existe modo de deshabilitar la whitelist. No hay wildcard, no hay escape.
- **Evidence**: La whitelist es el perímetro de seguridad. Sin ella, cualquier comando es ejecutable.

---

## AXIOMA 8: Scope Mínimo — Solo 5 Métodos
**Channel DEBE exponer exclusivamente estos 5 métodos:**
1. `execute_command` — Ejecuta comandos de la whitelist
2. `read_file` — Lee archivos dentro de BASE_DIR
3. `write_file` — Escribe archivos dentro de BASE_DIR
4. `list_dir` — Lista directorios dentro de BASE_DIR
5. `http_get` — Realiza GET HTTP

NO DEBE existir ningún otro método, endpoint, ni ruta expuesta.

- **Precondition**: Request con campo `method`.
- **Postcondition**: Si `method` no es uno de los 5 definidos, se devuelve `success:false` con `error:"method not found"`.
- **Invariant**: La superficie de ataque es exactamente 5 métodos. No se añaden nuevos métodos.
- **Evidence**: Cada método adicional es superficie de ataque. Channel es mínimo por diseño.

---

## AXIOMA 9: Manejo de Errores Dentro del Contrato
**Channel DEBE devolver todos los errores** (auth, validación, timeout, ejecución) DENTRO del JSON de respuesta con `success:false` y el campo `error` descriptivo. NUNCA DEBE devolver HTTP 5xx para errores de negocio o validación.

- **Precondition**: Cualquier condición de error.
- **Postcondition**: La respuesta siempre es HTTP 200 con JSON conteniendo `success:false` y `error` descriptivo.
- **Invariant**: HTTP 500 solo se devuelve si el servidor no puede generar una respuesta JSON (crash del proceso).
- **Evidence**: Los frameworks Adapter esperan respuestas en JSON. HTTP 5xx rompe la interoperabilidad.

---

## AXIOMA 10: Logging Mínimo Consistente
**Channel DEBE loguear en cada request:**
- Método llamado
- API Key ofuscada (últimos 4 caracteres)
- Comando + argumentos
- BASE_DIR o path
- exit_code
- duration_ms
- success

**NUNCA DEBE loguear información sensible** (API key completa, passwords, paths externos).

- **Precondition**: Request recibido.
- **Postcondition**: Los 7 campos de log están presentes. API Key nunca aparece completa.
- **Invariant**: El logging nunca contiene información que comprometa la seguridad si los logs se filtran.
- **Evidence**: Logs sin ofuscación son filtraciones de seguridad.

---

## AXIOMA 11: Dependencias — Solo Librería Estándar
**Channel DEBE usar exclusivamente:** `net/http`, `encoding/json`, `os/exec`, `context`, `time`, `log`, `os`, `path/filepath`, `strings`, `io`.

**NO DEBE instalar ni usar ninguna dependencia externa de terceros.**

- **Precondition**: Ninguna (aplica al código fuente completo).
- **Postcondition**: El binary final tiene cero dependencias externas.
- **Invariant**: `go.mod` contiene únicamente stdlib. No existe `require` de `github.com`, `gopkg.in`, ni similares.
- **Evidence**: Dependencias externas son vectores de ataque y complejidad. Channel es mínimo.

---

## AXIOMA 12: Nunca Interpretar Contenido
**Channel DEBE tratar stdout y stderr como bytes opacos.** NO DEBE parsear, analizar, interpretar, ni tomar decisiones basadas en el contenido de estos campos.

- **Precondition**: Comando ejecutado exitosamente.
- **Postcondition**: `stdout` y `stderr` se devuelven exactamente como los recibió `exec.Command`.
- **Invariant**: El código de Channel no contiene expresiones regulares, `strings.Contains`, ni lógica de parsing sobre stdout/stderr.
- **Evidence**: Interpretar stdout rompe la separación de responsabilidades. El análisis pertenece a quien llama.

---

## Validación AxiomForge

```
╔════════════════════════════════════════════════════════════════╗
║                    VALIDACIÓN AXIOMFORGE                     ║
╠════════════════════════════════════════════════════════════════╣
║  Axiomas generados:      12                                   ║
║  Rango requerido:       9-15  ✓ DENTRO DE RANGO              ║
╠════════════════════════════════════════════════════════════════╣
║  DIMENSIÓN MÁS CUBIERTA:                                      ║
║  → Invariantes (Axiomas 1-12)                                 ║
║  → Seguridad (Axiomas 4, 7, 10, 11)                          ║
║  → Negociable vs Inquebrantable (cada axioma es inquebrantable)║
╠════════════════════════════════════════════════════════════════╣
║  RIESGOS CUBIERTOS:                                           ║
║  ✓ IA añadiendo inteligencia a Channel → Axioma 2            ║
║  ✓ Romper contrato de respuesta       → Axioma 3             ║
║  ✓ Convertir en shell completo        → Axiomas 4, 7, 8      ║
║  ✓ Añadir estado interno              → Axioma 1             ║
║  ✓ Expandir alcance                  → Axioma 8             ║
╠════════════════════════════════════════════════════════════════╣
║  RIESGOS RESIDUALES (menores):                                ║
║  • Formato de logging: flexible por diseño, bajo riesgo      ║
║  • Extensión de whitelist: mitigated por Axioma 7 + evidencia║
╚════════════════════════════════════════════════════════════════╝
```

**Conclusión**: Channel queda formalmente especificado en 12 axiomas. El riesgo de malinterpretación más crítico (IA añadiendo inteligencia) está neutralizado por Axioma 2 con evidencia explícita.
