package main

import (
	"strings"
	"testing"
)

func TestAnswerQuestionLawFirmsUsesSQLiteTrace(t *testing.T) {
	t.Setenv("SABIO_DB", "../../../framework-indexa/data/panalbit.db")
	out, err := answerQuestion("qué estudios juridicos tienes", "missing-store.json", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"2 estudios jurídicos", "Zieme-Ledner", "Zemlak-Bartoletti", "\"capability\": \"data.entity.list\"", "\"source\": \"sqlite\"", "\"fallback_used\": false"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q; got:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{"Murazik", "Para hoy", "Hacé ahora"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("output contains forbidden production-regression text %q:\n%s", forbidden, out)
		}
	}
}

func TestAnswerQuestionMissingSQLiteDoesNotUseStoreFallback(t *testing.T) {
	t.Setenv("SABIO_DB", "../../../framework-indexa/data/no-existe.db")
	out, err := answerQuestion("cuántos clientes tengo", "../../../framework-indexa/data/store.json", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"No puedo responder con la fuente declarada", "\"source\": \"sqlite\"", "\"fallback_used\": false"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(strings.ToLower(out), "bm25") {
		t.Fatalf("output should not mention or use bm25 fallback:\n%s", out)
	}
}

func TestSemanticContextIncludesCatalogLimitsAndViews(t *testing.T) {
	t.Setenv("SABIO_SEMANTIC_CATALOG", "../../semantic/catalog.json")
	t.Setenv("SABIO_SEMANTIC_VIEWS", "../../semantic/views.sql")

	out := semanticContextForPrompt(runtimeContext{BusinessID: "panalbit"})
	for _, want := range []string{
		"CATÁLOGO SEMÁNTICO CURADO",
		"law_firms -> clients",
		"No hay direccion",
		"vw_collection_overview",
		"milestones",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("semantic context should contain %q; got:\n%s", want, out)
		}
	}
}

func TestValidateSQLScopeRequiresClientFilter(t *testing.T) {
	rt := runtimeContext{
		BusinessID: "panalbit",
		Audience:   "collector",
		Scope:      map[string]any{"allowed_client_ids": []any{"1", "2"}},
	}
	if err := validateSQLScope(`SELECT COUNT(*) AS clientes_count FROM "clients"`, rt); err == nil {
		t.Fatal("expected unscoped clients query to fail")
	}
	if err := validateSQLScope(`SELECT COUNT(*) AS clientes_count FROM "clients" WHERE "id" IN ('1','2')`, rt); err != nil {
		t.Fatalf("expected scoped clients query to pass: %v", err)
	}
	if err := validateSQLScope(`SELECT COUNT(*) AS cargos_count FROM "charges" WHERE "client_id" = '2'`, rt); err != nil {
		t.Fatalf("expected scoped charges query to pass: %v", err)
	}
}

func TestBusinessConfigValidates(t *testing.T) {
	result := validateBusinessConfig("panalbit", "../../../framework-indexa/data/panalbit.db")
	if ok, _ := result["ok"].(bool); !ok {
		t.Fatalf("expected panalbit business config to validate: %#v", result)
	}
}
