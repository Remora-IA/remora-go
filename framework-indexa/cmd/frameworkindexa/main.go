// frameworkindexa: ingesta data de un JSON-API dump y la persiste en un
// store consultable por framework-sabio (BM25 in-memory).
//
// Comandos:
//
//	./frameworkindexa init [--store <path>]
//	./frameworkindexa index --source <path-json> [--store <path>]
//	                       [--endpoints <csv>] [--max-records <n>] [--dry-run]
//	./frameworkindexa api-plan --docs-file <path> [--base-url <url>] [--out <path>]
//	./frameworkindexa status [--store <path>] [--json]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"framework-indexa/internal/llm"
	"framework-indexa/sqlbuilder"
	"framework-indexa/store"
)

const defaultStorePath = "data/store.json"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "init":
		cmdInit(os.Args[2:])
	case "index":
		cmdIndex(os.Args[2:])
	case "status":
		cmdStatus(os.Args[2:])
	case "api-plan":
		cmdAPIPlan(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`frameworkindexa: ingestor de APIs externas a un store BM25 local

Uso:
  frameworkindexa init [--store <path>]
  frameworkindexa index --source <path-json> [--store <path>]
                        [--endpoints <csv>] [--max-records <n>] [--dry-run]
  frameworkindexa api-plan --docs-file <path> [--base-url <url>] [--out <path>]
  frameworkindexa status [--store <path>] [--json]

Variables de entorno:
  INDEXA_STORE   override del path por defecto del store
`)
}

func resolveStorePath(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv("INDEXA_STORE"); v != "" {
		return v
	}
	return defaultStorePath
}

type connectorSpec struct {
	Version   string              `json:"version"`
	BaseURL   string              `json:"base_url"`
	AuthTypes []string            `json:"auth_types"`
	Resources []connectorResource `json:"resources"`
	Notes     []string            `json:"notes,omitempty"`
	Sources   []docSource         `json:"sources,omitempty"`
}

type connectorResource struct {
	Name        string         `json:"name"`
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	TableName   string         `json:"table_name"`
	RecordsPath string         `json:"records_path"`
	PrimaryKey  string         `json:"primary_key"`
	Pagination  map[string]any `json:"pagination"`
	Incremental map[string]any `json:"incremental,omitempty"`
}

