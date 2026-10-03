package routes

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/serveflow/serveflow/backend/internal/auth"
)

func TestAuthenticationRoutesAreRateLimitedPerClient(t *testing.T) {
	tokenManager, err := auth.NewTokenManager("serveflow-route-test-secret-with-more-than-32-bytes", time.Minute)
	if err != nil {
		t.Fatalf("create test token manager: %v", err)
	}
	handler := New(nil, tokenManager, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	for attempt := 1; attempt <= 10; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString("{"))
		request.RemoteAddr = "192.0.2.10:1234"
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d status = %d, want %d", attempt, recorder.Code, http.StatusBadRequest)
		}
	}
	limitedRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString("{"))
	limitedRequest.RemoteAddr = "192.0.2.10:1234"
	limitedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(limitedRecorder, limitedRequest)
	if limitedRecorder.Code != http.StatusTooManyRequests || limitedRecorder.Header().Get("Retry-After") == "" {
		t.Fatalf("limited request status/header = %d/%q", limitedRecorder.Code, limitedRecorder.Header().Get("Retry-After"))
	}

	otherClient := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString("{"))
	otherClient.RemoteAddr = "192.0.2.11:1234"
	otherRecorder := httptest.NewRecorder()
	handler.ServeHTTP(otherRecorder, otherClient)
	if otherRecorder.Code != http.StatusBadRequest {
		t.Fatalf("separate client status = %d, want %d", otherRecorder.Code, http.StatusBadRequest)
	}
}

func TestTechnicianRoutesRequireAuthentication(t *testing.T) {
	tokenManager, err := auth.NewTokenManager("serveflow-route-test-secret-with-more-than-32-bytes", time.Minute)
	if err != nil {
		t.Fatalf("create test token manager: %v", err)
	}
	handler := New(nil, tokenManager, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	testCases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/technicians"},
		{http.MethodGet, "/api/v1/technicians"},
		{http.MethodGet, "/api/v1/technicians/00000000-0000-0000-0000-000000000001"},
		{http.MethodPut, "/api/v1/technicians/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/api/v1/technicians/00000000-0000-0000-0000-000000000001"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			request := httptest.NewRequest(testCase.method, testCase.path, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestAppointmentRoutesRequireAuthentication(t *testing.T) {
	tokenManager, err := auth.NewTokenManager("serveflow-route-test-secret-with-more-than-32-bytes", time.Minute)
	if err != nil {
		t.Fatalf("create test token manager: %v", err)
	}
	handler := New(nil, tokenManager, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	testCases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/appointments"},
		{http.MethodGet, "/api/v1/appointments"},
		{http.MethodGet, "/api/v1/appointments/00000000-0000-0000-0000-000000000001"},
		{http.MethodPut, "/api/v1/appointments/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/api/v1/appointments/00000000-0000-0000-0000-000000000001"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			request := httptest.NewRequest(testCase.method, testCase.path, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestInvoiceAndPaymentRoutesRequireAuthentication(t *testing.T) {
	tokenManager, err := auth.NewTokenManager("serveflow-route-test-secret-with-more-than-32-bytes", time.Minute)
	if err != nil {
		t.Fatalf("create test token manager: %v", err)
	}
	handler := New(nil, tokenManager, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	testCases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/invoices"},
		{http.MethodGet, "/api/v1/invoices"},
		{http.MethodGet, "/api/v1/invoices/00000000-0000-0000-0000-000000000001"},
		{http.MethodPut, "/api/v1/invoices/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/api/v1/invoices/00000000-0000-0000-0000-000000000001"},
		{http.MethodPost, "/api/v1/invoices/00000000-0000-0000-0000-000000000001/payments"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			request := httptest.NewRequest(testCase.method, testCase.path, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestAnalyticsRouteRequiresAuthentication(t *testing.T) {
	tokenManager, err := auth.NewTokenManager("serveflow-route-test-secret-with-more-than-32-bytes", time.Minute)
	if err != nil {
		t.Fatalf("create test token manager: %v", err)
	}
	handler := New(nil, tokenManager, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/summary", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
