package main

import (
	"database/sql"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type uiContext struct {
	View             string            `json:"view"`
	Title            string            `json:"title"`
	Subtitle         string            `json:"subtitle"`
	Source           string            `json:"source"`
	Portfolio        *portfolioContext `json:"portfolio,omitempty"`
	Debtor           *derivedContext   `json:"debtor,omitempty"`
	SelectedPriority *priorityItem     `json:"selected_priority,omitempty"`
}

type portfolioSummary struct {
	Clients                   int     `json:"clients"`
	Projects                  int     `json:"projects"`
	Charges                   int     `json:"charges"`
	Documents                 int     `json:"documents"`
	Payments                  int     `json:"payments"`
	OpenCharges               int     `json:"open_charges"`
	PartialCharges            int     `json:"partial_charges"`
	ActiveCollectableProjects int     `json:"active_collectable_projects"`
	ClientsWithPending        int     `json:"clients_with_pending"`
	KnownOpenAmount           float64 `json:"known_open_amount"`
}

type prioritySchema struct {
	SchemaID    string         `json:"schema_id"`
	Description string         `json:"description"`
	Weights     map[string]int `json:"weights"`
}

type priorityCriterion struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Weight       int    `json:"weight"`
	Score        int    `json:"score"`
	Contribution int    `json:"contribution"`
	Summary      string `json:"summary"`
}

type priorityItem struct {
	Rank                       int                 `json:"rank"`
	Priority                   string              `json:"priority"`
	ClientID                   string              `json:"client_id"`
	Debtor                     string              `json:"debtor"`
	ChargesTotal               int                 `json:"charges_total"`
	PaidCharges                int                 `json:"paid_charges"`
	OpenCharges                int                 `json:"open_charges"`
	PartialCharges             int                 `json:"partial_charges"`
	ActiveCollectableProjects  int                 `json:"active_collectable_projects"`
	MissingResponsibleProjects int                 `json:"missing_responsible_projects"`
	MissingDocumentCharges     int                 `json:"missing_document_charges"`
	TimelineAnomalies          int                 `json:"timeline_anomalies"`
	OldestOpenDate             string              `json:"oldest_open_date"`
	OldestAgeDays              int                 `json:"oldest_age_days"`
	LastPaymentDate            string              `json:"last_payment_date"`
	DaysSinceLastPayment       int                 `json:"days_since_last_payment"`
	KnownOpenAmount            float64             `json:"known_open_amount"`
	ObservedPaidRatio          float64             `json:"observed_paid_ratio"`
	Score                      int                 `json:"score"`
	DominantCriterion          string              `json:"dominant_criterion"`
	Criteria                   []string            `json:"criteria"`
	CriteriaBreakdown          []priorityCriterion `json:"criteria_breakdown"`
	Motive                     string              `json:"motive"`
	SuggestedAction            string              `json:"suggested_action"`
	PrimaryExecutive           string              `json:"primary_executive"`
}

type executiveLoad struct {
	Executive                 string `json:"executive"`
	Clients                   int    `json:"clients"`
	Projects                  int    `json:"projects"`
	ActiveCollectableProjects int    `json:"active_collectable_projects"`
	OpenCharges               int    `json:"open_charges"`
	PartialCharges            int    `json:"partial_charges"`
}

type dataQualityItem struct {
	ClientID                   string   `json:"client_id"`
	Debtor                     string   `json:"debtor"`
	MissingResponsibleProjects int      `json:"missing_responsible_projects"`
	MissingDocumentCharges     int      `json:"missing_document_charges"`
	TimelineAnomalies          int      `json:"timeline_anomalies"`
	PrimaryIssues              []string `json:"primary_issues"`
	SuggestedAction            string   `json:"suggested_action"`
}

type portfolioContext struct {
	Summary        portfolioSummary  `json:"summary"`
	PrioritySchema prioritySchema    `json:"priority_schema"`
	TopPriorities  []priorityItem    `json:"top_priorities"`
	ExecutiveLoad  []executiveLoad   `json:"executive_load"`
	DataQuality    []dataQualityItem `json:"data_quality"`
}

type portfolioSnapshot struct {
	Context     portfolioContext
	AllItems    []priorityItem
	ByClientID  map[string]priorityItem
	ClientNames map[string]string
	Source      string
}

type priorityPortfolioStats struct {
	TotalKnownOpenAmount float64
	MaxKnownOpenAmount   float64
	MaxPendingCount      int
}

type execAccumulator struct {
	ClientSet                   map[string]struct{}
	ProjectSet                  map[string]struct{}
	ActiveCollectableProjectSet map[string]struct{}
	OpenCharges                 int
	PartialCharges              int
}

var defaultPrioritySchema = prioritySchema{
	SchemaID:    "collection_priority_40_30_30_v1",
	Description: "materialidad 40%, comportamiento histórico 30% y riesgo legal/antigüedad 30%",
	Weights:     map[string]int{"materialidad": 40, "comportamiento": 30, "riesgo_legal": 30},
}

