package paladin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TraceClient envía traces al servidor Paladin Server.
type TraceClient struct {
	serverURL string
	client    *http.Client
}

// FlowResult es la respuesta del servidor con el flow narrado.
type FlowResult struct {
	TraceID              string   `json:"trace_id"`
	Framework            string   `json:"framework"`
	FlowNarrative        string   `json:"flow_narrative"`
	BusinessRulesDetected []string `json:"business_rules_detected"`
	PotentialIssues      []string `json:"potential_issues"`
	Summary              string   `json:"summary"`
	RawResponse          string   `json:"raw_response,omitempty"`
}

// NewTraceClient crea un cliente para enviar traces al servidor.
func NewTraceClient(serverURL string) *TraceClient {
	if serverURL == "" {
		serverURL = "http://localhost:8099"
	}
	return &TraceClient{
		serverURL: serverURL,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// SendTrace envía un trace al servidor. No bloquea.
func (c *TraceClient) SendTrace(framework string, traceJSON []byte) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("cliente no inicializado")
	}

	payload := map[string]any{
		"framework":   framework,
		"trace_json":  json.RawMessage(traceJSON),
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling payload: %w", err)
	}

	// No blocking - goroutine para envío async
	go func() {
		req, err := http.NewRequest("POST", c.serverURL+"/trace", bytes.NewReader(body))
		if err != nil {
			fmt.Printf("[PALADIN] Error creando request: %v\n", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			fmt.Printf("[PALADIN] Servidor no disponible: %v\n", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
				traceID, _ := result["trace_id"].(string)
				status, _ := result["status"].(string)
				fmt.Printf("[PALADIN] Trace enviado: id=%s status=%s\n", traceID, status)
			}
		} else {
			fmt.Printf("[PALADIN] Servidor respondió: %d\n", resp.StatusCode)
		}
	}()

	return nil
}

// GetFlow consulta el flow narrado de un trace.
func (c *TraceClient) GetFlow(traceID string) (*FlowResult, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("cliente no inicializado")
	}

	req, err := http.NewRequest("GET", c.serverURL+"/flow/"+traceID, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result FlowResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Ask envía una pregunta sobre un trace específico.
func (c *TraceClient) Ask(traceID, question string) (string, error) {
	if c == nil || c.client == nil {
		return "", fmt.Errorf("cliente no inicializado")
	}

	payload := map[string]string{
		"trace_id":  traceID,
		"question":  question,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.serverURL+"/ask", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result["answer"], nil
}

// Global client instance para uso desde trace.go
var globalTraceClient *TraceClient

// SetGlobalClient configura el cliente global.
func SetGlobalClient(client *TraceClient) {
	globalTraceClient = client
}

// SendTraceAsync envía trace usando el cliente global (no blocking).
func SendTraceAsync(framework string, traceJSON []byte) {
	if globalTraceClient != nil {
		_ = globalTraceClient.SendTrace(framework, traceJSON)
	}
}