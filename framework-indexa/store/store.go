// Package store define la interface de almacenamiento de documentos
// indexados por framework-indexa y consultables por framework-sabio.
//
// La implementación incluida (FileStore) es BM25 in-memory sobre un
// corpus persistido en JSON. Para datasets pequeños/medianos (hasta
// ~50k docs) es suficiente y no requiere infra externa.
//
// Cuando el volumen lo requiera, se puede agregar otra implementación
// del interface Store (p.ej. pgvector + tsvector) sin tocar a los
// frameworks consumidores.
package store

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// Document es la unidad mínima indexada. Un record de la API origen
// se convierte en uno o más documentos según la estrategia de chunking.
type Document struct {
	ID       string                 `json:"id"`        // único: <endpoint>:<record_id>
	Endpoint string                 `json:"endpoint"`  // p.ej "clients", "projects"
	RecordID string                 `json:"record_id"` // id original del record
	Text     string                 `json:"text"`      // texto indexable
	Metadata map[string]interface{} `json:"metadata"`  // campos para filtros
	RawJSON  string                 `json:"raw_json"`  // record completo, para citaciones
}

// SearchResult es un Document con su score relativo a una query.
type SearchResult struct {
	Document Document `json:"document"`
	Score    float64  `json:"score"` // BM25 score (no normalizado)
}

// Store es la interface mínima que necesitan Indexa y Sabio.
type Store interface {
	Upsert(docs []Document) error
	// Search devuelve los topK documentos más relevantes para la query.
	// Si endpointFilter no está vacío, filtra solo a esos endpoints.
	Search(query string, topK int, endpointFilter []string) ([]SearchResult, error)
	Stats() (map[string]int, error)
	Close() error
}

// FileStore es BM25 in-memory persistido en JSON. Apta para hasta
// ~50k documentos. Para más, migrar a una implementación con tsvector
// o un motor dedicado.
type FileStore struct {
	path string
	mu   sync.RWMutex
	docs map[string]Document // por ID

	// Índice invertido (lazy: se reconstruye al primer Search tras un Upsert).
	dirty       bool
	docTokens   map[string][]string         // docID -> tokens (con repeticiones)
	docFreqs    map[string]map[string]int   // docID -> term -> tf
	docLengths  map[string]int              // docID -> número de tokens
	avgDocLen   float64                     // promedio
	termInDocs  map[string]int              // term -> número de docs en los que aparece
}

// Parámetros estándar de BM25.
const (
	bm25K1 = 1.5
	bm25B  = 0.75
)

