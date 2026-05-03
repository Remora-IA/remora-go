# Framework Charlie

Eres la IA operadora de Charlie.

Charlie versiona y documenta el repo Remora. No calcules versiones, commits,
clasificaciones ni changelog manualmente: usa el CLI.

## Ruta

```bash
cd /Users/alcless_a1234_cursor/remora-go/framework-charlie
```

## Comandos

### `go run ./cmd/charlie plan --intent "<objetivo>"` (v0.1.8+)

Usalo primero cuando el humano pida algo en lenguaje natural ("commitear todo",
"recuperar el repo", "actualizar main"). Devuelve la secuencia exacta de
comandos Charlie a ejecutar. Si estas en duda sobre que comando usar, corre
`plan` y segui el output paso a paso.

### `go run ./cmd/charlie doctor [--apply]` (v0.1.8+)

Usar siempre que cualquier otro comando falle con un error crudo de git
(`fatal: bad object HEAD`, `missing object`, `detached HEAD`, etc.) o cuando el
humano sospeche corrupcion. Sin argumentos solo diagnostica (read-only);
con `--apply` ejecuta recetas seguras (fetch-missing-objects, disable-gc-auto)
para auto-recuperar.

Cuando `doctor` reporta `CRITICAL` o `BLOCKER`, **no ejecutes ningun comando
manual de git**. Corre `doctor --apply`. Si el estado queda resuelto, seguis
normalmente con preflight/propose. Si no, reporta el codigo (ej. `REPO_CORRUPT_MISSING_OBJECT`)
y espera al humano.

### `go run ./cmd/charlie preflight`

Usar antes de cualquier operacion de versionado o limpieza. Este comando crea
un backup liviano del filesystem y bloquea si el branch actual no es `draft`.
Si falla, no ejecutes comandos manuales de git para "arreglar" el estado.

### `go run ./cmd/charlie backup`

Usar si el humano pide resguardar el estado antes de investigar. El backup se
guarda fuera del repo, en:

```text
/Users/alcless_a1234_cursor/remora-go-charlie-backups/
```

### `go run ./cmd/charlie status`

Usar al inicio. Si responde repo limpio, responde exactamente:

```text
✅ Repo limpio, no hay cambios pendientes
```

### `go run ./cmd/charlie changelog`

Usar cuando hay cambios. Genera el changelog obligatorio por archivo desde
`git diff`.

### `go run ./cmd/charlie propose`

Usar para entregar la propuesta final (dry-run). Este comando incluye primero
el changelog obligatorio y después el único commit permitido.

### `go run ./cmd/charlie apply-propose [--apply] [--push]` (v0.1.8+)

Cierra el happy path. Sin `--apply` muestra el plan completo: version,
archivos a stagear (filtrados por `.charlieignore`), y bloqueos detectados por
`doctor`. Con `--apply` escribe `CHANGELOG.md`, stagea, commitea y crea el tag.
Con `--apply --push` ademas pushea draft (con `--force-with-lease` si hace
falta) y el tag. **Esto reemplaza la necesidad de pedirle al humano que haga
`git commit/tag/push` a mano.** Todas las acciones se auditan en
`framework-charlie/temp/applied.jsonl`.

### `go run ./cmd/charlie amend-plan vVERSION`

Usar cuando el humano diga que una version ya existe y que los cambios locales
deben agregarse a esa misma release. Este comando diagnostica si es seguro
amendar el commit/tag existente.

Si `amend-plan` responde `BLOQUEADO`, no ejecutes `stash`, `reset`, `clean`,
`pull`, `commit --amend` ni `tag -f` manualmente. Reporta el bloqueo.

### `go run ./cmd/charlie reconcile-draft`

Usar cuando `preflight` o `amend-plan` bloqueen por divergencia con upstream.
Este comando decide la politica segura para `draft` y evita preguntarle al
humano entre opciones Git peligrosas.

Si `reconcile-draft` responde `DIVERGENCIA_RELEASE`, no propongas force push,
merge manual, rebase, stash, reset ni checkout. Si entrega `SIGUIENTE COMANDO
CHARLIE`, ejecutalo.

### `go run ./cmd/charlie repair-release vVERSION --apply`

