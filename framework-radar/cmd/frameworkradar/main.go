package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type semanticPack struct {
	BusinessID        string                    `json:"business_id"`
	Name              string                    `json:"name"`
	PrimaryEntities   map[string]semanticEntity `json:"primary_entities"`
	ScopePolicies     scopePolicies             `json:"scope_policies"`
	CollectionScoring *collectionScoring        `json:"collection_scoring,omitempty"`
}

type semanticEntity struct {
	Table         string `json:"table"`
	Label         string `json:"label"`
	ScopeKey      string `json:"scope_key"`
	ScopeColumn   string `json:"scope_column"`
	DisplayColumn string `json:"display_column"`
}

type scopePolicies struct {
	ScopeEntity string                    `json:"scope_entity"`
	Tables      map[string]scopeTableRule `json:"tables"`
}

type scopeTableRule struct {
	ScopeColumn string `json:"scope_column"`
	JoinToScope string `json:"join_to_scope"`
}

type collectionScoring struct {
	EntityTable       string   `json:"entity_table"`
	EntityIDColumn    string   `json:"entity_id_column"`
	EntityNameColumn  string   `json:"entity_name_column"`
	ItemTable         string   `json:"item_table"`
	ItemEntityColumn  string   `json:"item_entity_column"`
	AmountColumn      string   `json:"amount_column"`
	AmountTable       string   `json:"amount_table"`
	AmountJoinColumn  string   `json:"amount_join_column"`
	AmountDateColumn  string   `json:"amount_date_column"`
	ItemJoinColumn    string   `json:"item_join_column"`
	StatusColumn      string   `json:"status_column"`
	DateColumn        string   `json:"date_column"`
	OpenStatuses      []string `json:"open_statuses"`
	RecentContactDays int      `json:"recent_contact_days"`
}

type tableInfo struct {
	Name    string
	Columns map[string]bool
}

type rawItem struct {
	EntityID   string
	EntityName string
	Amount     float64
	DueDate    string
	Status     string
	ItemID     string
}

type aggregate struct {
	EntityID string
	Name     string
	Amount   float64
	Count    int
	Oldest   time.Time
	HasDate  bool
	Statuses map[string]bool
}

type paymentStats struct {
	Count   int
	Total   float64
	Last    time.Time
	HasLast bool
}

type priorityItem struct {
	ArtifactType    string         `json:"artifact_type"`
	Rank            int            `json:"rank"`
	Score           int            `json:"score"`
	ScoreBreakdown  map[string]int `json:"score_breakdown,omitempty"`
	Strategy        string         `json:"strategy,omitempty"`
	AnalysisOptions []string       `json:"analysis_options,omitempty"`
	DataGaps        []string       `json:"data_gaps,omitempty"`
	EntityRef       entityRef      `json:"entity_ref"`
	Reasons         []string       `json:"reasons"`
	Deudor          string         `json:"deudor,omitempty"`
	DeudorID        string         `json:"deudor_id,omitempty"`
	SaldoTotal      float64        `json:"saldo_total,omitempty"`
	DiasMoraMax     int            `json:"dias_mora_max,omitempty"`
	FacturasCount   int            `json:"facturas_count,omitempty"`
	Razon           string         `json:"razon,omitempty"`
}

type entityRef struct {
	ArtifactType string `json:"artifact_type"`
	Type         string `json:"type"`
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fail("uso: frameworkradar prioritize --business-id <id> --dataset-json <json> --semantic-pack <path>")
	}
	switch os.Args[1] {
	case "configure-analysis":
		if err := runConfigureAnalysis(os.Args[2:]); err != nil {
			fail("%v", err)
		}
	case "prioritize":
		if err := runPrioritize(os.Args[2:]); err != nil {
			fail("%v", err)
		}
	default:
		fail("comando desconocido: %s", os.Args[1])
	}
}