type docSource struct {
	URL     string `json:"url"`
	Status  string `json:"status"`
	Bytes   int    `json:"bytes,omitempty"`
	Error   string `json:"error,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

func cmdAPIPlan(args []string) {
	fs := flag.NewFlagSet("api-plan", flag.ExitOnError)
	docsFile := fs.String("docs-file", "", "archivo con documentación de la API")
	baseURL := fs.String("base-url", "", "base URL sugerida")
	outPath := fs.String("out", "", "path donde escribir connector spec JSON")
	fs.Parse(args)
	if *docsFile == "" {
		fail("api-plan: --docs-file requerido")
	}
	raw, err := os.ReadFile(*docsFile)
	if err != nil {
		fail("api-plan read docs: %v", err)
	}
	docs, sources := enrichDocsWithURLs(string(raw))
	spec, ok := connectorPlanFromOpenAPI(docs, *baseURL)
	err = nil
	if !ok {
		spec, err = planConnectorWithLLM(truncateText(docs, 120000), *baseURL)
	}
	if err != nil {
		spec = heuristicConnectorPlan(docs, *baseURL)
		spec.Notes = append(spec.Notes, "Plan heurístico: no se pudo usar LLM: "+err.Error())
	}
	spec.Sources = sources
	out, _ := json.MarshalIndent(spec, "", "  ")
	if *outPath != "" {
		if err := os.WriteFile(*outPath, append(out, '\n'), 0644); err != nil {
			fail("api-plan write: %v", err)
		}
	}
	fmt.Println(string(out))
}

func planConnectorWithLLM(docs, baseURL string) (connectorSpec, error) {
	client, err := llm.NewClient()
	if err != nil {
		return connectorSpec{}, err
	}
	system := `Eres Indexa API Planner. Convierte documentación de APIs REST en un ConnectorSpec JSON seguro para sincronizar datos read-only. Devuelve SOLO JSON válido, sin markdown. No incluyas credenciales. Solo endpoints GET listables. Usa records_path JSONPath simple como $, $.data, $.results o $.items.`
	user := fmt.Sprintf(`Base URL sugerida: %s

Documentación:
%s

Devuelve JSON con shape:
{"version":"api_connector.v1","base_url":"...","auth_types":["bearer"],"resources":[{"name":"clients","method":"GET","path":"/clients","table_name":"clients","records_path":"$.data","primary_key":"id","pagination":{"type":"page","page_param":"page","page_size_param":"limit","page_size":100,"max_pages":100},"incremental":{"type":"updated_since","request_param":"updated_since","record_field":"updated_at"}}],"notes":[]}`, baseURL, docs)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	text, err := client.Generate(ctx, system, user)
	if err != nil {
		return connectorSpec{}, err
	}
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	var spec connectorSpec
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &spec); err != nil {
		return connectorSpec{}, err
	}
	return normalizeConnectorSpec(spec, baseURL), nil
}

func connectorPlanFromOpenAPI(docs, fallbackBaseURL string) (connectorSpec, bool) {
	specs := extractJSONObjectStrings(docs)
	for _, raw := range specs {
		var doc map[string]any
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			continue
		}
		if doc["openapi"] == nil && doc["swagger"] == nil {
			continue
		}
		paths, ok := doc["paths"].(map[string]any)
		if !ok || len(paths) == 0 {
			continue
		}
		baseURL := fallbackBaseURL
		if baseURL == "" {
			baseURL = openAPIServerURL(doc)
		}
		resources := []connectorResource{}
		pathNames := make([]string, 0, len(paths))
		for path := range paths {
			pathNames = append(pathNames, path)
		}
		sort.Strings(pathNames)
		for _, path := range pathNames {
			if strings.Contains(path, "{") {
				continue
			}
			methods, ok := paths[path].(map[string]any)
			if !ok || methods["get"] == nil {
				continue
			}
			name := strings.Trim(strings.ReplaceAll(strings.Trim(path, "/"), "/", "_"), "_")
			if name == "" {
				continue
			}
			resources = append(resources, connectorResource{
				Name:        name,
				Method:      "GET",
				Path:        path,
				TableName:   safeName(name),
				RecordsPath: "$",
				PrimaryKey:  "id",
				Pagination:  openAPIPagination(methods["get"]),
			})
			if len(resources) >= 40 {
				break
			}
		}
		if len(resources) == 0 {
			continue
		}
		authTypes := openAPIAuthTypes(doc)
		out := normalizeConnectorSpec(connectorSpec{Version: "api_connector.v1", BaseURL: baseURL, AuthTypes: authTypes, Resources: resources, Notes: []string{"Plan generado determinísticamente desde OpenAPI/Swagger recuperado por Indexa."}}, fallbackBaseURL)
		return out, true
	}
	return connectorSpec{}, false
}

func extractJSONObjectStrings(text string) []string {
	out := []string{}
	for i := 0; i < len(text); i++ {
		if text[i] != '{' {
			continue
		}
		depth := 0
		inString := false
		escape := false
		for j := i; j < len(text); j++ {
			c := text[j]
			if inString {
				if escape {
					escape = false
				} else if c == '\\' {
					escape = true
				} else if c == '"' {
					inString = false
				}
				continue
			}
			switch c {
			case '"':
				inString = true
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					candidate := text[i : j+1]
					if strings.Contains(candidate, `"paths"`) && (strings.Contains(candidate, `"openapi"`) || strings.Contains(candidate, `"swagger"`)) {
						out = append(out, candidate)
					}
					i = j
					break
				}
			}
		}
	}
	return out
}

func openAPIServerURL(doc map[string]any) string {
	servers, _ := doc["servers"].([]any)
	if len(servers) == 0 {
		return ""
	}
	first, _ := servers[0].(map[string]any)
	raw, _ := first["url"].(string)
	return raw
}

func openAPIAuthTypes(doc map[string]any) []string {
	raw, ok := doc["components"].(map[string]any)
	if !ok {
		return []string{"api_key", "basic", "bearer", "none"}
	}
	sec, ok := raw["securitySchemes"].(map[string]any)
	if !ok {
		return []string{"api_key", "basic", "bearer", "none"}
	}
	out := []string{}
	for _, v := range sec {
		m, _ := v.(map[string]any)
		t, _ := m["type"].(string)
		scheme, _ := m["scheme"].(string)
		switch strings.ToLower(t) {
		case "api_key", "apikey":
			out = append(out, "api_key")
		case "http":
			switch strings.ToLower(scheme) {
			case "basic":
				out = append(out, "basic")
			case "bearer":
				out = append(out, "bearer")
			}
		}
	}
	return out
}

func openAPIPagination(method any) map[string]any {
	m, _ := method.(map[string]any)
	params, _ := m["parameters"].([]any)
	names := map[string]bool{}
	for _, p := range params {
		pm, _ := p.(map[string]any)
		name, _ := pm["name"].(string)
		names[strings.ToLower(name)] = true
	}
	if names["page"] {
		pageSize := "limit"
		if names["per_page"] {
			pageSize = "per_page"
		} else if names["page_size"] {
			pageSize = "page_size"
		}
		return map[string]any{"type": "page", "page_param": "page", "page_size_param": pageSize, "page_size": 100, "max_pages": 100}
	}
	if names["offset"] {
		limit := "limit"
		if names["per_page"] {
			limit = "per_page"
		}
		return map[string]any{"type": "offset", "offset_param": "offset", "limit_param": limit, "page_size": 100, "max_pages": 100}
	}
	return map[string]any{"type": "none"}
}

func safeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		return "records"
	}
	if s[0] >= '0' && s[0] <= '9' {
		return "t_" + s
	}
	return s
}

func enrichDocsWithURLs(docs string) (string, []docSource) {
	queue := extractURLs(docs)
	if len(queue) == 0 {
		return docs, nil
	}
	client := &http.Client{Timeout: 25 * time.Second}
	parts := []string{docs, "\n\n--- Documentación recuperada por Indexa ---\n"}
	sources := []docSource{}
	seen := map[string]bool{}
	for len(queue) > 0 && len(sources) < 8 {
		rawURL := queue[0]
		queue = queue[1:]
		if seen[rawURL] {
			continue
		}
		seen[rawURL] = true
		src, text, links := fetchDocURL(client, rawURL)
		sources = append(sources, src)
		for _, link := range links {
			if !seen[link] {
				queue = append(queue, link)
			}
		}
		if text == "" {
			continue
		}
		parts = append(parts, "\n\nFuente: "+rawURL+"\n"+text)
	}
	return truncateText(strings.Join(parts, "\n"), 900000), sources
}

func extractURLs(text string) []string {
	re := regexp.MustCompile(`https?://[^\s<>"']+`)
	matches := re.FindAllString(text, -1)
	out := []string{}
	seen := map[string]bool{}
	for _, m := range matches {
		u := strings.TrimRight(m, ".,);]")
		parsed, err := url.Parse(u)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
		if len(out) >= 5 {
			break
		}
	}
	return out
}