func loadPortfolioSnapshot(db *sql.DB) (portfolioSnapshot, error) {
	clients, err := queryRows(db, `select coalesce(id,'') as id, coalesce(code,'') as code, coalesce(name,'') as name, coalesce(active,'') as active, coalesce(agreement_start_date,'') as agreement_start_date from clients order by cast(id as integer)`)
	if err != nil {
		return portfolioSnapshot{}, err
	}
	projects, err := queryRows(db, `select coalesce(id,'') as id, coalesce(client_id,'') as client_id, coalesce(agreement_id,'') as agreement_id, coalesce(name,'') as name, coalesce(active,'') as active, coalesce(collectable,'') as collectable, coalesce(project_area_id,'') as project_area_id, coalesce(responsible_user_ids,'') as responsible_user_ids from projects order by cast(id as integer)`)
	if err != nil {
		return portfolioSnapshot{}, err
	}
	charges, err := queryRows(db, `select coalesce(id,'') as id, coalesce(client_id,'') as client_id, coalesce(agreement_id,'') as agreement_id, coalesce(date_to,'') as date_to, coalesce(description,'') as description, coalesce(state,'') as state from charges order by cast(id as integer)`)
	if err != nil {
		return portfolioSnapshot{}, err
	}
	docs, err := queryRows(db, `select coalesce(id,'') as id, coalesce(client_id,'') as client_id, coalesce(charge_id,'') as charge_id, coalesce(date,'') as date, coalesce(number,'') as number from billing_documents order by cast(id as integer)`)
	if err != nil {
		return portfolioSnapshot{}, err
	}
	payments, err := queryRows(db, `select coalesce(id,'') as id, coalesce(client_id,'') as client_id, coalesce(date,'') as date, coalesce(amount,'') as amount, coalesce(residue,'') as residue from payments order by date, cast(id as integer)`)
	if err != nil {
		return portfolioSnapshot{}, err
	}
	users, err := queryRows(db, `select coalesce(id,'') as id, coalesce(name,'') as name from users order by cast(id as integer)`)
	if err != nil {
		return portfolioSnapshot{}, err
	}
	milestones, err := queryRows(db, `select coalesce(id,'') as id, coalesce(charge_id,'') as charge_id, coalesce(amount,'') as amount, coalesce(date,'') as date from milestones order by cast(id as integer)`)
	if err != nil {
		return portfolioSnapshot{}, err
	}

	projectsByClient := map[string][]row{}
	chargesByClient := map[string][]row{}
	paymentsByClient := map[string][]row{}
	projectByAgreement := map[string]row{}
	docCountByCharge := map[string]int{}
	milestoneAmountByCharge := map[string]float64{}
	userByID := map[string]string{}
	clientNames := map[string]string{}

	for _, u := range users {
		userByID[u["id"]] = fallbackLabel(u["name"], "Usuario "+u["id"])
	}
	for _, p := range projects {
		projectsByClient[p["client_id"]] = append(projectsByClient[p["client_id"]], p)
		if strings.TrimSpace(p["agreement_id"]) != "" {
			projectByAgreement[p["agreement_id"]] = p
		}
	}
	for _, ch := range charges {
		chargesByClient[ch["client_id"]] = append(chargesByClient[ch["client_id"]], ch)
	}
	for _, p := range payments {
		paymentsByClient[p["client_id"]] = append(paymentsByClient[p["client_id"]], p)
	}
	for _, d := range docs {
		docCountByCharge[d["charge_id"]]++
	}
	for _, m := range milestones {
		milestoneAmountByCharge[m["charge_id"]] += num(m["amount"])
	}

	executiveAcc := map[string]*execAccumulator{}
	getExecAccumulator := func(name string) *execAccumulator {
		name = fallbackLabel(name, "Sin responsable")
		acc := executiveAcc[name]
		if acc == nil {
			acc = &execAccumulator{
				ClientSet:                   map[string]struct{}{},
				ProjectSet:                  map[string]struct{}{},
				ActiveCollectableProjectSet: map[string]struct{}{},
			}
			executiveAcc[name] = acc
		}
		return acc
	}

	items := []priorityItem{}
	byClientID := map[string]priorityItem{}
	totalKnownOpenAmount := 0.0
	totalOpenCharges := 0
	totalPartialCharges := 0
	clientsWithPending := 0
	activeCollectableProjectTotal := 0

	for _, client := range clients {
		clientID := client["id"]
		clientName := fallbackLabel(client["name"], "Cliente "+clientID)
		clientNames[clientID] = clientName
		projects := projectsByClient[clientID]
		clientCharges := chargesByClient[clientID]
		clientPayments := paymentsByClient[clientID]

		activeCollectableProjects := 0
		missingResponsibleProjects := 0
		primaryExecCounts := map[string]int{}
		for _, p := range projects {
			if p["active"] == "1" && p["collectable"] == "1" {
				activeCollectableProjects++
			}
			ids := safeJSONStringSlice(p["responsible_user_ids"])
			if len(ids) == 0 {
				missingResponsibleProjects++
				continue
			}
			for _, id := range ids {
				primaryExecCounts[fallbackLabel(userByID[id], "Usuario "+id)]++
			}
		}
		activeCollectableProjectTotal += activeCollectableProjects

		openCharges := 0
		partialCharges := 0
		paidCharges := 0
		missingDocumentCharges := 0
		timelineAnomalies := 0
		knownOpenAmount := 0.0
		oldestOpen := time.Time{}
		for _, ch := range clientCharges {
			state := ch["state"]
			isOpenLike := state == "FACTURADO" || state == "PAGO PARCIAL"
			switch state {
			case "FACTURADO":
				openCharges++
			case "PAGO PARCIAL":
				partialCharges++
			case "PAGADO":
				paidCharges++
			}
			if !isOpenLike {
				continue
			}
			if docCountByCharge[ch["id"]] == 0 {
				missingDocumentCharges++
			}
			knownOpenAmount += milestoneAmountByCharge[ch["id"]]
			chargeDate := parseDate(ch["date_to"])
			if state == "FACTURADO" && !chargeDate.IsZero() && (oldestOpen.IsZero() || chargeDate.Before(oldestOpen)) {
				oldestOpen = chargeDate
			}
			largestGap := 0.0
			for _, embedded := range extractEmbeddedDates(ch["description"]) {
				if chargeDate.IsZero() {
					continue
				}
				gap := chargeDate.Sub(embedded).Hours() / 24 / 365
				if gap < 0 {
					gap = -gap
				}
				if gap > largestGap {
					largestGap = gap
				}
			}
			if largestGap >= 3 {
				timelineAnomalies++
			}
			project := projectByAgreement[ch["agreement_id"]]
			responsibles := safeJSONStringSlice(project["responsible_user_ids"])
			if len(responsibles) == 0 {
				responsibles = []string{""}
			}
			for _, id := range responsibles {
				execName := "Sin responsable"
				if strings.TrimSpace(id) != "" {
					execName = fallbackLabel(userByID[id], "Usuario "+id)
				}
				acc := getExecAccumulator(execName)
				acc.ClientSet[clientID] = struct{}{}
				if pid := strings.TrimSpace(project["id"]); pid != "" {
					acc.ProjectSet[pid] = struct{}{}
					if project["active"] == "1" && project["collectable"] == "1" {
						acc.ActiveCollectableProjectSet[pid] = struct{}{}
					}
				}
				if state == "FACTURADO" {
					acc.OpenCharges++
				}
				if state == "PAGO PARCIAL" {
					acc.PartialCharges++
				}
			}
		}

		if openCharges == 0 && partialCharges == 0 {
			continue
		}
		clientsWithPending++
		totalOpenCharges += openCharges
		totalPartialCharges += partialCharges
		totalKnownOpenAmount += knownOpenAmount

		lastPaymentDate := ""
		lastPaymentTime := time.Time{}
		for _, pay := range clientPayments {
			t := parseDate(pay["date"])
			if t.IsZero() {
				continue
			}
			if lastPaymentTime.IsZero() || t.After(lastPaymentTime) {
				lastPaymentTime = t
				lastPaymentDate = pay["date"]
			}
		}
		daysSinceLastPayment := -1
		if !lastPaymentTime.IsZero() {
			daysSinceLastPayment = int(time.Since(lastPaymentTime).Hours() / 24)
		}
		oldestAgeDays := 0
		oldestOpenDate := ""
		if !oldestOpen.IsZero() {
			oldestOpenDate = oldestOpen.Format("2006-01-02")
			oldestAgeDays = int(time.Since(oldestOpen).Hours() / 24)
		}
		chargesTotal := len(clientCharges)
		observedPaidRatio := 0.0
		if chargesTotal > 0 {
			observedPaidRatio = float64(paidCharges) / float64(chargesTotal)
		}
		primaryExecutive := topKey(primaryExecCounts, "Sin responsable")
		item := priorityItem{
			ClientID:                   clientID,
			Debtor:                     clientName,
			ChargesTotal:               chargesTotal,
			PaidCharges:                paidCharges,
			OpenCharges:                openCharges,
			PartialCharges:             partialCharges,
			ActiveCollectableProjects:  activeCollectableProjects,
			MissingResponsibleProjects: missingResponsibleProjects,
			MissingDocumentCharges:     missingDocumentCharges,
			TimelineAnomalies:          timelineAnomalies,
			OldestOpenDate:             oldestOpenDate,
			OldestAgeDays:              oldestAgeDays,
			LastPaymentDate:            lastPaymentDate,
			DaysSinceLastPayment:       daysSinceLastPayment,
			KnownOpenAmount:            knownOpenAmount,
			ObservedPaidRatio:          observedPaidRatio,
			PrimaryExecutive:           primaryExecutive,
		}
		items = append(items, item)
	}

	modelStats := priorityPortfolioStats{
		TotalKnownOpenAmount: totalKnownOpenAmount,
		MaxKnownOpenAmount:   maxKnownOpenAmount(items),
		MaxPendingCount:      maxPendingCount(items),
	}
	for i := range items {
		breakdown := buildBusinessPriorityBreakdown(items[i], modelStats)
		score := weightedPriorityScore(breakdown)
		items[i].CriteriaBreakdown = breakdown
		items[i].DominantCriterion = dominantPriorityCriterion(breakdown)
		items[i].Criteria = buildPriorityCriteria(breakdown)
		items[i].Score = score
		items[i].Priority = classifyPriority(score)
		items[i].Motive = buildPriorityMotive(breakdown)
		items[i].SuggestedAction = suggestedActionForPriority(items[i])
		byClientID[items[i].ClientID] = items[i]
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		if items[i].OpenCharges != items[j].OpenCharges {
			return items[i].OpenCharges > items[j].OpenCharges
		}
		if items[i].OldestAgeDays != items[j].OldestAgeDays {
			return items[i].OldestAgeDays > items[j].OldestAgeDays
		}
		return items[i].Debtor < items[j].Debtor
	})
	for i := range items {
		items[i].Rank = i + 1
		byClientID[items[i].ClientID] = items[i]
	}

	executives := make([]executiveLoad, 0, len(executiveAcc))
	for name, acc := range executiveAcc {
		executives = append(executives, executiveLoad{
			Executive:                 name,
			Clients:                   len(acc.ClientSet),
			Projects:                  len(acc.ProjectSet),
			ActiveCollectableProjects: len(acc.ActiveCollectableProjectSet),
			OpenCharges:               acc.OpenCharges,
			PartialCharges:            acc.PartialCharges,
		})
	}
	sort.Slice(executives, func(i, j int) bool {
		if executives[i].OpenCharges != executives[j].OpenCharges {
			return executives[i].OpenCharges > executives[j].OpenCharges
		}
		if executives[i].PartialCharges != executives[j].PartialCharges {
			return executives[i].PartialCharges > executives[j].PartialCharges
		}
		if executives[i].ActiveCollectableProjects != executives[j].ActiveCollectableProjects {
			return executives[i].ActiveCollectableProjects > executives[j].ActiveCollectableProjects
		}
		return executives[i].Executive < executives[j].Executive
	})

	dataQuality := []dataQualityItem{}
	for _, item := range items {
		issues := []string{}
		if item.MissingResponsibleProjects > 0 {
			issues = append(issues, fmt.Sprintf("%d proyectos sin responsable", item.MissingResponsibleProjects))
		}
		if item.MissingDocumentCharges > 0 {
			issues = append(issues, fmt.Sprintf("%d cargos abiertos sin documento", item.MissingDocumentCharges))
		}
		if item.TimelineAnomalies > 0 {
			issues = append(issues, fmt.Sprintf("%d anomalías temporales", item.TimelineAnomalies))
		}
		if len(issues) == 0 {
			continue
		}
		dataQuality = append(dataQuality, dataQualityItem{
			ClientID:                   item.ClientID,
			Debtor:                     item.Debtor,
			MissingResponsibleProjects: item.MissingResponsibleProjects,
			MissingDocumentCharges:     item.MissingDocumentCharges,
			TimelineAnomalies:          item.TimelineAnomalies,
			PrimaryIssues:              issues,
			SuggestedAction:            suggestedActionForPriority(item),
		})
	}
	sort.Slice(dataQuality, func(i, j int) bool {
		scoreI := dataQuality[i].MissingResponsibleProjects + dataQuality[i].MissingDocumentCharges + dataQuality[i].TimelineAnomalies
		scoreJ := dataQuality[j].MissingResponsibleProjects + dataQuality[j].MissingDocumentCharges + dataQuality[j].TimelineAnomalies
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		return dataQuality[i].Debtor < dataQuality[j].Debtor
	})

	snapshot := portfolioSnapshot{
		Context: portfolioContext{
			Summary: portfolioSummary{
				Clients:                   len(clients),
				Projects:                  len(projects),
				Charges:                   len(charges),
				Documents:                 len(docs),
				Payments:                  len(payments),
				OpenCharges:               totalOpenCharges,
				PartialCharges:            totalPartialCharges,
				ActiveCollectableProjects: activeCollectableProjectTotal,
				ClientsWithPending:        clientsWithPending,
				KnownOpenAmount:           totalKnownOpenAmount,
			},
			PrioritySchema: clonePrioritySchema(defaultPrioritySchema),
			TopPriorities:  trimPriorityItems(items, 5),
			ExecutiveLoad:  trimExecutiveLoad(executives, 5),
			DataQuality:    trimDataQualityItems(dataQuality, 5),
		},
		AllItems:    items,
		ByClientID:  byClientID,
		ClientNames: clientNames,
		Source:      defaultSource,
	}
	return snapshot, nil
}

