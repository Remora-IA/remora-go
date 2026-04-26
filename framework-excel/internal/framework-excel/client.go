package frameworkexcel

import (
	"github.com/xuri/excelize/v2"

	"github.com/alcless/framework-excel/internal/paladin"
)

// Client es el cliente principal del framework Excel.
type Client struct {
	trace *paladin.Trace
	ctx   *paladin.Context
}

// New crea un nuevo cliente.
func New() *Client {
	return &Client{}
}

// NewWithTrace crea un cliente con tracing activo.
func NewWithTrace(name string) *Client {
	trace := paladin.NewTrace(name)
	ctx := trace.Start()
	return &Client{trace: trace, ctx: ctx}
}

// Flush guarda el trace actual.
func (c *Client) Flush() {
	if c.trace != nil {
		c.trace.Flush()
	}
}

// ReadExcel abre un archivo Excel y devuelve los datos de todas las hojas.
//	filePath: ruta al archivo .xlsx
func (c *Client) ReadExcel(filePath string) (map[string][][]string, error) {
	childCtx := c.ctx.Child("ReadExcel")
	defer childCtx.End()
	childCtx.Var("filePath", filePath)

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		childCtx.Error(err.Error())
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	result := make(map[string][][]string)

	for _, sheetName := range sheets {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			childCtx.Error("error leyendo hoja " + sheetName + ": " + err.Error())
			continue
		}
		result[sheetName] = rows
	}

	childCtx.Var("sheetsCount", len(result))
	childCtx.Decision("ReadExcel-completado", "Leo archivo Excel exitosamente")
	return result, nil
}

// ReadSheet abre una hoja especifica del archivo Excel.
//	filePath: ruta al archivo .xlsx
//	sheetName: nombre de la hoja a leer
func (c *Client) ReadSheet(filePath, sheetName string) ([][]string, error) {
	childCtx := c.ctx.Child("ReadSheet")
	defer childCtx.End()
	childCtx.Var("filePath", filePath)
	childCtx.Var("sheetName", sheetName)

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		childCtx.Error(err.Error())
		return nil, err
	}
	defer f.Close()

	rows, err := f.GetRows(sheetName)
	if err != nil {
		childCtx.Error(err.Error())
		return nil, err
	}

	childCtx.Var("rowsCount", len(rows))
	childCtx.Decision("ReadSheet-completado", "Lei hoja especifica exitosamente")
	return rows, nil
}

// GetCell obtiene el valor de una celda especifica.
//	filePath: ruta al archivo .xlsx
//	sheetName: nombre de la hoja
//	cell: referencia de celda (ej: "A1")
func (c *Client) GetCell(filePath, sheetName, cell string) (string, error) {
	childCtx := c.ctx.Child("GetCell")
	defer childCtx.End()
	childCtx.Var("filePath", filePath)
	childCtx.Var("sheetName", sheetName)
	childCtx.Var("cell", cell)

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		childCtx.Error(err.Error())
		return "", err
	}
	defer f.Close()

	value, err := f.GetCellValue(sheetName, cell)
	if err != nil {
		childCtx.Error(err.Error())
		return "", err
	}

	childCtx.Var("cellValue", value)
	childCtx.Decision("GetCell-completado", "Obtuve valor de celda exitosamente")
	return value, nil
}

// SetCell escribe un valor en una celda especifica.
//	filePath: ruta al archivo .xlsx
//	sheetName: nombre de la hoja
//	cell: referencia de celda (ej: "A1")
//	value: valor a escribir
func (c *Client) SetCell(filePath, sheetName, cell, value string) error {
	childCtx := c.ctx.Child("SetCell")
	defer childCtx.End()
	childCtx.Var("filePath", filePath)
	childCtx.Var("sheetName", sheetName)
	childCtx.Var("cell", cell)
	childCtx.Var("value", value)

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		childCtx.Error(err.Error())
		return err
	}

	err = f.SetCellValue(sheetName, cell, value)
	if err != nil {
		childCtx.Error(err.Error())
		f.Close()
		return err
	}

	err = f.Save()
	if err != nil {
		childCtx.Error(err.Error())
		f.Close()
		return err
	}

	f.Close()
	childCtx.Decision("SetCell-completado", "Escribi valor en celda exitosamente")
	return nil
}

