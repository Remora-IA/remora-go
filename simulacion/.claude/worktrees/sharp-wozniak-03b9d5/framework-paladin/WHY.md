# WHY - Framework Paladin

Paladin existe para entender el flujo real que siguió un programa en ejecución y
compararlo contra las reglas de negocio que el código cree estar aplicando.

No existe solo para saber si algo falló. Existe para responder:

- qué actor estaba a cargo;
- qué objetivo intentaba cumplir;
- qué regla de negocio evaluó;
- qué observó;
- qué decidió;
- qué esperaba que pasara después;
- qué handoff realizó;
- qué inconsistencia apareció entre intención y ejecución.

## Fuente De Verdad

La fuente de verdad no debe ser una IA leyendo logs crudos. La fuente de verdad
son eventos semánticos y reglas emitidas por código Go.

Una IA puede traducir, resumir, comparar y explicar. No debe inventar la
semántica del flujo desde variables sueltas.

## Contrato

Cada framework que usa Paladin debe declarar su lógica de negocio con eventos
semánticos estructurados:

- `Actor`: quién actúa.
- `Goal`: qué intenta lograr.
- `Event`: qué hecho de negocio ocurrió.
- `Rule`: qué regla existe.
- `Check`: cómo se evaluó esa regla.
- `Decision`: qué decisión se tomó.
- `Expect`: qué estado se espera después.
- `Handoff`: quién transfiere control a quién.
- `Violation`: qué desvío se detectó.

`Var` sigue siendo útil, pero es evidencia técnica. No reemplaza a la semántica.

## Uso Correcto

No tracear todo. Instrumentar donde vive la lógica de negocio:

1. antes de evaluar una regla;
2. cuando se toma una decisión;
3. cuando cambia el actor responsable;
4. cuando el sistema queda esperando algo;
5. cuando lo observado no coincide con lo esperado.

Si una IA o un humano necesita leer diez variables para entender una decisión,
falta instrumentación semántica.

## Auditoría

Paladin debe poder auditar otros repositorios con comandos Go:

```bash
go run ./cmd/paladin audit /path/al/repo
```

El audit no prueba que la lógica de negocio sea correcta. Verifica si el repo
está usando Paladin de forma compatible con este WHY:

- si crea traces;
- si usa spans;
- si declara actores y objetivos;
- si declara reglas y checks;
- si declara expectativas y handoffs;
- si registra violaciones;
- si hay exceso de variables técnicas sin semántica.

Ese resultado permite que una IA agentica modifique un framework con evidencia
estructurada, no con intuición.
