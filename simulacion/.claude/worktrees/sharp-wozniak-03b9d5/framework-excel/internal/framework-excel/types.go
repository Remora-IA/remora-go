package frameworkexcel

import "time"

// ExcelData representa los datos leidos de un archivo Excel.
type ExcelData struct {
	Sheets map[string][][]string
}

// SheetData representa los datos de una hoja.
type SheetData struct {
	Name  string
	Rows  [][]string
	Count int
}

// Cell representa una celda individual.
type Cell struct {
	Row    int
	Col    int
	Value  string
}

// Row representa una fila de datos.
type Row struct {
	Index int
	Cells []string
}

// Spec define la estructura del framework.
type Spec struct {
	Name        string
	Role        string
	Description string
	CreatedAt   string
}

// Result representa el resultado de una operacion.
type Result struct {
	Success   bool
	Data      interface{}
	Error     string
	Timestamp string
}

// ReadResult representa el resultado de leer un Excel.
type ReadResult struct {
	FilePath   string
	Sheets     []string
	TotalRows  int
	TotalCells int
	Timestamp  time.Time
}

// WriteResult representa el resultado de escribir en un Excel.
type WriteResult struct {
	FilePath  string
	Operation string
	Affected  int
	Timestamp time.Time
}