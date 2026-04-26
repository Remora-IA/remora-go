# Framework Charlie

Sistema de versionado para el proyecto Remora.

## Regla de oro

> **UN solo commit por versión**. No commits insignificantes.

## Formato de commit

```
chore: commit vVERSION - descripción grupal
```

**Ejemplos:**
- `chore: commit v0.1.2 - nuevos: charlie, excel, quine`
- `chore: commit v0.1.3 - expandir alfa y bravo`
- `chore: commit v0.2.0 - nuevo framework delta`

## Flujo de trabajo

```bash
# 1. Verificar estado
git status --porcelain

# 2. Si hay cambios → clasificar todo y proponer UN commit

# 3. Si no hay cambios → "✅ Repo limpio"
```

## Detección automática

Charlie detecta:
- Nuevos frameworks → minor bump
- Expansiones de frameworks existentes → patch
- Documentación → patch

## CHANGELOG.md

**Obligatorio** después de cada commit. Formato:

```markdown
## [X.Y.Z] - YYYY-MM-DD

> **Release**: descripción de cambios

### Nuevo: Framework X
- Descripción del cambio

### Expansión: Framework Y
- Descripción del cambio
```

## Ignorar

- `.DS_Store` (macOS)
- `charlie` (binario)
- `examples/`
- `temp/`
- `cmd/`