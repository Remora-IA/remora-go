package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type notification struct {
	ID         string      `json:"id"`
	BusinessID string      `json:"business_id"`
	FlowID     string      `json:"flow_id,omitempty"`
	RunID      string      `json:"run_id,omitempty"`
	Type       string      `json:"type"`
	Title      string      `json:"title"`
	Summary    string      `json:"summary,omitempty"`
	Payload    interface{} `json:"payload,omitempty"`
	Status     string      `json:"status"`
	CreatedAt  string      `json:"created_at"`
	AckedAt    string      `json:"acked_at,omitempty"`
}

func notificationsDBPath(rootDir, businessID string) string {
	return filepath.Join(rootDir, "profiles", businessID, "notifications.db")
}

func openNotificationsDB(rootDir, businessID string) (*sql.DB, error) {
	dbPath := notificationsDBPath(rootDir, businessID)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS notifications (
		id          TEXT PRIMARY KEY,
		business_id TEXT NOT NULL,
		flow_id     TEXT DEFAULT '',
		run_id      TEXT DEFAULT '',
		type        TEXT NOT NULL DEFAULT 'flow.result',
		title       TEXT NOT NULL,
		summary     TEXT DEFAULT '',
		payload     TEXT DEFAULT '{}',
		status      TEXT NOT NULL DEFAULT 'pending',
		created_at  TEXT NOT NULL,
		acked_at    TEXT DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_notif_status ON notifications(status)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// GET /api/v1/businesses/{business_id}/notifications
func (s *server) handleNotificationsList(w http.ResponseWriter, r *http.Request) {
	businessID := muxVar(r, "business_id")
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}

	db, err := openNotificationsDB(s.rootDir, businessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer db.Close()

	rows, err := db.Query(
		`SELECT id, business_id, flow_id, run_id, type, title, summary, payload, status, created_at, acked_at
		 FROM notifications WHERE status = ? ORDER BY created_at DESC`, status)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var notifications []notification
	for rows.Next() {
		var n notification
		var payloadStr string
		if err := rows.Scan(&n.ID, &n.BusinessID, &n.FlowID, &n.RunID, &n.Type, &n.Title, &n.Summary, &payloadStr, &n.Status, &n.CreatedAt, &n.AckedAt); err != nil {
			continue
		}
		var payload interface{}
		if json.Unmarshal([]byte(payloadStr), &payload) == nil {
			n.Payload = payload
		}
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []notification{}
	}

	writeOK(w, map[string]interface{}{
		"notifications": notifications,
		"count":         len(notifications),
	})
}

type createNotificationReq struct {
	FlowID  string      `json:"flow_id,omitempty"`
	RunID   string      `json:"run_id,omitempty"`
	Type    string      `json:"type,omitempty"`
	Title   string      `json:"title"`
	Summary string      `json:"summary,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
}

// POST /api/v1/businesses/{business_id}/notifications
func (s *server) handleNotificationCreate(w http.ResponseWriter, r *http.Request) {
	businessID := muxVar(r, "business_id")
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}

	var req createNotificationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}
	if req.Title == "" {
		writeErr(w, http.StatusBadRequest, "title requerido")
		return
	}
	if req.Type == "" {
		req.Type = "flow.result"
	}

	db, err := openNotificationsDB(s.rootDir, businessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer db.Close()

	n := notification{
		ID:         fmt.Sprintf("notif_%d", time.Now().UnixNano()),
		BusinessID: businessID,
		FlowID:     req.FlowID,
		RunID:      req.RunID,
		Type:       req.Type,
		Title:      req.Title,
		Summary:    req.Summary,
		Payload:    req.Payload,
		Status:     "pending",
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	payloadBytes, _ := json.Marshal(req.Payload)

	_, err = db.Exec(
		`INSERT INTO notifications (id, business_id, flow_id, run_id, type, title, summary, payload, status, created_at, acked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '')`,
		n.ID, n.BusinessID, n.FlowID, n.RunID, n.Type, n.Title, n.Summary, string(payloadBytes), n.Status, n.CreatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeOK(w, n)
}

// POST /api/v1/businesses/{business_id}/notifications/{notification_id}/ack
func (s *server) handleNotificationAck(w http.ResponseWriter, r *http.Request) {
	businessID := muxVar(r, "business_id")
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}

	notifID := notificationIDFromPath(r.URL.Path)
	if notifID == "" {
		writeErr(w, http.StatusBadRequest, "notification_id requerido")
		return
	}

	db, err := openNotificationsDB(s.rootDir, businessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer db.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.Exec(
		`UPDATE notifications SET status = 'acknowledged', acked_at = ? WHERE id = ? AND status = 'pending'`,
		now, notifID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeErr(w, http.StatusNotFound, "notificación no encontrada o ya fue confirmada")
		return
	}

	writeOK(w, map[string]interface{}{
		"id":       notifID,
		"status":   "acknowledged",
		"acked_at": now,
	})
}

func notificationIDFromPath(p string) string {
	const ackSuffix = "/ack"
	if strings.HasSuffix(p, ackSuffix) {
		p = p[:len(p)-len(ackSuffix)]
	}
	return filepath.Base(p)
}

// CreateNotification crea una notificación programáticamente (para uso interno desde flow completion).
func (s *server) CreateNotification(businessID string, req createNotificationReq) error {
	if req.Title == "" || businessID == "" {
		return fmt.Errorf("businessID y title requeridos")
	}
	if req.Type == "" {
		req.Type = "flow.result"
	}

	db, err := openNotificationsDB(s.rootDir, businessID)
	if err != nil {
		return err
	}
	defer db.Close()

	payloadBytes, _ := json.Marshal(req.Payload)
	id := fmt.Sprintf("notif_%d", time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = db.Exec(
		`INSERT INTO notifications (id, business_id, flow_id, run_id, type, title, summary, payload, status, created_at, acked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, '')`,
		id, businessID, req.FlowID, req.RunID, req.Type, req.Title, req.Summary, string(payloadBytes), now)
	return err
}
