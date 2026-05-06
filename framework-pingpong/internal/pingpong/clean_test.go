package pingpong

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanDeletesExactLineRangeOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.py")
	original := "keep1\ndelete1\ndelete2\nkeep2\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewWithTrace("test", DefaultLangConfigs["python"])
	result, err := c.Clean(path, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success: %+v", result)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	want := "keep1\nkeep2\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if strings.Contains(got, "delete") {
		t.Fatalf("deleted text still present: %q", got)
	}
}

func TestCleanRejectsInvalidRange(t *testing.T) {
	c := NewWithTrace("test", DefaultLangConfigs["python"])
	if _, err := c.Clean("file.py", 3, 2); err == nil {
		t.Fatal("expected invalid range error")
	}
}
