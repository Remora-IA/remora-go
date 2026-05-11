package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"channel/adapter"
	"channel/manifest"
)

type universalSingleResult struct {
	Message Message
	Idle    bool
}

func (s *server) createUniversalSingleMessage(conv *Conversation, m *manifest.Manifest) *Message {
	if m == nil || len(m.Commands) == 0 {
		return nil
	}
	commands := universalCommandNames(m)
	content := fmt.Sprintf("El framework %s no implementa session-start. No se puede iniciar una sesión IA fuera de Channel JSON-RPC. Comandos disponibles: %s.", m.Name, strings.Join(commands, ", "))
	return &Message{
		ID:        generateMessageID(),
		Role:      "framework",
		Framework: m.Name,
		Content:   content,
		Reasoning: "Wrapper universal rechazó iniciar una sesión IA sin session-start; todo inicio de framework debe pasar por Channel JSON-RPC.",
		Status:    "error",
		Events: []MessageEvent{{
			Type:      "framework.session_start_missing",
			Framework: m.Name,
			Message:   "session-start no implementado",
		}},
		Timestamp: time.Now(),
	}
}

func (s *server) runUniversalSingle(ctx context.Context, ch *adapter.Client, conv *Conversation, m *manifest.Manifest, input string, resources []MessageResource) (universalSingleResult, error) {
	commandName, commandInput, explicit := selectUniversalCommand(m, input)
	if commandName == "" {
		msg := Message{
			ID:        generateMessageID(),
			Role:      "framework",
			Framework: m.Name,
			Content:   "No encontré un comando seguro para ejecutar con ese mensaje. Usa /<comando> seguido del input.",
			Reasoning: fmt.Sprintf("Wrapper universal revisó comandos disponibles: %s.", strings.Join(universalCommandNames(m), ", ")),
			Status:    "needs_input",
			Events:    []MessageEvent{{Type: "framework.needs_input", Framework: m.Name, Message: "no safe command selected"}},
			Timestamp: time.Now(),
		}
		return universalSingleResult{Message: msg}, nil
	}
	cmd := m.Commands[commandName]
	params, missing := inferUniversalParams(conv, cmd, commandInput, resources)
	if len(missing) > 0 {
		msg := Message{
			ID:        generateMessageID(),
			Role:      "framework",
			Framework: m.Name,
			Content:   fmt.Sprintf("Para ejecutar %s necesito estos datos: %s. Puedes enviarlos como JSON o usar /%s con esos valores.", commandName, strings.Join(missing, ", "), commandName),
			Reasoning: fmt.Sprintf("Wrapper universal seleccionó %s, pero el manifest exige params que no se pueden inferir desde input libre: %v.", commandName, missing),
			Status:    "needs_input",
			Events:    []MessageEvent{{Type: "framework.needs_input", Framework: m.Name, Message: "missing params: " + strings.Join(missing, ",")}},
			Timestamp: time.Now(),
		}
		return universalSingleResult{Message: msg}, nil
	}
	args, err := cmd.ResolveArgs(params, frameworkIOPaths(s.rootDir, m.Inputs), frameworkIOPaths(s.rootDir, m.Outputs))
	if err != nil {
		return universalSingleResult{}, err
	}
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-" + m.Name
	}
	cwd := filepath.Join(s.rootDir, cwdRel)
	bin, argsPrefix := resolveManifestRuntime(cwd, m)
	fullArgs := append([]string{}, argsPrefix...)
	fullArgs = append(fullArgs, args...)
	resp, err := ch.ExecuteCommand(ctx, bin, fullArgs, cwd)
	if err != nil {
		return universalSingleResult{}, err
	}
	msg := normalizeUniversalResponse(m.Name, commandName, explicit, params, fullArgs, resp)
	return universalSingleResult{Message: msg, Idle: msg.Status == "done"}, nil
}

func selectUniversalCommand(m *manifest.Manifest, input string) (string, string, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || m == nil {
		return "", "", false
	}
	if strings.HasPrefix(trimmed, "/") {
		parts := strings.Fields(trimmed)
		name := strings.TrimPrefix(parts[0], "/")
		if _, ok := m.Commands[name]; ok {
			return name, strings.TrimSpace(strings.TrimPrefix(trimmed, parts[0])), true
		}
	}
	fields := strings.Fields(trimmed)
	if len(fields) > 1 {
		if _, ok := m.Commands[fields[0]]; ok {
			return fields[0], strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0])), true
		}
	}
	bestName := ""
	bestScore := -1
	for name, cmd := range m.Commands {
		if dangerousUniversalCommand(name) {
			continue
		}
		score := universalCommandScore(name, cmd)
		if score > bestScore || (score == bestScore && name < bestName) {
			bestName = name
			bestScore = score
		}
	}
	if bestScore < 0 {
		return "", "", false
	}
	return bestName, trimmed, false
}

