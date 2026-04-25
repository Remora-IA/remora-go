package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"framework-paladin/paladin"
)

func main() {
	path := ""
	if len(os.Args) > 1 {
		path = os.Args[1]
	} else {
		path = latestTrace()
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "no trace found")
		os.Exit(1)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	var trace paladin.TraceResult
	if err := json.Unmarshal(data, &trace); err != nil {
		fail(err)
	}
	fmt.Printf("Trace: %s\nStatus: %s\nErrors: %d\nDuration: %dms\nFile: %s\n\n",
		trace.TraceID, trace.Status, trace.TotalErrors, trace.TotalDuration, path)
	printSpan(trace.Root, 0)
}

func latestTrace() string {
	matches, _ := filepath.Glob("temp/paladin/trace_*.json")
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}

func printSpan(span *paladin.Span, depth int) {
	if span == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	fmt.Printf("%s- %s (%dms)\n", indent, span.Name, span.DurationMs)
	for key, value := range span.Vars {
		fmt.Printf("%s  var %s=%v\n", indent, key, value)
	}
	for _, decision := range span.Decisions {
		fmt.Printf("%s  decision %s: %s\n", indent, decision.What, decision.Why)
	}
	for _, err := range span.Errors {
		fmt.Printf("%s  error %s\n", indent, err)
	}
	for _, child := range span.Children {
		printSpan(child, depth+1)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
