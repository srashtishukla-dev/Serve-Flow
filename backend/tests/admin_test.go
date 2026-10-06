package tests_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const adminSummaryPath = "/api/v1/admin/dashboard/summary"

func getAdminSummary(t *testing.T, server *authTestServer, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, adminSummaryPath, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	server.handler.ServeHTTP(recorder, request)
	return recorder
}

func TestAdminSummaryRequiresAuthentication(t *testing.T) {
	server := newAuthTestServer(t)
	if recorder := getAdminSummary(t, server, ""); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestAdminSummaryRejectsNonAdminRoles(t *testing.T) {
	for _, role := range []string{"CUSTOMER", "TECHNICIAN"} {
		t.Run(role, func(t *testing.T) {
			server := newAuthTestServer(t)
			data := newAppointmentTestData(t, server, "admin-role+")
			recordID := data.customerID
			if role == "TECHNICIAN" {
				recordID = data.technicianID
			}
			token := roleUserToken(t, server, data, role, recordID)
			recorder := getAdminSummary(t, server, token)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.Error.Code != "FORBIDDEN" {
				t.Fatalf("body = %s, want FORBIDDEN error", recorder.Body.String())
			}
		})
	}
}

func TestAdminSummaryIsOrganizationScoped(t *testing.T) {
	server := newAuthTestServer(t)
	dataA := newAppointmentTestData(t, server, "admin-summary-a+")
	_ = newAppointmentTestData(t, server, "admin-summary-b+")
	emptyEmail := "admin-summary-empty+" + testEmail()
	t.Cleanup(func() { cleanupUser(t, server, emptyEmail) })
	_, emptyToken := createServiceTestUser(t, server, emptyEmail)

	request := httptest.NewRequest(http.MethodGet, adminSummaryPath+"?organization_id="+dataA.organizationID, nil)
	request.Header.Set("Authorization", "Bearer "+emptyToken)
	recorder := httptest.NewRecorder()
	server.handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body struct {
		Summary map[string]json.Number `json:"summary"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	for _, key := range []string{"total_customers", "total_technicians", "total_services", "total_bookings", "upcoming_appointments", "completed_appointments", "cancelled_appointments", "pending_payments", "completed_payments"} {
		if value, ok := body.Summary[key]; !ok || value.String() != "0" {
			t.Fatalf("%s = %v, want 0 for an empty organization", key, value)
		}
	}

	adminA := getAdminSummary(t, server, dataA.token)
	if adminA.Code != http.StatusOK {
		t.Fatalf("organization A status = %d: %s", adminA.Code, adminA.Body.String())
	}
	var bodyA struct {
		Summary map[string]json.Number `json:"summary"`
	}
	if err := json.Unmarshal(adminA.Body.Bytes(), &bodyA); err != nil {
		t.Fatalf("decode organization A summary: %v", err)
	}
	if bodyA.Summary["total_customers"].String() != "1" {
		t.Fatalf("organization A total_customers = %s, want only its own customer", bodyA.Summary["total_customers"])
	}
}
