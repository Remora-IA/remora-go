# Framework Excel

Framework para conectar, leer y escribir archivos Excel.

## Instalación

```bash
go get github.com/xuri/excelize/v2
```

## Uso

```go
package main

import (
    "github.com/alcless/framework-excel/internal/framework-excel"
)

func main() {
    client := frameworkexcel.NewWithTrace("mi-app")
    defer client.Flush()
    
    // Leer archivo Excel completo
    data, err := client.ReadExcel("archivo.xlsx")
    
    // Leer hoja especifica
    rows, err := client.ReadSheet("archivo.xlsx", "Hoja1")
    
    // Obtener valor de celda
    value, err := client.GetCell("archivo.xlsx", "Hoja1", "A1")
    
    // Escribir en celda
    err = client.SetCell("archivo.xlsx", "Hoja1", "A1", "Nuevo valor")
    
    // Crear nuevo archivo
    err = client.CreateExcel("nuevo.xlsx", "Datos")
    
    // Agregar fila
    err = client.AppendRow("archivo.xlsx", "Hoja1", []interface{}{"valor1", "valor2"})
}
```

## Metodos

| Metodo | Descripcion |
|--------|-------------|
| `ReadExcel(filePath)` | Lee todas las hojas del archivo |
| `ReadSheet(filePath, sheetName)` | Lee una hoja especifica |
| `GetCell(filePath, sheetName, cell)` | Obtiene valor de celda |
| `SetCell(filePath, sheetName, cell, value)` | Escribe en celda |
| `WriteSheet(filePath, sheetName, data)` | Escribe matriz de datos |
| `CreateExcel(filePath, sheetName)` | Crea nuevo archivo |
| `AppendRow(filePath, sheetName, values)` | Agrega fila |
| `GetSheets(filePath)` | Lista hojas |
| `DeleteRow(filePath, sheetName, row)` | Elimina fila |