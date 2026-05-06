package pingpong

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearchFindsFocusedEvidence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	src := "package main\n\ntype Reply struct {\n\tResultado int\n}\n"
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	matches, err := searchCode(path, "", "Reply", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected Reply evidence")
	}
	if matches[0].Line != 3 {
		t.Fatalf("expected line 3, got %d", matches[0].Line)
	}
}

func TestGoSymbolsFindsStructFieldsAndMethod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	src := `package main

type Reply struct {
	Resultado int
}

type Servicio struct{}

func (s *Servicio) Suma(args *Args, reply *Reply) error {
	return nil
}

type Args struct {
	Num1 int
	Num2 int
}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	symbols, err := goSymbols(path)
	if err != nil {
		t.Fatal(err)
	}
	var foundReply, foundMethod bool
	for _, s := range symbols {
		if s.Kind == "struct" && s.Name == "Reply" && len(s.Fields) == 1 && s.Fields[0] == "Resultado int" {
			foundReply = true
		}
		if s.Kind == "method" && s.Name == "Suma" && s.Receiver == "*Servicio" {
			foundMethod = true
		}
	}
	if !foundReply {
		t.Fatal("expected Reply struct with Resultado field")
	}
	if !foundMethod {
		t.Fatal("expected Suma method")
	}
}
