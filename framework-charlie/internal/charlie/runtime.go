package charlie

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	RepoRoot      string
	BackupRoot    string
	FrameworkRoot string
)

func init() {
	FrameworkRoot = detectFrameworkRoot()
	RepoRoot = mustResolveRepoRoot("")
	BackupRoot = deriveBackupRoot(RepoRoot)
}

func mustResolveRepoRoot(root string) string {
	resolved, err := resolveRepoRoot(root)
	if err != nil {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			abs, absErr := filepath.Abs(wd)
			if absErr == nil {
				return abs
			}
		}
		return "."
	}
	return resolved
}

func SetRepoRoot(root string) error {
	resolved, err := resolveRepoRoot(root)
	if err != nil {
		return err
	}
	RepoRoot = resolved
	BackupRoot = deriveBackupRoot(resolved)
	resetCharlieIgnoreCache()
	return nil
}

func CurrentRepoRoot() string {
	return RepoRoot
}

func CurrentFrameworkRoot() string {
	return frameworkDataRoot()
}

func frameworkDataRoot() string {
	if FrameworkRoot != "" {
		return FrameworkRoot
	}
	candidate := filepath.Join(RepoRoot, "framework-charlie")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	return RepoRoot
}

func resolveRepoRoot(root string) (string, error) {
	if root != "" {
		return resolveExplicitRepoRoot(root)
	}
	if envRoot := os.Getenv("CHARLIE_ROOT"); envRoot != "" {
		return resolveExplicitRepoRoot(envRoot)
	}
	if wd, err := os.Getwd(); err == nil {
		if resolved, ok := gitTopLevel(wd); ok {
			return resolved, nil
		}
	}
	if FrameworkRoot != "" {
		if resolved, ok := gitTopLevel(FrameworkRoot); ok {
			return resolved, nil
		}
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, ok := gitTopLevel(filepath.Dir(exe)); ok {
			return resolved, nil
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Abs(wd)
	}
	return "", fmt.Errorf("no se pudo resolver repo root")
}

func resolveExplicitRepoRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("no se pudo resolver --root %q: %v", root, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("root %q no existe: %v", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("root %q no es un directorio", abs)
	}
	if resolved, ok := gitTopLevel(abs); ok {
		return resolved, nil
	}
	return "", fmt.Errorf("root %q no es un repo git", abs)
}

func gitTopLevel(start string) (string, bool) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = start
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", false
	}
	resolved := filepath.Clean(strings.TrimSpace(string(out)))
	if resolved == "" {
		return "", false
	}
	return resolved, true
}

func deriveBackupRoot(repoRoot string) string {
	name := repoName(repoRoot)
	parent := filepath.Dir(repoRoot)
	if parent == "." || parent == "" {
		parent = os.TempDir()
	}
	return filepath.Join(parent, name+"-charlie-backups")
}

func repoName(repoRoot string) string {
	name := filepath.Base(repoRoot)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "repo"
	}
	return name
}

func detectFrameworkRoot() string {
	if envRoot := os.Getenv("CHARLIE_FRAMEWORK_ROOT"); envRoot != "" {
		if resolved, ok := findFrameworkRoot(envRoot); ok {
			return resolved
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if resolved, ok := findFrameworkRoot(wd); ok {
			return resolved
		}
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, ok := findFrameworkRoot(filepath.Dir(exe)); ok {
			return resolved
		}
	}
	return ""
}

func findFrameworkRoot(start string) (string, bool) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	dir := abs
	for {
		if looksLikeFrameworkRoot(dir) {
			return dir, true
		}
		nested := filepath.Join(dir, "framework-charlie")
		if looksLikeFrameworkRoot(nested) {
			return nested, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func looksLikeFrameworkRoot(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "framework.manifest.json")); err == nil {
		if _, err := os.Stat(filepath.Join(path, "cmd", "charlie")); err == nil {
			return true
		}
	}
	return false
}