func runPrioritize(args []string) error {
	fs := flag.NewFlagSet("prioritize", flag.ExitOnError)
	businessID := fs.String("business-id", "", "negocio activo")
	dbPath := fs.String("db", "", "path SQLite (solo debug/admin; en runtime Radar debe recibir dataset mediado por Sabio)")
	semanticPath := fs.String("semantic-pack", "", "path semantic pack")
	datasetArtifact := fs.String("dataset-artifact", "", "path a dataset.raw.v1 exportado por Sabio")
	datasetJSON := fs.String("dataset-json", "", "dataset como JSON string exportado por Sabio")
	contextB64 := fs.String("context-b64", "", "contexto runtime codificado")
	_ = contextB64
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedDB := *dbPath
	if strings.TrimSpace(*datasetJSON) != "" {
		tmp, err := writeTempJSONToDB(*datasetJSON)
		if err != nil {
			emitNeedsConfiguration(*businessID, "dataset_json_error", err.Error())
			return nil
		}
		defer os.Remove(tmp)
		resolvedDB = tmp
	} else if strings.TrimSpace(*datasetArtifact) != "" {
		tmp, err := loadDatasetArtifactToTempDB(*datasetArtifact)
		if err != nil {
			emitNeedsConfiguration(*businessID, "dataset_artifact_error", err.Error())
			return nil
		}
		defer os.Remove(tmp)
		resolvedDB = tmp
	}
	if strings.TrimSpace(resolvedDB) == "" {
		emitNeedsConfiguration(*businessID, "missing_dataset", "Falta dataset.raw.v1 mediado por Sabio para calcular prioridades.")
		return nil
	}
	if strings.TrimSpace(*semanticPath) == "" {
		emitNeedsConfiguration(*businessID, "missing_semantic_pack", "Falta business.semantic_pack.v1; no se infiere scoring sin configuración semántica.")
		return nil
	}

	pack, err := loadSemanticPack(*semanticPath)
	if err != nil {
		emitNeedsConfiguration(*businessID, "invalid_semantic_pack", err.Error())
		return nil
	}
	model, err := inferScoringModel(pack)
	if err != nil {
		emitNeedsConfiguration(firstNonEmpty(*businessID, pack.BusinessID), "needs_configuration", err.Error())
		return nil
	}
	bid := firstNonEmpty(*businessID, pack.BusinessID)
	if persisted, ok := loadPersistedAnalysisPlan(bid); ok {
		model = persisted
	}
	items, resolvedModel, err := scoreSQLite(resolvedDB, model)
	if err != nil {
		emitNeedsConfiguration(bid, "query_error", err.Error())
		return nil
	}
	plan := persistAnalysisPlan(bid, resolvedModel)
	emitPriorityList(bid, items, resolvedModel, plan)
	return nil
}

func runConfigureAnalysis(args []string) error {
	fs := flag.NewFlagSet("configure-analysis", flag.ExitOnError)
	businessID := fs.String("business-id", "", "negocio activo")
	dbPath := fs.String("db", "", "path SQLite")
	semanticPath := fs.String("semantic-pack", "", "path semantic pack")
	contextB64 := fs.String("context-b64", "", "contexto runtime codificado")
	_ = contextB64
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*semanticPath) == "" {
		emitNeedsConfiguration(*businessID, "missing_semantic_pack", "Falta business.semantic_pack.v1 para diseñar el algoritmo de análisis.")
		return nil
	}
	pack, err := loadSemanticPack(*semanticPath)
	if err != nil {
		emitNeedsConfiguration(*businessID, "invalid_semantic_pack", err.Error())
		return nil
	}
	model, err := inferScoringModel(pack)
	if err != nil {
		emitNeedsConfiguration(firstNonEmpty(*businessID, pack.BusinessID), "needs_configuration", err.Error())
		return nil
	}
	resolvedModel := model
	if strings.TrimSpace(*dbPath) != "" {
		if _, resolved, err := scoreSQLite(*dbPath, model); err == nil {
			resolvedModel = resolved
		}
	}
	bid := firstNonEmpty(*businessID, pack.BusinessID)
	plan := persistAnalysisPlan(bid, resolvedModel)
	printJSON(map[string]interface{}{
		"artifact_type": "analysis.schema.v1",
		"artifacts":     []string{"analysis.schema.v1", "analysis.proposal.v1"},
		"business_id":   bid,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"schema_id":     "collection_priority_40_30_30_v1",
		"schema_path":   plan.SchemaPath,
		"plan_path":     plan.PlanPath,
		"sql_path":      plan.SQLPath,
		"weights":       map[string]int{"materialidad": 40, "comportamiento": 30, "riesgo_legal": 30},
		"model":         analysisModelPayload(resolvedModel),
		"text":          "Radar propone analizar la cartera con un algoritmo configurable: materialidad 40%, comportamiento histórico 30% y riesgo legal/antigüedad 30%. Esta configuración queda plasmada como analysis.schema.v1 para reutilizarse en código antes de priorizar el día.",
		"options": []string{
			"Aceptar configuración y calcular lista de hoy",
			"Ajustar ponderaciones del scoring",
			"Agregar o quitar señales de análisis",
		},
	})
	return nil
}

