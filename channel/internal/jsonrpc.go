package internal

// JSONRPCRequest representa un request JSON-RPC 2.0 (Axioma 6)
type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"` // Debe ser "2.0"
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
	ID     interface{}            `json:"id"`
}

// ValidateJSONRPC valida que el request cumpla JSON-RPC 2.0 (Axioma 6)
func ValidateJSONRPC(req *JSONRPCRequest) (bool, string) {
	// Verificar versión JSON-RPC
	if req.JSONRPC != "2.0" {
		return false, "invalid jsonrpc version: must be 2.0"
	}

	// Verificar que method sea un string no vacío
	if req.Method == "" {
		return false, "method is required"
	}

	// Verificar que params sea un objeto (map)
	if req.Params == nil {
		return false, "params must be an object"
	}

	return true, ""
}

// AllowedMethods son los 5 métodos únicos expuestos (Axioma 8)
var AllowedMethods = map[string]bool{
	"execute_command": true,
	"read_file":       true,
	"write_file":      true,
	"list_dir":        true,
	"http_get":        true,
}

// IsMethodAllowed verifica si el método está permitido (Axioma 8)
func IsMethodAllowed(method string) bool {
	return AllowedMethods[method]
}
