# WHY - Framework Quine

Quine existe porque crear y mantener frameworks debería seguir un estándar,
no la improvisación de cada desarrollador.

Quine genera scaffolding para nuevos frameworks, revisa la calidad de los
existentes contra checklists por tipo, y registra frameworks en un catálogo
centralizado.

## Problema Que Resuelve

Sin Quine, cada framework se crea desde cero con estructura distinta, sin
INITIAL_PROMPT, sin AGENTS.md, sin Paladin integrado. Quine estandariza la
creación y audita la completitud.

## Relación Con Otros Frameworks

- **Paladin** es inyectado por Quine en cada framework nuevo.
- **Crítico** puede ser invocado por Quine para evaluar calidad.
- Todos los frameworks son revisables con `quine review`.

Quine no ejecuta la lógica de ningún framework. Solo los crea y los revisa.
