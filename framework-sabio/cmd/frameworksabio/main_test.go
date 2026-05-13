package main

import (
	"path/filepath"
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

func TestRunEntity360ArtifactBuildsStructuredCustomerView(t *testing.T) {
	dbPath := filepath.Clean("../../../framework-indexa/data/panalbit.db")
	artifact := runEntity360Artifact(dbPath, runtimeContext{BusinessID: "panalbit", Audience: "collector"}, "Construye vista 360 del cliente activo", "customer", "184", "case_baseline")
	if artifact["artifact_type"] != "entity_360.v1" {
		t.Fatalf("expected entity_360.v1, got %#v", artifact)
	}
	if verified, _ := artifact["verified"].(bool); !verified {
		t.Fatalf("expected verified artifact, got %#v", artifact)
	}
	entity, _ := artifact["entity"].(map[string]any)
	if entity["name"] != "Thiel-Effertz" {
		t.Fatalf("expected Thiel-Effertz entity, got %#v", entity)
	}
	financial, _ := artifact["financial_position"].(map[string]any)
	if financial["open_amount"] != 7500.0 {
		t.Fatalf("expected open_amount 7500, got %#v", financial)
	}
	aging, _ := artifact["aging"].(map[string]any)
	if aging["oldest_open_debt_days"] == 0 {
		t.Fatalf("expected oldest_open_debt_days, got %#v", aging)
	}
	structured, _ := artifact["structured"].(map[string]any)
	if structured["invoice_number"] == "" {
		t.Fatalf("expected structured invoice_number, got %#v", structured)
	}
	if text, _ := artifact["text"].(string); !strings.Contains(text, "saldo abierto 7500.00") {
		t.Fatalf("expected explanatory text with open amount, got %q", text)
	}
}

func TestSabioQueryArtifactExposesPlainTextAndTrace(t *testing.T) {
	answer := "Texto verificable.\n\nEvidencia:\n```json\n{\"capability\":\"data.query.sql\",\"source\":\"sqlite\",\"fallback_used\":false}\n```"
	artifact := sabioQueryArtifact("pregunta", answer, runtimeContext{BusinessID: "panalbit"}, "data.query.sql")
	if artifact["text"] != "Texto verificable." {
		t.Fatalf("expected plain text extracted, got %#v", artifact["text"])
	}
	trace, _ := artifact["trace"].(map[string]any)
	if trace["capability"] != "data.query.sql" {
		t.Fatalf("expected parsed trace, got %#v", trace)
	}
}

func TestRunAnalyticalQueryArtifactPortfolioComparison(t *testing.T) {
	dbPath := filepath.Clean("../../../framework-indexa/data/panalbit.db")
	artifact := runAnalyticalQueryArtifact(dbPath, runtimeContext{BusinessID: "panalbit", Audience: "collector", ActiveEntity: map[string]any{"id": "184", "type": "client"}}, "Compara este caso con la cartera", "portfolio_comparison", "evidence.portfolio_comparison", "client", "184", []string{"open_amount", "days_past_due"}, "similar_clients")
	if artifact == nil {
		t.Fatal("expected analytical artifact")
	}
	if verified, _ := artifact["verified"].(bool); !verified {
		t.Fatalf("expected verified analytical artifact, got %#v", artifact)
	}
	if text, _ := artifact["text"].(string); !strings.Contains(text, "materialidad") {
		t.Fatalf("expected portfolio comparison insight, got %q", text)
	}
	structured, _ := artifact["structured"].(map[string]any)
	if structured["peer_strategy"] != "similar_clients" {
		t.Fatalf("expected peer_strategy, got %#v", structured)
	}
}

func TestRunAnalyticalQueryArtifactPortfolioComparisonFromFreeTextIntentAndSemanticCapability(t *testing.T) {
	dbPath := filepath.Clean("../../../framework-indexa/data/panalbit.db")
	artifact := runAnalyticalQueryArtifact(
		dbPath,
		runtimeContext{BusinessID: "panalbit", Audience: "collector", ActiveEntity: map[string]any{"id": "184", "type": "client"}},
		"Compáralo contra clientes similares de la cartera: mora, saldo y comportamiento relativo",
		"comparar cliente con clientes similares de la cartera en mora, saldo y comportamiento relativo",
		"evidence.portfolio_comparison",
		"client",
		"184",
		[]string{"open_amount", "days_past_due", "payment_behavior"},
		"similar_clients",
	)
	if artifact == nil {
		t.Fatal("expected analytical artifact")
	}
	if verified, _ := artifact["verified"].(bool); !verified {
		t.Fatalf("expected verified artifact, got %#v", artifact)
	}
	structured, _ := artifact["structured"].(map[string]any)
	if structured["open_amount_percentile"] == nil || structured["days_past_due_percentile"] == nil {
		t.Fatalf("expected percentile-rich structured output, got %#v", structured)
	}
	if peers, _ := structured["peers"].([]map[string]any); peers == nil {
		if _, ok := structured["peers"].([]any); !ok {
			t.Fatalf("expected peers in structured output, got %#v", structured)
		}
	}
	if text, _ := artifact["text"].(string); !strings.Contains(text, "percentil") {
		t.Fatalf("expected comparative text with percentiles, got %q", text)
	}
}

func TestRunAnalyticalQueryArtifactPaymentBehaviorFromFreeTextIntentAndSemanticCapability(t *testing.T) {
	dbPath := filepath.Clean("../../../framework-indexa/data/panalbit.db")
	artifact := runAnalyticalQueryArtifact(
		dbPath,
		runtimeContext{BusinessID: "panalbit", Audience: "collector", ActiveEntity: map[string]any{"id": "184", "type": "client"}},
		"Explica la priorización del caso usando comportamiento de pago",
		"explicar priorización de caso de cobranza",
		"evidence.payment_behavior_summary",
		"client",
		"184",
		[]string{"payment_behavior"},
		"",
	)
	if artifact == nil {
		t.Fatal("expected analytical artifact")
	}
	if verified, _ := artifact["verified"].(bool); !verified {
		t.Fatalf("expected verified artifact, got %#v", artifact)
	}
	if intent, _ := artifact["analysis_intent"].(string); intent != "payment_behavior_summary" {
		t.Fatalf("expected canonical payment_behavior_summary intent, got %#v", artifact)
	}
	structured, _ := artifact["structured"].(map[string]any)
	if structured["payments_count"] == nil || structured["payments_total"] == nil {
		t.Fatalf("expected payment history summary, got %#v", structured)
	}
	if text, _ := artifact["text"].(string); !strings.Contains(strings.ToLower(text), "pagos históricos") {
		t.Fatalf("expected payment behavior summary text, got %q", text)
	}
}
