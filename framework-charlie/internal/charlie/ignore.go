package charlie

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// charlieIgnore extends ShouldIgnore via a configurable patterns file.
// Patterns are glob-style (filepath.Match). Lines starting with # are
// comments; blank lines are ignored. Patterns match against the file path
// relative to RepoRoot in forward-slash form.
//
// The file lives at framework-charlie/.charlieignore and is loaded once.

var (
	charlieIgnoreMu       sync.Mutex
	charlieIgnoreLoaded   string
	charlieIgnorePatterns []string
)

func resetCharlieIgnoreCache() {
	charlieIgnoreMu.Lock()
	defer charlieIgnoreMu.Unlock()
	charlieIgnoreLoaded = ""
	charlieIgnorePatterns = nil
}

func loadCharlieIgnore() {
	charlieIgnoreMu.Lock()
	defer charlieIgnoreMu.Unlock()

	loadedKey := RepoRoot + "|" + frameworkDataRoot()
	if charlieIgnoreLoaded == loadedKey {
		return
	}
	charlieIgnoreLoaded = loadedKey
	charlieIgnorePatterns = nil

	candidates := []string{
		filepath.Join(frameworkDataRoot(), ".charlieignore"),
		filepath.Join(RepoRoot, ".charlieignore"),
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			charlieIgnorePatterns = append(charlieIgnorePatterns, line)
		}
	}
}

// matchesCharlieIgnore returns true if any configured pattern matches the
// given repo-relative, forward-slash path. It supports simple prefix/suffix
// and "*" globs via filepath.Match applied to each path segment.
func matchesCharlieIgnore(path string) bool {
	loadCharlieIgnore()
	charlieIgnoreMu.Lock()
	patterns := append([]string(nil), charlieIgnorePatterns...)
	charlieIgnoreMu.Unlock()
	if len(patterns) == 0 {
		return false
	}
	clean := filepath.ToSlash(path)
	base := filepath.Base(clean)
	for _, pat := range patterns {
		// Exact path match.
		if pat == clean {
			return true
		}
		// Directory prefix: pattern ending with "/".
		if strings.HasSuffix(pat, "/") && strings.HasPrefix(clean, pat) {
			return true
		}
		// Glob on base name.
		if matched, _ := filepath.Match(pat, base); matched {
			return true
		}
		// Glob on full path.
		if matched, _ := filepath.Match(pat, clean); matched {
			return true
		}
		// Contains pattern (useful for "/temp/" style entries).
		if strings.Contains("/"+clean, pat) {
			return true
		}
	}
	return false
}
