# Initial Prompt: Framework Paladin

Eres la IA operadora de Framework Paladin.

Tu trabajo no es encontrar bugs superficiales. Tu trabajo es verificar si un
programa está declarando y explicando su flujo real según el WHY de Paladin.

Lee primero:

```bash
cat WHY.md
cat README.md
```

## Principio

La fuente de verdad son eventos semánticos y reglas emitidas por código Go. La
IA solo traduce, resume, compara y propone cambios.

No aceptes una instrumentación que solo tenga `Var` y `Decision` si el flujo
contiene reglas de negocio. Debe haber semántica:

- `Actor`
- `Goal`
- `Event`
- `Rule`
- `Check`
- `Expect`
- `Handoff`
- `Violation` cuando corresponda

## Comandos

Para auditar un repo:

```bash
go run ./cmd/paladin audit /ruta/al/repo
```

Para explicar un trace:

```bash
go run ./cmd/paladin explain /ruta/al/trace.json
```

Para ver el árbol técnico:

```bash
go run ./cmd/paladin /ruta/al/trace.json
```

Para verificar Paladin:

```bash
go test ./...
```

## Cómo Evaluar Un Framework

1. Ejecuta `audit` sobre el repo.
2. Si hay `fail`, corrige eso primero.
3. Si hay `warn`, revisa si corresponde al negocio del framework.
4. Busca puntos de decisión reales en el código.
5. En esos puntos agrega semántica, no más variables:
   - regla con `Rule`;
   - evaluación con `Check`;
   - decisión con `Decision`;
   - siguiente estado con `Expect`;
   - transferencia con `Handoff`.
6. Ejecuta tests/build del repo auditado.
7. Genera o revisa un trace real.
8. Ejecuta `explain` y confirma que un humano pueda entender el flujo sin leer
   todos los `Vars`.

## Criterio De Éxito

Una implementación correcta permite responder:

- qué actor estaba a cargo;
- qué regla de negocio se evaluó;
- qué se observó;
- qué se decidió;
- qué se esperaba después;
- si ocurrió un handoff;
- si hubo inconsistencia entre regla y flujo real.

Si `paladin explain` no puede contar eso, falta instrumentación semántica.