// NewFileStore abre o crea un store en el path dado.
func NewFileStore(path string) (*FileStore, error) {
	s := &FileStore{
		path:       path,
		docs:       map[string]Document{},
		dirty:      true,
		docTokens:  map[string][]string{},
		docFreqs:   map[string]map[string]int{},
		docLengths: map[string]int{},
		termInDocs: map[string]int{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// fileFormat es la estructura serializada en disco.
type fileFormat struct {
	Version int        `json:"version"`
	Docs    []Document `json:"documents"`
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("filestore load: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	// Soportamos dos shapes para retrocompatibilidad:
	//   - {"version":1,"documents":[...]}  (v1, también si documents es [])
	//   - [...]                            (legacy)
	trimmed := data
	for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\n' || trimmed[0] == '\r' || trimmed[0] == '\t') {
		trimmed = trimmed[1:]
	}
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var ff fileFormat
		if err := json.Unmarshal(data, &ff); err != nil {
			return fmt.Errorf("filestore parse v1: %w", err)
		}
		for _, d := range ff.Docs {
			s.docs[d.ID] = d
		}
		return nil
	}
	var legacy []Document
	if err := json.Unmarshal(data, &legacy); err != nil {
		return fmt.Errorf("filestore parse: %w", err)
	}
	for _, d := range legacy {
		s.docs[d.ID] = d
	}
	return nil
}

func (s *FileStore) flush() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Document, 0, len(s.docs))
	for _, d := range s.docs {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })

	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	data, err := json.MarshalIndent(fileFormat{Version: 1, Docs: out}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Upsert implementa Store.
func (s *FileStore) Upsert(docs []Document) error {
	s.mu.Lock()
	for _, d := range docs {
		if d.ID == "" {
			s.mu.Unlock()
			return fmt.Errorf("upsert: documento sin ID")
		}
		s.docs[d.ID] = d
	}
	s.dirty = true
	s.mu.Unlock()
	return s.flush()
}

// rebuildIndex regenera los índices BM25 desde s.docs. Debe llamarse
// con el lock tomado en modo escritura, o desde un contexto donde ningún
// Search esté corriendo.
func (s *FileStore) rebuildIndex() {
	s.docTokens = map[string][]string{}
	s.docFreqs = map[string]map[string]int{}
	s.docLengths = map[string]int{}
	s.termInDocs = map[string]int{}

	totalLen := 0
	for id, d := range s.docs {
		tokens := tokenize(d.Endpoint + " " + d.Text)
		freqs := map[string]int{}
		for _, t := range tokens {
			freqs[t]++
		}
		s.docTokens[id] = tokens
		s.docFreqs[id] = freqs
		s.docLengths[id] = len(tokens)
		totalLen += len(tokens)
		for term := range freqs {
			s.termInDocs[term]++
		}
	}
	if len(s.docs) > 0 {
		s.avgDocLen = float64(totalLen) / float64(len(s.docs))
	} else {
		s.avgDocLen = 0
	}
	s.dirty = false
}

// Search implementa Store con ranking BM25.
func (s *FileStore) Search(query string, topK int, endpointFilter []string) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search: query vacía")
	}

	s.mu.Lock()
	if s.dirty {
		s.rebuildIndex()
	}
	s.mu.Unlock()

	allowed := map[string]bool{}
	for _, e := range endpointFilter {
		allowed[e] = true
	}

	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return nil, fmt.Errorf("search: query sin términos útiles tras tokenizar")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	N := float64(len(s.docs))
	if N == 0 {
		return nil, nil
	}

	results := make([]SearchResult, 0, len(s.docs))
	for id, d := range s.docs {
		if len(allowed) > 0 && !allowed[d.Endpoint] {
			continue
		}
		score := 0.0
		dl := float64(s.docLengths[id])
		for _, q := range queryTerms {
			tf := float64(s.docFreqs[id][q])
			if tf == 0 {
				continue
			}
			df := float64(s.termInDocs[q])
			idf := math.Log(1 + (N-df+0.5)/(df+0.5))
			norm := tf * (bm25K1 + 1) /
				(tf + bm25K1*(1-bm25B+bm25B*dl/s.avgDocLen))
			score += idf * norm
		}
		if score > 0 {
			results = append(results, SearchResult{Document: d, Score: score})
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// Stats implementa Store.
func (s *FileStore) Stats() (map[string]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]int{}
	for _, d := range s.docs {
		out[d.Endpoint]++
	}
	return out, nil
}

// Close implementa Store. Asegura que cualquier write pendiente esté en disco.
func (s *FileStore) Close() error {
	return s.flush()
}

// tokenize transforma un texto en términos normalizados.
//   - Lowercase
//   - Separación por todo lo no alfanumérico
//   - Remueve tokens de longitud < 2 (excepto dígitos relevantes)
//   - Mantiene números completos (útil para IDs como "000239", "3522")
func tokenize(text string) []string {
	if text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	for _, r := range lower {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
			continue
		}
		if current.Len() > 0 {
			tokens = appendIfUseful(tokens, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		tokens = appendIfUseful(tokens, current.String())
	}
	return tokens
}

func appendIfUseful(tokens []string, t string) []string {
	if len(t) == 0 {
		return tokens
	}
	// Conservamos números aunque sean cortos (ej "id", "1", "42").
	if len(t) == 1 && !isAllDigits(t) {
		return tokens
	}
	if isStopword(t) {
		return tokens
	}
	return append(tokens, t)
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// stopwords mínimas español + inglés (no aplicamos un stemmer para
// mantener el match sobre IDs y códigos exactos).
var stopwordSet = map[string]bool{
	"el": true, "la": true, "los": true, "las": true, "un": true, "una": true,
	"y": true, "o": true, "a": true, "de": true, "del": true, "al": true,
	"es": true, "se": true, "que": true, "en": true, "por": true, "para": true,
	"con": true, "su": true, "sus": true, "lo": true, "como": true, "mas": true,
	"the": true, "of": true, "and": true, "to": true, "in": true, "is": true,
	"it": true, "for": true, "on": true, "at": true, "by": true, "an": true,
	"as": true, "be": true, "are": true, "was": true,
}

func isStopword(s string) bool { return stopwordSet[s] }
