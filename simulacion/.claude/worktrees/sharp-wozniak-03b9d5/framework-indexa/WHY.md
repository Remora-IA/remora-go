# WHY - Framework Indexa

Indexa existe porque un dump crudo de una API no es consultable.

Indexa toma un JSON grande de una fuente externa, lo estructura, lo indexa y
lo persiste en un store que Sabio puede consultar con SQL.

## Problema Que Resuelve

Sin Indexa, cada consulta sobre datos externos requiere parsear el dump
completo. Indexa hace esa transformación una vez y deja los datos listos
para consulta rápida.

## Relación Con Otros Frameworks

- **Sabio** consulta los datos que Indexa indexó.
- **Auditor** puede auditar la calidad de esos datos después de indexar.
- **Excel** puede ser la fuente original que Indexa procesa.

Indexa no responde preguntas. Sabio no indexa. Cada uno hace su parte.