func fetchDocURL(client *http.Client, rawURL string) (docSource, string, []string) {
	src := docSource{URL: rawURL, Status: "fetching"}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		src.Status, src.Error = "error", err.Error()
		return src, "", nil
	}
	req.Header.Set("User-Agent", "Remora-Indexa/0.1 API documentation reader")
	req.Header.Set("Accept", "text/html,application/json,application/yaml,text/yaml,text/plain,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		src.Status, src.Error = "error", err.Error()
		return src, "", nil
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	src.Bytes = len(raw)
	if resp.StatusCode >= 300 {
		src.Status = fmt.Sprintf("http_%d", resp.StatusCode)
		src.Error = snippetText(string(raw), 240)
		return src, "", nil
	}
	links := discoverDocLinks(raw, resp.Request.URL.String())
	text := extractReadableDocText(raw, resp.Header.Get("Content-Type"))
	if looksLikeOpenAPIDoc(text) {
		text = truncateText(text, 700000)
	} else {
		text = truncateText(text, 30000)
	}
	src.Status = "ok"
	src.Snippet = snippetText(text, 220)
	return src, text, links
}

func discoverDocLinks(raw []byte, base string) []string {
	text := string(raw)
	re := regexp.MustCompile(`(?i)(?:href|src|spec-url)=["']([^"']+)["']`)
	out := []string{}
	seen := map[string]bool{}
	for _, match := range re.FindAllStringSubmatch(text, -1) {
		if len(match) < 2 || !looksLikeDocSpec(match[1]) {
			continue
		}
		u, err := url.Parse(match[1])
		if err != nil {
			continue
		}
		baseURL, err := url.Parse(base)
		if err == nil {
			u = baseURL.ResolveReference(u)
		}
		resolved := u.String()
		if !seen[resolved] {
			seen[resolved] = true
			out = append(out, resolved)
		}
	}
	return out
}