func buildPortfolioAgenda() []agendaGroup {
	return []agendaGroup{
		{ID: "priority", Title: "1. ¿Qué casos conviene trabajar primero y por qué?", Why: "La cartera solo mejora si convierto la foto general en una lista diaria accionable.", Items: []agendaItem{{Text: "¿Cuál es el ranking diario de deudores/casos?", Status: "active"}, {Text: "¿Qué criterio pesa más en cada prioridad?"}, {Text: "¿Qué acción conviene sugerir para cada caso?"}}},
		{ID: "debtor", Title: "2. ¿Qué revela el diagnóstico 360 del deudor prioritario?", Why: "El ranking debe abrirse en un análisis concreto antes de decidir la gestión.", Items: []agendaItem{{Text: "¿Cuál es el estado actual del deudor seleccionado?"}, {Text: "¿Qué oportunidad o recuperabilidad visible tiene?"}, {Text: "¿Cuál es la próxima mejor acción?"}}},
		{ID: "chat", Title: "3. ¿Qué preguntas gerenciales de cartera sí puedo responder hoy?", Why: "El chat interno debe convertir la cartera en una herramienta de gestión diaria.", Items: []agendaItem{{Text: "¿Qué ejecutivos cargan más atraso visible?"}, {Text: "¿Qué casos muestran datos incompletos?"}, {Text: "¿Qué clientes requieren comparación o resumen ejecutivo?"}}},
	}
}

func buildDebtor360Agenda(ctx derivedContext, item priorityItem) []agendaGroup {
	titleName := fallbackLabel(ctx.Profile.Name, item.Debtor)
	if strings.TrimSpace(titleName) == "" {
		titleName = "el deudor"
	}
	return []agendaGroup{
		{ID: "state", Title: "1. ¿Cuál es el estado actual del deudor?", Why: "Necesito partir desde hechos visibles: cargos abiertos, pagos, soporte y concentración del problema.", Items: []agendaItem{{Text: fmt.Sprintf("¿Qué pendiente visible tiene %s hoy?", titleName), Status: "active"}, {Text: "¿Dónde se concentra el foco operativo?"}, {Text: "¿Qué soporte documental y pagos existen?"}}},
		{ID: "opportunity", Title: "2. ¿Qué oportunidad de recuperación veo?", Why: "Antes de escalar, debo distinguir cobrabilidad real de ruido administrativo o de conciliación.", Items: []agendaItem{{Text: "¿Hay señales de pago parcial o actividad reciente?"}, {Text: "¿Hay anomalías o vacíos que cambien la lectura?"}, {Text: "¿Cómo se compara este deudor contra la cartera?"}}},
		{ID: "action", Title: "3. ¿Cuál es la próxima mejor acción?", Why: "La vista 360 solo sirve si termina en una gestión concreta y proporcionada a la evidencia.", Items: []agendaItem{{Text: "¿Qué debería hacer hoy cobranzas?"}, {Text: "¿Qué debo validar antes de contactar o escalar?"}, {Text: "¿Qué límites de la fuente debo explicitar?"}}},
	}
}

func buildUIContext(s *session) uiContext {
	if s.View == "debtor" && strings.TrimSpace(s.Context.Profile.Name) != "" {
		ctx := uiContext{
			View:     "debtor",
			Title:    "Diagnóstico 360 del deudor",
			Subtitle: "Fuente delimitada: panalbit.sqlite",
			Source:   defaultSource,
			Debtor:   &s.Context,
		}
		if item, ok := s.Portfolio.ByClientID[s.CurrentClientID]; ok {
			copyItem := item
			ctx.SelectedPriority = &copyItem
		}
		return ctx
	}
	return uiContext{
		View:      "portfolio",
		Title:     "Cartera Panalbit AI",
		Subtitle:  "Panorama general, priorización diaria y chat gerencial",
		Source:    defaultSource,
		Portfolio: &s.Portfolio.Context,
	}
}

func (a *app) deterministicResponseForPrompt(s *session, prompt string) (*aiResponse, error) {
	normalized := normalizeLabel(prompt)
	if normalized == "" {
		return nil, nil
	}
	if isClarificationPrompt(normalized) {
		switch s.LastIntent {
		case "priority_list":
			return priorityExplanationResponse(s), nil
		case "debtor_360", "debtor_opportunity", "debtor_action":
			return debtorClarificationResponse(s), nil
		case "executive_load":
			return executiveClarificationResponse(s), nil
		case "data_quality":
			return dataQualityClarificationResponse(s), nil
		}
	}
	switch normalized {
	case "ver priorizacion de hoy", "mostrar priorizacion", "muestrame la priorizacion":
		return portfolioPriorityResponse(s, 10), nil
	case "analizar deudor prioritario", "analiza al deudor mas prioritario", "analiza el deudor mas prioritario":
		return a.openTopDebtor(s, prompt)
	case "ver ejecutivos con atraso", "mostrar ejecutivos":
		return executiveLoadResponse(s), nil
	case "ver casos con datos incompletos", "mostrar casos con datos incompletos":
		return dataQualityResponse(s), nil
	case "volver a cartera", "volver al panorama general":
		return a.returnToPortfolio(s), nil
	case "ver siguiente deudor", "analizar siguiente deudor":
		return a.openNextDebtor(s, prompt)
	}
	if !a.hasGroqKey(s) && isSmallTalkPrompt(prompt) {
		return socialSmallTalkResponse(s, prompt), nil
	}
	if clientID := findClientIDInPrompt(s, normalized); clientID != "" && (strings.Contains(normalized, "resume la cartera") || strings.Contains(normalized, "resumen de la cartera") || strings.Contains(normalized, "analiza") || strings.Contains(normalized, "analizar") || strings.Contains(normalized, "deudor") || strings.Contains(normalized, "cliente")) {
		return a.openDebtorByClientID(s, clientID, prompt)
	}
	if isPriorityPrompt(normalized) {
		return portfolioPriorityResponse(s, requestedLimit(normalized, 10)), nil
	}
	if strings.Contains(normalized, "analiza al deudor mas prioritario") || strings.Contains(normalized, "analiza el deudor mas prioritario") || strings.Contains(normalized, "analizar deudor prioritario") {
		return a.openTopDebtor(s, prompt)
	}
	if strings.Contains(normalized, "ejecutiv") && (strings.Contains(normalized, "cartera") || strings.Contains(normalized, "atrasad") || strings.Contains(normalized, "atraso")) {
		return executiveLoadResponse(s), nil
	}
	if strings.Contains(normalized, "datos incomplet") || strings.Contains(normalized, "vacios") || strings.Contains(normalized, "vacios") {
		return dataQualityResponse(s), nil
	}
	if strings.Contains(normalized, "mandantes") || strings.Contains(normalized, "peor recuperacion") || strings.Contains(normalized, "peor recuperación") {
		return recoveryRiskResponse(s), nil
	}
	if strings.Contains(normalized, "promesa") {
		return unsupportedPromesasResponse(s), nil
	}
	if strings.Contains(normalized, "gestion") || strings.Contains(normalized, "gestión") {
		return unsupportedGestionesResponse(s), nil
	}
	if !a.hasGroqKey(s) && s.View == "debtor" && strings.TrimSpace(s.Context.Profile.Name) != "" {
		return debtorFollowupResponse(s), nil
	}
	if !a.hasGroqKey(s) {
		return portfolioLandingResponse(s), nil
	}
	return nil, nil
}

