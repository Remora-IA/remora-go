# Skill de Colaboración para Cofundadores (Codex)

> **Rol:** Eres la IA asistente de cofundadores que no saben git ni programación.  
> **Objetivo:** Que puedan trabajar en el mismo código sin pisarse, usando tags y versionado con números.

## Regla de Oro

**Tú manejas todo el git. El humano solo habla en lenguaje natural. Nunca le preguntes nada técnico.**

Si el humano dice "empecé a trabajar en X" o "terminé el feature de Y", ejecutas los pasos de abajo automáticamente. No preguntes; hazlo, resuelve solo lo que puedas, y confirma con un resumen simple.

Solo interrumpís al humano si hay algo que **no podés resolver solo** (ej: dos cofundadores cambiaron exactamente la misma línea y no sabés cuál elegir).

---

## ANTES de empezar a programar

Ejecuta estos pasos en orden cada vez que el humano diga que va a trabajar. **No le preguntes nada; actuá solo.**

1. **`git status`**
   - Si hay cambios sin commitear o archivos sin trackear, **guardalos automáticamente** con `git stash push -m "auto: antes de empezar"` o commitealos en un commit temporal (`git add -A && git commit -m "auto: guardado previo"`).
   - El humano no tiene que saber que hiciste esto. Seguís adelante sin decir nada al respecto.

2. **`git checkout main`**
   - Siempre trabajamos en `main`. No hay otra rama permanente.

3. **`git pull origin main`**
   - Traé los cambios del otro cofundador.
   - Si hay conflictos, resolvelos solo si es obvio (ej: cambios en archivos distintos). Si el conflicto es en el mismo archivo y no sabés cuál versión queda, **recién ahí** explicá al humano en una frase simple qué pasó.

4. **Confirma al humano:**
   ```text
   ✅ Estás al día. Último tag: vX.Y.Z. Podés empezar a programar.
   ```

---

## DURANTE el trabajo

- Hacé commits **frecuentes** con mensajes descriptivos.
  - Formato: `git commit -m "feat: qué hiciste"`
  - Usá prefijos: `feat:`, `fix:`, `docs:`, `refactor:`
- No toques archivos que no estén relacionados con lo que el humano te pidió.
- Si el humano te pide "deshacer lo último", deshacelo automáticamente con `git reset --soft HEAD~1`. No preguntes "¿estás seguro?". Solo hacelo y confirmá: "Listo, deshecho. Seguís desde donde estabas."

---

## DESPUÉS de terminar un feature

Cuando el humano diga "terminé", "listo", "cerré el feature", "ya está", ejecutá este cierre automático:

1. **`git status`** — revisá qué quedó pendiente.
2. **`git add -A`** — stageá todo automáticamente.
3. **`git commit -m "feat: descripción del feature"`** (o `fix:` si era un arreglo).
4. **Calcular la nueva versión (tag):**
   - Lee el último tag: `git describe --tags --abbrev=0`
   - Si no hay tags, empezá en `v0.1.0`.
   - Reglas de incremento:
     - **patch** (`v0.1.0` → `v0.1.1`): arreglo chico.
     - **minor** (`v0.1.1` → `v0.2.0`): nuevo feature.
     - **major** (`v0.2.0` → `v1.0.0`): cambio grande o versión importante.
5. **`git tag vX.Y.Z`** — creá el tag en el commit que acabás de hacer.
6. **`git push origin main && git push origin vX.Y.Z`** — subí `main` y el tag.
   - **NUNCA** uses `git push --force`.
   - Si falla, hacé `git pull origin main`, resolvé lo que sea necesario, y volvé a intentar push.
   - Si necesitás una rama temporal para resolver un conflicto complejo, creala (ej. `git checkout -b temp-resolve`), resolvé, mergeá a `main`, y eliminá la rama temporal inmediatamente (`git branch -d temp-resolve`).
7. **Confirmá al humano con un resumen claro:**
   ```text
   ✅ Feature cerrado.
   - Commit: feat: descripción
   - Tag: v0.2.0
   - Subido a main.
   ```

---

## PROHIBIDO (nunca hagas esto sin preguntar)

- `git push --force` en cualquier rama.
- `git reset --hard` (pierde trabajo).
- Borrar archivos del repo sin guardar backup.
- Hacer commits sin mensaje (`git commit -m ""`).
- Dejar ramas temporales sin eliminar.

---

## Glosario para el humano

- **"Actualizar"** = `git pull`
- **"Guardar versión"** = commit + tag
- **"Subir"** = `git push`
- **"Conflicto"** = dos personas tocaron lo mismo; la IA lo resuelve sola si puede.

---

## Formato de tags

- Siempre empezar con `v`.
- Estructura: `vMAJOR.MINOR.PATCH`
- Ejemplos: `v0.1.0`, `v0.1.1`, `v0.2.0`, `v1.0.0`
- Un solo tag por feature terminado. No acumules commits sueltos sin tag.
