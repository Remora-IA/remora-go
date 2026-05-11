# WHY - Framework Sabio

Sabio existe porque tener datos indexados no sirve si nadie puede
preguntarles nada.

Sabio recibe preguntas en lenguaje natural, genera SQL, consulta SQLite y
devuelve respuestas con evidencia trazable. No inventa: si la base no tiene
el dato, lo dice.

## Problema Que Resuelve

Sin Sabio, consultar datos requiere saber SQL o explorar tablas manualmente.
Sabio traduce la intención del usuario a SQL y devuelve la respuesta
formateada.

## Relación Con Otros Frameworks

- **Indexa** prepara los datos que Sabio consulta.
- **Auditor** valida la calidad de esos datos.
- **Crítico** puede evaluar la calidad de las respuestas de Sabio.

Sabio no indexa. Indexa no responde preguntas. Cada uno hace su parte.
