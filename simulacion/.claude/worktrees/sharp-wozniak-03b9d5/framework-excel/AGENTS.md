# AGENTS.md

## Integracion con otros frameworks

Framework Excel puede ser usado junto con otros frameworks para automatizar tareas que involucren datos en spreadsheets.

## Integracion tipica

```
[Framework Input] → [Framework Excel] → [Framework Output]
```

## Ejemplo: Procesar datos de ventas

1. **Recibir archivo**: Otro framework pasa la ruta del archivo
2. **Procesar con Excel**: Leer datos, calcular totales
3. **Devolver resultado**: El cliente procesa los datos

## Llamadas desde otros frameworks

```go
// Desde cualquier framework
import "github.com/alcless/framework-excel/internal/framework-excel"

client := frameworkexcel.New()

// Leer datos
data, err := client.ReadExcel(filePath)

// Procesar
for _, row := range data["Hoja1"] {
    // hacer algo con cada fila
}

// Guardar cambios
client.SetCell(filePath, "Hoja1", "B10", "total calculado")
```

## Estructura de datos

Los datos se devuelven como:
- `map[string][][]string` para archivos completos
- `[][]string` para hojas individuales

Cada fila es un slice de strings, una celda por posicion.