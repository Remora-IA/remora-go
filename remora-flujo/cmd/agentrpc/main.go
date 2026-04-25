package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"remora-flujo/nativeagent"
)

type command struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Role    string `json:"role,omitempty"`
	Message string `json:"message,omitempty"`
}

type response struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Command string `json:"command,omitempty"`
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var cmd command
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			write(response{Type: "response", Success: false, Error: err.Error()})
			continue
		}
		handle(cmd)
		if cmd.Type == "shutdown" {
			return
		}
	}
	if err := scanner.Err(); err != nil {
		write(response{Type: "response", Success: false, Error: err.Error()})
	}
}

func handle(cmd command) {
	switch cmd.Type {
	case "prompt":
		role := cmd.Role
		if role == "" {
			role = "agent"
		}
		agent, err := nativeagent.New(nativeagent.Options{
			CWD:          "/Users/alcless_a1234_cursor/remora-go/remora-flujo",
			SessionPath:  filepath.Join("temp", "rpc-sessions", role+".json"),
			AllowedTools: allowedTools(role),
		})
		if err != nil {
			write(response{ID: cmd.ID, Type: "response", Command: "prompt", Success: false, Error: err.Error()})
			return
		}
		text, err := agent.Prompt(cmd.Message)
		if err != nil {
			write(response{ID: cmd.ID, Type: "response", Command: "prompt", Success: false, Error: err.Error(), Data: map[string]string{"partial": text}})
			return
		}
		write(response{ID: cmd.ID, Type: "response", Command: "prompt", Success: true, Data: map[string]string{"text": text}})
	case "shutdown":
		write(response{ID: cmd.ID, Type: "response", Command: "shutdown", Success: true})
	default:
		write(response{ID: cmd.ID, Type: "response", Command: cmd.Type, Success: false, Error: "comando desconocido"})
	}
}

func allowedTools(role string) []string {
	switch role {
	case "bravo":
		return []string{"bash", "read_file", "write_file", "list_files"}
	default:
		return []string{"bash", "read_file", "list_files"}
	}
}

func write(resp response) {
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stdout, `{"type":"response","success":false,"error":%q}`+"\n", err.Error())
		return
	}
	fmt.Println(string(data))
}
