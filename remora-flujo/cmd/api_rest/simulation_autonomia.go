package main

import (
	"encoding/json"
	"net/http"

	"remora-flujo/simulations/autonomia"
)

type autonomiaMessageRequest struct {
	Message string                 `json:"message"`
	State   autonomia.SessionState `json:"state"`
}

func (srv *server) handleAutonomiaBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	writeOK(w, autonomia.Bootstrap())
}

func (srv *server) handleAutonomiaMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	var req autonomiaMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body json inválido")
		return
	}
	writeOK(w, autonomia.HandleMessage(req.State, req.Message))
}