func looksLikeDocSpec(path string) bool {
	p := strings.ToLower(path)
	return strings.Contains(p, "openapi") || strings.Contains(p, "swagger") || strings.Contains(p, "api-doc") || strings.Contains(p, "bundled-") || strings.HasSuffix(p, ".json") || strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml")
}

func extractReadableDocText(raw []byte, contentType string) string {
	text := string(raw)
	lowerType := strings.ToLower(contentType)
	if strings.Contains(lowerType, "html") || strings.Contains(strings.ToLower(text[:minInt(len(text), 200)]), "<html") {
		text = stripTagBlocks(text, "script")
		text = stripTagBlocks(text, "style")
		text = regexp.MustCompile(`(?is)<br\s*/?>`).ReplaceAllString(text, "\n")
		text = regexp.MustCompile(`(?is)</(p|div|section|article|h[1-6]|li|tr|pre|code)>`).ReplaceAllString(text, "\n")
		text = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(text, " ")
	}
	text = htmlEntityCleanup(text)
	text = regexp.MustCompile(`[ \t\r\f\v]+`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`\n\s+`).ReplaceAllString(text, "\n")
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

func looksLikeOpenAPIDoc(text string) bool {
	head := text[:minInt(len(text), 2000)]
	return (strings.Contains(head, `"openapi"`) || strings.Contains(head, `"swagger"`)) && strings.Contains(text, `"paths"`)
}

func stripTagBlocks(text, tag string) string {
	re := regexp.MustCompile(`(?is)<` + tag + `\b[^>]*>.*?</` + tag + `>`)
	return re.ReplaceAllString(text, " ")
}

func htmlEntityCleanup(text string) string {
	repl := strings.NewReplacer(
		"&nbsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
	)
	return repl.Replace(text)
}

func truncateText(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max] + "\n...[truncado por Indexa]..."
}

func snippetText(text string, max int) string {
	text = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(text, " "))
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func heuristicConnectorPlan(docs, baseURL string) connectorSpec {
	resources := []connectorResource{}
	seen := map[string]bool{}
	for _, tok := range strings.Fields(docs) {
		if !strings.HasPrefix(tok, "/") {
			continue
		}
		path := strings.Trim(tok, " ,.;:)")
		if strings.ContainsAny(path, "{}") || seen[path] {
			continue
		}
		seen[path] = true
		name := strings.ReplaceAll(strings.Trim(path, "/"), "/", "_")
		if name == "" || strings.Contains(name, ":") {
			continue
		}
		resources = append(resources, connectorResource{Name: name, Method: "GET", Path: path, TableName: name, RecordsPath: "$.data", PrimaryKey: "id", Pagination: map[string]any{"type": "none"}})
		if len(resources) >= 8 {
			break
		}
	}
	if len(resources) == 0 {
		resources = append(resources, connectorResource{Name: "records", Method: "GET", Path: "/", TableName: "records", RecordsPath: "$", PrimaryKey: "id", Pagination: map[string]any{"type": "none"}})
	}
	return normalizeConnectorSpec(connectorSpec{Version: "api_connector.v1", BaseURL: baseURL, AuthTypes: []string{"bearer", "api_key", "basic", "none"}, Resources: resources}, baseURL)
}

