package tests_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/serveflow/serveflow/backend/internal/services"
	"github.com/serveflow/serveflow/backend/internal/users"
)

func TestServicesCRUDIsTenantScoped(t *testing.T) {
	server := newAuthTestServer(t)
	emailA := "tenant-a+" + testEmail()
	emailB := "tenant-b+" + testEmail()
	t.Cleanup(func() {
		cleanupUser(t, server, emailA)
		cleanupUser(t, server, emailB)
	})
	userA, tokenA := createServiceTestUser(t, server, emailA)
	userB, tokenB := createServiceTestUser(t, server, emailB)

	createdA := requestService(t, server, http.MethodPost, "/api/v1/services", tokenA, map[string]any{
		"name": "Website Development", "description": "Custom sites", "duration_minutes": 120, "price": json.Number("5000.00"),
	})
	if createdA.Code != http.StatusCreated {
		t.Fatalf("create service status = %d, want %d: %s", createdA.Code, http.StatusCreated, createdA.Body.String())
	}
	serviceA := decodeCreatedService(t, createdA)
	if serviceA.Name != "Website Development" || serviceA.Price.String() != "5000.00" {
		t.Fatalf("created service = %+v", serviceA)
	}

	createdB := requestService(t, server, http.MethodPost, "/api/v1/services", tokenB, map[string]any{
		"name": "API Development", "description": "", "duration_minutes": 60, "price": json.Number("250.50"),
	})
	if createdB.Code != http.StatusCreated {
		t.Fatalf("create organization B service status = %d: %s", createdB.Code, createdB.Body.String())
	}
	serviceB := decodeCreatedService(t, createdB)

	listA := requestService(t, server, http.MethodGet, "/api/v1/services?organization_id="+userB.OrganizationID, tokenA, nil)
	if listA.Code != http.StatusOK {
		t.Fatalf("list services status = %d, want %d", listA.Code, http.StatusOK)
	}
	var response struct {
		Services []services.Service `json:"services"`
	}
	if err := json.Unmarshal(listA.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode service list: %v", err)
	}
	if len(response.Services) != 1 || response.Services[0].ID != serviceA.ID {
		t.Fatalf("organization A list = %+v, want only its own service", response.Services)
	}

	getOwn := requestService(t, server, http.MethodGet, "/api/v1/services/"+serviceA.ID, tokenA, nil)
	if getOwn.Code != http.StatusOK {
		t.Fatalf("get own service status = %d, want %d", getOwn.Code, http.StatusOK)
	}
	getOther := requestService(t, server, http.MethodGet, "/api/v1/services/"+serviceA.ID, tokenB, nil)
	if getOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization get status = %d, want %d", getOther.Code, http.StatusNotFound)
	}

	updateInput := map[string]any{
		"name": "Advanced Website Development", "description": "Custom responsive site", "duration_minutes": 180, "price": json.Number("7500.25"),
	}
	updateOwn := requestService(t, server, http.MethodPut, "/api/v1/services/"+serviceA.ID, tokenA, updateInput)
	if updateOwn.Code != http.StatusOK {
		t.Fatalf("update own service status = %d: %s", updateOwn.Code, updateOwn.Body.String())
	}
	updated := decodeCreatedService(t, updateOwn)
	if updated.Name != "Advanced Website Development" || updated.Price.String() != "7500.25" {
		t.Fatalf("updated service = %+v", updated)
	}
	updateOther := requestService(t, server, http.MethodPut, "/api/v1/services/"+serviceA.ID, tokenB, updateInput)
	if updateOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization update status = %d, want %d", updateOther.Code, http.StatusNotFound)
	}

	deleteOther := requestService(t, server, http.MethodDelete, "/api/v1/services/"+serviceA.ID, tokenB, nil)
	if deleteOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization delete status = %d, want %d", deleteOther.Code, http.StatusNotFound)
	}
	deleteOwn := requestService(t, server, http.MethodDelete, "/api/v1/services/"+serviceA.ID, tokenA, nil)
	if deleteOwn.Code != http.StatusNoContent {
		t.Fatalf("delete own service status = %d, want %d", deleteOwn.Code, http.StatusNoContent)
	}
	if userA.OrganizationID == "" {
		t.Fatal("test user does not have an organization")
	}
	getDeleted := requestService(t, server, http.MethodGet, "/api/v1/services/"+serviceA.ID, tokenA, nil)
	if getDeleted.Code != http.StatusNotFound {
		t.Fatalf("get deleted service status = %d, want %d", getDeleted.Code, http.StatusNotFound)
	}
	getOwnB := requestService(t, server, http.MethodGet, "/api/v1/services/"+serviceB.ID, tokenB, nil)
	if getOwnB.Code != http.StatusOK {
		t.Fatalf("organization B cannot get its own service: status = %d", getOwnB.Code)
	}
}

