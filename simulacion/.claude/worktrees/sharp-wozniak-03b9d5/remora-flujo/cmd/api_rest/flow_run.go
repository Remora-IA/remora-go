package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (s *server) runFlow(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	var req flowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Flow.BusinessID) != "" {
		if _, _, ok := s.requireMembershipContext(w, r, req.Flow.BusinessID, nil); !ok {
			return
		}
	}
	result := s.runFlowManifest(r.Context(), req, nil)
	status := http.StatusOK
	if result.Status == "invalid" {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, APIResponse{Success: status < 400, Data: result})
}

func (s *server) runFlowStream(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	var req flowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Flow.BusinessID) != "" {
		if _, _, ok := s.requireMembershipContext(w, r, req.Flow.BusinessID, nil); !ok {
			return
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sendSSE := func(event string, data interface{}) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		flusher.Flush()
	}

	result := s.runFlowManifest(r.Context(), req, func(event string, step flowRunStep, totalSteps int) {
		sendSSE(event, map[string]interface{}{
			"step":        step,
			"total_steps": totalSteps,
		})
	})

	sendSSE("flow_complete", result)
}