func normalizeConnectorSpec(spec connectorSpec, fallbackBaseURL string) connectorSpec {
	if spec.Version == "" {
		spec.Version = "api_connector.v1"
	}
	if spec.BaseURL == "" {
		spec.BaseURL = fallbackBaseURL
	}
	spec.AuthTypes = normalizeAuthTypes(spec.AuthTypes)
	if len(spec.AuthTypes) == 0 {
		spec.AuthTypes = []string{"bearer", "api_key", "basic", "none"}
	}
	for i := range spec.Resources {
		r := &spec.Resources[i]
		if r.Method == "" {
			r.Method = "GET"
		}
		if r.TableName == "" {
			r.TableName = r.Name
		}
		if r.RecordsPath == "" {
			r.RecordsPath = "$.data"
		}
		if r.PrimaryKey == "" {
			r.PrimaryKey = "id"
		}
		if r.Pagination == nil {
			r.Pagination = map[string]any{"type": "none"}
		}
	}
	return spec
}

func normalizeAuthTypes(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range values {
		key := strings.ToLower(strings.TrimSpace(v))
		key = strings.ReplaceAll(key, "-", "_")
		key = strings.ReplaceAll(key, " ", "_")
		switch key {
		case "bearer", "bearer_token", "token":
			key = "bearer"
		case "api_key", "apikey", "x_api_key":
			key = "api_key"
		case "basic", "basic_auth":
			key = "basic"
		case "none", "no_auth":
			key = "none"
		default:
			continue
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	storeFlag := fs.String("store", "", "path del archivo store")
	fs.Parse(args)

	path := resolveStorePath(*storeFlag)
	st, err := store.NewFileStore(path)
	if err != nil {
		fail("init: %v", err)
	}
	defer st.Close()
	fmt.Printf("store inicializado en %s\n", path)
}

func cmdIndex(args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	source := fs.String("source", "", "path del JSON con la data")
	storeFlag := fs.String("store", "", "path del store")
	endpoints := fs.String("endpoints", "", "lista CSV de endpoints a indexar (vacío = todos)")
	maxRecords := fs.Int("max-records", 0, "máximo records por endpoint (0 = sin límite)")
	dryRun := fs.Bool("dry-run", false, "no persiste, solo reporta qué se indexaría")
	sqlitePath := fs.String("sqlite", "data/panalbit.db", "path donde generar la DB SQLite (vacío = no generar)")
	fs.Parse(args)

	if *source == "" {
		fail("index: --source requerido")
	}
	storePath := resolveStorePath(*storeFlag)

	st, err := store.NewFileStore(storePath)
	if err != nil {
		fail("store: %v", err)
	}
	defer st.Close()

	raw, err := os.ReadFile(*source)
	if err != nil {
		fail("read source: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		fail("parse source: %v", err)
	}

	// Soportamos:
	//   {"endpoints": {"clients": [...], ...}}
	//   {"clients": [...], ...}
	endpointsMap := top
	if epRaw, ok := top["endpoints"]; ok {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(epRaw, &inner); err == nil {
			endpointsMap = inner
		}
	}

	wantedEndpoints := map[string]bool{}
	for _, e := range strings.Split(*endpoints, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			wantedEndpoints[e] = true
		}
	}

	endpointNames := make([]string, 0, len(endpointsMap))
	for name := range endpointsMap {
		if len(wantedEndpoints) > 0 && !wantedEndpoints[name] {
			continue
		}
		// Endpoints especiales del shape original.
		if name == "auth_token" || name == "login" {
			continue
		}
		endpointNames = append(endpointNames, name)
	}
	sort.Strings(endpointNames)

	totalIndexed := 0
	for _, ep := range endpointNames {
		records := extractRecords(endpointsMap[ep])
		if len(records) == 0 {
			fmt.Printf("  %-40s   0 records (skip)\n", ep)
			continue
		}
		if *maxRecords > 0 && len(records) > *maxRecords {
			records = records[:*maxRecords]
		}

		docs := make([]store.Document, 0, len(records)+1)

		// Summary doc sintético: permite que BM25 conteste preguntas
		// agregadas tipo "cuántos clientes tengo" sin tener que recuperar
		// los N registros individuales.
		summary := buildSummaryDoc(ep, records)
		docs = append(docs, summary)

		for i, rec := range records {
			text, recID := buildDocumentText(ep, rec, i)
			rawJSON, _ := json.Marshal(rec)
			docs = append(docs, store.Document{
				ID:       ep + ":" + recID,
				Endpoint: ep,
				RecordID: recID,
				Text:     text,
				Metadata: map[string]interface{}{"index": i},
				RawJSON:  string(rawJSON),
			})
		}
		if !*dryRun && len(docs) > 0 {
			if err := st.Upsert(docs); err != nil {
				fail("upsert %s: %v", ep, err)
			}
		}
		totalIndexed += len(docs)
		mark := ""
		if *dryRun {
			mark = " (dry-run)"
		}
		fmt.Printf("  %-40s %3d records%s\n", ep, len(docs), mark)
	}

	fmt.Printf("\ntotal indexado: %d documentos en %s\n", totalIndexed, storePath)

	// SQLite mirror para queries agregadas / con joins.
	if *sqlitePath != "" && !*dryRun {
		dbMap := map[string][]map[string]any{}
		for _, ep := range endpointNames {
			records := extractRecords(endpointsMap[ep])
			if len(records) == 0 {
				continue
			}
			if *maxRecords > 0 && len(records) > *maxRecords {
				records = records[:*maxRecords]
			}
			// Convertir []map[string]interface{} a []map[string]any (mismo tipo).
			conv := make([]map[string]any, len(records))
			for i, r := range records {
				conv[i] = r
			}
			dbMap[ep] = conv
		}
		fmt.Printf("\nGenerando SQLite en %s...\n", *sqlitePath)
		res, err := sqlbuilder.BuildFromEndpoints(*sqlitePath, dbMap, sqlbuilder.BuildOptions{})
		if err != nil {
			fail("sqlite build: %v", err)
		}
		for _, t := range res.Tables {
			fmt.Printf("  %-40s %5d rows × %3d cols\n", t.Name, t.Rows, t.Columns)
		}
		fmt.Printf("\nSQLite: %d tablas, %d filas totales\n", len(res.Tables), res.TotalRows)
	}
}

func cmdStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	storeFlag := fs.String("store", "", "path del store")
	asJSON := fs.Bool("json", false, "salida JSON")
	fs.Parse(args)

	st, err := store.NewFileStore(resolveStorePath(*storeFlag))
	if err != nil {
		fail("status: %v", err)
	}
	defer st.Close()

	stats, _ := st.Stats()
	if *asJSON {
		out, _ := json.MarshalIndent(stats, "", "  ")
		fmt.Println(string(out))
		return
	}
	if len(stats) == 0 {
		fmt.Println("store vacío. Corre 'frameworkindexa index --source <json>' primero.")
		return
	}
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	total := 0
	for _, k := range keys {
		fmt.Printf("  %-40s %4d\n", k, stats[k])
		total += stats[k]
	}
	fmt.Printf("\ntotal: %d documentos\n", total)
}

// extractRecords devuelve la lista de records para un endpoint.
func extractRecords(raw json.RawMessage) []map[string]interface{} {
	var arr []map[string]interface{}
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		for _, k := range []string{"data", "items", "results"} {
			if v, ok := obj[k]; ok {
				var inner []map[string]interface{}
				if err := json.Unmarshal(v, &inner); err == nil {
					return inner
				}
			}
		}
		var single map[string]interface{}
		if err := json.Unmarshal(raw, &single); err == nil && len(single) > 0 {
			return []map[string]interface{}{single}
		}
	}
	return nil
}

