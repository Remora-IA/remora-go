package internal

import (
	"os"
	"path/filepath"
	"strings"
)

// SecurityLayer representa una capa de validación de seguridad (Axioma 4)
type SecurityLayer int

const (
	LayerAPIKey SecurityLayer = iota + 1
	LayerWhitelist
	LayerPathSanitization
	LayerNoShellExecution
	LayerNoDestructive
)

// ValidateSecurity ejecuta todas las capas de seguridad (Axioma 4 - Defense in Depth)
func ValidateSecurity(cmd string, args []string, baseDir string) (bool, string) {
	// Capa 4.2: Whitelist de comandos (si hay comando)
	if cmd != "" {
		if !IsCommandAllowed(cmd) {
			return false, "command not allowed: " + cmd
		}

		// Capa 4.5: Rechazo de comandos destructivos
		if IsDestructiveCommand(cmd) {
			return false, "destructive command rejected: " + cmd
		}
	}

	// Capa 4.3: Sanitización de paths
	for _, arg := range args {
		if !isPathSafe(arg, baseDir) {
			return false, "path not safe: " + sanitizeForError(arg)
		}
	}

	// Capa 4.4: Verificar que no se usa shell (manejado en exec.go con exec.Command)

	return true, ""
}

// ValidatePath valida solo un path (para métodos de archivo, Axioma 4.3)
func ValidatePath(path string, baseDir string) (bool, string) {
	if !isPathSafe(path, baseDir) {
		return false, "path not safe: " + sanitizeForError(path)
	}
	return true, ""
}

// isPathSafe verifica que un path sea seguro (Axioma 4.3)
// Rechaza: .., symlinks, escapes fuera de BASE_DIR
func isPathSafe(path string, baseDir string) bool {
	// Rechazar paths vacíos
	if path == "" {
		return true // Los args vacíos son válidos
	}

	// Rechazar .. (directory traversal)
	if strings.Contains(path, "..") {
		return false
	}

	// Rechazar patrones de escape shell
	if strings.Contains(path, "|") ||
		strings.Contains(path, ";") ||
		strings.Contains(path, "`") ||
		strings.Contains(path, "$(") ||
		strings.Contains(path, "&&") ||
		strings.Contains(path, "||") ||
		strings.Contains(path, ">") ||
		strings.Contains(path, "<") ||
		strings.Contains(path, "\n") ||
		strings.Contains(path, "\r") {
		return false
	}

	// Si no es un path absoluto, es seguro (relativo al cwd)
	if !filepath.IsAbs(path) {
		return true
	}

	// Verificar que el path absoluto está dentro de BASE_DIR
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Resolver symlinks
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// Si no es un symlink o no existe, verificamos el path directo
		resolved = absPath
	}

	// Verificar que está dentro de BASE_DIR
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}

	// El path seguro debe estar dentro de baseDir
	rel, err := filepath.Rel(absBaseDir, resolved)
	if err != nil {
		return false
	}

	// Si el relative path empieza con .., está fuera
	if strings.HasPrefix(rel, "..") {
		return false
	}

	// Verificar que no haya symlinks que escapen
	if !strings.HasPrefix(resolved, absBaseDir) {
		return false
	}

	return true
}

// ValidateBaseDir verifica que BASE_DIR exista y sea un directorio (Axioma 4.3)
func ValidateBaseDir(baseDir string) (bool, string) {
	info, err := os.Stat(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "base_dir does not exist: " + baseDir
		}
		return false, "base_dir error: " + err.Error()
	}
	if !info.IsDir() {
		return false, "base_dir is not a directory: " + baseDir
	}
	return true, ""
}

// sanitizeForError sanitiza un path para no incluir información sensible en errores
func sanitizeForError(path string) string {
	if len(path) > 50 {
		return path[:50] + "..."
	}
	return path
}
