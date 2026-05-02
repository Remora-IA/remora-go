// panalbit-sync: descarga TODA la data de la API Panalbit / TimeBilling
// y la deja en un dump.json con shape {"endpoints": {<resource>: [records]}}
// que framework-indexa puede consumir para rebuild del store BM25.
//
// Uso:
//
//	export PANALBIT_USER='soporte@panalbit.com'
//	export PANALBIT_PASSWORD='xxx'
//	export PANALBIT_APP_KEY='panalbit'
//	export PANALBIT_BASE_URL='https://panalbit.thetimebilling.com/time_tracking/api/v2'
//
//	./panalbit-sync --out data/dump.json [--resources clients,projects] [--page-size 3000] [--concurrency 4]
//
// El programa loguea progreso por stderr y deja el dump en --out.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Resources listables a sincronizar (top-level GET en OpenAPI).
// Excluimos `login` (POST de auth) y `webhooks` (suele requerir permisos extra).
var defaultResources = []string{
	"activities", "advances", "agreements", "areas", "bank_accounts",
	"banks", "billing_document_statuses", "billing_document_types",
	"billing_documents", "charges", "client_groups", "clients",
	"codes", "contacts", "countries", "currencies", "expenses",
	"expenses_categories", "languages", "law_firms", "legal_entities",
	"milestones", "national_id_document_types", "payment_concepts",
	"payment_conditions", "payment_types", "payments", "project_areas",
	"project_sub_areas", "project_types", "projects", "providers",
	"rates", "related_document_types", "retributions", "time_entries",
	"user_areas", "user_categories", "users",
}

type config struct {
	BaseURL     string
	User        string
	Password    string
	AppKey      string
	OutPath     string
	Resources   []string
	PageSize    int
	Concurrency int
	Timeout     time.Duration
	UpdatedFrom string
}

type loginResponse struct {
	AuthToken string `json:"auth_token"`
	UserID    string `json:"user_id"`
}

type resourceResult struct {
	Name    string
	Records []map[string]any
	Err     error
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fail("config: %v", err)
	}

	logf("=== Panalbit Sync ===")
	logf("base url:    %s", cfg.BaseURL)
	logf("user:        %s", cfg.User)
	logf("out:         %s", cfg.OutPath)
	logf("resources:   %d", len(cfg.Resources))
	logf("page size:   %d", cfg.PageSize)
	logf("concurrency: %d", cfg.Concurrency)
	if cfg.UpdatedFrom != "" {
		logf("updated from: %s (incremental)", cfg.UpdatedFrom)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	httpClient := &http.Client{Timeout: 60 * time.Second}

	authToken, err := login(ctx, httpClient, cfg)
	if err != nil {
		fail("login: %v", err)
	}
	logf("login OK (token %s…)", authToken[:8])

	results := fetchAll(ctx, httpClient, cfg, authToken)

	dump := map[string]any{
		"endpoints": map[string]any{},
		"_meta": map[string]any{
			"generated_at":  time.Now().UTC().Format(time.RFC3339),
			"base_url":      cfg.BaseURL,
			"user":          cfg.User,
			"updated_from":  cfg.UpdatedFrom,
			"resource_keys": cfg.Resources,
		},
	}
	endpoints := dump["endpoints"].(map[string]any)

	totalRecords := 0
	failed := 0
	for _, r := range results {
		if r.Err != nil {
			logf("  %-30s ERROR: %v", r.Name, r.Err)
			failed++
			continue
		}
		endpoints[r.Name] = r.Records
		totalRecords += len(r.Records)
		logf("  %-30s %6d records", r.Name, len(r.Records))
	}

	if err := writeDump(cfg.OutPath, dump); err != nil {
		fail("write dump: %v", err)
	}

	fi, _ := os.Stat(cfg.OutPath)
	sizeMB := float64(0)
	if fi != nil {
		sizeMB = float64(fi.Size()) / (1024 * 1024)
	}
	logf("\n=== Sync done ===")
	logf("total:    %d records across %d resources", totalRecords, len(endpoints))
	logf("failed:   %d resources", failed)
	logf("dump:     %s (%.2f MB)", cfg.OutPath, sizeMB)

	if failed > 0 {
		os.Exit(1)
	}
}

