package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestScoreSQLiteUsesSemanticMappingsWithoutBusinessSpecificFallback(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acme.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE customers (id TEXT PRIMARY KEY, name TEXT)`,
		`CREATE TABLE invoices (id TEXT PRIMARY KEY, customer_id TEXT, status TEXT, due_date TEXT, amount REAL)`,
		`INSERT INTO customers (id, name) VALUES ('c1', 'Cliente Uno'), ('c2', 'Cliente Dos')`,
		`INSERT INTO invoices (id, customer_id, status, due_date, amount) VALUES ('i1', 'c1', 'open', '2026-01-01', 1000)`,
		`INSERT INTO invoices (id, customer_id, status, due_date, amount) VALUES ('i2', 'c2', 'open', '2025-01-01', 9000)`,
		`INSERT INTO invoices (id, customer_id, status, due_date, amount) VALUES ('i3', 'c2', 'paid', '2024-01-01', 50000)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	items, _, err := scoreSQLite(dbPath, collectionScoring{
		EntityTable:      "customers",
		EntityIDColumn:   "id",
		EntityNameColumn: "name",
		ItemTable:        "invoices",
		ItemEntityColumn: "customer_id",
		AmountColumn:     "amount",
		StatusColumn:     "status",
		DateColumn:       "due_date",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items len=%d items=%#v", len(items), items)
	}
	if got := items[0].EntityRef.ID; got != "c2" {
		t.Fatalf("selected=%s want c2; items=%#v", got, items)
	}
}

func TestInferScoringModelNeedsSemanticConfiguration(t *testing.T) {
	_, err := inferScoringModel(semanticPack{})
	if err == nil {
		t.Fatal("expected configuration error")
	}
}

func TestLoadSemanticPackAcceptsGenericBusiness(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sabio.business.json")
	raw := `{
		"business_id": "acme",
		"primary_entities": {
			"customer": {"table": "customers", "scope_key": "id", "display_column": "name"},
			"invoice": {"table": "invoices", "scope_column": "customer_id"}
		},
		"scope_policies": {"scope_entity": "customer"}
	}`
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	pack, err := loadSemanticPack(path)
	if err != nil {
		t.Fatal(err)
	}
	model, err := inferScoringModel(pack)
	if err != nil {
		t.Fatal(err)
	}
	if model.EntityTable != "customers" || model.ItemTable != "invoices" {
		t.Fatalf("unexpected model %#v", model)
	}
}

func TestPersistAnalysisPlanWritesTangibleJSONAndSQL(t *testing.T) {
	cwd := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	model := collectionScoring{
		EntityTable:      "customers",
		EntityIDColumn:   "id",
		EntityNameColumn: "name",
		ItemTable:        "invoices",
		ItemEntityColumn: "customer_id",
		ItemJoinColumn:   "id",
		AmountColumn:     "amount",
		StatusColumn:     "status",
		DateColumn:       "due_date",
	}
	paths := persistAnalysisPlan("acme", model)
	for _, path := range []string{paths.SchemaPath, paths.PlanPath, paths.SQLPath} {
		if path == "" {
			t.Fatalf("expected non-empty path in %#v", paths)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	sqlRaw, err := os.ReadFile(paths.SQLPath)
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(sqlRaw)
	for _, want := range []string{"FROM \"invoices\" i", "JOIN \"customers\" e", "COALESCE(CAST(i.\"amount\" AS REAL), 0)"} {
		if !strings.Contains(sqlText, want) {
			t.Fatalf("expected SQL to contain %q, got:\n%s", want, sqlText)
		}
	}
	var plan struct {
		ArtifactType string            `json:"artifact_type"`
		Model        collectionScoring `json:"model"`
		SQLFile      string            `json:"sql_file"`
	}
	raw, err := os.ReadFile(paths.PlanPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.ArtifactType != "analysis.plan.v1" || plan.Model.EntityTable != "customers" || plan.SQLFile == "" {
		t.Fatalf("unexpected plan %#v", plan)
	}
}

func TestLoadPersistedAnalysisPlanReusesConfiguredModel(t *testing.T) {
	cwd := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	original := collectionScoring{
		EntityTable:      "configured_entities",
		EntityIDColumn:   "uuid",
		EntityNameColumn: "display_name",
		ItemTable:        "configured_items",
		ItemEntityColumn: "entity_uuid",
		ItemJoinColumn:   "uuid",
		AmountColumn:     "balance",
	}
	persistAnalysisPlan("acme", original)
	loaded, ok := loadPersistedAnalysisPlan("acme")
	if !ok {
		t.Fatal("expected persisted plan to load")
	}
	if loaded.EntityTable != original.EntityTable || loaded.AmountColumn != original.AmountColumn {
		t.Fatalf("loaded=%#v original=%#v", loaded, original)
	}
}

func TestLoadJSONPayloadPrefersPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "priority.json")
	if err := os.WriteFile(path, []byte(`{"source":"path","text":"saldo; > 30"}`), 0644); err != nil {
		t.Fatal(err)
	}
	got := loadJSONPayload(path, `{"source":"inline"}`)
	if got != `{"source":"path","text":"saldo; > 30"}` {
		t.Fatalf("expected file payload, got %q", got)
	}
}

func TestLoadJSONPayloadFallsBackToInline(t *testing.T) {
	got := loadJSONPayload("", `{"source":"inline"}`)
	if got != `{"source":"inline"}` {
		t.Fatalf("expected inline fallback, got %q", got)
	}
}

func TestRadarDeepDivePriorityItemFindsSelectedEntity(t *testing.T) {
	raw := `{
		"priority_item": {
			"deudor_id": "cust_1",
			"deudor": "Cliente Uno",
			"score": 82
		},
		"items": [
			{"deudor_id": "cust_2", "deudor": "Cliente Dos", "score": 50},
			{"deudor_id": "cust_1", "deudor": "Cliente Uno", "score": 82}
		]
	}`
	item := radarDeepDivePriorityItem(raw, "cust_1")
	if got := jsonStringFromMap(item, "deudor_id"); got != "cust_1" {
		t.Fatalf("selected item=%#v", item)
	}
}

func TestRadarDeepDiveNarrativeIncludesUsefulSections(t *testing.T) {
	text := radarDeepDiveNarrative(
		"Cliente Uno",
		"Se priorizó porque quedó rankeado #1 y score 82/100.",
		[]string{"saldo abierto estimado: 9000", "mora máxima observada: 120 días"},
		"Riesgo alto.",
		"La mora ya está en 120 días.",
		"Se observan 3 documentos abiertos.",
		[]string{"falta validar email de contacto"},
		[]string{"pedir a Sabio contexto 360"},
		[]string{"si luego quisieras actuar, preparar borrador"},
		[]string{"¿por qué quedó por encima de casos parecidos?"},
	)
	for _, want := range []string{
		"Resumen del análisis profundo de Cliente Uno",
		"Por qué se priorizó:",
		"Qué sabemos:",
		"Qué falta:",
		"Qué haría después, sin ejecutarlo:",
		"Preguntas abiertas:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected narrative to contain %q, got:\n%s", want, text)
		}
	}
}