func loadSemanticPack(path string) (semanticPack, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return semanticPack{}, err
	}
	var pack semanticPack
	if err := json.Unmarshal(raw, &pack); err != nil {
		return semanticPack{}, err
	}
	if len(pack.PrimaryEntities) == 0 && pack.CollectionScoring == nil {
		return semanticPack{}, errors.New("semantic pack no declara primary_entities ni collection_scoring")
	}
	return pack, nil
}

func inferScoringModel(pack semanticPack) (collectionScoring, error) {
	if pack.CollectionScoring != nil {
		return *pack.CollectionScoring, nil
	}
	scopeEntity := pack.ScopePolicies.ScopeEntity
	entity := pack.PrimaryEntities[scopeEntity]
	if entity.Table == "" {
		for key, candidate := range pack.PrimaryEntities {
			if strings.Contains(strings.ToLower(key), "client") || strings.Contains(strings.ToLower(candidate.Label), "cliente") {
				entity = candidate
				break
			}
		}
	}
	if entity.Table == "" {
		return collectionScoring{}, errors.New("semantic pack no declara entidad cobrable primaria")
	}
	item := pack.PrimaryEntities["charge"]
	if item.Table == "" {
		for key, candidate := range pack.PrimaryEntities {
			k := strings.ToLower(key + " " + candidate.Label + " " + candidate.Table)
			if strings.Contains(k, "charge") || strings.Contains(k, "cobro") || strings.Contains(k, "invoice") || strings.Contains(k, "debt") {
				item = candidate
				break
			}
		}
	}
	if item.Table == "" {
		return collectionScoring{}, errors.New("semantic pack no declara tabla de cobros/deuda para priorizar")
	}
	return collectionScoring{
		EntityTable:      entity.Table,
		EntityIDColumn:   firstNonEmpty(entity.ScopeKey, "id"),
		EntityNameColumn: firstNonEmpty(entity.DisplayColumn, "name"),
		ItemTable:        item.Table,
		ItemEntityColumn: firstNonEmpty(item.ScopeColumn, singular(entity.Table)+"_id", "client_id", "customer_id"),
	}, nil
}

func scoreSQLite(dbPath string, model collectionScoring) ([]priorityItem, collectionScoring, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_pragma=query_only(true)")
	if err != nil {
		return nil, model, err
	}
	defer db.Close()

	tables, err := inspectTables(db)
	if err != nil {
		return nil, model, err
	}
	entityTable, ok := tables[model.EntityTable]
	if !ok {
		return nil, model, fmt.Errorf("tabla de entidad no existe: %s", model.EntityTable)
	}
	itemTable, ok := tables[model.ItemTable]
	if !ok {
		return nil, model, fmt.Errorf("tabla de cobros no existe: %s", model.ItemTable)
	}

	model.EntityIDColumn = existingColumn(entityTable, model.EntityIDColumn, "id", "code")
	model.EntityNameColumn = existingColumn(entityTable, model.EntityNameColumn, "name", "nombre", "code", model.EntityIDColumn)
	model.ItemEntityColumn = existingColumn(itemTable, model.ItemEntityColumn, singular(model.EntityTable)+"_id", "client_id", "customer_id", "entity_id")
	model.ItemJoinColumn = existingColumn(itemTable, model.ItemJoinColumn, "id")
	model.StatusColumn = existingColumn(itemTable, model.StatusColumn, "status", "state", "estado")
	model.DateColumn = existingColumn(itemTable, model.DateColumn, "due_date", "date", "date_to", "created_at")
	model.AmountColumn = existingColumn(itemTable, model.AmountColumn, "amount", "balance", "saldo", "total", "residue")
	if model.AmountColumn == "" {
		model.AmountTable, model.AmountJoinColumn, model.AmountColumn, model.AmountDateColumn = findAmountTable(tables, model.ItemTable)
	}
	if model.ItemEntityColumn == "" || model.ItemJoinColumn == "" {
		return nil, model, errors.New("no se pudo mapear la relación entre entidad y cobros")
	}
	if model.AmountColumn == "" {
		return nil, model, errors.New("no se pudo mapear columna de monto para scoring")
	}

	rows, err := fetchRawItems(db, model)
	if err != nil {
		return nil, model, err
	}
	payments := fetchPaymentStats(db, tables, model)
	return aggregateItems(rows, payments), model, nil
}

func inspectTables(db *sql.DB) (map[string]tableInfo, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type IN ('table','view')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]tableInfo{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols, err := tableColumns(db, name)
		if err != nil {
			return nil, err
		}
		out[name] = tableInfo{Name: name, Columns: cols}
	}
	return out, rows.Err()
}

