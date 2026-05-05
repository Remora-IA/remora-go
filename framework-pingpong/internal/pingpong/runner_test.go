package pingpong

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "main.go")
	if err := os.WriteFile(p, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunFileCompileError(t *testing.T) {
	path := writeTemp(t, `package main
func main() {
	x := undefined_var
}
`)
	rep, err := RunFile(path, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if rep.CompileOK {
		t.Fatal("expected compile fail")
	}
	if rep.CompileLog == "" {
		t.Fatal("expected compile log")
	}
}

func TestRunFileSuccess(t *testing.T) {
	path := writeTemp(t, `package main
import "fmt"
func main() { fmt.Println("hello") }
`)
	rep, err := RunFile(path, "", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if !rep.CompileOK {
		t.Fatalf("compile fail: %s", rep.CompileLog)
	}
	if !rep.RunOK {
		t.Fatalf("run fail: %s", rep.Stderr)
	}
	if !rep.Match {
		t.Fatalf("mismatch: got=%q want=%q", rep.Stdout, "hello")
	}
}

func TestRunFileMismatch(t *testing.T) {
	path := writeTemp(t, `package main
import "fmt"
func main() { fmt.Println("wrong") }
`)
	rep, err := RunFile(path, "", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if !rep.CompileOK || !rep.RunOK {
		t.Fatal("expected compile+run OK")
	}
	if rep.Match {
		t.Fatal("expected mismatch")
	}
}

func TestRunFileWithStdin(t *testing.T) {
	path := writeTemp(t, `package main
import (
	"bufio"
	"fmt"
	"os"
)
func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	fmt.Println("echo: " + scanner.Text())
}
`)
	rep, err := RunFile(path, "test input", "echo: test input")
	if err != nil {
		t.Fatal(err)
	}
	if !rep.CompileOK || !rep.RunOK {
		t.Fatalf("compile=%v run=%v stderr=%s", rep.CompileOK, rep.RunOK, rep.Stderr)
	}
	if !rep.Match {
		t.Fatalf("mismatch: got=%q", rep.Stdout)
	}
}

func TestRunFilePalindromeFunctional(t *testing.T) {
	path := writeTemp(t, `package main
import "fmt"
func isPalindrome(x int) bool {
	if x < 0 { return false }
	original := x
	rev := 0
	for x > 0 {
		rev = rev*10 + x%10
		x = x / 10
	}
	return original == rev
}
func main() {
	fmt.Println(isPalindrome(121))
	fmt.Println(isPalindrome(-121))
	fmt.Println(isPalindrome(10))
}
`)
	rep, err := RunFile(path, "", "true\nfalse\nfalse")
	if err != nil {
		t.Fatal(err)
	}
	if !rep.CompileOK || !rep.RunOK {
		t.Fatalf("compile=%v run=%v log=%s stderr=%s", rep.CompileOK, rep.RunOK, rep.CompileLog, rep.Stderr)
	}
	if !rep.Match {
		t.Fatalf("palindrome functional mismatch: got=%q", rep.Stdout)
	}
}
