# WHY - Framework Auditor

Auditor existe porque los datos de un ERP nunca están limpios.

Campos vacíos, fechas inválidas, referencias huérfanas, duplicados silenciosos.
Nadie los ve hasta que algo falla en producción o un reporte sale mal.

Auditor corre checks deterministas sobre un dump del ERP y emite hallazgos
clasificados por severidad. No corrige nada. Solo detecta y explica.

## Problema Que Resuelve

Sin Auditor, la calidad de datos depende de que alguien revise manualmente o
de que un error en downstream lo delate. Auditor hace esa revisión antes de
que el error llegue al usuario final.

## Relación Con Otros Frameworks

- **Mecánico** consume los findings de Auditor para proponer correcciones.
- **Sabio** consulta datos que Auditor validó.
- **Foco** puede disparar un scan al inicio del día.

Auditor no corrige. Mecánico no detecta. Cada uno hace su parte.