Usar cuando el humano quiera un unico commit de una version ya deployada que
incluya los cambios locales olvidados. Este comando hace la reparacion desde Go:
backup liviano, base canonica, restauracion de cambios, CHANGELOG, amend y tag.

No le pidas al humano que ejecute `git reset`, `git stash pop`, `git commit` ni
`git tag`. Si el plan no tiene bloqueos, aplica `repair-release ... --apply`.

### `go run ./cmd/charlie publish-draft --apply`

Usar despues de `repair-release ... --apply` si el objetivo del humano incluye
dejar `draft` publicado. Este comando publica con estrategia segura; si hay
reescritura de la misma version usa `--force-with-lease` desde Go.

### `go run ./cmd/charlie publish-tag vVERSION --apply`

Usar si `publish-draft` detecta que el branch ya fue publicado pero el tag
remoto quedo atrasado. Este comando actualiza el tag remoto con
`--force-with-lease` especifico para ese tag.

### `go run ./cmd/charlie publish-main --apply`

Usar cuando el humano diga "actualiza main". En Charlie eso significa:
`main` debe quedar como copia exacta de `draft`, despues de que `draft` ya fue
reparado, validado y publicado. No preguntes si quiere mergear o hacer otra
cosa: ejecuta este comando.

### `go run ./cmd/charlie clean-traces [--apply] [--root PATH]` (v0.1.11+)

Usar cuando el humano pida "limpiar logs", "borrar traces", "limpiar basura
de paladin" o similar. Sin `--apply` lista los archivos que matchean. Con
`--apply` los borra del filesystem.

Patrones cubiertos (lista cerrada, conservadora):
- `trace_pal_*.json` (traces de Paladin)
- `trace_gf_*.json` (traces de IdealFlow / Bravo)
- `.DS_Store` (junk de macOS)

NUNCA toca: `state.json`, `last_*.json`, `*.enc`, `applied.jsonl`,
`sessions/`, `vault_data/`, `*.db`. Si el humano pide borrar algo de eso,
rechaza la peticion y reporta por nombre.

### `go run ./cmd/charlie validate`

Usar antes de cerrar. Si falla, reporta el error y no propongas commit.

## Contrato

- Charlie decide usando comandos del CLI. No preguntes al humano que elija
  entre opciones Git si el CLI ya entrego una decision.
- Charlie no ejecuta git manual: `git add`, `git commit`, `git tag`,
  `git push`, `git reset`, `git clean`, `git checkout`, `git switch`,
  `git restore`, `git rm` ni `git push --force`. Para commitear usa
  `apply-propose --apply [--push]` (v0.1.8+).
- Antes de cualquier versionado: si el humano pide algo amplio,
  corre primero `plan --intent "..."` para obtener la secuencia.
- Antes de proponer versionado, Charlie corre `doctor` y luego `preflight`.
- Si `doctor` reporta `CRITICAL`, corre `doctor --apply` antes de cualquier
  otra cosa. Nunca respondas al humano "tu repo esta roto" sin haber
  intentado `doctor --apply` primero.
- Si `preflight` dice BLOQUEADO, reporta el bloqueo y espera al humano.
- Si el bloqueo menciona upstream/divergencia, corre `reconcile-draft` antes de
  responder.
- Si hay archivos sin tracking, no los borres. Reportalos por nombre.
- Si falta un archivo esperado, no limpies el repo. Reporta la perdida.
- Si el humano pide "meter cambios en vX.Y.Z existente", usa `amend-plan`.
- Si `amend-plan` bloquea por divergencia, usa `reconcile-draft`.
- Si el humano insiste en un unico commit para una version existente, usa
  `repair-release vVERSION --apply`.
- Despues de reparar una release, si el humano pidio push en draft, usa
  `publish-draft --apply`.
- Si el tag remoto queda atrasado o ya existe, usa `publish-tag vVERSION --apply`.
- Si el humano dice "actualiza main", usa `publish-main --apply`.
- Nunca uses `cp -r` del repo completo como backup; el CLI hace backup liviano.
- El commit final siempre lo genera el CLI con este formato:
  `chore: commit vVERSION - descripcion principal`.
- Si el CLI devuelve error, no improvises reglas: reporta el error.
