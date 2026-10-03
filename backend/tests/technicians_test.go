package tests_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/serveflow/serveflow/backend/internal/technicians"
)

func TestTechniciansCRUDIsTenantScoped(t *testing.T) {
	server := newAuthTestServer(t)
	emailA := "technician-a+" + testEmail()
	emailB := "technician-b+" + testEmail()
	t.Cleanup(func() {
		cleanupUser(t, server, emailA)
		cleanupUser(t, server, emailB)
	})
	userA, tokenA := createServiceTestUser(t, server, emailA)
	userB, tokenB := createServiceTestUser(t, server, emailB)

	createA := requestTechnician(t, server, http.MethodPost, "/api/v1/technicians", tokenA, map[string]any{
		"name": "Rahul Sharma", "email": "rahul@example.test", "phone": "9876543210",
	})
	if createA.Code != http.StatusCreated {
		t.Fatalf("create technician status = %d, want %d: %s", createA.Code, http.StatusCreated, createA.Body.String())
	}
	technicianA := decodeCreatedTechnician(t, createA)
	if technicianA.Name != "Rahul Sharma" || technicianA.Status != technicians.StatusActive || technicianA.OrganizationID != userA.OrganizationID {
		t.Fatalf("created technician = %+v", technicianA)
	}
	createB := requestTechnician(t, server, http.MethodPost, "/api/v1/technicians", tokenB, map[string]any{"name": "Amit Kumar"})
	if createB.Code != http.StatusCreated {
		t.Fatalf("organization B create status = %d: %s", createB.Code, createB.Body.String())
	}
	technicianB := decodeCreatedTechnician(t, createB)

	listA := requestTechnician(t, server, http.MethodGet, "/api/v1/technicians?organization_id="+userB.OrganizationID, tokenA, nil)
	if listA.Code != http.StatusOK {
		t.Fatalf("list technicians status = %d, want %d", listA.Code, http.StatusOK)
	}
	var listResponse struct {
		Technicians []technicians.Technician `json:"technicians"`
	}
	if err := json.Unmarshal(listA.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode technician list: %v", err)
	}
	if len(listResponse.Technicians) != 1 || listResponse.Technicians[0].ID != technicianA.ID {
		t.Fatalf("organization A list = %+v, want only its technician", listResponse.Technicians)
	}

	getOwn := requestTechnician(t, server, http.MethodGet, "/api/v1/technicians/"+technicianA.ID, tokenA, nil)
	if getOwn.Code != http.StatusOK {
		t.Fatalf("get own technician status = %d, want %d", getOwn.Code, http.StatusOK)
	}
	getOther := requestTechnician(t, server, http.MethodGet, "/api/v1/technicians/"+technicianB.ID, tokenA, nil)
	if getOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization get status = %d, want %d", getOther.Code, http.StatusNotFound)
	}

	updateInput := map[string]any{"name": "Rahul Updated", "email": "rahul.updated@example.test", "phone": "1234567890"}
	updateOwn := requestTechnician(t, server, http.MethodPut, "/api/v1/technicians/"+technicianA.ID, tokenA, updateInput)
	if updateOwn.Code != http.StatusOK {
		t.Fatalf("update own technician status = %d: %s", updateOwn.Code, updateOwn.Body.String())
	}
	updated := decodeCreatedTechnician(t, updateOwn)
	if updated.Name != "Rahul Updated" || updated.Email != "rahul.updated@example.test" {
		t.Fatalf("updated technician = %+v", updated)
	}
	updateOther := requestTechnician(t, server, http.MethodPut, "/api/v1/technicians/"+technicianB.ID, tokenA, updateInput)
	if updateOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization update status = %d, want %d", updateOther.Code, http.StatusNotFound)
	}
	deactivateOther := requestTechnician(t, server, http.MethodDelete, "/api/v1/technicians/"+technicianB.ID, tokenA, nil)
	if deactivateOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization deactivate status = %d, want %d", deactivateOther.Code, http.StatusNotFound)
	}
	deactivateOwn := requestTechnician(t, server, http.MethodDelete, "/api/v1/technicians/"+technicianA.ID, tokenA, nil)
	if deactivateOwn.Code != http.StatusNoContent {
		t.Fatalf("deactivate own technician status = %d, want %d", deactivateOwn.Code, http.StatusNoContent)
	}
	getInactive := requestTechnician(t, server, http.MethodGet, "/api/v1/technicians/"+technicianA.ID, tokenA, nil)
	if getInactive.Code != http.StatusOK || decodeCreatedTechnician(t, getInactive).Status != technicians.StatusInactive {
		t.Fatalf("deactivated technician was not retained as INACTIVE: %s", getInactive.Body.String())
	}
	updatedInactive := requestTechnician(t, server, http.MethodPut, "/api/v1/technicians/"+technicianA.ID, tokenA, updateInput)
	if updatedInactive.Code != http.StatusOK || decodeCreatedTechnician(t, updatedInactive).Status != technicians.StatusInactive {
		t.Fatalf("normal update reactivated technician: %s", updatedInactive.Body.String())
	}
	if userA.OrganizationID == userB.OrganizationID {
		t.Fatal("test accounts unexpectedly share an organization")
	}
}

