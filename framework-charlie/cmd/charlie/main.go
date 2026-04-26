package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Charlie CLI")
		fmt.Println("  ./charlie status      # Ver estado")
		fmt.Println("  ./charlie classify    # Clasificar cambios")
		fmt.Println("  ./charlie next-version # Ver siguiente versión")
		os.Exit(0)
	}

	dir := "/Users/alcless_a1234_cursor/remora-go"
	command := os.Args[1]

	switch command {
	case "status":
		runStatus(dir)
	case "classify":
		runClassify(dir)
	case "next-version":
		runNextVersion(dir)
	default:
		fmt.Println("Comando desconocido:", command)
	}
}

// Archivos a ignorar en clasificación
var ignoredPatterns = []string{
	".DS_Store",
	"charlie",       // binario compilado
	"examples/",     // ejemplos no son código principal
	"temp/",         // archivos temporales
	"cmd/",          // ejecutables compilados
	"go.work",       // archivos de workspace
	"go.work.sum",
}

func runStatus(dir string) {
	status := execCmd(dir, "git", "status", "--porcelain")
	tag := execCmd(dir, "git", "describe", "--tags", "--abbrev=0")

	// Filtrar solo archivos relevantes
	files := filterRelevant(status)

	if len(files) == 0 {
		fmt.Println("✅ Repo limpio, no hay cambios pendientes")
		return
	}

	fmt.Println("=== CHARLIE ===")
	fmt.Printf("archivos: %d\n", len(files))
	fmt.Printf("tag: %s\n\n", tag)

	// Agrupar por tipo
	grouped := classifyFiles(files)

	if len(grouped["feat"]) > 0 {
		fmt.Printf("feat: %d archivos\n", len(grouped["feat"]))
	}
	if len(grouped["docs"]) > 0 {
		fmt.Printf("docs: %d archivos\n", len(grouped["docs"]))
	}
	if len(grouped["test"]) > 0 {
		fmt.Printf("test: %d archivos\n", len(grouped["test"]))
	}
	if len(grouped["chore"]) > 0 {
		fmt.Printf("chore: %d archivos\n", len(grouped["chore"]))
	}

	// Próximo paso
	scope := detectScope(files)
	fmt.Printf("\nscope: %s\n", scope)
}

func runClassify(dir string) {
	status := execCmd(dir, "git", "status", "--porcelain")
	files := filterRelevant(status)

	if len(files) == 0 {
		fmt.Println("✅ Repo limpio, no hay cambios pendientes")
		return
	}

	fmt.Println("=== CLASIFICACIÓN ===\n")

	grouped := classifyFiles(files)
	scope := detectScope(files)

	// Mostrar por tipo
	for t, f := range grouped {
		if len(f) > 0 {
			fmt.Printf("%s:\n", strings.ToUpper(t))
			for _, file := range f {
				fmt.Printf("  - %s\n", file)
			}
			fmt.Println()
		}
	}

	// Proponer commit
	fmt.Println("=== PROPUESTA ===")
	if len(grouped["feat"]) > 0 {
		fmt.Printf("commit: feat(%s): agregar funcionalidad\n", scope)
	} else if len(grouped["docs"]) > 0 {
		fmt.Printf("commit: docs(%s): actualizar documentación\n", scope)
	} else {
		fmt.Printf("commit: chore(%s): tareas de mantenimiento\n", scope)
	}
}

func runNextVersion(dir string) {
	tag := execCmd(dir, "git", "describe", "--tags", "--abbrev=0")
	if tag == "" || strings.TrimSpace(tag) == "no-tags" {
		fmt.Println("versión: v0.1.0 (primer tag)")
		return
	}

	version := strings.TrimSpace(tag)
	fmt.Printf("versión actual: %s\n", version)
	fmt.Println("siguiente: patch bump → " + nextPatch(version))
}

func filterRelevant(status string) []string {
	var result []string
	lines := strings.Split(status, "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		file := strings.TrimSpace(parts[1])

		// Ignorar patrones
		skip := false
		for _, pattern := range ignoredPatterns {
			if strings.Contains(file, pattern) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		result = append(result, file)
	}
	return result
}

func classifyFiles(files []string) map[string][]string {
	grouped := map[string][]string{
		"feat":  {},
		"docs":  {},
		"test":  {},
		"chore": {},
	}

	for _, file := range files {
		var t string
		switch {
		case strings.HasSuffix(file, "_test.go"):
			t = "test"
		case strings.HasSuffix(file, ".md"):
			t = "docs"
		case strings.HasSuffix(file, ".go"):
			t = "feat"
		case strings.HasSuffix(file, ".mod") || strings.HasSuffix(file, ".sum") || strings.HasSuffix(file, ".json"):
			t = "chore"
		default:
			t = "chore"
		}
		grouped[t] = append(grouped[t], file)
	}
	return grouped
}

func detectScope(files []string) string {
	if len(files) == 0 {
		return ""
	}
	first := files[0]

	// Detectar scope desde la ruta
	if strings.Contains(first, "framework-alfa") {
		return "alfa"
	}
	if strings.Contains(first, "framework-echo") {
		return "echo"
	}
	if strings.Contains(first, "framework-bravo") {
		return "bravo"
	}
	if strings.Contains(first, "framework-charlie") {
		return "charlie"
	}
	if strings.Contains(first, "remora-flujo") {
		return "flujo"
	}
	return ""
}

func nextPatch(version string) string {
	v := strings.TrimPrefix(version, "v")
	parts := strings.Split(v, ".")
	if len(parts) < 3 {
		return "0.0.1"
	}
	var maj, min, pat int
	fmt.Sscanf(parts[0], "%d", &maj)
	fmt.Sscanf(parts[1], "%d", &min)
	fmt.Sscanf(parts[2], "%d", &pat)
	return fmt.Sprintf("%d.%d.%d", maj, min, pat+1)
}

func execCmd(dir, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}