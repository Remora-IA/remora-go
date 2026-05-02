// frameworkindexa: ingesta data de un JSON-API dump y la persiste en un
// store consultable por framework-sabio (BM25 in-memory).
//
// Comandos:
//
//	./frameworkindexa init [--store <path>]
//	./frameworkindexa index --source <path-json> [--store <path>]
//	                       [--endpoints <csv>] [--max-records <n>] [--dry-run]
//	./frameworkindexa status [--store <path>] [--json]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

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

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
