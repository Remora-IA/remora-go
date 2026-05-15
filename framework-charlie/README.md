# Framework Charlie

Sistema de versionado para repos Git. Charlie puede operar el repo actual o uno
externo via `--root`.

## ⚠️ REGLA DE ORO

> **UN solo commit por versión**. No commits separados.

❌ **INCORRECTO:**
```bash
git commit -m "feat(quine): crear Framework Quine"
git commit -m "feat(paladin): expandir Framework"
```

✅ **CORRECTO:**
```bash
git commit -m "chore: commit v0.2.0 - nuevos: quine, excel, expandir: paladin"
```

## Formato de commit

```
chore: commit vVERSION - descripción grupal
```

**Ejemplos:**
- `chore: commit v0.1.2 - nuevos: charlie, excel, quine`
- `chore: commit v0.1.3 - expandir alfa y bravo`
- `chore: commit v0.2.0 - nuevo framework delta`

## Novedades v0.1.8

- **`charlie doctor [--apply]`** — diagnostica y auto-recupera corrupcion
  de repo (HEAD apuntando a objeto faltante, gc agresivo, detached HEAD,
  divergencia). Sin `--apply` es 100% read-only. Con `--apply` corre recetas
  seguras ordenadas por nivel de riesgo (fetch primero, rewrite de ref ultimo).
- **`charlie apply-propose [--apply] [--push]`** — cierra el happy path.
  Antes Charlie solo proponia y obligaba al humano/Alfa a `git commit`.
  Ahora Charlie commitea via su propio runGitControlled, respetando la
  politica, con backup automatico y audit log.
- **`charlie plan --intent "..."`** — router de intenciones. La IA
  operadora describe el objetivo en lenguaje natural y Charlie devuelve
  la secuencia de comandos a ejecutar.
- **`charlie --root /ruta/al/repo ...`** — selecciona explicitamente el repo
  objetivo. Si no pasas `--root`, Charlie intenta usar el repo git actual y,
  si no existe, el repo que contiene `framework-charlie`.
- **`.charlieignore`** — archivo en `framework-charlie/` con globs que
  filtran binarios compilados, bases de datos y trazas. Evita commits
  monstruo como el de v0.1.7 (125 files / 817k lineas).
- **Audit log** en `temp/applied.jsonl` dentro del `framework-charlie`
  ejecutado — cada `--apply` deja una linea JSON. Sobrevive a una corrupcion
  de reflog.
- **Preflight ahora llama a Doctor** — si hay corrupcion, preflight la
  reporta como bloqueo con prefijo `[doctor]` antes de continuar.

## Flujo de trabajo

```bash
# Operar el repo actual
go run ./cmd/charlie status

# Operar un repo externo
go run ./cmd/charlie --root /ruta/al/repo status

# 0. (opcional) Si el humano pide algo ambiguo, traduce a comandos
go run ./cmd/charlie --root /ruta/al/repo plan --intent "commitear todo y pushear"

# 1. Chequeo de salud (v0.1.8+)
go run ./cmd/charlie --root /ruta/al/repo doctor
# Si reporta CRITICAL/BLOCKER:
go run ./cmd/charlie --root /ruta/al/repo doctor --apply

# 2. Crear backup y verificar que estas en draft
go run ./cmd/charlie --root /ruta/al/repo preflight

# 2. Verificar estado
go run ./cmd/charlie --root /ruta/al/repo status

# 3. Si hay cambios → generar changelog/propuesta para nueva version
go run ./cmd/charlie --root /ruta/al/repo propose

# 4. Si los cambios deben entrar en una version existente
go run ./cmd/charlie amend-plan v0.1.4

# 5. Si hay divergencia entre draft y origin/draft
go run ./cmd/charlie reconcile-draft

# 6. Si el humano quiere reparar una version existente
go run ./cmd/charlie repair-release v0.1.4 --apply

# 7. Si el humano pidio publicar draft
go run ./cmd/charlie publish-draft --apply

# 8. Si el humano dice "actualiza main"
go run ./cmd/charlie publish-main --apply

# 9. Si no hay cambios → "✅ Repo limpio"
```

