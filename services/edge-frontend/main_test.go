package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	appConfig = Config{
		Environment: "test",
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	healthHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var payload ResponsePayload
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.Status != "UP" {
		t.Errorf("expected status UP, got %v", payload.Status)
	}
	if payload.Service != "edge-frontend" {
		t.Errorf("expected service edge-frontend, got %v", payload.Service)
	}
}

