# WHY - Framework Bravo

Bravo existe porque tener un flujo ideal escrito no garantiza que el código
lo cumpla.

Bravo compara traces reales contra el ideal_flow generado por Alfa. Si el
programa hizo el flujo esperado, Bravo lo confirma. Si no, Bravo dice
exactamente dónde divergió.

## Problema Que Resuelve

Sin Bravo, la verificación es manual: alguien lee logs, compara contra una
spec mental y decide si "funciona". Bravo hace esa comparación
determinísticamente con artefactos estructurados.

## Relación Con Otros Frameworks

- **Alfa** genera el ideal_flow que Bravo consume.
- **Paladin** provee los traces que Bravo analiza.
- **Charlie** genera el código que Bravo verifica.

Bravo no genera código ni specs. Solo verifica.
