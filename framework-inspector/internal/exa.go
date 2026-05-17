package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const exaAPIURL = "https://api.exa.ai/search"

type exaRequest struct {
	Query          string `json:"query"`
	NumResults     int    `json:"numResults"`
	UseAutoprompt  bool   `json:"useAutoprompt"`
	Type           string `json:"type"`
	Contents       struct {
		Text struct {
			MaxCharacters int `json:"maxCharacters"`
		} `json:"text"`
	} `json:"contents"`
}

type exaResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Text    string `json:"text"`
		Score   float64 `json:"score"`
	} `json:"results"`
}

func SearchDocs(query string) ([]ExaResult, error) {
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("EXA_API_KEY no configurada")
	}

	reqBody := exaRequest{
		Query:         query + " API documentation REST",
		NumResults:    4,
		UseAutoprompt: true,
		Type:          "neural",
	}
	reqBody.Contents.Text.MaxCharacters = 400

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", exaAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no pude conectar con Exa: %w", err)
	}
	defer resp.Body.Close()

	var exaResp exaResponse
	if err := json.NewDecoder(resp.Body).Decode(&exaResp); err != nil {
		return nil, err
	}

	results := make([]ExaResult, 0, len(exaResp.Results))
	for _, r := range exaResp.Results {
		snippet := r.Text
		if len(snippet) > 300 {
			snippet = snippet[:300] + "…"
		}
		results = append(results, ExaResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: snippet,
		})
	}
	return results, nil
}

func FormatDocsForQuestion(docs []ExaResult, apiName string) string {
	if len(docs) == 0 {
		return fmt.Sprintf("No encontré documentación pública para \"%s\".", apiName)
	}
	msg := fmt.Sprintf("Encontré esto sobre \"%s\" en internet:\n", apiName)
	for i, d := range docs {
		if i >= 2 {
			break
		}
		msg += fmt.Sprintf("• %s — %s\n", d.Title, d.URL)
	}
	return msg
}
