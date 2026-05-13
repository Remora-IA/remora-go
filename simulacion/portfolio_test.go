package main

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Clean(filepath.Join(".", "panalbit.sqlite"))
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("ping sqlite: %v", err)
	}
	return db
}

func TestLoadPortfolioSnapshot(t *testing.T) {
	db := openTestDB(t)
	snapshot, err := loadPortfolioSnapshot(db)
	if err != nil {
		t.Fatalf("loadPortfolioSnapshot: %v", err)
	}
	if snapshot.Context.Summary.Clients == 0 {
		t.Fatalf("expected clients in summary")
	}
	if len(snapshot.AllItems) == 0 {
		t.Fatalf("expected prioritized items")
	}
	if snapshot.AllItems[0].OpenCharges == 0 {
		t.Fatalf("expected top priority to have open charges, got %#v", snapshot.AllItems[0])
	}
	if snapshot.Context.PrioritySchema.SchemaID != "collection_priority_40_30_30_v1" {
		t.Fatalf("unexpected priority schema: %#v", snapshot.Context.PrioritySchema)
	}
	if len(snapshot.AllItems[0].CriteriaBreakdown) != 3 {
		t.Fatalf("expected three business criteria, got %#v", snapshot.AllItems[0].CriteriaBreakdown)
	}
	if strings.TrimSpace(snapshot.AllItems[0].DominantCriterion) == "" {
		t.Fatalf("expected dominant criterion in top item")
	}
}

func TestOpenTopDebtorSwitchesSessionToDebtor(t *testing.T) {
	db := openTestDB(t)
	snapshot, err := loadPortfolioSnapshot(db)
	if err != nil {
		t.Fatalf("loadPortfolioSnapshot: %v", err)
	}
	app := &app{db: db}
	session := &session{
		ID:        "test",
		View:      "portfolio",
		Portfolio: snapshot,
		Agenda:    normalizeAgenda(buildPortfolioAgenda(), derivedContext{}),
		Phase:     "exploring",
	}
	resp, err := app.openTopDebtor(session, "Analiza al deudor más prioritario")
	if err != nil {
		t.Fatalf("openTopDebtor: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected response")
	}
	if session.View != "debtor" {
		t.Fatalf("expected debtor view, got %s", session.View)
	}
	if session.CurrentClientID == "" {
		t.Fatalf("expected current client id")
	}
	if session.Context.Profile.Name == "" {
		t.Fatalf("expected debtor context profile")
	}
}

func TestBusinessPriorityBreakdownUsesThreeConfiguredCriteria(t *testing.T) {
	item := priorityItem{
		KnownOpenAmount:      4000,
		OpenCharges:          3,
		PartialCharges:       1,
		PaidCharges:          2,
		OldestAgeDays:        420,
		DaysSinceLastPayment: 25,
	}
	breakdown := buildBusinessPriorityBreakdown(item, priorityPortfolioStats{
		TotalKnownOpenAmount: 10000,
		MaxKnownOpenAmount:   4000,
		MaxPendingCount:      4,
	})
	if len(breakdown) != 3 {
		t.Fatalf("expected 3 criteria, got %#v", breakdown)
	}
	if got := breakdown[0].Key; got != "materialidad" {
		t.Fatalf("expected first criterion to be materialidad, got %q", got)
	}
	score := weightedPriorityScore(breakdown)
	if score <= 0 {
		t.Fatalf("expected weighted score > 0, got %d", score)
	}
	if dominant := dominantPriorityCriterion(breakdown); dominant == "" {
		t.Fatalf("expected dominant criterion, got empty")
	}
	motive := buildPriorityMotive(breakdown)
	if !strings.Contains(strings.ToLower(motive), "materialidad") {
		t.Fatalf("expected motive to mention business criterion, got %q", motive)
	}
}
