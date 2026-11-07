package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoreHealthHandler(t *testing.T) {
	app := &App{
		Config: Config{
			SQSQueueURL: "https://sqs.mock/queue",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	app.healthHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status: got %v want %v", status, http.StatusOK)
	}

	var payload ResponsePayload
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}

	if payload.Status != "UP" {
		t.Errorf("expected UP, got %v", payload.Status)
	}
}