func socialSmallTalkResponse(s *session, prompt string) *aiResponse {
	s.LastIntent = "small_talk"
	normalized := cleanedPrompt(prompt)
	if s.View == "debtor" && strings.TrimSpace(s.Context.Profile.Name) != "" {
		switch {
		case strings.Contains(normalized, "como") || strings.Contains(normalized, "y tu") || strings.Contains(normalized, "que tal"):
			return &aiResponse{Phase: "exploring", Text: fmt.Sprintf("Bien, gracias. Ya tengo abierto el diagnóstico 360 de <strong>%s</strong>.", s.Context.Profile.Name), Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver siguiente deudor", Primary: true}, {Label: "Volver a cartera"}}}
		case strings.Contains(normalized, "gracias"):
			return &aiResponse{Phase: "exploring", Text: fmt.Sprintf("De nada. Sigo atento al caso de <strong>%s</strong>.", s.Context.Profile.Name), Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver siguiente deudor", Primary: true}, {Label: "Volver a cartera"}}}
		case strings.Contains(normalized, "dale") || strings.Contains(normalized, "ok") || strings.Contains(normalized, "vale") || strings.Contains(normalized, "perfecto"):
			return &aiResponse{Phase: "exploring", Text: fmt.Sprintf("Perfecto. Sigo sobre el diagnóstico de <strong>%s</strong>.", s.Context.Profile.Name), Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver siguiente deudor", Primary: true}, {Label: "Volver a cartera"}}}
		default:
			return &aiResponse{Phase: "exploring", Text: fmt.Sprintf("Hola. Ya tengo abierto el diagnóstico 360 de <strong>%s</strong>.", s.Context.Profile.Name), Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver siguiente deudor", Primary: true}, {Label: "Volver a cartera"}}}
		}
	}
	switch {
	case strings.Contains(normalized, "como") || strings.Contains(normalized, "y tu") || strings.Contains(normalized, "que tal"):
		return &aiResponse{Phase: "exploring", Text: "Bien, gracias. Ya tengo cargada la cartera completa y lista para priorizar.", Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}}}
	case strings.Contains(normalized, "gracias"):
		return &aiResponse{Phase: "exploring", Text: "De nada. Sigo listo para ayudarte con la cartera.", Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}}}
	case strings.Contains(normalized, "dale") || strings.Contains(normalized, "ok") || strings.Contains(normalized, "vale") || strings.Contains(normalized, "perfecto"):
		return &aiResponse{Phase: "exploring", Text: "Perfecto. Tengo la cartera lista para seguir.", Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}}}
	default:
		return &aiResponse{Phase: "exploring", Text: "Hola. Ya tengo cargada la cartera completa de Panalbit.", Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}}}
	}
}

func (a *app) deterministicResponseForAction(s *session, label string) (*aiResponse, error) {
	normalized := normalizeLabel(label)
	switch normalized {
	case "ver priorizacion de hoy":
		return portfolioPriorityResponse(s, 10), nil
	case "analizar deudor prioritario", "abrir top caso":
		return a.openTopDebtor(s, label)
	case "ver ejecutivos con atraso", "mostrar ejecutivos":
		return executiveLoadResponse(s), nil
	case "ver casos con datos incompletos", "mostrar casos con datos incompletos":
		return dataQualityResponse(s), nil
	case "volver a cartera", "volver al panorama general":
		return a.returnToPortfolio(s), nil
	case "ver siguiente deudor", "analizar siguiente deudor":
		return a.openNextDebtor(s, label)
	case "pausar analisis":
		return &aiResponse{Phase: "done", Text: "Análisis pausado. La cartera y el deudor actual quedan listos para retomarlos después.", Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}}}, nil
	}
	if s.View == "debtor" && normalized == "seguir analizando" {
		return debtorFollowupResponse(s), nil
	}
	if s.View == "portfolio" && normalized == "seguir analizando" {
		return portfolioLandingResponse(s), nil
	}
	return nil, nil
}

func portfolioLandingResponse(s *session) *aiResponse {
	s.LastIntent = "portfolio_landing"
	top := trimPriorityItems(s.Portfolio.AllItems, 5)
	rows := [][]string{}
	for _, item := range top {
		rows = append(rows, []string{
			fmt.Sprintf("%d. %s", item.Rank, item.Debtor),
			fmt.Sprintf("%s · score %d", item.Priority, item.Score),
			fmt.Sprintf("%s", item.Motive),
		})
	}
	return &aiResponse{
		Phase:          "exploring",
		Agenda:         buildPortfolioAgenda(),
		AgendaProgress: makeAgendaProgress("priority", 0, nil),
		Text:           fmt.Sprintf("Ya tengo cargada la cartera completa sobre <strong>panalbit.sqlite</strong>. Hoy puedo operar en tres modos sobre la misma base: <strong>priorizador inteligente</strong>, <strong>diagnóstico 360 del deudor</strong> y <strong>chat interno gerencial</strong>.<br><br>El priorizador quedó alineado con el esquema de negocio <strong>%s</strong>: <strong>materialidad 40%%</strong>, <strong>comportamiento histórico 30%%</strong> y <strong>riesgo legal/antigüedad 30%%</strong>. Con eso ordeno la cartera por exposición, señales de pago y envejecimiento del pendiente, sin perder de vista si la acción correcta es cobrar, validar o depurar datos.<br><br>También puedo contestar preguntas sobre ejecutivos con atraso visible, casos con datos incompletos y resúmenes por cliente. Si quieres partir por acción, te muestro la lista diaria o bajo directo al deudor más prioritario.", s.Portfolio.Context.PrioritySchema.SchemaID),
		Metrics:        []metric{{Value: strconv.Itoa(s.Portfolio.Context.Summary.ClientsWithPending), Label: "Deudores con pendiente"}, {Value: strconv.Itoa(s.Portfolio.Context.Summary.OpenCharges), Label: "Cargos abiertos"}, {Value: strconv.Itoa(s.Portfolio.Context.Summary.PartialCharges), Label: "Pagos parciales"}, {Value: strconv.FormatFloat(s.Portfolio.Context.Summary.KnownOpenAmount, 'f', 0, 64), Label: "Monto abierto conocido"}},
		Table:          &table{Header: "Top deudores a revisar hoy", Rows: rows},
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}},
	}
}

func portfolioPriorityResponse(s *session, limit int) *aiResponse {
	s.LastIntent = "priority_list"
	items := trimPriorityItems(s.Portfolio.AllItems, limit)
	rows := [][]string{}
	for _, item := range items {
		rows = append(rows, []string{
			fmt.Sprintf("%d. %s", item.Rank, item.Debtor),
			fmt.Sprintf("%s · %s · %d abiertos · %s", item.Priority, priorityCriterionTitle(item.DominantCriterion), item.OpenCharges, item.PrimaryExecutive),
			fmt.Sprintf("%s → %s", item.Motive, item.SuggestedAction),
		})
	}
	text := "Este es el ranking diario de casos que conviene trabajar primero. Lo ordené con los tres criterios de negocio ya definidos para Panalbit AI: <strong>materialidad 40%</strong>, <strong>comportamiento histórico 30%</strong> y <strong>riesgo legal/antigüedad 30%</strong>.<br><br>La prioridad no significa siempre “cobrar más fuerte”. Si un caso sube por <strong>riesgo/antigüedad</strong> o por <strong>datos incompletos</strong>, la acción sugerida puede cambiar a validación, conciliación o depuración. Así evito mezclar gestión comercial con pendientes administrativos.<br><br>La tabla deja cada fila en el formato que pediste: <strong>prioridad</strong>, <strong>caso</strong>, <strong>motivo</strong> y <strong>acción sugerida</strong>."
	completed := []agendaCompletedItem{{QuestionID: "priority", ItemIndex: 0}}
	if len(items) > 0 {
		completed = append(completed, agendaCompletedItem{QuestionID: "priority", ItemIndex: 1})
	}
	return &aiResponse{
		Phase:          "synthesizing",
		Agenda:         buildPortfolioAgenda(),
		AgendaProgress: makeAgendaProgress("priority", 2, completed),
		Text:           text,
		Metrics:        []metric{{Value: strconv.Itoa(len(items)), Label: "Casos listados"}, {Value: strconv.Itoa(s.Portfolio.Context.Summary.OpenCharges), Label: "Cargos abiertos"}, {Value: strconv.Itoa(s.Portfolio.Context.Summary.ClientsWithPending), Label: "Deudores con pendiente"}, {Value: strconv.FormatFloat(s.Portfolio.Context.Summary.KnownOpenAmount, 'f', 0, 64), Label: "Monto conocido"}},
		Table:          &table{Header: "Priorización operativa de cartera", Rows: rows},
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Analizar deudor prioritario", Primary: true}, {Label: "Ver ejecutivos con atraso"}},
	}
}

