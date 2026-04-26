package charlie

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestClassifyChangesDetectsGoFiles(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Crear archivo .go
	writeFile(t, dir, "internal/alfa/compile.go", "package alfa\nfunc Compile() {}")

	// Commit para que git lo detecte
	gitAdd(t, dir, "internal/alfa/compile.go")
	gitCommit(t, dir, "feat: initial")

	// Ahora modificar el archivo
	writeFile(t, dir, "internal/alfa/compile.go", "package alfa\nfunc Compile() {}\nfunc Extra() {}")

	changes, err := ClassifyChanges(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(changes) == 0 {
		t.Fatal("expected at least one change")
	}

	// Verificar que al menos uno sea feat o refactor
	found := false
	for _, c := range changes {
		if c.ChangeType == TypeFeat || c.ChangeType == TypeRefactor {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected feat or refactor change, got: %v", changes)
	}
}

func TestClassifyChangesDetectsMarkdown(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, dir, "README.md", "# Test\nContent")

	gitAdd(t, dir, "README.md")
	gitCommit(t, dir, "docs: initial")

	writeFile(t, dir, "README.md", "# Test\nContent\nMore")

	changes, err := ClassifyChanges(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range changes {
		if c.FilePath == "README.md" && c.ChangeType != TypeDocs {
			t.Errorf("expected docs for .md file, got: %s", c.ChangeType)
		}
	}
}

func TestClassifyChangesDetectsTestFiles(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, dir, "internal/alfa/compile_test.go", "package alfa\nfunc TestX(t *testing.T) {}")

	gitAdd(t, dir, "internal/alfa/compile_test.go")
	gitCommit(t, dir, "test: initial")

	writeFile(t, dir, "internal/alfa/compile_test.go", "package alfa\nfunc TestX(t *testing.T) {}\nfunc TestY(t *testing.T) {}")

	changes, err := ClassifyChanges(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range changes {
		if c.FilePath == "internal/alfa/compile_test.go" && c.ChangeType != TypeTest {
			t.Errorf("expected test for _test.go file, got: %s", c.ChangeType)
		}
	}
}

func TestCalculateNextVersionPatch(t *testing.T) {
	result := CalculateNextVersion("v0.1.0", []Change{
		{ChangeType: TypeFix, Message: "fix bug"},
	})

	if result.NewVersion != "v0.1.1" {
		t.Errorf("expected v0.1.1, got: %s", result.NewVersion)
	}
	if result.BumpType != "patch" {
		t.Errorf("expected patch bump, got: %s", result.BumpType)
	}
}

func TestCalculateNextVersionMinorOnFeat(t *testing.T) {
	result := CalculateNextVersion("v0.1.0", []Change{
		{ChangeType: TypeFeat, Message: "new feature"},
	})

	if result.NewVersion != "v0.2.0" {
		t.Errorf("expected v0.2.0, got: %s", result.NewVersion)
	}
	if result.BumpType != "minor (nuevas funcionalidades)" {
		// también puede ser "minor"
		if result.BumpType != "minor" {
			t.Errorf("expected minor bump, got: %s", result.BumpType)
		}
	}
}

func TestCalculateNextVersionMajorOnBreaking(t *testing.T) {
	result := CalculateNextVersion("v0.1.0", []Change{
		{ChangeType: TypeBreaking, Message: "breaking change", IsBreaking: true},
	})

	if result.NewVersion != "v1.0.0" {
		t.Errorf("expected v1.0.0, got: %s", result.NewVersion)
	}
	if result.BumpType != "major (breaking change detectado)" {
		t.Errorf("expected major bump, got: %s", result.BumpType)
	}
}

func TestValidateCommitMessageValid(t *testing.T) {
	valid, msg := ValidateCommitMessage("feat(alfa): agregar nueva función")
	if !valid {
		t.Errorf("expected valid, got: %s", msg)
	}
}

func TestValidateCommitMessageInvalid(t *testing.T) {
	valid, _ := ValidateCommitMessage("Agregar algo")
	if valid {
		t.Error("expected invalid for non-conventional message")
	}
}

func TestValidateCommitMessageTooLong(t *testing.T) {
	longDesc := "Esta es una descripción muy larga que supera los setenta y dos caracteres permitidos en un mensaje de commit convencional"
	valid, _ := ValidateCommitMessage("feat: " + longDesc)
	if valid {
		t.Error("expected invalid for message over 72 chars")
	}
}

func TestDetectScope(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"framework-alfa/internal/alfa/compile.go", "alfa"},
		{"framework-echo/cli/main.go", "echo"},
		{"framework-bravo/internal/bravo/rules.go", "bravo"},
		{"framework-charlie/charlie.go", "charlie"},
		{"remora-flujo/main.go", "flujo"},
		{"README.md", ""},
	}

	for _, tt := range tests {
		result := detectScope(tt.path)
		if result != tt.expected {
			t.Errorf("detectScope(%s) = %s, expected %s", tt.path, result, tt.expected)
		}
	}
}

