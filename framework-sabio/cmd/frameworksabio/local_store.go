package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"unicode"
)

type localDocument struct {
	ID       string                 `json:"id"`
	Endpoint string                 `json:"endpoint"`
	RecordID string                 `json:"record_id"`
	Text     string                 `json:"text"`
	Metadata map[string]interface{} `json:"metadata"`
	RawJSON  string                 `json:"raw_json"`
}

type localSearchResult struct {
	Document localDocument `json:"document"`
	Score    float64       `json:"score"`
}

type localFileStore struct {
	docs []localDocument
}

type localFileFormat struct {
	Version int             `json:"version"`
	Docs    []localDocument `json:"documents"`
}

func newLocalFileStore(path string) (*localFileStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &localFileStore{}, nil
		}
		return nil, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return &localFileStore{}, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var ff localFileFormat
		if err := json.Unmarshal(data, &ff); err != nil {
			return nil, fmt.Errorf("filestore parse v1: %w", err)
		}
		return &localFileStore{docs: ff.Docs}, nil
	}
	var docs []localDocument
	if err := json.Unmarshal(data, &docs); err != nil {
		return nil, fmt.Errorf("filestore parse: %w", err)
	}
	return &localFileStore{docs: docs}, nil
}

func (s *localFileStore) Close() error { return nil }

func (s *localFileStore) Stats() (map[string]int, error) {
	out := map[string]int{}
	for _, d := range s.docs {
		out[d.Endpoint]++
	}
	return out, nil
}

func (s *localFileStore) Search(query string, topK int, endpointFilter []string) ([]localSearchResult, error) {
	queryTerms := localTokenize(query)
	if len(queryTerms) == 0 {
		return nil, fmt.Errorf("search: query sin términos útiles tras tokenizar")
	}
	allowed := map[string]bool{}
	for _, e := range endpointFilter {
		allowed[e] = true
	}
	docTokens := map[int][]string{}
	docFreqs := map[int]map[string]int{}
	docLengths := map[int]int{}
	termInDocs := map[string]int{}
	totalLen := 0
	for i, d := range s.docs {
		if len(allowed) > 0 && !allowed[d.Endpoint] {
			continue
		}
		tokens := localTokenize(d.Endpoint + " " + d.Text)
		freqs := map[string]int{}
		for _, t := range tokens {
			freqs[t]++
		}
		docTokens[i] = tokens
		docFreqs[i] = freqs
		docLengths[i] = len(tokens)
		totalLen += len(tokens)
		for term := range freqs {
			termInDocs[term]++
		}
	}
	if len(docTokens) == 0 {
		return nil, nil
	}
	avgDocLen := float64(totalLen) / float64(len(docTokens))
	results := make([]localSearchResult, 0, len(docTokens))
	for i, d := range s.docs {
		freqs, ok := docFreqs[i]
		if !ok {
			continue
		}
		score := 0.0
		dl := float64(docLengths[i])
		for _, q := range queryTerms {
			tf := float64(freqs[q])
			if tf == 0 {
				continue
			}
			df := float64(termInDocs[q])
			idf := math.Log(1 + (float64(len(docTokens))-df+0.5)/(df+0.5))
			norm := tf * 2.5 / (tf + 1.5*(1-0.75+0.75*dl/avgDocLen))
			score += idf * norm
		}
		if score > 0 {
			results = append(results, localSearchResult{Document: d, Score: score})
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func localTokenize(text string) []string {
	lower := strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	for _, r := range lower {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
			continue
		}
		if current.Len() > 0 {
			tokens = appendLocalToken(tokens, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		tokens = appendLocalToken(tokens, current.String())
	}
	return tokens
}

func appendLocalToken(tokens []string, t string) []string {
	if len(t) == 0 {
		return tokens
	}
	if len(t) == 1 && !localIsAllDigits(t) {
		return tokens
	}
	if localStopwords[t] {
		return tokens
	}
	return append(tokens, t)
}

func localIsAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var localStopwords = map[string]bool{
	"el": true, "la": true, "los": true, "las": true, "un": true, "una": true,
	"y": true, "o": true, "a": true, "de": true, "del": true, "al": true,
	"es": true, "se": true, "que": true, "en": true, "por": true, "para": true,
	"con": true, "su": true, "sus": true, "lo": true, "como": true, "mas": true,
	"the": true, "of": true, "and": true, "to": true, "in": true, "is": true,
	"it": true, "for": true, "on": true, "at": true, "by": true, "an": true,
	"as": true, "be": true, "are": true, "was": true,
}