func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + quoteIdent(table) + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

func fetchRawItems(db *sql.DB, model collectionScoring) ([]rawItem, error) {
	selects := []string{
		"e." + quoteIdent(model.EntityIDColumn),
		"e." + quoteIdent(model.EntityNameColumn),
		"i." + quoteIdent(model.ItemEntityColumn),
		"i." + quoteIdent(model.ItemJoinColumn),
	}
	if model.StatusColumn != "" {
		selects = append(selects, "i."+quoteIdent(model.StatusColumn))
	} else {
		selects = append(selects, "''")
	}
	if model.AmountTable != "" && model.AmountDateColumn != "" {
		selects = append(selects, "MIN(a."+quoteIdent(model.AmountDateColumn)+")")
	} else if model.DateColumn != "" {
		selects = append(selects, "i."+quoteIdent(model.DateColumn))
	} else {
		selects = append(selects, "''")
	}
	if model.AmountTable != "" {
		selects = append(selects, "COALESCE(SUM(CAST(a."+quoteIdent(model.AmountColumn)+" AS REAL)), 0)")
	} else {
		selects = append(selects, "COALESCE(CAST(i."+quoteIdent(model.AmountColumn)+" AS REAL), 0)")
	}
	query := "SELECT " + strings.Join(selects, ", ") +
		" FROM " + quoteIdent(model.ItemTable) + " i" +
		" JOIN " + quoteIdent(model.EntityTable) + " e ON e." + quoteIdent(model.EntityIDColumn) + " = i." + quoteIdent(model.ItemEntityColumn)
	if model.AmountTable != "" {
		query += " LEFT JOIN " + quoteIdent(model.AmountTable) + " a ON a." + quoteIdent(model.AmountJoinColumn) + " = i." + quoteIdent(model.ItemJoinColumn)
		query += " GROUP BY i." + quoteIdent(model.ItemJoinColumn)
	}
	query += " LIMIT 5000"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []rawItem
	for rows.Next() {
		var entityID, name, fk, itemID, status, dueDate sql.NullString
		var amount sql.NullFloat64
		if err := rows.Scan(&entityID, &name, &fk, &itemID, &status, &dueDate, &amount); err != nil {
			return nil, err
		}
		if !isEligibleStatus(status.String) || amount.Float64 <= 0 {
			continue
		}
		out = append(out, rawItem{
			EntityID:   firstNonEmpty(entityID.String, fk.String),
			EntityName: firstNonEmpty(name.String, entityID.String, fk.String),
			Amount:     amount.Float64,
			DueDate:    dueDate.String,
			Status:     status.String,
			ItemID:     itemID.String,
		})
	}
	return out, rows.Err()
}

func fetchPaymentStats(db *sql.DB, tables map[string]tableInfo, model collectionScoring) map[string]paymentStats {
	tableName, entityColumn, amountColumn, dateColumn := findPaymentTable(tables, model)
	if tableName == "" || entityColumn == "" {
		return map[string]paymentStats{}
	}
	selects := []string{
		quoteIdent(entityColumn),
		"COUNT(*)",
	}
	if amountColumn != "" {
		selects = append(selects, "COALESCE(SUM(CAST("+quoteIdent(amountColumn)+" AS REAL)), 0)")
	} else {
		selects = append(selects, "0")
	}
	if dateColumn != "" {
		selects = append(selects, "MAX("+quoteIdent(dateColumn)+")")
	} else {
		selects = append(selects, "''")
	}
	query := "SELECT " + strings.Join(selects, ", ") + " FROM " + quoteIdent(tableName) + " GROUP BY " + quoteIdent(entityColumn)
	rows, err := db.Query(query)
	if err != nil {
		return map[string]paymentStats{}
	}
	defer rows.Close()
	out := map[string]paymentStats{}
	for rows.Next() {
		var entityID, lastDate sql.NullString
		var count int
		var total sql.NullFloat64
		if err := rows.Scan(&entityID, &count, &total, &lastDate); err != nil {
			continue
		}
		st := paymentStats{Count: count, Total: total.Float64}
		if parsed, ok := parseDate(lastDate.String); ok {
			st.Last = parsed
			st.HasLast = true
		}
		if entityID.String != "" {
			out[entityID.String] = st
		}
	}
	return out
}

