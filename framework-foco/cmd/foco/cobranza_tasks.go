package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// tasksLedgerItem es la representación parcial de una tarea pendiente leída
// del ledger (`frameworktareas list --status pending`). Mapea solo los
// campos que Foco necesita para reconstruir un priorityItem.
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

// queryTaskLedger invoca el binario `frameworktareas` para listar las tareas
// pendientes del perfil. Devuelve nil, nil si el binario no está accesible o
// no hay tareas pendientes (caller cae al fallback SQL sobre panalbit.db).
//
// Variables de entorno:
//   - REMORA_TAREAS_BIN: path al binario (default: ../framework-tareas/frameworktareas)
//   - REMORA_PROFILE:    perfil activo (default: cobranza-chile)
func queryTaskLedger() ([]priorityItem, error) {
	bin := os.Getenv("REMORA_TAREAS_BIN")
	if bin == "" {
		bin = "../framework-tareas/frameworktareas"
	}
	if _, err := os.Stat(bin); err != nil {
		return nil, nil
	}
	profile := os.Getenv("REMORA_PROFILE")
	if profile == "" {
		profile = "cobranza-chile"
	}

	cmd := exec.Command(bin, "list", "--profile", profile, "--status", "pending", "--limit", "5")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tareas list: %w", err)
	}
	var resp tasksLedgerResp
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parse tareas list: %w", err)
	}
	if resp.Count == 0 {
		return nil, nil
	}

	items := make([]priorityItem, 0, len(resp.Tasks))
	for _, t := range resp.Tasks {
		saldo, mora := parseTaskNotes(t.Notes)
		razon, accion := classifyDebtor(saldo, mora)
		score := computeScore(saldo, mora)
		deudor := t.Title
		if deudor == "" {
			deudor = t.EntityRef
		}
		items = append(items, priorityItem{
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

// parseTaskNotes extrae saldo + mora de notes con formato
// "saldo=2500000 mora=45" (producido por framework-tareas seed-from-foco).
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
