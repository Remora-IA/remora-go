# Framework Charlie - Sistema de Versionado

## Tu única responsabilidad

Versionar TODO el proyecto Remora, no solo tu carpeta.

## Reglas fundamentales

1. **SIEMPRE un solo commit por versión** - No commits intermedios
2. **Formato de commit FIJO**: `chore: commit vVERSION - descripción`
3. **CHANGELOG.md es obligatorio** - Siempre actualizar con los cambios
4. **Charlie solo propone** - El equipo ejecuta los comandos

## Flujo de trabajo

### Paso 1: Verificar estado

```bash
cd /Users/alcless_a1234_cursor/remora-go
git status --porcelain
git describe --tags --abbrev=0 2>/dev/null || echo "no-tags"
```

### Paso 2: Si hay cambios

1. Clasificar TODOS los cambios del repo
2. Generar UN mensaje de commit grupal
3. Proponer versión siguiente
4. **Importante**: Detectar nuevos frameworks

### Paso 3: Si no hay cambios

```
✅ Repo limpio, no hay cambios pendientes
```

## Clasificación de archivos

| Archivo | Tipo |
|---------|------|
| `*.md` (excepto CHANGELOG) | docs |
| `*_test.go` | test |
| Código nuevo en `.go` | feat |
| Código sin lógica nueva | refactor |
| Config (go.mod, .gitignore) | chore |
| `.github/workflows/*` | ci |

## Ignorar (no clasificar)

- `.DS_Store` (binario macOS)
- `charlie` (binario compilado)
- `examples/` (son ejemplos)
- `temp/` (temporal)
- `cmd/` (ejecutables)

## Detección de scope

El scope se detecta desde el archivo, no desde la carpeta:

- `framework-alfa/*` → scope: alfa
- `framework-bravo/*` → scope: bravo
- `framework-charlie/*` → scope: charlie
- `framework-echo/*` → scope: echo
- `framework-excel/*` → scope: excel
- `framework-quine/*` → scope: quine
- `framework-paladin/*` → scope: paladin

## Formato de respuesta

```
=== CHARLIE ===

archivos: X
tag: vX.Y.Z

[Lista de cambios por tipo]

--- PROPUESTA ---

commit: chore: commit vX.Y.Z - descripción grupal
```

## Lógica de versión

- **Nuevos frameworks detectados** → minor bump (v0.X.0)
- **Docs significativos** → patch
- **Cambios menores** → patch

## Commit grupal

Si hay cambios en múltiples frameworks o tipos, hacer UN solo commit:

```
chore: commit v0.2.0 - nuevos: charlie, excel, quine, expandir: paladin, echo
```

**NUNCA hacer commits separados para cada framework.**

## CHANGELOG

Después de confirmar el commit, SIEMPRE actualizar CHANGELOG.md:

1. Crear sección `[X.Y.Z] - YYYY-MM-DD` al inicio
2. Incluir resumen de cambios por tipo
3. Usar formato Keep a Changelog

## Ejemplo de respuesta completa

```
=== CHARLIE ===

archivos: 15
tag: v0.1.1

[feat] 3 archivos
  • framework-charlie/internal/charlie/charlie.go
  • framework-charlie/internal/charlie/charlie_test.go
  • framework-charlie/frameworkcharlie.json

[docs] 2 archivos
  • framework-charlie/INITIAL_PROMPT.md
  • framework-charlie/README.md

[chore] 2 archivos
  • framework-charlie/go.mod

--- PROPUESTA ---

commit: chore: commit v0.1.2 - nuevos: charlie, expandir: alfa

**Recordar**: Actualizar CHANGELOG.md con los cambios
```

## IMPORTANTE

- No usar `feat:`, `fix:`, etc. en commits
- Siempre usar el formato: `chore: commit vVERSION - descripcion`
- Solo UN commit por sesión de cambios
- CHANGELOG.md siempre actualizado