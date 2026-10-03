package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRateLimitAndWindowRecovery(t *testing.T) {
	handler := RateLimit(2, 40*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for attempt := 0; attempt < 2; attempt++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("request %d status = %d", attempt+1, recorder.Code)
		}
	}
	limited := httptest.NewRecorder()
	handler.ServeHTTP(limited, httptest.NewRequest(http.MethodGet, "/", nil))
	if limited.Code != http.StatusTooManyRequests || limited.Header().Get("Retry-After") == "" {
		t.Fatalf("rate limit status/header = %d/%q", limited.Code, limited.Header().Get("Retry-After"))
	}
	time.Sleep(50 * time.Millisecond)
	recovered := httptest.NewRecorder()
	handler.ServeHTTP(recovered, httptest.NewRequest(http.MethodGet, "/", nil))
	if recovered.Code != http.StatusNoContent {
		t.Fatalf("request after the rate-limit window returned %d", recovered.Code)
	}
}

func TestRequestLoggingDoesNotLogQueryOrAuthorization(t *testing.T) {
	var output bytes.Buffer
	handler := RequestLogging(log.New(&output, "", 0))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Pattern = "GET /api/v1/me"
		w.WriteHeader(http.StatusUnauthorized)
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me?email=private@example.test", nil)
	request.Header.Set("Authorization", "Bearer private-token")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	logged := output.String()
	for _, secret := range []string{"private@example.test", "private-token", "Authorization"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("request log contains sensitive input %q: %s", secret, logged)
		}
	}
	for _, field := range []string{`"method":"GET"`, `"route":"GET /api/v1/me"`, `"status":401`, `"duration_ms":`} {
		if !strings.Contains(logged, field) {
			t.Fatalf("request log missing field %s: %s", field, logged)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" ||
		recorder.Header().Get("X-Frame-Options") != "DENY" ||
		recorder.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("security headers missing: %v", recorder.Header())
	}
}