func TestServicesRequireAuthentication(t *testing.T) {
	server := newAuthTestServer(t)
	payload := map[string]any{"name": "Support", "duration_minutes": 30, "price": json.Number("0")}
	testCases := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "/api/v1/services", payload},
		{http.MethodGet, "/api/v1/services", nil},
		{http.MethodGet, "/api/v1/services/00000000-0000-0000-0000-000000000001", nil},
		{http.MethodPut, "/api/v1/services/00000000-0000-0000-0000-000000000001", payload},
		{http.MethodDelete, "/api/v1/services/00000000-0000-0000-0000-000000000001", nil},
	}
	for _, testCase := range testCases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			recorder := requestService(t, server, testCase.method, testCase.path, "", testCase.body)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestServiceValidationAndOrganizationOverride(t *testing.T) {
	server := newAuthTestServer(t)
	emailA := "tenant-a+" + testEmail()
	emailB := "tenant-b+" + testEmail()
	t.Cleanup(func() {
		cleanupUser(t, server, emailA)
		cleanupUser(t, server, emailB)
	})
	_, tokenA := createServiceTestUser(t, server, emailA)
	userB, _ := createServiceTestUser(t, server, emailB)

	valid := map[string]any{"name": "Support", "description": "", "duration_minutes": 30, "price": json.Number("0")}
	invalidCases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"blank name", func(payload map[string]any) { payload["name"] = "   " }},
		{"missing name", func(payload map[string]any) { delete(payload, "name") }},
		{"zero duration", func(payload map[string]any) { payload["duration_minutes"] = 0 }},
		{"negative duration", func(payload map[string]any) { payload["duration_minutes"] = -10 }},
		{"negative price", func(payload map[string]any) { payload["price"] = json.Number("-0.01") }},
		{"price precision", func(payload map[string]any) { payload["price"] = json.Number("1.001") }},
		{"non-decimal price", func(payload map[string]any) { payload["price"] = "1/2" }},
	}
	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			payload := cloneServicePayload(valid)
			testCase.change(payload)
			recorder := requestService(t, server, http.MethodPost, "/api/v1/services", tokenA, payload)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}

	spoofed := cloneServicePayload(valid)
	spoofed["organization_id"] = userB.OrganizationID
	spoofedResponse := requestService(t, server, http.MethodPost, "/api/v1/services", tokenA, spoofed)
	if spoofedResponse.Code != http.StatusBadRequest {
		t.Fatalf("request with organization_id status = %d, want %d", spoofedResponse.Code, http.StatusBadRequest)
	}
	listA := requestService(t, server, http.MethodGet, "/api/v1/services", tokenA, nil)
	if listA.Code != http.StatusOK {
		t.Fatalf("list services status = %d, want %d", listA.Code, http.StatusOK)
	}
	var emptyList struct {
		Services []services.Service `json:"services"`
	}
	if err := json.Unmarshal(listA.Body.Bytes(), &emptyList); err != nil {
		t.Fatalf("decode service list: %v", err)
	}
	if len(emptyList.Services) != 0 {
		t.Fatalf("spoofed request created a service: %+v", emptyList.Services)
	}
}

func createServiceTestUser(t *testing.T, server *authTestServer, email string) (users.User, string) {
	t.Helper()
	registerTestUser(t, server, email)
	user, _, err := users.NewRepository(server.pool).FindByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("find service test user: %v", err)
	}
	token, err := server.tokens.Issue(user.ID, user.OrganizationID)
	if err != nil {
		t.Fatalf("issue service test token: %v", err)
	}
	return user, token
}

func requestService(t *testing.T, server *authTestServer, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if payload == nil {
		body = strings.NewReader("")
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("encode service request: %v", err)
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

func decodeCreatedService(t *testing.T, recorder *httptest.ResponseRecorder) services.Service {
	t.Helper()
	var body struct {
		Service services.Service `json:"service"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode service response: %v", err)
	}
	return body.Service
}

func cloneServicePayload(payload map[string]any) map[string]any {
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}
