package paladin

import (
	"fmt"
)

// Console imprime el trace en la consola.
func Console(t *Trace) {
	fmt.Println("========================================")
	fmt.Printf(" Paladin Trace\n")
	fmt.Printf(" Trace: %s\n", t.ID)
	fmt.Printf(" App:   %s\n", t.Name)
	fmt.Println("========================================")
	
	if t.Root != nil {
		printSpan(t.Root, 0)
	}
}

func printSpan(s *Span, level int) {
	indent := ""
	for i := 0; i < level; i++ {
		indent += "  "
	}
	
	fmt.Printf("%s[PALADIN] START %s (%s)\n", indent, s.Name, s.ID)
	
	for key, val := range s.Vars {
		fmt.Printf("%s[FLOW]   VAR  %s → %s = %s\n", indent, s.Name, key, val)
	}
	
	for _, d := range s.Decisions {
		fmt.Printf("%s[DEC]    %s: %s\n", indent, d.ID, d.Description)
	}
	
	if s.Error != "" {
		fmt.Printf("%s[ERROR]  %s\n", indent, s.Error)
	}
	
	for _, child := range s.Children {
		printSpan(child, level+1)
	}
	
	if s.EndTime.IsZero() {
		fmt.Printf("%s[PALADIN] END %s\n", indent, s.Name)
	}
}