## Guardrails

Charlie decide con comandos del framework y no le pregunta al humano que elija
entre opciones Git peligrosas cuando el CLI ya dio una decision.

Charlie no ejecuta operaciones destructivas ni de escritura en Git manual. Bloquea:

- `git add`
- `git commit`
- `git tag`
- `git push`
- `git reset`
- `git clean`
- `git checkout`
- `git switch`
- `git restore`
- `git rm`

El framework puede crear backups de filesystem con:

```bash
go run ./cmd/charlie backup
```

Los backups son livianos: no copian `.git`, `temp/`, `bin/`, binarios generados
ni `.DS_Store`. Viven fuera del repo:

```text
<directorio-padre-del-repo>/<nombre-del-repo>-charlie-backups/
```

## Release existente

Cuando el humano diga que una version ya existe y que los cambios locales deben
quedar dentro de esa misma release, no uses el flujo lineal `propose` a ciegas.
Primero diagnostica:

```bash
go run ./cmd/charlie amend-plan v0.1.4
```

El comando bloquea si:
- no estas en `draft`
- `draft` esta detras de su upstream
- el tag objetivo no apunta al `HEAD`

Si esta bloqueado, reporta el bloqueo. No intentes arreglar con `stash`,
`reset`, `clean`, `pull`, `commit --amend` o `tag -f` manuales.

## Reconciliacion de draft

Cuando `preflight` o `amend-plan` bloqueen por divergencia con upstream:

```bash
go run ./cmd/charlie reconcile-draft
```

Este comando decide la politica segura. En particular, si local y remoto tienen
commits distintos que declaran la misma release, Charlie responde
`DIVERGENCIA_RELEASE` y bloquea:
- `push --force`
- merge/rebase manual
- reset/checkout manual
- stash pop/drop automatico

La IA no debe preguntar "A o B" en ese caso. Si el comando entrega
`SIGUIENTE COMANDO CHARLIE`, debe ejecutarlo.

## Reparacion de release

Cuando el humano quiere que una version ya deployada quede como un unico commit
con cambios locales olvidados:

```bash
go run ./cmd/charlie repair-release v0.1.4 --apply
```

El comando aplica una secuencia controlada desde Go:
- crea backup liviano
- captura cambios locales sin usar stash
- usa `origin/draft` como base canonica si declara la misma version
- restaura los cambios locales encima
- actualiza `CHANGELOG.md`
- hace `commit --amend`
- mueve el tag local de la version

La IA no debe pedirle al humano que ejecute `git reset`, `git stash pop`,
`git commit` o `git tag` para este caso.

## Publicacion de draft

Despues de reparar una release, si el objetivo incluye dejar `draft` publicado:

```bash
go run ./cmd/charlie publish-draft --apply
```

El comando exige working tree limpio y tag local apuntando al `HEAD`. Si detecta
reescritura de la misma version, publica `draft` con `--force-with-lease`.
Si el branch ya esta publicado pero el tag remoto quedo atrasado:

```bash
go run ./cmd/charlie publish-tag v0.1.4 --apply
```

El tag remoto se actualiza con `--force-with-lease=refs/tags/vVERSION:<sha-remoto>`,
no con `git push --force` manual.

## Actualizacion de main

En Charlie, `draft` es la copia de trabajo segura de `main`. Cuando el humano
dice "actualiza main", significa:

```bash
go run ./cmd/charlie publish-main --apply
```

El comando hace que `main` local y `origin/main` queden exactamente en el mismo
commit que `draft`. Bloquea si `origin/draft` no coincide con `HEAD`, si hay
cambios locales significativos, o si `main` no puede actualizarse de forma
controlada. Si `main` declara la misma version pero tiene otro hash, usa
`--force-with-lease` especifico para `refs/heads/main`.

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
- `temp/`
- `bin/`