func executiveLoadResponse(s *session) *aiResponse {
	s.LastIntent = "executive_load"
	rows := [][]string{}
	for _, exec := range trimExecutiveLoad(s.Portfolio.Context.ExecutiveLoad, 10) {
		rows = append(rows, []string{
			exec.Executive,
			fmt.Sprintf("%d abiertos · %d parciales", exec.OpenCharges, exec.PartialCharges),
			fmt.Sprintf("%d clientes · %d proyectos cobrables", exec.Clients, exec.ActiveCollectableProjects),
		})
	}
	return &aiResponse{
		Phase:          "synthesizing",
		Agenda:         buildPortfolioAgenda(),
		AgendaProgress: makeAgendaProgress("chat", 1, []agendaCompletedItem{{QuestionID: "chat", ItemIndex: 0}}),
		Text:           "Sí puedo responder qué <strong>ejecutivos</strong> concentran más atraso visible usando los proyectos activos/cobrables asignados y sus cargos abiertos. Esto no reemplaza una bandeja formal de cobranza, pero sí te da una lectura gerencial inmediata de carga y foco operativo.",
		Metrics:        []metric{{Value: strconv.Itoa(len(s.Portfolio.Context.ExecutiveLoad)), Label: "Ejecutivos visibles"}, {Value: strconv.Itoa(s.Portfolio.Context.Summary.OpenCharges), Label: "Abiertos visibles"}, {Value: strconv.Itoa(s.Portfolio.Context.Summary.ActiveCollectableProjects), Label: "Proyectos cobrables"}},
		Table:          &table{Header: "Ejecutivos con mayor atraso visible", Rows: rows},
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Ver casos con datos incompletos"}},
	}
}

func dataQualityResponse(s *session) *aiResponse {
	s.LastIntent = "data_quality"
	rows := [][]string{}
	for _, item := range trimDataQualityItems(s.Portfolio.Context.DataQuality, 10) {
		rows = append(rows, []string{
			item.Debtor,
			strings.Join(item.PrimaryIssues, " · "),
			item.SuggestedAction,
		})
	}
	return &aiResponse{
		Phase:          "synthesizing",
		Agenda:         buildPortfolioAgenda(),
		AgendaProgress: makeAgendaProgress("chat", 2, []agendaCompletedItem{{QuestionID: "chat", ItemIndex: 1}}),
		Text:           "También puedo detectar <strong>casos con datos incompletos</strong> a partir de tres señales visibles en la base: proyectos sin responsable, cargos abiertos sin documento y anomalías temporales entre el periodo descrito y la fecha administrativa. Esto convierte el chat en una herramienta de saneamiento operativo, no solo de lectura.",
		Metrics:        []metric{{Value: strconv.Itoa(len(s.Portfolio.Context.DataQuality)), Label: "Casos con brechas"}, {Value: strconv.Itoa(s.Portfolio.Context.Summary.ClientsWithPending), Label: "Casos observados"}},
		Table:          &table{Header: "Casos con datos incompletos", Rows: rows},
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}},
	}
}

func recoveryRiskResponse(s *session) *aiResponse {
	s.LastIntent = "recovery_risk"
	items := append([]priorityItem(nil), s.Portfolio.AllItems...)
	sort.Slice(items, func(i, j int) bool {
		riskI := recoveryRiskScore(items[i])
		riskJ := recoveryRiskScore(items[j])
		if riskI != riskJ {
			return riskI > riskJ
		}
		return items[i].Debtor < items[j].Debtor
	})
	items = trimPriorityItems(items, 10)
	rows := [][]string{}
	for _, item := range items {
		rows = append(rows, []string{
			item.Debtor,
			fmt.Sprintf("riesgo %d · ratio pagado %.0f%%", recoveryRiskScore(item), item.ObservedPaidRatio*100),
			fmt.Sprintf("%d abiertos · %d días · %s", item.OpenCharges, item.OldestAgeDays, item.SuggestedAction),
		})
	}
	return &aiResponse{
		Phase:          "synthesizing",
		Agenda:         buildPortfolioAgenda(),
		AgendaProgress: makeAgendaProgress("chat", 2, []agendaCompletedItem{{QuestionID: "chat", ItemIndex: 2}}),
		Text:           "No tengo una métrica nativa de <strong>recuperación</strong> por mandante, pero sí puedo armar una lectura de <strong>riesgo de recuperación visible</strong> con la data actual: mezcla de cargos abiertos, antigüedad, pagos recientes y proporción histórica de cargos pagados. Tómalo como proxy operativo, no como KPI financiero oficial.",
		Table:          &table{Header: "Clientes con peor recuperación visible", Rows: rows},
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}},
	}
}

func unsupportedPromesasResponse(s *session) *aiResponse {
	s.LastIntent = "unsupported_promesas"
	rows := [][]string{}
	for _, item := range trimPriorityItems(s.Portfolio.AllItems, 10) {
		rows = append(rows, []string{item.Debtor, fmt.Sprintf("%d abiertos · %d días", item.OpenCharges, item.OldestAgeDays), item.SuggestedAction})
	}
	return &aiResponse{
		Phase:          "exploring",
		Agenda:         buildPortfolioAgenda(),
		AgendaProgress: makeAgendaProgress("chat", 2, nil),
		Text:           "No veo una entidad de <strong>promesas</strong> en <strong>panalbit.sqlite</strong>, así que no puedo afirmar qué promesas vencen esta semana sin inventar. Lo más cercano que sí puedo darte con evidencia es la lista de casos abiertos que ameritan seguimiento inmediato o conciliación hoy.",
		Table:          &table{Header: "Análogo soportado: casos abiertos para seguimiento", Rows: rows},
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}},
	}
}

func unsupportedGestionesResponse(s *session) *aiResponse {
	s.LastIntent = "unsupported_gestiones"
	rows := [][]string{}
	for _, item := range trimPriorityItems(s.Portfolio.AllItems, 10) {
		rows = append(rows, []string{item.Debtor, fmt.Sprintf("último pago %s", fallbackLabel(item.LastPaymentDate, "sin dato")), fmt.Sprintf("%d abiertos · %s", item.OpenCharges, item.SuggestedAction)})
	}
	return &aiResponse{
		Phase:          "exploring",
		Agenda:         buildPortfolioAgenda(),
		AgendaProgress: makeAgendaProgress("chat", 2, nil),
		Text:           "La base actual no trae una tabla de <strong>gestiones</strong> ni una fecha de <strong>última gestión</strong>, así que no puedo responder literalmente qué causas llevan más de 15 días sin gestión. El proxy más útil que sí puedo sostener es ver casos abiertos combinados con ausencia de pago reciente visible y con necesidad de revisión hoy.",
		Table:          &table{Header: "Análogo soportado: casos abiertos sin pago reciente visible", Rows: rows},
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}},
	}
}

func (a *app) openTopDebtor(s *session, prompt string) (*aiResponse, error) {
	if len(s.Portfolio.AllItems) == 0 {
		return &aiResponse{Phase: "exploring", Text: "No encontré deudores con cargos abiertos o pagos parciales visibles en la cartera actual.", Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Pausar análisis"}}}, nil
	}
	return a.openDebtorByClientID(s, s.Portfolio.AllItems[0].ClientID, prompt)
}

func (a *app) openNextDebtor(s *session, prompt string) (*aiResponse, error) {
	if len(s.Portfolio.AllItems) == 0 {
		return portfolioLandingResponse(s), nil
	}
	if strings.TrimSpace(s.CurrentClientID) == "" {
		return a.openTopDebtor(s, prompt)
	}
	for idx, item := range s.Portfolio.AllItems {
		if item.ClientID != s.CurrentClientID {
			continue
		}
		if idx+1 < len(s.Portfolio.AllItems) {
			return a.openDebtorByClientID(s, s.Portfolio.AllItems[idx+1].ClientID, prompt)
		}
		break
	}
	return a.openTopDebtor(s, prompt)
}

func (a *app) returnToPortfolio(s *session) *aiResponse {
	s.View = "portfolio"
	s.CurrentClientID = ""
	s.Context = derivedContext{}
	s.ScriptStep = 0
	s.LastIntent = "portfolio_landing"
	s.Agenda = normalizeAgenda(buildPortfolioAgenda(), derivedContext{})
	return &aiResponse{
		Phase:          "exploring",
		Agenda:         buildPortfolioAgenda(),
		AgendaProgress: makeAgendaProgress("priority", 0, nil),
		Text:           "Volví al panorama general de la cartera. Mantengo la memoria del caso revisado, pero vuelvo a operar desde priorización diaria, comparación entre deudores y consultas gerenciales sobre toda la base.",
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}},
	}
}