func TestTechnicianRequestsRequireAuthentication(t *testing.T) {
	server := newAuthTestServer(t)
	payload := map[string]any{"name": "Sample Technician"}
	testCases := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "/api/v1/technicians", payload},
		{http.MethodGet, "/api/v1/technicians", nil},
		{http.MethodGet, "/api/v1/technicians/00000000-0000-0000-0000-000000000001", nil},
		{http.MethodPut, "/api/v1/technicians/00000000-0000-0000-0000-000000000001", payload},
		{http.MethodDelete, "/api/v1/technicians/00000000-0000-0000-0000-000000000001", nil},
	}
	for _, testCase := range testCases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			recorder := requestTechnician(t, server, testCase.method, testCase.path, "", testCase.body)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestTechnicianValidationAndOrganizationOverride(t *testing.T) {
	server := newAuthTestServer(t)
	emailA := "technician-validation-a+" + testEmail()
	emailB := "technician-validation-b+" + testEmail()
	t.Cleanup(func() {
		cleanupUser(t, server, emailA)
		cleanupUser(t, server, emailB)
	})
	_, tokenA := createServiceTestUser(t, server, emailA)
	userB, _ := createServiceTestUser(t, server, emailB)
	valid := map[string]any{"name": "Sample Technician", "email": "", "phone": ""}
	invalidCases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing name", func(input map[string]any) { delete(input, "name") }},
		{"blank name", func(input map[string]any) { input["name"] = "   " }},
		{"invalid email", func(input map[string]any) { input["email"] = "not-an-email" }},
	}
	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			payload := cloneTechnicianPayload(valid)
			testCase.change(payload)
			recorder := requestTechnician(t, server, http.MethodPost, "/api/v1/technicians", tokenA, payload)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}
	invalidID := requestTechnician(t, server, http.MethodGet, "/api/v1/technicians/not-a-uuid", tokenA, nil)
	if invalidID.Code != http.StatusNotFound {
		t.Fatalf("invalid ID status = %d, want %d", invalidID.Code, http.StatusNotFound)
	}

	spoofed := cloneTechnicianPayload(valid)
	spoofed["organization_id"] = userB.OrganizationID
	spoofedResponse := requestTechnician(t, server, http.MethodPost, "/api/v1/technicians", tokenA, spoofed)
	if spoofedResponse.Code != http.StatusBadRequest {
		t.Fatalf("request with organization_id status = %d, want %d", spoofedResponse.Code, http.StatusBadRequest)
	}
	listA := requestTechnician(t, server, http.MethodGet, "/api/v1/technicians", tokenA, nil)
	var listResponse struct {
		Technicians []technicians.Technician `json:"technicians"`
	}
	if err := json.Unmarshal(listA.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode technician list: %v", err)
	}
	if len(listResponse.Technicians) != 0 {
		t.Fatalf("spoofed request created a technician: %+v", listResponse.Technicians)
	}
}

func requestTechnician(t *testing.T, server *authTestServer, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if payload == nil {
		body = strings.NewReader("")
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("encode technician request: %v", err)
		}
		body = strings.NewReader(string(encoded))
	}
	request := httptest.NewRequest(method, path, body)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	server.handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeCreatedTechnician(t *testing.T, recorder *httptest.ResponseRecorder) technicians.Technician {
	t.Helper()
	var body struct {
		Technician technicians.Technician `json:"technician"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode technician response: %v", err)
	}
	return body.Technician
}

func cloneTechnicianPayload(payload map[string]any) map[string]any {
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}
