package charlie

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CleanTracesResult resume la operacion de limpieza.
type CleanTracesResult struct {
	Mode    string   // "plan" | "apply"
	Matched []string // archivos que matchearon los patrones
	Deleted []string // archivos efectivamente borrados (solo en apply)
	Errors  []string // errores no fatales por archivo
}

// cleanPatterns son los unicos patrones que clean-traces puede tocar.
// Conservador a proposito: solo logs y junk regenerables. NUNCA state,
// secrets, applied.jsonl, sessions, ni databases.
var cleanPatterns = []func(name, full string) bool{
	func(name, full string) bool { return strings.HasPrefix(name, "trace_pal_") && strings.HasSuffix(name, ".json") },
	func(name, full string) bool { return strings.HasPrefix(name, "trace_gf_") && strings.HasSuffix(name, ".json") },
	func(name, full string) bool { return name == ".DS_Store" },
}

// cleanSkipDirs son directorios que no se recorren bajo ninguna circunstancia.
var cleanSkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vault_data":   true, // secrets encriptados
	"sessions":     true, // logs de conversaciones (auditables)
}

// matchesCleanPattern devuelve true si el archivo corresponde a un patron seguro.
func matchesCleanPattern(name, full string) bool {
	for _, p := range cleanPatterns {
		if p(name, full) {
			return true
		}
	}
	return false
}

// CleanTraces escanea el repo desde root y devuelve los archivos que matchean
// los patrones de limpieza. Si apply=true, los borra.
func CleanTraces(root string, apply bool) (*CleanTracesResult, error) {
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("no pude resolver root: %v", err)
	}

	res := &CleanTracesResult{Mode: "plan"}
	if apply {
		res.Mode = "apply"
	}

	walkErr := filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors silenciosamente
		}
		if d.IsDir() {
			if cleanSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if matchesCleanPattern(d.Name(), path) {
			rel, _ := filepath.Rel(abs, path)
			res.Matched = append(res.Matched, rel)
			if apply {
				if err := os.Remove(path); err != nil {
					res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", rel, err))
				} else {
					res.Deleted = append(res.Deleted, rel)
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		return res, walkErr
	}
	sort.Strings(res.Matched)
	sort.Strings(res.Deleted)
	sort.Strings(res.Errors)
	return res, nil
}

// FormatCleanTraces produce el output humano-legible.
func FormatCleanTraces(res *CleanTracesResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "=== CHARLIE CLEAN-TRACES ===")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "modo: %s\n", res.Mode)
	fmt.Fprintf(&b, "archivos matched: %d\n", len(res.Matched))

	if res.Mode == "apply" {
		fmt.Fprintf(&b, "archivos borrados: %d\n", len(res.Deleted))
		if len(res.Errors) > 0 {
			fmt.Fprintf(&b, "errores: %d\n", len(res.Errors))
		}
	}

	if len(res.Matched) == 0 {
		fmt.Fprintln(&b, "\n✅ Nada que limpiar.")
		return b.String()
	}

	fmt.Fprintln(&b, "\nArchivos:")
	max := 50
	for i, f := range res.Matched {
		if i >= max {
			fmt.Fprintf(&b, "  ... (+%d mas)\n", len(res.Matched)-max)
			break
		}
		fmt.Fprintf(&b, "  - %s\n", f)
	}

	if res.Mode == "plan" {
		fmt.Fprintln(&b, "\nsiguiente: charlie clean-traces --apply")
	}
	if len(res.Errors) > 0 {
		fmt.Fprintln(&b, "\nErrores:")
		for _, e := range res.Errors {
			fmt.Fprintf(&b, "  - %s\n", e)
		}
	}
	return b.String()
}