func TestRadarManifestParamReferencesStayWithinCommandContract(t *testing.T) {
	var m struct {
		Commands map[string]struct {
			Args     []string          `json:"args"`
			Params   []string          `json:"params"`
			Defaults map[string]string `json:"defaults"`
		} `json:"commands"`
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "framework.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	tokenRe := regexp.MustCompile(`\{params\.([a-zA-Z0-9_]+)\}`)
	for name, cmd := range m.Commands {
		declared := map[string]bool{}
		for _, param := range cmd.Params {
			declared[param] = true
		}
		for param := range cmd.Defaults {
			declared[param] = true
		}
		for _, arg := range cmd.Args {
			matches := tokenRe.FindAllStringSubmatch(arg, -1)
			for _, match := range matches {
				if !declared[match[1]] {
					t.Fatalf("command %s references undeclared param %q in arg %q", name, match[1], arg)
				}
			}
		}
	}
}

func TestRadarManifestPrioritizeResolvesWithoutDeepDiveParams(t *testing.T) {
	var m struct {
		Commands map[string]struct {
			Args     []string          `json:"args"`
			Params   []string          `json:"params"`
			Defaults map[string]string `json:"defaults"`
		} `json:"commands"`
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "framework.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	cmd, ok := m.Commands["prioritize"]
	if !ok {
		t.Fatal("missing prioritize command")
	}
	params := map[string]string{}
	for key, value := range cmd.Defaults {
		params[key] = value
	}
	params["business_id"] = "acme"
	params["semantic_pack"] = "/tmp/semantic.json"
	params["dataset_artifact"] = "/tmp/dataset.json"
	params["context_b64"] = "ctx"
	joined := strings.Join(cmd.Args, " ")
	for _, forbidden := range []string{"{params.priority_list_path}", "{params.priority_list_json}", "{params.strategy_path}", "{params.strategy_json}"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("prioritize args should not reference %s: %v", forbidden, cmd.Args)
		}
	}
	for _, param := range cmd.Params {
		if _, ok := params[param]; !ok {
			t.Fatalf("missing declared param %s", param)
		}
	}
}

func TestRadarManifestDeepDiveResolvesPathAndFallbackParams(t *testing.T) {
	var m struct {
		Commands map[string]struct {
			Args     []string          `json:"args"`
			Params   []string          `json:"params"`
			Defaults map[string]string `json:"defaults"`
		} `json:"commands"`
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "framework.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	cmd, ok := m.Commands["deep-dive"]
	if !ok {
		t.Fatal("missing deep-dive command")
	}
	joined := strings.Join(cmd.Args, " ")
	for _, want := range []string{"{params.priority_list_path}", "{params.priority_list_json}", "{params.strategy_path}", "{params.strategy_json}", "{params.context_b64}"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in deep-dive args, got %v", want, cmd.Args)
		}
	}
}