func universalCommandScore(name string, cmd manifest.Command) int {
	if name == "next-question" || name == "ingest-answer" {
		return -1
	}
	score := 0
	for _, p := range cmd.Params {
		switch p {
		case "answer", "input", "query", "text", "message", "prompt", "title", "action":
			score += 10
		case "conv_id", "conversation_id", "history", "profile", "limit", "data":
			score += 2
		default:
			if _, ok := cmd.Defaults[p]; ok {
				score++
			} else {
				score -= 3
			}
		}
	}
	if len(cmd.Params) == 0 {
		score = 1
	}
	return score
}

func dangerousUniversalCommand(name string) bool {
	switch strings.ToLower(name) {
	case "apply", "apply-all", "reset", "delete", "send", "complete", "event", "seed-from-foco":
		return true
	default:
		return false
	}
}

func inferUniversalParams(conv *Conversation, cmd manifest.Command, input string, resources []MessageResource) (map[string]string, []string) {
	params := map[string]string{}
	for k, v := range cmd.Defaults {
		params[k] = v
	}
	if jsonParams := parseJSONParams(input); len(jsonParams) > 0 {
		for k, v := range jsonParams {
			params[k] = v
		}
	}
	missing := []string{}
	for _, p := range cmd.Params {
		if _, ok := params[p]; ok {
			continue
		}
		switch p {
		case "conv_id", "conversation_id":
			params[p] = conv.ID
		case "question_id", "internal_question_id":
			params[p] = "universal_" + conv.ID
		case "answer", "input", "query", "text", "message", "prompt", "title", "action":
			params[p] = input
		case "history":
			params[p] = encodeRecentHistory(conv.ID, "")
		case "profile":
			params[p] = envOr("REMORA_PROFILE", "default")
		case "limit":
			params[p] = "10"
		case "data":
			params[p] = "{}"
		case "file", "path":
			if len(resources) > 0 {
				params[p] = resources[0].Path
			} else {
				missing = append(missing, p)
			}
		case "body_b64":
			params[p] = base64.StdEncoding.EncodeToString([]byte(input))
		default:
			missing = append(missing, p)
		}
	}
	return params, missing
}

func parseJSONParams(input string) map[string]string {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "{") {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range raw {
		switch t := v.(type) {
		case string:
			out[k] = t
		default:
			data, _ := json.Marshal(t)
			out[k] = string(data)
		}
	}
	return out
}

func normalizeUniversalResponse(framework, command string, explicit bool, params map[string]string, args []string, resp *adapter.Response) Message {
	status := "done"
	if resp == nil || !resp.Success {
		status = "error"
	}
	content := ""
	if resp != nil {
		content = strings.TrimSpace(resp.Stdout)
		if content == "" {
			content = strings.TrimSpace(resp.Stderr)
		}
		if content == "" && resp.Error != "" {
			content = resp.Error
		}
	}
	if content == "" {
		content = fmt.Sprintf("%s ejecutó %s sin salida visible.", framework, command)
	}
	artifact := MessageArtifact{Type: "command_result", Name: command, Content: content}
	if json.Valid([]byte(content)) {
		artifact.Type = "json"
	}
	reasoning := fmt.Sprintf("Wrapper universal ejecutó %s vía Channel usando el manifest de %s.", command, framework)
	if !explicit {
		reasoning += " El comando fue seleccionado automáticamente desde el input libre."
	}
	return Message{
		ID:        generateMessageID(),
		Role:      "framework",
		Framework: framework,
		Content:   content,
		Reasoning: reasoning,
		Status:    status,
		Artifacts: []MessageArtifact{artifact},
		Events: []MessageEvent{{
			Type:      "framework." + status,
			Framework: framework,
			Message:   command,
		}},
		SuggestedChips: universalFollowupChips(command, status, params, args),
		Timestamp:      time.Now(),
	}
}

func universalFollowupChips(command, status string, params map[string]string, args []string) []string {
	if status == "error" {
		return []string{"Reintentar con JSON", "/" + command}
	}
	return []string{"Seguir probando", "/" + command}
}

func universalCommandNames(m *manifest.Manifest) []string {
	out := make([]string, 0, len(m.Commands))
	for name := range m.Commands {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func resolveManifestRuntime(cwd string, m *manifest.Manifest) (string, []string) {
	if m == nil {
		return "", nil
	}
	command := m.Binary.Command
	args := append([]string{}, m.Binary.ArgsPrefix...)
	if command != "go" || len(args) < 2 || args[0] != "run" {
		return command, args
	}
	out := manifestBuildOutput(m)
	if out == "" {
		return command, args
	}
	if _, err := os.Stat(filepath.Join(cwd, out)); err == nil {
		return "./" + out, args[2:]
	}
	return command, args
}

func manifestBuildOutput(m *manifest.Manifest) string {
	if m == nil {
		return ""
	}
	for i, arg := range m.Build.Args {
		if arg == "-o" && i+1 < len(m.Build.Args) {
			return m.Build.Args[i+1]
		}
	}
	return ""
}
