package pingpong

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompileCheckPythonOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.py")
	if err := os.WriteFile(path, []byte("print('ok')\n"), 0644); err != nil {
		t.Fatal(err)
	}

	rep := CompileCheck(path, DefaultLangConfigs["python"])
	if !rep.CompileOK {
		t.Fatalf("expected python compile ok, got %q", rep.CompileLog)
	}
	if rep.FileContent == "" {
		t.Fatal("expected file content in report")
	}
	if rep.FileHash == "" {
		t.Fatal("expected file hash in report")
	}
}

func TestCompileCheckPythonError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.py")
	if err := os.WriteFile(path, []byte("def broken(:\n    pass\n"), 0644); err != nil {
		t.Fatal(err)
	}

	rep := CompileCheck(path, DefaultLangConfigs["python"])
	if rep.CompileOK {
		t.Fatal("expected python compile error")
	}
	if rep.CompileLog == "" {
		t.Fatal("expected compile log")
	}
}