func TestGetStatus(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Crear archivo y commit inicial
	writeFile(t, dir, "README.md", "# Test")
	gitAdd(t, dir, "README.md")
	gitCommit(t, dir, "chore: initial")

	// Crear tag
	gitTag(t, dir, "v0.1.0")

	// Agregar más archivos
	writeFile(t, dir, "CHANGELOG.md", "# Changelog")
	gitAdd(t, dir, "CHANGELOG.md")
	gitCommit(t, dir, "docs: add changelog")

	status, err := GetStatus(dir)
	if err != nil {
		t.Fatal(err)
	}

	if status.LastTag != "v0.1.0" {
		t.Errorf("expected last tag v0.1.0, got: %s", status.LastTag)
	}

	if status.CommitsSinceTag != 1 {
		t.Errorf("expected 1 commit since tag, got: %d", status.CommitsSinceTag)
	}
}

func TestFormatStatus(t *testing.T) {
	status := &Status{
		CurrentVersion:    "v0.1.0",
		LastTag:           "v0.1.0",
		CommitsSinceTag:   3,
		RepoDirty:         true,
		ChangelogExists:   true,
		HasUnreleased:     false,
	}

	changes := []Change{
		{FilePath: "alpha.go", ChangeType: TypeFeat, Scope: "alfa"},
		{FilePath: "README.md", ChangeType: TypeDocs},
	}

	output := FormatStatus(status, changes)

	if !contains(output, "v0.1.0") {
		t.Error("expected version in output")
	}
	if !contains(output, "3") {
		t.Error("expected commits count in output")
	}
}

func TestSuggestRelease(t *testing.T) {
	status := &Status{
		RepoDirty:         false,
		CommitsSinceTag:   5,
		LastTag:           "v0.1.0",
	}

	suggestion := SuggestRelease(status, nil)
	if !contains(suggestion, "Listo para release") {
		t.Errorf("expected release suggestion, got: %s", suggestion)
	}
}

func TestGenerateCommitMessage(t *testing.T) {
	changes := []Change{
		{FilePath: "framework-alfa/internal/alfa/compile.go", ChangeType: TypeFeat, Scope: "alfa"},
		{FilePath: "README.md", ChangeType: TypeDocs},
	}

	msg := GenerateCommitMessage(changes, true)

	if !contains(msg, "feat") {
		t.Errorf("expected feat in message, got: %s", msg)
	}
	if !contains(msg, "alfa") {
		t.Errorf("expected scope alfa in message, got: %s", msg)
	}
}

// Helper functions

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	cmd.Run()
}

func gitAdd(t *testing.T, dir, file string) {
	t.Helper()
	cmd := exec.Command("git", "add", file)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
}

func gitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	cmd := exec.Command("git", "commit", "-m", msg)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}
}

func gitTag(t *testing.T, dir, tag string) {
	t.Helper()
	cmd := exec.Command("git", "tag", tag)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git tag failed: %v", err)
	}
}

func writeFile(t *testing.T, dir, path, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, path)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte(content), 0644)
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstr(s, substr)))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}