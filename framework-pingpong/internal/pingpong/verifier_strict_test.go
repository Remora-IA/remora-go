package pingpong

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestForEmptyBodyRejected: un loop con cuerpo vacío no debe pasar el paso "for".
func TestForEmptyBodyRejected(t *testing.T) {
	src := `package main
func main() {}
func twoSum(nums []int, target int) []int {
	for i := 0; i < len(nums); i++ {
	}
	return nil
}
`
	rep := mustVerify(t, src, Step{ID: 5, Instruction: "Crear loop for que recorra el array"})
	if rep.Passed {
		t.Fatalf("expected loop con cuerpo vacío FALLE, pero passed=true. missing=%q", rep.Missing)
	}
	if rep.Missing == "" {
		t.Fatalf("expected missing message, got empty")
	}
}

// TestForWithBodyAccepted: un loop con al menos un statement debe pasar.
func TestForWithBodyAccepted(t *testing.T) {
	src := `package main
func main() {}
func twoSum(nums []int, target int) []int {
	for i := 0; i < len(nums); i++ {
		_ = nums[i]
	}
	return nil
}
`
	rep := mustVerify(t, src, Step{ID: 5, Instruction: "Crear loop for que recorra el array"})
	if !rep.Passed {
		t.Fatalf("expected loop con cuerpo PASE, pero passed=false. missing=%q", rep.Missing)
	}
}

// TestReturnHardcodedRejected: return de literal compuesto sin identifiers debe fallar.
func TestReturnHardcodedRejected(t *testing.T) {
	src := `package main
func main() {}
func twoSum(nums []int, target int) []int {
	return []int{1, 2}
}
`
	rep := mustVerify(t, src, Step{ID: 6, Instruction: "Implementar return de los índices encontrados"})
	if rep.Passed {
		t.Fatalf("expected return hardcoded FALLE, pero passed=true. evidence=%v", rep.Evidence)
	}
}

// TestReturnLiteralBoolRejected: `return true` también es trivial.
func TestReturnLiteralBoolRejected(t *testing.T) {
	src := `package main
func isPalindrome(x int) bool {
	return true
}
`
	rep := mustVerify(t, src, Step{ID: 7, Instruction: "Implementar return de la comparación"})
	if rep.Passed {
		t.Fatalf("expected return true FALLE, pero passed=true")
	}
}

// TestReturnUsingVariableAccepted: return que referencia una variable local debe pasar.
func TestReturnUsingVariableAccepted(t *testing.T) {
	src := `package main
func twoSum(nums []int, target int) []int {
	var ans []int
	return ans
}
`
	rep := mustVerify(t, src, Step{ID: 6, Instruction: "Implementar return de los índices encontrados"})
	if !rep.Passed {
		t.Fatalf("expected return ans PASE, pero passed=false. missing=%q", rep.Missing)
	}
}

// TestReturnUsingParamAccepted: return que referencia un parámetro de la función.
func TestReturnUsingParamAccepted(t *testing.T) {
	src := `package main
func isPalindrome(x int) bool {
	return x == x
}
`
	rep := mustVerify(t, src, Step{ID: 7, Instruction: "Implementar return de la comparación"})
	if !rep.Passed {
		t.Fatalf("expected return x==x PASE, pero passed=false. missing=%q", rep.Missing)
	}
}

// TestReturnNilRejected: `return nil` solo no es implementación.
func TestReturnNilRejected(t *testing.T) {
	src := `package main
func twoSum(nums []int, target int) []int {
	return nil
}
`
	rep := mustVerify(t, src, Step{ID: 6, Instruction: "Implementar return"})
	if rep.Passed {
		t.Fatalf("expected return nil FALLE, pero passed=true")
	}
}

// TestTypeCheckCatchesUndefinedVar: `suma := suma + 1` usa suma antes de declararla.
func TestTypeCheckCatchesUndefinedVar(t *testing.T) {
	src := `package main
func twoSum(nums []int, target int) []int {
	for i := 0; i < len(nums); i++ {
		suma := suma + 1
		_ = suma
	}
	return nums
}
`
	rep := mustVerify(t, src, Step{ID: 5, Instruction: "Crear loop for que recorra el array"})
	if rep.Passed {
		t.Fatalf("expected type-check reject, but passed. missing=%q", rep.Missing)
	}
	if !strings.Contains(rep.Missing, "suma") {
		t.Errorf("expected error about 'suma', got %q", rep.Missing)
	}
}

// TestTypeCheckCatchesReturnOutOfScope: `return []int{i, suma}` con i/suma fuera de scope.
func TestTypeCheckCatchesReturnOutOfScope(t *testing.T) {
	src := `package main
func twoSum(nums []int, target int) []int {
	for i := 0; i < len(nums); i++ {
		_ = nums[i]
	}
	return []int{i, 0}
}
`
	rep := mustVerify(t, src, Step{ID: 6, Instruction: "Implementar return"})
	if rep.Passed {
		t.Fatalf("expected type-check reject de 'i' fuera de scope, but passed. missing=%q", rep.Missing)
	}
}

// TestTypeCheckCatchesMissingReturnType: función sin tipo de retorno pero con return.
func TestTypeCheckCatchesMissingReturnType(t *testing.T) {
	src := `package main
func twoSum(nums []int) {
	return []int{1, 2}
}
`
	rep := mustVerify(t, src, Step{ID: 6, Instruction: "Implementar return"})
	if rep.Passed {
		t.Fatalf("expected reject: función sin tipo de retorno, but passed. missing=%q", rep.Missing)
	}
}

// TestTypeCheckAcceptsValidCode: código válido debe pasar.
func TestTypeCheckAcceptsValidCode(t *testing.T) {
	src := `package main
func isPalindrome(x int) bool {
	original := x
	rev := 0
	for x > 0 {
		rev = rev*10 + x%10
		x = x / 10
	}
	return original == rev
}
`
	rep := mustVerify(t, src, Step{ID: 7, Instruction: "Implementar return de la comparación"})
	if !rep.Passed {
		t.Fatalf("código válido debe pasar, pero missing=%q", rep.Missing)
	}
}

// mustVerify escribe src a un archivo temporal y corre VerifyFile contra el step.
func mustVerify(t *testing.T, src string, step Step) *VerifyReport {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	rep, err := VerifyFile(path, step)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}
