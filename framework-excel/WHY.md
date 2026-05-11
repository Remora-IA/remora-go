# WHY - Framework Excel

Excel existe porque la mitad de los procesos del mundo real viven en
planillas.

Excel lee, escribe, modifica y crea archivos .xlsx desde Go. Cualquier
framework que necesite procesar una planilla importa el cliente de Excel.

## Problema Que Resuelve

Sin Excel, cada framework reimplementa su propia lectura de archivos. Excel
centraliza el acceso a planillas con una API limpia: ReadExcel, SetCell,
AppendRow, GetSheets.

## Relación Con Otros Frameworks

- **Indexa** puede usar Excel para importar datos desde planillas.
- **Sabio** puede consultar datos que vinieron de un Excel procesado.
- **Auditor** puede auditar datos que originalmente estaban en Excel.

Excel no decide qué hacer con los datos. Solo los lee y escribe.
