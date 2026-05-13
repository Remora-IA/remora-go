package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"remora-flujo/simulations/autonomia"
)

func TestAutonomiaBootstrapAndMessageHandlers(t *testing.T) {
	srv := &server{}
	r := mux.NewRouter()
	r.HandleFunc(apiBase+"/simulations/autonomia-controlada/bootstrap", srv.handleAutonomiaBootstrap).Methods(http.MethodGet)
	r.HandleFunc(apiBase+"/simulations/autonomia-controlada/message", srv.handleAutonomiaMessage).Methods(http.MethodPost)

	bootReq := httptest.NewRequest(http.MethodGet, apiBase+"/simulations/autonomia-controlada/bootstrap", nil)
	bootRec := httptest.NewRecorder()
	r.ServeHTTP(bootRec, bootReq)
	if bootRec.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body=%s", bootRec.Code, bootRec.Body.String())
	}

	var boot APIResponse
	if err := json.Unmarshal(bootRec.Body.Bytes(), &boot); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(boot.Data)
	if err != nil {
		t.Fatal(err)
	}
	var bootResp autonomia.Response
	if err := json.Unmarshal(data, &bootResp); err != nil {
		t.Fatal(err)
	}
	if bootResp.Mode != autonomia.ModeGeneral {
		t.Fatalf("bootstrap mode = %s", bootResp.Mode)
	}

	payload, err := json.Marshal(map[string]any{
		"message": "Abramos el análisis individual del foco principal",
		"state":   bootResp.NextSessionState,
	})
	if err != nil {
		t.Fatal(err)
	}
	msgReq := httptest.NewRequest(http.MethodPost, apiBase+"/simulations/autonomia-controlada/message", bytes.NewReader(payload))
	msgReq.Header.Set("Content-Type", "application/json")
	msgRec := httptest.NewRecorder()
	r.ServeHTTP(msgRec, msgReq)
	if msgRec.Code != http.StatusOK {
		t.Fatalf("message status = %d body=%s", msgRec.Code, msgRec.Body.String())
	}

	var msg APIResponse
	if err := json.Unmarshal(msgRec.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(msg.Data)
	if err != nil {
		t.Fatal(err)
	}
	var msgResp autonomia.Response
	if err := json.Unmarshal(data, &msgResp); err != nil {
		t.Fatal(err)
	}
	if msgResp.Mode != autonomia.ModeCase {
		t.Fatalf("message mode = %s", msgResp.Mode)
	}
	if msgResp.CaseContext == nil || msgResp.CaseContext.Label == "" {
		t.Fatalf("expected case context, got %+v", msgResp.CaseContext)
	}
}