// WriteSheet escribe datos completos en una hoja.
//	filePath: ruta al archivo .xlsx
//	sheetName: nombre de la hoja
//	data: matriz de datos (cada slice es una fila)
func (c *Client) WriteSheet(filePath, sheetName string, data [][]interface{}) error {
	childCtx := c.ctx.Child("WriteSheet")
	defer childCtx.End()
	childCtx.Var("filePath", filePath)
	childCtx.Var("sheetName", sheetName)
	childCtx.Var("rowsCount", len(data))

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		childCtx.Error(err.Error())
		return err
	}

	for rowIdx, row := range data {
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			if err := f.SetCellValue(sheetName, cell, value); err != nil {
				childCtx.Error(err.Error())
				f.Close()
				return err
			}
		}
	}

	err = f.Save()
	if err != nil {
		childCtx.Error(err.Error())
		f.Close()
		return err
	}

	f.Close()
	childCtx.Decision("WriteSheet-completado", "Escribi datos en hoja exitosamente")
	return nil
}

// CreateExcel crea un nuevo archivo Excel.
//	filePath: ruta donde guardar el archivo .xlsx
//	sheetName: nombre de la hoja inicial
func (c *Client) CreateExcel(filePath, sheetName string) error {
	childCtx := c.ctx.Child("CreateExcel")
	defer childCtx.End()
	childCtx.Var("filePath", filePath)
	childCtx.Var("sheetName", sheetName)

	f := excelize.NewFile()
	if sheetName != "" {
		f.SetSheetName("Sheet1", sheetName)
	}

	err := f.SaveAs(filePath)
	if err != nil {
		childCtx.Error(err.Error())
		return err
	}

	childCtx.Decision("CreateExcel-completado", "Archivo Excel creado exitosamente")
	return nil
}

// AppendRow agrega una fila al final de una hoja.
//	filePath: ruta al archivo .xlsx
//	sheetName: nombre de la hoja
//	values: valores a agregar en cada columna
func (c *Client) AppendRow(filePath, sheetName string, values []interface{}) error {
	childCtx := c.ctx.Child("AppendRow")
	defer childCtx.End()
	childCtx.Var("filePath", filePath)
	childCtx.Var("sheetName", sheetName)
	childCtx.Var("valuesCount", len(values))

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		childCtx.Error(err.Error())
		return err
	}

	rows, _ := f.GetRows(sheetName)
	maxRow := len(rows) + 1

	for colIdx, value := range values {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, maxRow)
		if err := f.SetCellValue(sheetName, cell, value); err != nil {
			childCtx.Error(err.Error())
			f.Close()
			return err
		}
	}

	err = f.Save()
	if err != nil {
		childCtx.Error(err.Error())
		f.Close()
		return err
	}

	f.Close()
	childCtx.Decision("AppendRow-completado", "Fila agregada exitosamente")
	return nil
}

// GetSheets devuelve la lista de nombres de hojas en el archivo.
//	filePath: ruta al archivo .xlsx
func (c *Client) GetSheets(filePath string) ([]string, error) {
	childCtx := c.ctx.Child("GetSheets")
	defer childCtx.End()
	childCtx.Var("filePath", filePath)

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		childCtx.Error(err.Error())
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	childCtx.Var("sheetsCount", len(sheets))
	childCtx.Decision("GetSheets-completado", "Liste hojas exitosamente")
	return sheets, nil
}

// DeleteRow elimina una fila especifica.
//	filePath: ruta al archivo .xlsx
//	sheetName: nombre de la hoja
//	row: numero de fila a eliminar (1-indexed)
func (c *Client) DeleteRow(filePath, sheetName string, row int) error {
	childCtx := c.ctx.Child("DeleteRow")
	defer childCtx.End()
	childCtx.Var("filePath", filePath)
	childCtx.Var("sheetName", sheetName)
	childCtx.Var("row", row)

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		childCtx.Error(err.Error())
		return err
	}

	err = f.RemoveRow(sheetName, row)
	if err != nil {
		childCtx.Error(err.Error())
		f.Close()
		return err
	}

	err = f.Save()
	if err != nil {
		childCtx.Error(err.Error())
		f.Close()
		return err
	}

	f.Close()
	childCtx.Decision("DeleteRow-completado", "Fila eliminada exitosamente")
	return nil
}