func (a *app) openDebtorByClientID(s *session, clientID, prompt string) (*aiResponse, error) {
	ctx, err := loadDerivedContext(a.db, clientID)
	if err != nil {
		return nil, err
	}
	s.View = "debtor"
	s.CurrentClientID = clientID
	s.Context = ctx
	s.ScriptStep = 0
	s.LastIntent = "debtor_360"
	item := s.Portfolio.ByClientID[clientID]
	s.Agenda = normalizeAgenda(buildDebtor360Agenda(ctx, item), ctx)
	completed := []agendaCompletedItem{{QuestionID: "state", ItemIndex: 0}}
	pendingProject := "Sin dato"
	if len(ctx.PendingDetails) > 0 && strings.TrimSpace(ctx.PendingDetails[0].ProjectName) != "" {
		pendingProject = ctx.PendingDetails[0].ProjectName
	}
	docCoverage := fmt.Sprintf("%d/%d", ctx.Coverage.ChargesWithDocuments, ctx.Counts.Charges)
	knownAmount := "No trazable"
	if item.KnownOpenAmount > 0 {
		knownAmount = strconv.FormatFloat(item.KnownOpenAmount, 'f', 0, 64)
	}
	return &aiResponse{
		Phase:          "synthesizing",
		Agenda:         buildDebtor360Agenda(ctx, item),
		AgendaProgress: makeAgendaProgress("state", 1, completed),
		Text:           fmt.Sprintf("Abrí el <strong>diagnóstico 360</strong> de <strong>%s</strong>. Hoy lo tengo arriba en la priorización por <strong>%s</strong>.<br><br>El estado actual muestra <strong>%d cargos abiertos</strong>, <strong>%d pagos parciales</strong>, <strong>%d pagos históricos</strong> y <strong>%d proyectos activos y cobrables</strong>. El foco visible está concentrado en <strong>%s</strong>.<br><br>Como oportunidad, veo %s. Mi lectura inicial es que la próxima acción debe combinar evidencia operativa con prudencia: %s.", ctx.Profile.Name, item.Motive, item.OpenCharges, item.PartialCharges, ctx.Counts.Payments, item.ActiveCollectableProjects, pendingProject, debtorOpportunitySummary(ctx, item), item.SuggestedAction),
		Metrics:        []metric{{Value: strconv.Itoa(item.OpenCharges), Label: "Abiertos"}, {Value: strconv.Itoa(item.PartialCharges), Label: "Pagos parciales"}, {Value: strconv.Itoa(ctx.Counts.Payments), Label: "Pagos"}, {Value: strconv.Itoa(item.Score), Label: "Score cartera"}},
		Table:          &table{Header: "Lectura 360 del deudor", Rows: [][]string{{"Estado actual", fmt.Sprintf("%d abiertos · %d parciales", item.OpenCharges, item.PartialCharges), fmt.Sprintf("%d pagos históricos visibles", ctx.Counts.Payments)}, {"Proyecto foco", pendingProject, "Dónde hoy aparece concentrado el seguimiento"}, {"Cobertura documental", docCoverage, "Soporte visible del histórico"}, {"Monto abierto conocido", knownAmount, "Solo suma milestones ligados a cargos abiertos"}, {"Señal de oportunidad", debtorOpportunitySummary(ctx, item), "Lectura de recuperabilidad visible"}, {"Próxima mejor acción", item.SuggestedAction, "Recomendación operativa inmediata"}}},
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver siguiente deudor", Primary: true}, {Label: "Volver a cartera"}},
	}, nil
}

func debtorFollowupResponse(s *session) *aiResponse {
	item := s.Portfolio.ByClientID[s.CurrentClientID]
	ctx := s.Context
	if s.ScriptStep == 0 {
		s.ScriptStep = 1
		s.LastIntent = "debtor_opportunity"
		return &aiResponse{
			Phase:          "deciding",
			AgendaProgress: makeAgendaProgress("opportunity", 1, []agendaCompletedItem{{QuestionID: "state", ItemIndex: 0}, {QuestionID: "state", ItemIndex: 1}, {QuestionID: "opportunity", ItemIndex: 0}}),
			Text:           fmt.Sprintf("Si profundizo la <strong>oportunidad</strong>, veo tres señales. Primero, <strong>%d pagos históricos</strong> y %s. Segundo, el caso tiene <strong>%d proyecto(s) activo(s) y cobrable(s)</strong>, o sea hay un frente donde sí vale la pena actuar. Tercero, %s.<br><br>Esto me lleva a una lectura concreta: %s es un caso %s dentro de la cartera, pero no siempre para escalar de inmediato; a veces la mejor gestión es validar antes de cobrar fuerte.", ctx.Counts.Payments, paymentSignal(item), item.ActiveCollectableProjects, anomalySignal(item), ctx.Profile.Name, classifyRecoverability(item)),
			Metrics:        []metric{{Value: strconv.Itoa(item.ActiveCollectableProjects), Label: "Proyectos cobrables"}, {Value: strconv.Itoa(item.TimelineAnomalies), Label: "Anomalías"}, {Value: fallbackLabel(item.LastPaymentDate, "Sin dato"), Label: "Último pago"}},
			Table:          &table{Header: "Oportunidad y recuperabilidad", Rows: [][]string{{"Pago/actividad", paymentSignal(item), "Señal de respuesta o inercia"}, {"Cobrabilidad visible", fmt.Sprintf("%d proyecto(s) cobrables", item.ActiveCollectableProjects), "Capacidad de acción hoy"}, {"Anomalías", anomalySignal(item), "Qué puede obligar a validar antes"}, {"Comparación cartera", fmt.Sprintf("rank %d de %d", item.Rank, len(s.Portfolio.AllItems)), "Posición relativa del deudor"}}},
			Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver siguiente deudor", Primary: true}, {Label: "Volver a cartera"}},
		}
	}
	s.ScriptStep = 2
	s.LastIntent = "debtor_action"
	return &aiResponse{
		Phase:          "acting",
		AgendaProgress: makeAgendaProgress("action", 2, []agendaCompletedItem{{QuestionID: "state", ItemIndex: 0}, {QuestionID: "state", ItemIndex: 1}, {QuestionID: "state", ItemIndex: 2}, {QuestionID: "opportunity", ItemIndex: 0}, {QuestionID: "opportunity", ItemIndex: 1}, {QuestionID: "opportunity", ItemIndex: 2}, {QuestionID: "action", ItemIndex: 0}, {QuestionID: "action", ItemIndex: 1}}),
		Text:           fmt.Sprintf("Con esta evidencia, mi <strong>próxima mejor acción</strong> para <strong>%s</strong> es <strong>%s</strong>.<br><br>Si el caso llega a gestión, yo no hablaría de promesas, mora exacta o saldo total si la base no lo soporta. Sí usaría hechos verificables: proyecto foco, cargo(s) abierto(s), documento(s) visibles, pagos históricos y necesidad de confirmación/conciliación.<br><br>En otras palabras: la vista 360 ya te deja decidir hoy, pero sin sobreafirmar lo que <strong>panalbit.sqlite</strong> no trae.", ctx.Profile.Name, item.SuggestedAction),
		Table:          &table{Header: "Próxima mejor acción", Rows: [][]string{{"Sí hacer", item.SuggestedAction, "Acción proporcionada a la evidencia"}, {"Validar antes", validationChecklist(ctx, item), "Chequeos previos a contacto o escalamiento"}, {"No afirmar", "promesas, mora exacta o saldo total", "La fuente no lo soporta de forma completa"}}},
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver siguiente deudor", Primary: true}, {Label: "Volver a cartera"}},
	}
}

func isPriorityPrompt(normalized string) bool {
	if (strings.Contains(normalized, "casos") || strings.Contains(normalized, "deudores") || strings.Contains(normalized, "cartera") || strings.Contains(normalized, "cobrar primero")) && (strings.Contains(normalized, "urg") || strings.Contains(normalized, "prior") || strings.Contains(normalized, "conviene")) {
		return true
	}
	return strings.Contains(normalized, "muestrame los 100 casos mas urgentes") || strings.Contains(normalized, "muestrame la priorizacion") || strings.Contains(normalized, "ver priorizacion")
}

func isClarificationPrompt(normalized string) bool {
	cleaned := strings.NewReplacer("?", "", "¿", "", "!", "", "¡", "", ".", "", ",", "").Replace(normalized)
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	switch cleaned {
	case "y eso", "eso", "por que", "por qué", "explicame", "explica", "a que te refieres":
		return true
	default:
		return cleaned == "que" || cleaned == "como asi"
	}
}

func isMetaQuestionPrompt(text string) bool {
	normalized := cleanedPrompt(text)
	switch {
	case strings.Contains(normalized, "que dia es hoy"),
		strings.Contains(normalized, "que fecha es hoy"),
		strings.Contains(normalized, "como te llamas"),
		strings.Contains(normalized, "quien eres"),
		strings.Contains(normalized, "que eres"),
		strings.Contains(normalized, "y tu como te llamas"),
		strings.Contains(normalized, "cual es tu nombre"):
		return true
	default:
		return false
	}
}