// buildSummaryDoc genera un documento sintético por endpoint con conteo
// total y, cuando aplique, distribuciones por campos comunes (status, active,
// type, etc.). Permite que queries tipo "cuántos clientes tengo", "qué
// status hay en billing_documents", etc, encuentren un doc relevante en BM25.
func buildSummaryDoc(endpoint string, records []map[string]interface{}) store.Document {
	total := len(records)

	// Agregamos por algunos campos típicos si existen.
	commonFields := []string{"active", "status", "status_id", "type", "type_id", "currency_code", "country_code", "is_active", "deleted"}
	aggregates := map[string]map[string]int{}
	for _, f := range commonFields {
		aggregates[f] = map[string]int{}
	}

	// Sample de IDs para que el LLM pueda referenciar.
	sampleIDs := []string{}
	for i, rec := range records {
		if i < 5 {
			if id := pickID(rec); id != "" {
				sampleIDs = append(sampleIDs, id)
			}
		}
		for _, f := range commonFields {
			if v, ok := rec[f]; ok && v != nil {
				aggregates[f][fmt.Sprintf("%v", v)]++
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(endpoint)
	sb.WriteString(" SUMMARY:\n")
	fmt.Fprintf(&sb, "Total de registros en %s: %d\n", endpoint, total)
	fmt.Fprintf(&sb, "Endpoint: %s\n", endpoint)
	fmt.Fprintf(&sb, "Cantidad: %d\n", total)
	fmt.Fprintf(&sb, "Conteo: %d\n", total)
	fmt.Fprintf(&sb, "Hay %d %s en total.\n", total, endpoint)

	for _, f := range commonFields {
		dist := aggregates[f]
		if len(dist) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "Distribución por %s:\n", f)
		keys := make([]string, 0, len(dist))
		for k := range dist {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&sb, "  %s = %s: %d\n", f, k, dist[k])
		}
	}

	if len(sampleIDs) > 0 {
		fmt.Fprintf(&sb, "Ejemplos de IDs: %s\n", strings.Join(sampleIDs, ", "))
	}

	rawMeta, _ := json.Marshal(map[string]any{
		"endpoint":   endpoint,
		"total":      total,
		"aggregates": aggregates,
		"is_summary": true,
	})

	return store.Document{
		ID:       endpoint + ":__summary__",
		Endpoint: endpoint,
		RecordID: "__summary__",
		Text:     sb.String(),
		Metadata: map[string]interface{}{"is_summary": true, "total": total},
		RawJSON:  string(rawMeta),
	}
}

// buildDocumentText construye una representación textual indexable.
// Aplana clave:valor para que BM25 pueda matchear tanto nombres de
// campos como valores. Devuelve también un recordID estable.
func buildDocumentText(endpoint string, rec map[string]interface{}, idx int) (text string, recordID string) {
	recordID = pickID(rec)
	if recordID == "" {
		recordID = fmt.Sprintf("idx_%d", idx)
	}

	var sb strings.Builder
	sb.WriteString(endpoint)
	sb.WriteString(" record:\n")

	keys := make([]string, 0, len(rec))
	for k := range rec {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := rec[k]
		if v == nil {
			continue
		}
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(stringify(v, 0))
		sb.WriteString("\n")
	}
	return sb.String(), recordID
}

func pickID(rec map[string]interface{}) string {
	for _, k := range []string{"id", "ID", "uuid", "code"} {
		if v, ok := rec[k]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

func stringify(v interface{}, depth int) string {
	if depth > 2 {
		return "…"
	}
	switch x := v.(type) {
	case string:
		return x
	case float64, int, int64, bool:
		return fmt.Sprintf("%v", x)
	case []interface{}:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, stringify(e, depth+1))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			if x[k] == nil {
				continue
			}
			parts = append(parts, k+"="+stringify(x[k], depth+1))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
