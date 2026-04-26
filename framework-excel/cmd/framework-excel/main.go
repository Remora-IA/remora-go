package main

import (
	"fmt"

	"github.com/alcless/framework-excel/internal/framework-excel"
)

func main() {
	fmt.Println("Framework Excel - Conector de archivos Excel")
	fmt.Println("============================================")
	
	client := frameworkexcel.NewWithTrace("excel-cli")
	defer client.Flush()
	
	// Demostrar metodos disponibles
	fmt.Println("\nMetodos disponibles:")
	fmt.Println("  - ReadExcel(filePath string)")
	fmt.Println("  - ReadSheet(filePath, sheetName string)")
	fmt.Println("  - GetCell(filePath, sheetName, cell string)")
	fmt.Println("  - SetCell(filePath, sheetName, cell, value string)")
	fmt.Println("  - WriteSheet(filePath, sheetName string, data [][]interface{})")
	fmt.Println("  - CreateExcel(filePath, sheetName string)")
	fmt.Println("  - AppendRow(filePath, sheetName string, values []interface{})")
	fmt.Println("  - GetSheets(filePath string)")
	fmt.Println("  - DeleteRow(filePath, sheetName string, row int)")
}