func findPaymentTable(tables map[string]tableInfo, model collectionScoring) (table, entityColumn, amountColumn, dateColumn string) {
	candidates := []tableInfo{}
	for _, t := range tables {
		name := strings.ToLower(t.Name)
		if strings.Contains(name, "payment") || strings.Contains(name, "pago") || strings.Contains(name, "receipt") {
			candidates = append(candidates, t)
		}
	}
	for _, t := range candidates {
		entity := existingColumn(t, model.ItemEntityColumn, singular(model.EntityTable)+"_id", "client_id", "customer_id", "entity_id")
		if entity == "" {
			continue
		}
		amount := existingColumn(t, "amount", "monto", "total", "paid_amount", "value")
		date := existingColumn(t, "date", "paid_at", "payment_date", "created_at", "updated_at")
		return t.Name, entity, amount, date
	}
	return "", "", "", ""
}

func aggregateItems(rows []rawItem, payments map[string]paymentStats) []priorityItem {
	now := time.Now()
	agg := map[string]*aggregate{}
	for _, row := range rows {
		if row.EntityID == "" {
			continue
		}
		a := agg[row.EntityID]
		if a == nil {
			a = &aggregate{EntityID: row.EntityID, Name: row.EntityName, Statuses: map[string]bool{}}
			agg[row.EntityID] = a
		}
		a.Amount += row.Amount
		a.Count++
		if row.Status != "" {
			a.Statuses[row.Status] = true
		}
		if parsed, ok := parseDate(row.DueDate); ok && (!a.HasDate || parsed.Before(a.Oldest)) {
			a.Oldest = parsed
			a.HasDate = true
		}
	}
	portfolioTotal := 0.0
	for _, a := range agg {
		portfolioTotal += a.Amount
	}
	items := make([]priorityItem, 0, len(agg))
	for _, a := range agg {
		days := 0
		if a.HasDate {
			days = int(now.Sub(a.Oldest).Hours() / 24)
			if days < 0 {
				days = 0
			}
		}
		pay := payments[a.EntityID]
		breakdown, score, gaps := computeRiskScore(a.Amount, days, portfolioTotal, pay, now)
		reasons := []string{fmt.Sprintf("Saldo total %.0f", a.Amount)}
		if days > 0 {
			reasons = append(reasons, fmt.Sprintf("Más de %d días desde la fecha más antigua", days))
		}
		if a.Count > 1 {
			reasons = append(reasons, fmt.Sprintf("%d documentos/cobros abiertos", a.Count))
		}
		if pay.Count > 0 {
			reasons = append(reasons, fmt.Sprintf("%d pagos históricos registrados", pay.Count))
		}
		items = append(items, priorityItem{
			ArtifactType:    "collection.priority_item.v1",
			Score:           score,
			ScoreBreakdown:  breakdown,
			Strategy:        recommendedStrategy(score, days, gaps),
			AnalysisOptions: recommendedAnalysisOptions(gaps),
			DataGaps:        gaps,
			EntityRef:       entityRef{ArtifactType: "entity.ref.v1", Type: "customer", ID: a.EntityID, Name: a.Name},
			Reasons:         reasons,
			Deudor:          a.Name,
			DeudorID:        a.EntityID,
			SaldoTotal:      a.Amount,
			DiasMoraMax:     days,
			FacturasCount:   a.Count,
			Razon:           strings.Join(reasons, "; "),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].SaldoTotal > items[j].SaldoTotal
		}
		return items[i].Score > items[j].Score
	})
	if len(items) > 10 {
		items = items[:10]
	}
	for i := range items {
		items[i].Rank = i + 1
	}
	return items
}

func loadDatasetArtifactToTempDB(artifactPath string) (string, error) {
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		return "", fmt.Errorf("read artifact: %w", err)
	}
	return writeTempJSONToDB(string(raw))
}

