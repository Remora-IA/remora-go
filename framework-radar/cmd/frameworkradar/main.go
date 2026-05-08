package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
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

type priorityItem struct {
	ArtifactType      string    `json:"artifact_type"`
	Rank              int       `json:"rank"`
	Score             int       `json:"score"`
	EntityRef         entityRef `json:"entity_ref"`
	Reasons           []string  `json:"reasons"`
	RecommendedAction string    `json:"recommended_action"`
	Deudor            string    `json:"deudor,omitempty"`
	DeudorID          string    `json:"deudor_id,omitempty"`
	SaldoTotal        float64   `json:"saldo_total,omitempty"`
	DiasMoraMax       int       `json:"dias_mora_max,omitempty"`
	FacturasCount     int       `json:"facturas_count,omitempty"`
	Razon             string    `json:"razon,omitempty"`
	AccionSugerida    string    `json:"accion_sugerida,omitempty"`
}

type entityRef struct {
	ArtifactType string `json:"artifact_type"`
	Type         string `json:"type"`
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fail("uso: frameworkradar prioritize --business-id <id> --db <path> --semantic-pack <path>")
	}
	switch os.Args[1] {
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
	dbPath := fs.String("db", "", "path SQLite")
	semanticPath := fs.String("semantic-pack", "", "path semantic pack")
	contextB64 := fs.String("context-b64", "", "contexto runtime codificado")
	_ = contextB64
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*dbPath) == "" {
		emitNeedsConfiguration(*businessID, "missing_db", "Falta data.sqlite_db.v1 para calcular prioridades.")
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
	items, err := scoreSQLite(*dbPath, model)
	if err != nil {
		emitNeedsConfiguration(firstNonEmpty(*businessID, pack.BusinessID), "query_error", err.Error())
		return nil
	}
	emitPriorityList(firstNonEmpty(*businessID, pack.BusinessID), items, model)
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

func scoreSQLite(dbPath string, model collectionScoring) ([]priorityItem, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_pragma=query_only(true)")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	tables, err := inspectTables(db)
	if err != nil {
		return nil, err
	}
	entityTable, ok := tables[model.EntityTable]
	if !ok {
		return nil, fmt.Errorf("tabla de entidad no existe: %s", model.EntityTable)
	}
	itemTable, ok := tables[model.ItemTable]
	if !ok {
		return nil, fmt.Errorf("tabla de cobros no existe: %s", model.ItemTable)
	}

	model.EntityIDColumn = existingColumn(entityTable, model.EntityIDColumn, "id", "code")
	model.EntityNameColumn = existingColumn(entityTable, model.EntityNameColumn, "name", "nombre", "code", model.EntityIDColumn)
	model.ItemEntityColumn = existingColumn(itemTable, model.ItemEntityColumn, singular(model.EntityTable)+"_id", "client_id", "customer_id", "entity_id")
	model.ItemJoinColumn = existingColumn(itemTable, model.ItemJoinColumn, "id")
	model.StatusColumn = existingColumn(itemTable, model.StatusColumn, "status", "state", "estado")
	model.DateColumn = existingColumn(itemTable, model.DateColumn, "due_date", "date", "date_to", "created_at")
	model.AmountColumn = existingColumn(itemTable, model.AmountColumn, "amount", "balance", "saldo", "total", "residue")
	if model.AmountColumn == "" {
		model.AmountTable, model.AmountJoinColumn, model.AmountColumn = findAmountTable(tables, model.ItemTable)
	}
	if model.ItemEntityColumn == "" || model.ItemJoinColumn == "" {
		return nil, errors.New("no se pudo mapear la relación entre entidad y cobros")
	}
	if model.AmountColumn == "" {
		return nil, errors.New("no se pudo mapear columna de monto para scoring")
	}

	rows, err := fetchRawItems(db, model)
	if err != nil {
		return nil, err
	}
	return aggregateItems(rows), nil
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
	if model.DateColumn != "" {
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

func aggregateItems(rows []rawItem) []priorityItem {
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
	items := make([]priorityItem, 0, len(agg))
	for _, a := range agg {
		days := 0
		if a.HasDate {
			days = int(now.Sub(a.Oldest).Hours() / 24)
			if days < 0 {
				days = 0
			}
		}
		score := computeScore(a.Amount, days)
		reasons := []string{fmt.Sprintf("Saldo total %.0f", a.Amount)}
		if days > 0 {
			reasons = append(reasons, fmt.Sprintf("Más de %d días desde la fecha más antigua", days))
		}
		if a.Count > 1 {
			reasons = append(reasons, fmt.Sprintf("%d documentos/cobros abiertos", a.Count))
		}
		action := recommendedAction(days)
		items = append(items, priorityItem{
			ArtifactType:      "collection.priority_item.v1",
			Score:             score,
			EntityRef:         entityRef{ArtifactType: "entity.ref.v1", Type: "customer", ID: a.EntityID, Name: a.Name},
			Reasons:           reasons,
			RecommendedAction: action,
			Deudor:            a.Name,
			DeudorID:          a.EntityID,
			SaldoTotal:        a.Amount,
			DiasMoraMax:       days,
			FacturasCount:     a.Count,
			Razon:             strings.Join(reasons, "; "),
			AccionSugerida:    action,
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

func emitPriorityList(businessID string, items []priorityItem, model collectionScoring) {
	out := map[string]interface{}{
		"artifact_type":  "collection.priority_list.v1",
		"artifacts":      []string{"collection.priority_list.v1", "collection.priority_item.v1", "entity.ref.v1"},
		"business_id":    businessID,
		"generated_at":   time.Now().UTC().Format(time.RFC3339),
		"scoring_model":  "collection_priority_v1",
		"scoring_source": "framework-radar",
		"items":          items,
		"count":          len(items),
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

func emitNeedsConfiguration(businessID, code, message string) {
	printJSON(map[string]interface{}{
		"artifact_type":        "collection.priority_list.v1",
		"artifacts":            []string{"collection.priority_list.v1"},
		"business_id":          businessID,
		"generated_at":         time.Now().UTC().Format(time.RFC3339),
		"items":                []interface{}{},
		"count":                0,
		"needs_configuration":  true,
		"configuration_code":   code,
		"configuration_reason": message,
	})
}

func findAmountTable(tables map[string]tableInfo, itemTable string) (table, joinColumn, amountColumn string) {
	itemFK := singular(itemTable) + "_id"
	for _, t := range tables {
		join := existingColumn(t, itemFK, "charge_id", "invoice_id", "debt_id", "item_id")
		amount := existingColumn(t, "amount", "balance", "saldo", "total", "residue")
		if join != "" && amount != "" {
			return t.Name, join, amount
		}
	}
	return "", "", ""
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

func computeScore(amount float64, days int) int {
	if amount <= 0 {
		return 0
	}
	age := math.Min(float64(days)/90.0, 1.0) * 60
	size := math.Min(math.Log10(amount+1)/math.Log10(1e7), 1.0) * 40
	return int(math.Round(age + size))
}

func recommendedAction(days int) string {
	switch {
	case days >= 60:
		return "Enviar recordatorio formal y preparar escalamiento si no hay respuesta."
	case days >= 30:
		return "Contactar hoy para negociar fecha de pago."
	default:
		return "Enviar recordatorio amistoso y confirmar recepción."
	}
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
