# WHY - Radar

Radar existe para separar la priorización data-aware del foco operativo.

Foco decide qué hacer hoy con la información disponible. Radar calcula, de forma
determinística y trazable, qué entidades del negocio parecen más importantes
según datos declarados en SQLite y en el semantic pack.

Reglas:

- No hardcodear clientes ni fixtures.
- No ejecutar SQL de escritura.
- No adivinar silenciosamente si falta configuración semántica.
- Producir artifacts versionados para que Foco, Sabio y otros frameworks los consuman.
