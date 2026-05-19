package agentloop

import (
	"context"
	"encoding/json"
	"strings"

	"remora-flujo/internal/llm"
)

type ToolExecutor interface {
	Execute(ctx context.Context, tool string, args map[string]any) string
}

type EventSink interface {
	OnToolStart(tool string)
	OnToolEnd(tool string, result string)
	OnText(text string)
}

type Config struct {
	MaxTurns       int
	MaxTokens      int
	FinalMaxTokens int
	System         string
	User           string
	Framework      string
	Spec           llm.Spec
}

type Event struct {
	Type      string `json:"type"`
	Framework string `json:"framework,omitempty"`
	Message   string `json:"message,omitempty"`
}

type Result struct {
	Text   string
	Events []Event
}

type toolDecision struct {
	Action string         `json:"action"`
	Tool   string         `json:"tool,omitempty"`
	Args   map[string]any `json:"args,omitempty"`
	Final  string         `json:"final,omitempty"`
}

func Run(ctx context.Context, client llm.Client, tools ToolExecutor, sink EventSink, cfg Config) (Result, error) {
	maxTurns := cfg.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 30
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1200
	}

	var observations []string
	var events []Event
	consecutiveDups := 0
	var lastKey string

	textOnlyTurns := 0

	for i := 0; i < maxTurns; i++ {
		out, err := client.Complete(ctx, llm.CompletionRequest{
			System:    cfg.System,
			User:      buildUserMessage(cfg.User, observations),
			MaxTokens: maxTokens,
		})
		if err != nil {
			return Result{}, err
		}

		decisions := ParseToolDecisions(out)
		if len(decisions) == 0 {
			text := strings.TrimSpace(out)
			sink.OnText(text)
			textOnlyTurns++
			if textOnlyTurns >= 3 {
				return Result{Text: text, Events: events}, nil
			}
			observations = append(observations, "ASSISTANT_MESSAGE: "+Truncate(text, 1000)+"\n\nSeguí con la siguiente herramienta. No repitas lo que ya dijiste. Ejecutá la acción.")
			continue
		}
		textOnlyTurns = 0

		executed := false
		for _, decision := range decisions {
			if decision.Action == "final" {
				text := strings.TrimSpace(decision.Final)
				if text == "" {
					text = strings.TrimSpace(out)
				}
				sink.OnText(text)
				return Result{Text: text, Events: events}, nil
			}
			if decision.Action != "tool" || decision.Tool == "" {
				continue
			}

			key := decisionKey(decision)
			if key == lastKey {
				consecutiveDups++
				if consecutiveDups >= 2 {
					return streamFinal(ctx, client, sink, cfg, observations, events)
				}
			} else {
				consecutiveDups = 0
			}
			lastKey = key

			ev := Event{Type: "tool_execution_start", Framework: cfg.Framework, Message: decision.Tool}
			events = append(events, ev)
			sink.OnToolStart(decision.Tool)

			result := tools.Execute(ctx, decision.Tool, decision.Args)

			endEv := Event{Type: "tool_execution_end", Framework: cfg.Framework, Message: decision.Tool + ": " + Truncate(result, 800)}
			events = append(events, endEv)
			sink.OnToolEnd(decision.Tool, result)

			observations = append(observations, "TOOL "+decision.Tool+" RESULT:\n"+Truncate(result, 5000))
			executed = true
		}

		if !executed {
			text := strings.TrimSpace(out)
			sink.OnText(text)
			textOnlyTurns++
			if textOnlyTurns >= 3 {
				return Result{Text: text, Events: events}, nil
			}
			observations = append(observations, "ASSISTANT_MESSAGE: "+Truncate(text, 1000)+"\n\nSeguí con la siguiente herramienta. No repitas lo que ya dijiste. Ejecutá la acción.")
			continue
		}
	}

	return streamFinal(ctx, client, sink, cfg, observations, events)
}

func streamFinal(ctx context.Context, client llm.Client, sink EventSink, cfg Config, observations []string, events []Event) (Result, error) {
	finalMaxTokens := cfg.FinalMaxTokens
	if finalMaxTokens <= 0 {
		finalMaxTokens = 1200
	}
	finalUser := cfg.User + "\n\nResultados de herramientas:\n" + strings.Join(observations, "\n\n") + "\n\nResponde al usuario con la conclusión final. No menciones JSON ni herramientas salvo que sea necesario."
	text, err := client.Stream(ctx, llm.CompletionRequest{
		System:    cfg.System,
		User:      finalUser,
		MaxTokens: finalMaxTokens,
	}, func(se llm.StreamEvent) {
		sink.OnText(se.Delta)
	})
	if err != nil {
		return Result{Events: events}, err
	}
	return Result{Text: strings.TrimSpace(text), Events: events}, nil
}

func buildUserMessage(user string, observations []string) string {
	if len(observations) == 0 {
		return user
	}
	return user + "\n\nObservaciones previas:\n" + strings.Join(observations, "\n\n")
}

func ParseToolDecisions(raw string) []toolDecision {
	objects := extractJSONObjects(raw)
	out := make([]toolDecision, 0, len(objects))
	for _, obj := range objects {
		var d toolDecision
		if err := json.Unmarshal([]byte(obj), &d); err != nil {
			continue
		}
		if d.Action == "final" {
			out = append(out, d)
			continue
		}
		if d.Action != "" && d.Action != "tool" && d.Tool == "" {
			d.Tool = d.Action
			d.Action = "tool"
		}
		if d.Action == "" && d.Tool != "" {
			d.Action = "tool"
		}
		if d.Action == "" {
			d.Action = "final"
		}
		out = append(out, d)
	}
	return out
}

func extractJSONObject(raw string) string {
	objects := extractJSONObjects(raw)
	if len(objects) == 0 {
		return ""
	}
	return objects[0]
}

func extractJSONObjects(raw string) []string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	var objects []string
	start := -1
	depth := 0
	inString := false
	escape := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && inString {
			escape = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				objects = append(objects, s[start:i+1])
				start = -1
			}
		}
	}
	return objects
}

func decisionKey(d toolDecision) string {
	b, _ := json.Marshal(d)
	return string(b)
}

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
