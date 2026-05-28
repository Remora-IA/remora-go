package paladin

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type Explanation struct {
	Timeline   []ExplanationItem
	Rules      []ExplanationItem
	Handoffs   []ExplanationItem
	Violations []ExplanationItem
	Technical  []ExplanationItem
}

type ExplanationItem struct {
	Span     string
	Kind     string
	Subject  string
	Summary  string
	Expected string
	Actual   string
	Passed   *bool
	Depth    int
	StartNs  int64
}

func BuildExplanation(trace TraceResult) Explanation {
	var items []ExplanationItem
	collectExplanationItems(trace.Root, 0, &items)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].StartNs < items[j].StartNs
	})

	var out Explanation
	for _, item := range items {
		switch item.Kind {
		case "rule", "check", "expect":
			out.Rules = append(out.Rules, item)
			if item.Kind == "check" && item.Passed != nil && !*item.Passed {
				out.Violations = append(out.Violations, item)
			}
		case "handoff":
			out.Handoffs = append(out.Handoffs, item)
			out.Timeline = append(out.Timeline, item)
		case "violation":
			out.Violations = append(out.Violations, item)
			out.Timeline = append(out.Timeline, item)
		case "error":
			out.Technical = append(out.Technical, item)
		default:
			out.Timeline = append(out.Timeline, item)
		}
	}
	return out
}

func WriteExplanation(w io.Writer, trace TraceResult) {
	explanation := BuildExplanation(trace)
	fmt.Fprintf(w, "Paladin Explain\n")
	fmt.Fprintf(w, "Trace: %s\nStatus: %s\nDuration: %dms\nErrors: %d\n\n", trace.TraceID, trace.Status, trace.TotalDuration, trace.TotalErrors)

	writeSection(w, "Timeline Semantico", explanation.Timeline)
	writeSection(w, "Reglas y Expectativas", explanation.Rules)
	writeSection(w, "Handoffs", explanation.Handoffs)
	writeSection(w, "Inconsistencias", explanation.Violations)
	writeSection(w, "Senales Tecnicas", explanation.Technical)
}

func collectExplanationItems(span *Span, depth int, out *[]ExplanationItem) {
	if span == nil {
		return
	}
	for _, event := range span.Semantic {
		*out = append(*out, ExplanationItem{
			Span:     span.Name,
			Kind:     event.Kind,
			Subject:  event.Subject,
			Summary:  event.Summary,
			Expected: event.Expected,
			Actual:   event.Actual,
			Passed:   event.Passed,
			Depth:    depth,
			StartNs:  span.StartNs,
		})
	}
	for _, decision := range span.Decisions {
		*out = append(*out, ExplanationItem{
			Span:    span.Name,
			Kind:    "decision",
			Subject: decision.What,
			Summary: decision.Why,
			Depth:   depth,
			StartNs: span.StartNs,
		})
	}
	for _, err := range span.Errors {
		*out = append(*out, ExplanationItem{
			Span:    span.Name,
			Kind:    "error",
			Summary: err,
			Depth:   depth,
			StartNs: span.StartNs,
		})
	}
	for _, child := range span.Children {
		collectExplanationItems(child, depth+1, out)
	}
}

func writeSection(w io.Writer, title string, items []ExplanationItem) {
	fmt.Fprintf(w, "%s\n", title)
	if len(items) == 0 {
		fmt.Fprintf(w, "  - sin eventos\n\n")
		return
	}
	for _, item := range items {
		fmt.Fprintf(w, "  - %s\n", item.Sentence())
	}
	fmt.Fprintln(w)
}

func (i ExplanationItem) Sentence() string {
	location := i.Span
	if location == "" {
		location = "trace"
	}
	subject := strings.TrimSpace(i.Subject)
	summary := strings.TrimSpace(i.Summary)
	switch i.Kind {
	case "actor":
		return fmt.Sprintf("%s define actor %q: %s", location, subject, summary)
	case "goal":
		return fmt.Sprintf("%s busca: %s", location, summary)
	case "event":
		return fmt.Sprintf("%s observa %q: %s", location, subject, summary)
	case "decision":
		return fmt.Sprintf("%s decide %q porque %s", location, subject, summary)
	case "rule":
		return fmt.Sprintf("%s declara regla %q: %s", location, subject, summary)
	case "check":
		status := "OK"
		if i.Passed != nil && !*i.Passed {
			status = "FALLO"
		}
		return fmt.Sprintf("%s evalua regla %q [%s]. Esperado: %s. Actual: %s", location, subject, status, i.Expected, i.Actual)
	case "expect":
		return fmt.Sprintf("%s espera %q: %s", location, subject, i.Expected)
	case "handoff":
		return fmt.Sprintf("%s transfiere %s: %s", location, subject, summary)
	case "violation":
		return fmt.Sprintf("%s detecta inconsistencia en %q. Esperado: %s. Actual: %s", location, subject, i.Expected, i.Actual)
	case "error":
		return fmt.Sprintf("%s error tecnico: %s", location, summary)
	default:
		if subject != "" {
			return fmt.Sprintf("%s %s %q: %s", location, i.Kind, subject, summary)
		}
		return fmt.Sprintf("%s %s: %s", location, i.Kind, summary)
	}
}