func priorityExplanationResponse(s *session) *aiResponse {
	top := trimPriorityItems(s.Portfolio.AllItems, 5)
	rows := [][]string{}
	for _, item := range top {
		rows = append(rows, []string{
			item.Debtor,
			item.Motive,
			item.SuggestedAction,
		})
	}
	return &aiResponse{
		Phase:          "synthesizing",
		Agenda:         buildPortfolioAgenda(),
		AgendaProgress: makeAgendaProgress("priority", 2, []agendaCompletedItem{{QuestionID: "priority", ItemIndex: 0}, {QuestionID: "priority", ItemIndex: 1}}),
		Text:           "Eso significa que no estoy improvisando un orden arbitrario. Cada caso sube o baja por la combinación ponderada de <strong>materialidad</strong>, <strong>comportamiento histórico</strong> y <strong>riesgo legal/antigüedad</strong>, bajo el esquema 40/30/30. Cuando una fila trae anomalías o faltas de responsable, la prioridad puede seguir siendo alta, pero la acción cambia desde cobranza dura a validación o conciliación.",
		Table:          &table{Header: "Cómo leer la priorización", Rows: rows},
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Analizar deudor prioritario", Primary: true}, {Label: "Ver ejecutivos con atraso"}},
	}
}

func debtorClarificationResponse(s *session) *aiResponse {
	item := s.Portfolio.ByClientID[s.CurrentClientID]
	return &aiResponse{
		Phase:          "deciding",
		Agenda:         buildDebtor360Agenda(s.Context, item),
		AgendaProgress: makeAgendaProgress("action", 0, nil),
		Text:           fmt.Sprintf("Eso significa que, para <strong>%s</strong>, la evidencia visible alcanza para decidir una siguiente acción, pero no para afirmar cualquier cosa. Puedo sostener cargos abiertos, pagos visibles, proyecto foco y algunas señales de recuperabilidad; no puedo sostener promesas, última gestión o mora exacta si la base no las trae.", fallbackLabel(s.Context.Profile.Name, item.Debtor)),
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver siguiente deudor", Primary: true}, {Label: "Volver a cartera"}},
	}
}

func executiveClarificationResponse(s *session) *aiResponse {
	return &aiResponse{
		Phase:          "synthesizing",
		Agenda:         buildPortfolioAgenda(),
		AgendaProgress: makeAgendaProgress("chat", 1, []agendaCompletedItem{{QuestionID: "chat", ItemIndex: 0}}),
		Text:           "Eso significa carga operativa visible, no desempeño humano definitivo. Estoy ordenando ejecutivos por cargos abiertos y proyectos cobrables asignados, para detectar dónde se concentra el atraso observable en la base hoy.",
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Ver casos con datos incompletos"}},
	}
}

func dataQualityClarificationResponse(s *session) *aiResponse {
	return &aiResponse{
		Phase:          "synthesizing",
		Agenda:         buildPortfolioAgenda(),
		AgendaProgress: makeAgendaProgress("chat", 2, []agendaCompletedItem{{QuestionID: "chat", ItemIndex: 1}}),
		Text:           "Eso significa brecha operativa detectable en la base, no necesariamente error definitivo. Hoy marco tres tipos: proyectos sin responsable, cargos abiertos sin documento y anomalías temporales que cambian la lectura del caso.",
		Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}},
	}
}

func requestedLimit(normalized string, fallback int) int {
	matches := regexp.MustCompile(`\b(\d{1,3})\b`).FindStringSubmatch(normalized)
	if len(matches) == 2 {
		if n, err := strconv.Atoi(matches[1]); err == nil && n > 0 {
			if n > 100 {
				return 100
			}
			return n
		}
	}
	return fallback
}

func findClientIDInPrompt(s *session, normalized string) string {
	bestID := ""
	bestLen := 0
	for id, name := range s.Portfolio.ClientNames {
		normName := normalizeLabel(name)
		if normName == "" {
			continue
		}
		if strings.Contains(normalized, normName) && len(normName) > bestLen {
			bestID = id
			bestLen = len(normName)
		}
		if strings.Contains(normalized, "cliente "+normalizeLabel(id)) || strings.Contains(normalized, "deudor "+normalizeLabel(id)) {
			bestID = id
			bestLen = len(normName) + 10
		}
	}
	return bestID
}

func buildPriorityCriteria(breakdown []priorityCriterion) []string {
	ranked := rankedPriorityCriteria(breakdown)
	criteria := make([]string, 0, len(ranked))
	for _, item := range ranked {
		if strings.TrimSpace(item.Summary) == "" {
			continue
		}
		criteria = append(criteria, item.Summary)
		if len(criteria) == 3 {
			break
		}
	}
	return criteria
}

func buildPriorityMotive(breakdown []priorityCriterion) string {
	ranked := rankedPriorityCriteria(breakdown)
	if len(ranked) == 0 {
		return "sin evidencia suficiente"
	}
	parts := []string{}
	for _, item := range ranked {
		if strings.TrimSpace(item.Summary) == "" {
			continue
		}
		parts = append(parts, item.Summary)
		if len(parts) == 2 {
			break
		}
	}
	if len(parts) == 0 {
		return "sin evidencia suficiente"
	}
	return strings.Join(parts, " + ")
}

func buildBusinessPriorityBreakdown(item priorityItem, stats priorityPortfolioStats) []priorityCriterion {
	breakdown := []priorityCriterion{}
	for _, key := range []string{"materialidad", "comportamiento", "riesgo_legal"} {
		score, summary := priorityCriterionScoreAndSummary(key, item, stats)
		weight := defaultPrioritySchema.Weights[key]
		breakdown = append(breakdown, priorityCriterion{
			Key:          key,
			Label:        priorityCriterionTitle(key),
			Weight:       weight,
			Score:        score,
			Contribution: weightedPriorityContribution(score, weight),
			Summary:      summary,
		})
	}
	return breakdown
}

func priorityCriterionScoreAndSummary(key string, item priorityItem, stats priorityPortfolioStats) (int, string) {
	switch key {
	case "materialidad":
		return scoreMaterialidad(item, stats)
	case "comportamiento":
		return scoreComportamiento(item)
	case "riesgo_legal":
		return scoreRiesgoLegal(item)
	default:
		return 0, ""
	}
}

func scoreMaterialidad(item priorityItem, stats priorityPortfolioStats) (int, string) {
	amountScore := 0
	sharePct := 0
	if item.KnownOpenAmount > 0 && stats.TotalKnownOpenAmount > 0 {
		share := item.KnownOpenAmount / stats.TotalKnownOpenAmount
		amountScore = clampScore(share * 300)
		sharePct = clampScore(share * 100)
	} else if item.KnownOpenAmount > 0 && stats.MaxKnownOpenAmount > 0 {
		amountScore = clampScore(item.KnownOpenAmount / stats.MaxKnownOpenAmount * 100)
	}
	pendingCount := item.OpenCharges + item.PartialCharges
	volumeScore := 0
	if pendingCount > 0 && stats.MaxPendingCount > 0 {
		volumeScore = clampScore(float64(pendingCount) / float64(stats.MaxPendingCount) * 100)
	}
	score := 0
	switch {
	case amountScore > 0 && volumeScore > 0:
		score = clampScore(float64(amountScore)*0.7 + float64(volumeScore)*0.3)
	case amountScore > 0:
		score = amountScore
	default:
		score = volumeScore
	}
	switch {
	case item.KnownOpenAmount > 0 && sharePct > 0:
		return score, fmt.Sprintf("materialidad por %d%% del monto abierto conocido", sharePct)
	case item.KnownOpenAmount > 0:
		return score, fmt.Sprintf("materialidad por monto conocido %.0f", item.KnownOpenAmount)
	case pendingCount > 0:
		return score, fmt.Sprintf("materialidad estimada por %d pendiente(s) visibles", pendingCount)
	default:
		return score, "materialidad sin exposición visible suficiente"
	}
}

func scoreComportamiento(item priorityItem) (int, string) {
	switch {
	case item.PartialCharges > 0 && item.DaysSinceLastPayment >= 0 && item.DaysSinceLastPayment <= 45:
		return 82, fmt.Sprintf("comportamiento activo: pagos parciales y último pago hace %d días", item.DaysSinceLastPayment)
	case item.PartialCharges > 0:
		return 74, "comportamiento activo: hay pagos parciales visibles"
	case item.DaysSinceLastPayment > 180:
		return 78, fmt.Sprintf("comportamiento deteriorado: último pago hace %d días", item.DaysSinceLastPayment)
	case item.DaysSinceLastPayment > 90:
		return 68, fmt.Sprintf("comportamiento en alerta: último pago hace %d días", item.DaysSinceLastPayment)
	case item.DaysSinceLastPayment >= 0:
		return 56, fmt.Sprintf("comportamiento con pago visible hace %d días", item.DaysSinceLastPayment)
	case item.PaidCharges == 0:
		return 70, "sin pagos históricos visibles para contrastar el caso"
	default:
		return 50, "comportamiento histórico con evidencia parcial"
	}
}