func writeTempJSONToDB(jsonStr string) (string, error) {
	var payload struct {
		Tables map[string][]map[string]interface{} `json:"tables"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		// Try wrapped artifact format (dataset.raw.v1 with tables nested).
		var wrap map[string]interface{}
		if err2 := json.Unmarshal([]byte(jsonStr), &wrap); err2 != nil {
			return "", fmt.Errorf("parse artifact: %w", err)
		}
		if t, ok := wrap["tables"].(map[string]interface{}); ok {
			payload.Tables = make(map[string][]map[string]interface{}, len(t))
			for name, rows := range t {
				if arr, ok := rows.([]interface{}); ok {
					var records []map[string]interface{}
					for _, r := range arr {
						if rec, ok := r.(map[string]interface{}); ok {
							records = append(records, rec)
						}
					}
					payload.Tables[name] = records
				}
			}
		}
	}
	if len(payload.Tables) == 0 {
		return "", errors.New("artifact has no tables")
	}

	f, err := os.CreateTemp("", "radar_dataset_*.db")
	if err != nil {
		return "", err
	}
	f.Close()
	db, err := sql.Open("sqlite", f.Name())
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	defer db.Close()

	for tableName, rows := range payload.Tables {
		if len(rows) == 0 {
			continue
		}
		cols := make([]string, 0, len(rows[0]))
		for k := range rows[0] {
			cols = append(cols, k)
		}
		sort.Strings(cols)
		var createCols []string
		for _, c := range cols {
			createCols = append(createCols, quoteIdent(c)+" TEXT")
		}
		createSQL := "CREATE TABLE " + quoteIdent(tableName) + " (" + strings.Join(createCols, ", ") + ")"
		if _, err := db.Exec(createSQL); err != nil {
			continue
		}
		placeholders := make([]string, len(cols))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		insertSQL := "INSERT INTO " + quoteIdent(tableName) + " (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
		stmt, err := db.Prepare(insertSQL)
		if err != nil {
			continue
		}
		for _, row := range rows {
			vals := make([]interface{}, len(cols))
			for i, c := range cols {
				if v, ok := row[c]; ok {
					vals[i] = fmt.Sprint(v)
				} else {
					vals[i] = nil
				}
			}
			_, _ = stmt.Exec(vals...)
		}
		stmt.Close()
	}
	return f.Name(), nil
}

type analysisPlanPaths struct {
	SchemaPath string
	PlanPath   string
	SQLPath    string
}

func emitPriorityList(businessID string, items []priorityItem, model collectionScoring, plan analysisPlanPaths) {
	out := map[string]interface{}{
		"artifact_type":  "collection.priority_list.v1",
		"artifacts":      []string{"collection.priority_list.v1", "collection.priority_item.v1", "entity.ref.v1", "risk.score.v1", "strategy.recommendation.v1", "data.gaps.v1", "analysis.schema.v1"},
		"business_id":    businessID,
		"generated_at":   time.Now().UTC().Format(time.RFC3339),
		"scoring_model":  "collection_priority_40_30_30_v1",
		"scoring_source": "framework-radar",
		"analysis_schema": map[string]interface{}{
			"artifact_type": "analysis.schema.v1",
			"schema_id":     "collection_priority_40_30_30_v1",
			"weights":       map[string]int{"materialidad": 40, "comportamiento": 30, "riesgo_legal": 30},
			"path":          plan.SchemaPath,
			"plan_path":     plan.PlanPath,
			"sql_path":      plan.SQLPath,
		},
		"items": items,
		"count": len(items),
		"trace": map[string]string{
			"entity_table": model.EntityTable,
			"item_table":   model.ItemTable,
		},
	}
	if len(items) > 0 {
		out["selected"] = items[0].EntityRef
		out["priority_item"] = items[0]
	}
	printJSON(out)
}

func analysisModelPayload(model collectionScoring) map[string]string {
	return map[string]string{
		"entity_table":       model.EntityTable,
		"entity_id_column":   model.EntityIDColumn,
		"entity_name_column": model.EntityNameColumn,
		"item_table":         model.ItemTable,
		"item_entity_column": model.ItemEntityColumn,
		"item_join_column":   model.ItemJoinColumn,
		"amount_table":       model.AmountTable,
		"amount_column":      model.AmountColumn,
		"date_column":        firstNonEmpty(model.AmountDateColumn, model.DateColumn),
		"status_column":      model.StatusColumn,
	}
}

func radarAnalysisDir(businessID string) string {
	return filepath.Join("temp", "radar", safePathPart(firstNonEmpty(businessID, "default")))
}

func loadPersistedAnalysisPlan(businessID string) (collectionScoring, bool) {
	path := filepath.Join(radarAnalysisDir(businessID), "collection_analysis_plan.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return collectionScoring{}, false
	}
	var plan struct {
		Model collectionScoring `json:"model"`
	}
	if err := json.Unmarshal(raw, &plan); err != nil {
		return collectionScoring{}, false
	}
	if plan.Model.EntityTable == "" || plan.Model.ItemTable == "" {
		return collectionScoring{}, false
	}
	return plan.Model, true
}

func emitNeedsConfiguration(businessID, code, message string) {
	out := map[string]interface{}{
		"artifact_type":        "collection.priority_list.v1",
		"artifacts":            []string{"collection.priority_list.v1"},
		"business_id":          businessID,
		"generated_at":         time.Now().UTC().Format(time.RFC3339),
		"items":                []interface{}{},
		"count":                0,
		"needs_configuration":  true,
		"configuration_code":   code,
		"configuration_reason": message,
	}
	if strings.Contains(code, "db") || strings.Contains(code, "dataset") || strings.Contains(code, "query") {
		out["artifacts"] = []string{"collection.priority_list.v1", "data.request.v1"}
		out["request"] = map[string]interface{}{
			"artifact_type": "data.request.v1",
			"target":        "sabio",
			"capability":    "dataset.export",
			"reason":        message,
			"code":          code,
		}
	}
	printJSON(out)
}

func persistAnalysisPlan(businessID string, model collectionScoring) analysisPlanPaths {
	dir := radarAnalysisDir(businessID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return analysisPlanPaths{}
	}
	schemaPath := filepath.Join(dir, "collection_analysis_schema.json")
	planPath := filepath.Join(dir, "collection_analysis_plan.json")
	sqlPath := filepath.Join(dir, "collection_priority_query.sql")
	sqlText := analysisSQLTemplate(model)
	schema := map[string]interface{}{
		"artifact_type": "analysis.schema.v1",
		"schema_id":     "collection_priority_40_30_30_v1",
		"framework":     "radar",
		"weights":       map[string]int{"materialidad": 40, "comportamiento": 30, "riesgo_legal": 30},
		"model":         analysisModelPayload(model),
		"plan_path":     planPath,
		"sql_path":      sqlPath,
		"notes": []string{
			"Esquema genérico inferido desde semantic pack/dataset y persistido como plan tangible.",
			"Radar no ejecuta acciones operativas; solo analiza y prioriza.",
			"Los ciclos siguientes reutilizan collection_analysis_plan.json salvo que el usuario reconfigure el análisis.",
		},
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	plan := map[string]interface{}{
		"artifact_type": "analysis.plan.v1",
		"plan_id":       "collection_priority_40_30_30_v1",
		"framework":     "radar",
		"business_id":   businessID,
		"model":         model,
		"weights":       map[string]int{"materialidad": 40, "comportamiento": 30, "riesgo_legal": 30},
		"sql_file":      sqlPath,
		"schema_file":   schemaPath,
		"reconfigure_by": []string{
			"Ejecutar configure-analysis con un semantic pack actualizado.",
			"Reemplazar este plan mediante una acción aprobada por el usuario/staff.",
		},
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	if raw, err := json.MarshalIndent(schema, "", "  "); err == nil {
		_ = os.WriteFile(schemaPath, raw, 0644)
	}
	if raw, err := json.MarshalIndent(plan, "", "  "); err == nil {
		_ = os.WriteFile(planPath, raw, 0644)
	}
	_ = os.WriteFile(sqlPath, []byte(sqlText), 0644)
	return analysisPlanPaths{SchemaPath: schemaPath, PlanPath: planPath, SQLPath: sqlPath}
}

func analysisSQLTemplate(model collectionScoring) string {
	var sb strings.Builder
	sb.WriteString("-- Radar collection priority query\n")
	sb.WriteString("-- Generated from business semantic pack and persisted as the tangible analysis plan.\n")
	sb.WriteString("-- Runtime scoring weights: materialidad=40, comportamiento=30, riesgo_legal=30.\n")
	sb.WriteString("SELECT\n")
	sb.WriteString("  e." + quoteIdent(model.EntityIDColumn) + " AS entity_id,\n")
	sb.WriteString("  e." + quoteIdent(model.EntityNameColumn) + " AS entity_name,\n")
	sb.WriteString("  i." + quoteIdent(model.ItemJoinColumn) + " AS item_id,\n")
	if model.StatusColumn != "" {
		sb.WriteString("  i." + quoteIdent(model.StatusColumn) + " AS status,\n")
	} else {
		sb.WriteString("  '' AS status,\n")
	}
	dateExpr := "''"
	if model.AmountTable != "" && model.AmountDateColumn != "" {
		dateExpr = "MIN(a." + quoteIdent(model.AmountDateColumn) + ")"
	} else if model.DateColumn != "" {
		dateExpr = "i." + quoteIdent(model.DateColumn)
	}
	sb.WriteString("  " + dateExpr + " AS due_date,\n")
	if model.AmountTable != "" {
		sb.WriteString("  COALESCE(SUM(CAST(a." + quoteIdent(model.AmountColumn) + " AS REAL)), 0) AS amount\n")
	} else {
		sb.WriteString("  COALESCE(CAST(i." + quoteIdent(model.AmountColumn) + " AS REAL), 0) AS amount\n")
	}
	sb.WriteString("FROM " + quoteIdent(model.ItemTable) + " i\n")
	sb.WriteString("JOIN " + quoteIdent(model.EntityTable) + " e ON e." + quoteIdent(model.EntityIDColumn) + " = i." + quoteIdent(model.ItemEntityColumn) + "\n")
	if model.AmountTable != "" {
		sb.WriteString("LEFT JOIN " + quoteIdent(model.AmountTable) + " a ON a." + quoteIdent(model.AmountJoinColumn) + " = i." + quoteIdent(model.ItemJoinColumn) + "\n")
		sb.WriteString("GROUP BY i." + quoteIdent(model.ItemJoinColumn) + "\n")
	}
	sb.WriteString("LIMIT 5000;\n")
	return sb.String()
}

func safePathPart(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '_'
		}
	}, s)
}

func findAmountTable(tables map[string]tableInfo, itemTable string) (table, joinColumn, amountColumn, dateColumn string) {
	itemFK := singular(itemTable) + "_id"
	for _, t := range tables {
		join := existingColumn(t, itemFK, "charge_id", "invoice_id", "debt_id", "item_id")
		amount := existingColumn(t, "amount", "balance", "saldo", "total", "residue")
		if join != "" && amount != "" {
			date := existingColumn(t, "due_date", "date", "created_at", "updated_at")
			return t.Name, join, amount, date
		}
	}
	return "", "", "", ""
}

func existingColumn(table tableInfo, candidates ...string) string {
	for _, c := range candidates {
		if c != "" && table.Columns[c] {
			return c
		}
	}
	return ""
}

func isEligibleStatus(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "" {
		return true
	}
	blocked := []string{"pagado", "paid", "cancel", "anulad", "void", "closed", "cerrad", "cobrado"}
	for _, token := range blocked {
		if strings.Contains(s, token) {
			return false
		}
	}
	return true
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if len(s) >= 10 {
		s = s[:10]
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006", "2006/01/02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func computeRiskScore(amount float64, days int, portfolioTotal float64, pay paymentStats, now time.Time) (map[string]int, int, []string) {
	gaps := []string{}
	materiality := 0
	if portfolioTotal > 0 {
		materiality = clampScore((amount / portfolioTotal) * 300)
	} else if amount > 0 {
		materiality = 50
		gaps = append(gaps, "No hay total de portafolio suficiente para materialidad relativa.")
	}
	legal := clampScore(float64(days) / 365.0 * 100)
	behavior := 0
	if pay.Count == 0 {
		behavior = 70
		gaps = append(gaps, "No hay pagos históricos para calcular desviación de comportamiento.")
	} else if pay.HasLast {
		daysSincePayment := int(now.Sub(pay.Last).Hours() / 24)
		if daysSincePayment < 0 {
			daysSincePayment = 0
		}
		behavior = clampScore(float64(daysSincePayment) / 180.0 * 100)
	} else {
		behavior = 50
		gaps = append(gaps, "Hay pagos históricos, pero sin fecha confiable de último pago.")
	}
	breakdown := map[string]int{
		"materialidad":   materiality,
		"comportamiento": behavior,
		"riesgo_legal":   legal,
	}
	score := int(math.Round(float64(materiality)*0.40 + float64(behavior)*0.30 + float64(legal)*0.30))
	return breakdown, score, gaps
}

func clampScore(v float64) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return int(math.Round(v))
}

func recommendedStrategy(score, days int, gaps []string) string {
	switch {
	case len(gaps) > 0:
		return "Lectura de riesgo incompleta: los datos faltantes reducen confianza del scoring."
	case score >= 75 || days >= 120:
		return "Riesgo alto: alta materialidad o antigüedad explican la prioridad del caso."
	case score >= 50:
		return "Riesgo medio: conviene revisar comportamiento histórico y evidencia antes de operar."
	default:
		return "Riesgo bajo: prioridad menor frente a casos de mayor materialidad o exposición."
	}
}

func recommendedAnalysisOptions(gaps []string) []string {
	if len(gaps) > 0 {
		return []string{"Revisar datos faltantes", "Ajustar criterio de scoring", "Aceptar esquema y continuar"}
	}
	return []string{"Aceptar esquema de análisis", "Ver explicación del scoring", "Ajustar ponderaciones"}
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func singular(s string) string {
	s = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), "s")
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func printJSON(v interface{}) {
	raw, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(raw))
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "radar_error: "+format+"\n", args...)
	os.Exit(1)
}
