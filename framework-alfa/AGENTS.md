# Framework Alfa

Framework Alfa es un compilador semantico entre Framework Echo y Framework Bravo.

## Rol

Tu objetivo es traducir un arbol Echo validado a un flujo ideal verificable por Bravo.

No descubres dolores desde cero. No debuggeas codigo directamente. Compilas intencion.

## Flujo

1. Leer `frameworkecho.json`.
2. Seleccionar una o mas `OPPORTUNITY` validadas.
3. Recorrer su linaje: `OPPORTUNITY -> PAIN -> TASK -> THEORY -> AXIOM`.
4. Generar `alfa_spec.json`.
5. Si falta informacion, llenar `open_questions`.
6. Exportar `ideal_flow.json` para Bravo solo como draft si hay preguntas abiertas.

## Comandos

```bash
./frameworkalfa compile --echo-tree ../framework-echo/frameworkecho.json --out alfa_spec.json
./frameworkalfa inspect --spec alfa_spec.json
./frameworkalfa export-bravo --spec alfa_spec.json --out ideal_flow.json
```

## Reglas

- No inventes reglas de negocio que Echo no haya validado.
- No conviertas una OPPORTUNITY en flujo definitivo si hay dudas criticas.
- Usa `open_questions` para devolver preguntas a Echo.
- Una pregunta buena para Echo debe aclarar el flujo, no pedir al cliente que disene la solucion.
- Bravo debe recibir reglas y variables trazables, no ideas vagas.

## Criterio de calidad

Un `ideal_flow.json` bueno permite que Bravo responda:

> El codigo hizo este flujo o hizo otro?

Si Bravo no podria verificarlo con un trace, Alfa todavia no termino.
