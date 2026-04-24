# Framework Alfa

Framework Alfa es el mediador entre Framework Echo y Framework Bravo.

Echo descubre el dolor real del usuario. Bravo verifica si el codigo ejecuta el flujo ideal. Alfa traduce entre ambos mundos.

## Why

El problema que resuelve Alfa es semantico:

- Echo habla en `AXIOM`, `THEORY`, `TASK`, `PAIN`, `OPPORTUNITY` y `perceptions`.
- Bravo habla en `IdealFlow`, reglas, variables criticas y path critico.
- La IA que programa puede escribir codigo correcto, sin bugs obvios, pero aun asi implementar un flujo distinto al que el usuario imaginaba.

Alfa existe para compilar intencion:

```text
frameworkecho.json
  -> alfa_spec.json
  -> ideal_flow.json
  -> Framework Bravo compara ideal vs trace real
```

Si falta informacion, Alfa no inventa. Devuelve `open_questions` para que Echo pregunte mejor.

## Comandos

```bash
go build -o frameworkalfa ./cmd/frameworkalfa

./frameworkalfa compile \
  --echo-tree ../framework-echo/frameworkecho.json \
  --out alfa_spec.json

./frameworkalfa inspect --spec alfa_spec.json

./frameworkalfa export-bravo \
  --spec alfa_spec.json \
  --out ../framework-bravo/examples/mi-app/temp/ideal_flow.json
```

Para compilar una oportunidad especifica:

```bash
./frameworkalfa compile \
  --echo-tree ../framework-echo/frameworkecho.json \
  --opportunity op_001 \
  --out alfa_spec.json
```

## Contrato

`alfa_spec.json` contiene:

- `automation_intent`: intencion de automatizacion derivada del arbol Echo.
- `selected_opportunities`: oportunidades Echo compiladas.
- `confirmed_pains`: dolores validados que la automatizacion debe resolver.
- `ideal_steps`: pasos esperados del flujo.
- `business_rules`: reglas verificables por Bravo.
- `critical_variables`: variables que el codigo debe trazar.
- `success_criteria`: criterios de exito.
- `open_questions`: preguntas que deben volver a Echo si falta informacion.
- `export_ready`: `false` si aun hay preguntas abiertas.

## Regla central

Alfa no decide por el usuario y no rellena huecos de negocio con imaginacion tecnica.

Si el arbol Echo no dice como calcular riesgo, Alfa pregunta:

> "Cuando dices riesgo de no pago, que senales pesan mas: antiguedad, monto, comportamiento historico u otra cosa?"

Ese es su valor.