func parseFlags() (config, error) {
	out := flag.String("out", "data/dump.json", "path donde escribir el dump")
	resourcesCSV := flag.String("resources", "", "lista CSV de recursos (vacío = todos los default)")
	pageSize := flag.Int("page-size", 3000, "page size (max 3000)")
	concurrency := flag.Int("concurrency", 4, "número de recursos a sincronizar en paralelo")
	timeoutFlag := flag.Duration("timeout", 30*time.Minute, "timeout global")
	updatedFrom := flag.String("updated-from", "", "filtra updated_from (ej 2026-01-01); si vacío, full sync")
	flag.Parse()

	cfg := config{
		BaseURL:     envOr("PANALBIT_BASE_URL", "https://panalbit.thetimebilling.com/time_tracking/api/v2"),
		User:        os.Getenv("PANALBIT_USER"),
		Password:    os.Getenv("PANALBIT_PASSWORD"),
		AppKey:      envOr("PANALBIT_APP_KEY", "panalbit"),
		OutPath:     *out,
		PageSize:    *pageSize,
		Concurrency: *concurrency,
		Timeout:     *timeoutFlag,
		UpdatedFrom: *updatedFrom,
	}

	if cfg.User == "" || cfg.Password == "" {
		return cfg, errors.New("PANALBIT_USER y PANALBIT_PASSWORD son requeridos")
	}
	if cfg.PageSize < 10 || cfg.PageSize > 3000 {
		return cfg, fmt.Errorf("page-size fuera de rango [10,3000]: %d", cfg.PageSize)
	}
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}

	cfg.Resources = defaultResources
	if *resourcesCSV != "" {
		picked := []string{}
		for _, r := range strings.Split(*resourcesCSV, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				picked = append(picked, r)
			}
		}
		cfg.Resources = picked
	}
	return cfg, nil
}

func login(ctx context.Context, c *http.Client, cfg config) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"user":     cfg.User,
		"password": cfg.Password,
		"app_key":  cfg.AppKey,
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", cfg.BaseURL+"/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet(raw, 200))
	}
	var lr loginResponse
	if err := json.Unmarshal(raw, &lr); err != nil {
		return "", fmt.Errorf("parse login: %v", err)
	}
	if lr.AuthToken == "" {
		return "", fmt.Errorf("respuesta sin auth_token: %s", snippet(raw, 200))
	}
	return lr.AuthToken, nil
}

// fetchAll itera sobre cfg.Resources en paralelo (limitado por concurrency).
func fetchAll(ctx context.Context, c *http.Client, cfg config, token string) []resourceResult {
	results := make([]resourceResult, len(cfg.Resources))
	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	for i, name := range cfg.Resources {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, name string) {
			defer wg.Done()
			defer func() { <-sem }()
			records, err := fetchResource(ctx, c, cfg, token, name)
			results[i] = resourceResult{Name: name, Records: records, Err: err}
		}(i, name)
	}
	wg.Wait()

	sort.SliceStable(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results
}

// fetchResource pagina un endpoint con page_size hasta agotarlo.
// Detecta el fin de paginación cuando recibimos < page_size resultados.
func fetchResource(ctx context.Context, c *http.Client, cfg config, token, resource string) ([]map[string]any, error) {
	all := make([]map[string]any, 0, cfg.PageSize)
	page := 1
	for {
		batch, err := fetchPage(ctx, c, cfg, token, resource, page)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}
		all = append(all, batch...)
		if len(batch) < cfg.PageSize {
			return all, nil
		}
		page++
		if page > 1000 { // sanity
			return all, fmt.Errorf("safety break: too many pages")
		}
	}
}

func fetchPage(ctx context.Context, c *http.Client, cfg config, token, resource string, page int) ([]map[string]any, error) {
	q := url.Values{}
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("page_size", fmt.Sprintf("%d", cfg.PageSize))
	if cfg.UpdatedFrom != "" {
		q.Set("updated_from", cfg.UpdatedFrom)
	}
	full := cfg.BaseURL + "/" + resource + "?" + q.Encode()

	req, _ := http.NewRequestWithContext(ctx, "GET", full, nil)
	req.Header.Set("AUTHTOKEN", token)
	req.Header.Set("Accept", "application/json")

	// Reintento simple ante errores transitorios o 5xx.
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := c.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
			continue
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet(raw, 200))
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
			continue
		}
		if resp.StatusCode >= 300 {
			// 4xx no se reintenta. Algunos endpoints pueden estar deshabilitados.
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet(raw, 200))
		}
		return parseListResponse(raw)
	}
	return nil, lastErr
}

// parseListResponse acepta:
//   - []object (lo que devuelve la API normalmente)
//   - {"data": [...]} / {"items": [...]} / {"results": [...]} (defensivo)
//   - {} (objeto único — lo envolvemos en lista de uno)
func parseListResponse(raw []byte) ([]map[string]any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}
	var asArray []map[string]any
	if err := json.Unmarshal(trimmed, &asArray); err == nil {
		return asArray, nil
	}
	var asObj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &asObj); err == nil {
		for _, k := range []string{"data", "items", "results"} {
			if v, ok := asObj[k]; ok {
				var inner []map[string]any
				if err := json.Unmarshal(v, &inner); err == nil {
					return inner, nil
				}
			}
		}
		// Single object → wrap.
		var single map[string]any
		if err := json.Unmarshal(trimmed, &single); err == nil && len(single) > 0 {
			return []map[string]any{single}, nil
		}
	}
	return nil, fmt.Errorf("respuesta no parseable: %s", snippet(raw, 200))
}

func writeDump(path string, dump map[string]any) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(dump); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func snippet(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func logf(f string, args ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", args...)
}

func fail(f string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+f+"\n", args...)
	os.Exit(1)
}
