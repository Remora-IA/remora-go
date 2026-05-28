package main

import (
	"strconv"
	"strings"
)

// tasksLedgerItem es la representación parcial de una tarea pendiente de Foco.
type tasksLedgerItem struct {
	ID         string `json:"id"`
	EntityType string `json:"entity_type"`
	EntityRef  string `json:"entity_ref"`
	Action     string `json:"action"`
	Title      string `json:"title"`
	Priority   int    `json:"priority"`
	Status     string `json:"status"`
	Notes      string `json:"notes"`
}

type tasksLedgerResp struct {
	Tasks []tasksLedgerItem `json:"tasks"`
	Count int               `json:"count"`
}

// queryTaskLedger lista las tareas pendientes desde el estado de Foco.
func queryTaskLedger() ([]priorityItem, error) {
	plan, err := load()
	if err != nil {
		return nil, nil
	}
	tasks := []tasksLedgerItem{}
	for _, task := range plan.Tasks {
		if task.Status == "done" || task.Status == "completed" {
			continue
		}
		meta := parseTaskNotesMap(task.Evidence + " " + task.Why)
		entityRef := firstNonEmptyStr(meta["entity_ref"], task.ID)
		priority := task.Importance
		if priority == 0 {
			if n, err := strconv.Atoi(strings.TrimSpace(task.Priority)); err == nil {
				priority = n
			}
		}
		if priority == 0 {
			priority = len(tasks) + 1
		}
		tasks = append(tasks, tasksLedgerItem{
			ID:         task.ID,
			EntityType: meta["entity_type"],
			EntityRef:  entityRef,
			Action:     firstNonEmptyStr(meta["action"], task.Expected),
			Title:      task.Title,
			Priority:   priority,
			Status:     task.Status,
			Notes:      task.Evidence + " " + task.Why,
		})
	}

	items := make([]priorityItem, 0, len(tasks))
	for _, t := range tasks {
		saldo, mora := parseTaskNotes(t.Notes)
		razon, accion := classifyDebtor(saldo, mora)
		score := computeScore(saldo, mora)
		deudor := t.Title
		if deudor == "" {
			deudor = t.EntityRef
		}
		items = append(items, priorityItem{
			TaskID:         t.ID,
			Rank:           t.Priority,
			Deudor:         deudor,
			DeudorID:       t.EntityRef,
			SaldoTotal:     saldo,
			DiasMoraMax:    mora,
			FacturasCount:  0,
			Score:          score,
			Razon:          razon,
			AccionSugerida: accion,
		})
	}
	return items, nil
}

func parseTaskNotesMap(notes string) map[string]string {
	out := map[string]string{}
	for _, kv := range strings.Fields(notes) {
		key, value, ok := strings.Cut(kv, "=")
		if ok {
			out[key] = strings.ReplaceAll(value, "_", " ")
		}
	}
	return out
}

// parseTaskNotes extrae saldo + mora de notes con formato
// "saldo=2500000 mora=45".
// Devuelve ceros si no encuentra los campos.
func parseTaskNotes(notes string) (saldo float64, mora int) {
	for _, kv := range strings.Fields(notes) {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		k, v := kv[:eq], kv[eq+1:]
		switch k {
		case "saldo":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				saldo = f
			}
		case "mora":
			if n, err := strconv.Atoi(v); err == nil {
				mora = n
			}
		}
	}
	return
}