func scoreRiesgoLegal(item priorityItem) (int, string) {
	score := 0
	switch {
	case item.OldestAgeDays >= 720:
		score = 95
	case item.OldestAgeDays >= 365:
		score = 82
	case item.OldestAgeDays >= 180:
		score = 68
	case item.OldestAgeDays >= 60:
		score = 52
	case item.OpenCharges > 0:
		score = 35
	}
	if item.TimelineAnomalies > 0 {
		score = clampScore(float64(score + minInt(item.TimelineAnomalies*8, 18)))
	}
	if item.MissingDocumentCharges > 0 {
		score = clampScore(float64(score + minInt(item.MissingDocumentCharges*5, 10)))
	}
	switch {
	case item.OldestAgeDays > 0 && item.TimelineAnomalies > 0:
		return score, fmt.Sprintf("riesgo legal/antigüedad por %d días abiertos y %d anomalía(s)", item.OldestAgeDays, item.TimelineAnomalies)
	case item.OldestAgeDays > 0:
		return score, fmt.Sprintf("riesgo legal/antigüedad por %d días del cargo abierto más antiguo", item.OldestAgeDays)
	case item.TimelineAnomalies > 0:
		return score, fmt.Sprintf("riesgo legal/antigüedad por %d anomalía(s) temporal(es)", item.TimelineAnomalies)
	default:
		return score, "riesgo legal/antigüedad sin señales críticas visibles"
	}
}

func weightedPriorityScore(breakdown []priorityCriterion) int {
	score := 0
	for _, item := range breakdown {
		score += item.Contribution
	}
	if score > 100 {
		return 100
	}
	return score
}

func weightedPriorityContribution(score, weight int) int {
	return int(math.Round(float64(score) * float64(weight) / 100.0))
}

func dominantPriorityCriterion(breakdown []priorityCriterion) string {
	ranked := rankedPriorityCriteria(breakdown)
	if len(ranked) == 0 {
		return ""
	}
	return ranked[0].Key
}

func rankedPriorityCriteria(breakdown []priorityCriterion) []priorityCriterion {
	ranked := append([]priorityCriterion(nil), breakdown...)
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Contribution != ranked[j].Contribution {
			return ranked[i].Contribution > ranked[j].Contribution
		}
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].Key < ranked[j].Key
	})
	return ranked
}

func priorityCriterionTitle(key string) string {
	switch key {
	case "materialidad":
		return "Materialidad"
	case "comportamiento":
		return "Comportamiento"
	case "riesgo_legal":
		return "Riesgo legal/antigüedad"
	default:
		return fallbackLabel(key, "Criterio")
	}
}

func suggestedActionForPriority(item priorityItem) string {
	switch {
	case item.MissingResponsibleProjects > 0 || item.MissingDocumentCharges > 0:
		return "Revisar datos y asignación hoy"
	case item.TimelineAnomalies > 0:
		return "Validar conciliación hoy"
	case item.DominantCriterion == "materialidad" && item.ActiveCollectableProjects > 0:
		return "Gestionar hoy con foco ejecutivo"
	case item.DominantCriterion == "comportamiento" && item.PartialCharges > 0:
		return "Revisar pago parcial y proponer siguiente paso"
	case item.DominantCriterion == "riesgo_legal":
		return "Escalar revisión del pendiente hoy"
	case item.OpenCharges > 0 && item.ActiveCollectableProjects > 0:
		return "Gestionar hoy"
	case item.OpenCharges > 0:
		return "Revisar elegibilidad de cobro"
	default:
		return "Monitorear"
	}
}

func classifyPriority(score int) string {
	switch {
	case score >= 70:
		return "Alta"
	case score >= 45:
		return "Media"
	default:
		return "Baja"
	}
}

func recoveryRiskScore(item priorityItem) int {
	risk := item.OpenCharges*12 + item.PartialCharges*6 + item.OldestAgeDays/30
	if item.ObservedPaidRatio < 0.5 {
		risk += 10
	}
	if item.DaysSinceLastPayment > 180 {
		risk += 10
	}
	if item.ActiveCollectableProjects == 0 {
		risk += 6
	}
	return risk
}

func topKey(counts map[string]int, fallback string) string {
	best := fallback
	bestCount := -1
	for key, count := range counts {
		if count > bestCount || (count == bestCount && key < best) {
			best = key
			bestCount = count
		}
	}
	return fallbackLabel(best, fallback)
}

func trimPriorityItems(items []priorityItem, limit int) []priorityItem {
	if limit <= 0 || len(items) == 0 {
		return nil
	}
	if len(items) < limit {
		limit = len(items)
	}
	out := make([]priorityItem, limit)
	copy(out, items[:limit])
	return out
}

func trimExecutiveLoad(items []executiveLoad, limit int) []executiveLoad {
	if limit <= 0 || len(items) == 0 {
		return nil
	}
	if len(items) < limit {
		limit = len(items)
	}
	out := make([]executiveLoad, limit)
	copy(out, items[:limit])
	return out
}

func trimDataQualityItems(items []dataQualityItem, limit int) []dataQualityItem {
	if limit <= 0 || len(items) == 0 {
		return nil
	}
	if len(items) < limit {
		limit = len(items)
	}
	out := make([]dataQualityItem, limit)
	copy(out, items[:limit])
	return out
}

func debtorOpportunitySummary(ctx derivedContext, item priorityItem) string {
	parts := []string{}
	if item.PartialCharges > 0 {
		parts = append(parts, fmt.Sprintf("%d pago(s) parcial(es) visibles", item.PartialCharges))
	}
	if item.DaysSinceLastPayment >= 0 && item.DaysSinceLastPayment <= 90 {
		parts = append(parts, fmt.Sprintf("pago reciente hace %d días", item.DaysSinceLastPayment))
	}
	if item.ActiveCollectableProjects > 0 {
		parts = append(parts, fmt.Sprintf("%d proyecto(s) cobrable(s)", item.ActiveCollectableProjects))
	}
	if item.TimelineAnomalies > 0 {
		parts = append(parts, "anomalías temporales que exigen validar antes")
	}
	if len(parts) == 0 {
		return "sin señales fuertes de recuperabilidad adicional más allá del pendiente visible"
	}
	return strings.Join(parts, " · ")
}

func paymentSignal(item priorityItem) string {
	switch {
	case item.PartialCharges > 0 && item.DaysSinceLastPayment >= 0:
		return fmt.Sprintf("hay pagos parciales y último pago visible hace %d días", item.DaysSinceLastPayment)
	case item.PartialCharges > 0:
		return "hay pagos parciales visibles"
	case item.DaysSinceLastPayment >= 0 && item.DaysSinceLastPayment <= 90:
		return fmt.Sprintf("último pago visible hace %d días", item.DaysSinceLastPayment)
	case item.DaysSinceLastPayment >= 0:
		return fmt.Sprintf("último pago visible hace %d días", item.DaysSinceLastPayment)
	default:
		return "no aparece un último pago reciente claro"
	}
}

func anomalySignal(item priorityItem) string {
	issues := []string{}
	if item.TimelineAnomalies > 0 {
		issues = append(issues, fmt.Sprintf("%d anomalía(s) temporal(es)", item.TimelineAnomalies))
	}
	if item.MissingResponsibleProjects > 0 {
		issues = append(issues, fmt.Sprintf("%d proyecto(s) sin responsable", item.MissingResponsibleProjects))
	}
	if item.MissingDocumentCharges > 0 {
		issues = append(issues, fmt.Sprintf("%d cargo(s) sin documento", item.MissingDocumentCharges))
	}
	if len(issues) == 0 {
		return "sin anomalías operativas críticas visibles"
	}
	return strings.Join(issues, " · ")
}

func classifyRecoverability(item priorityItem) string {
	switch {
	case item.ActiveCollectableProjects > 0 && item.MissingDocumentCharges == 0 && item.MissingResponsibleProjects == 0 && item.TimelineAnomalies == 0:
		return "accionable"
	case item.ActiveCollectableProjects > 0:
		return "cobrable con validaciones"
	default:
		return "más administrativo que comercial"
	}
}

func validationChecklist(ctx derivedContext, item priorityItem) string {
	checks := []string{}
	if len(ctx.PendingDetails) > 0 {
		checks = append(checks, fmt.Sprintf("cargo %s", ctx.PendingDetails[0].ChargeID))
		if strings.TrimSpace(ctx.PendingDetails[0].DocumentNumber) != "" {
			checks = append(checks, fmt.Sprintf("documento %s", ctx.PendingDetails[0].DocumentNumber))
		}
	}
	if item.TimelineAnomalies > 0 {
		checks = append(checks, "brecha temporal/conciliación")
	}
	if item.MissingResponsibleProjects > 0 {
		checks = append(checks, "responsable actual")
	}
	if len(checks) == 0 {
		return "confirmar vigencia del pendiente visible"
	}
	return strings.Join(checks, " · ")
}

func maxKnownOpenAmount(items []priorityItem) float64 {
	best := 0.0
	for _, item := range items {
		if item.KnownOpenAmount > best {
			best = item.KnownOpenAmount
		}
	}
	return best
}

func maxPendingCount(items []priorityItem) int {
	best := 0
	for _, item := range items {
		pending := item.OpenCharges + item.PartialCharges
		if pending > best {
			best = pending
		}
	}
	return best
}

func clonePrioritySchema(schema prioritySchema) prioritySchema {
	weights := map[string]int{}
	for key, value := range schema.Weights {
		weights[key] = value
	}
	return prioritySchema{
		SchemaID:    schema.SchemaID,
		Description: schema.Description,
		Weights:     weights,
	}
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
