package tests_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/serveflow/serveflow/backend/internal/handler"
)

type testPinger struct {
	err error
}

func (pinger testPinger) Ping(context.Context) error {
	return pinger.err
}

func TestHealthEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	handler.Health(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}

	var body struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" || body.Service != "serveflow-api" {
		t.Fatalf("body = %+v, want status ok and service serveflow-api", body)
	}
}

func TestHealthWithDependenciesKeepsUnavailableCacheOptional(t *testing.T) {
	endpoint := handler.HealthWithDependencies(testPinger{}, testPinger{err: errors.New("redis unavailable")})
	recorder := httptest.NewRecorder()
	endpoint.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	var body struct {
		Status       string `json:"status"`
		Dependencies struct {
			Database string `json:"database"`
			Cache    string `json:"cache"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if recorder.Code != http.StatusOK || body.Status != "ok" || body.Dependencies.Database != "available" || body.Dependencies.Cache != "unavailable" {
		t.Fatalf("health response = %+v, status %d; want API ok with database available and cache unavailable", body, recorder.Code)
	}
}
