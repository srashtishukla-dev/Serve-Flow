package tests_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/serveflow/serveflow/backend/internal/customers"
)

func TestCustomersCRUDIsTenantScoped(t *testing.T) {
	server := newAuthTestServer(t)
	emailA := "customer-a+" + testEmail()
	emailB := "customer-b+" + testEmail()
	t.Cleanup(func() {
		cleanupUser(t, server, emailA)
		cleanupUser(t, server, emailB)
	})
	userA, tokenA := createServiceTestUser(t, server, emailA)
	userB, tokenB := createServiceTestUser(t, server, emailB)

	createA := requestCustomer(t, server, http.MethodPost, "/api/v1/customers", tokenA, map[string]any{
		"name": "Rahul Sharma", "email": "customer@example.test", "phone": "+91 555 123 4567", "notes": "Evening appointments",
	})
	if createA.Code != http.StatusCreated {
		t.Fatalf("create customer status = %d, want %d: %s", createA.Code, http.StatusCreated, createA.Body.String())
	}
	customerA := decodeCreatedCustomer(t, createA)
	if customerA.Name != "Rahul Sharma" || customerA.Email != "customer@example.test" || customerA.Phone == "" {
		t.Fatalf("created customer = %+v", customerA)
	}

	createB := requestCustomer(t, server, http.MethodPost, "/api/v1/customers", tokenB, map[string]any{
		"name": "Another Rahul", "email": "customer@example.test",
	})
	if createB.Code != http.StatusCreated {
		t.Fatalf("same email in another organization was rejected: status %d: %s", createB.Code, createB.Body.String())
	}
	customerB := decodeCreatedCustomer(t, createB)

	listA := requestCustomer(t, server, http.MethodGet, "/api/v1/customers?organization_id="+userB.OrganizationID, tokenA, nil)
	if listA.Code != http.StatusOK {
		t.Fatalf("list customers status = %d, want %d", listA.Code, http.StatusOK)
	}
	var listResponse struct {
		Customers []customers.Customer `json:"customers"`
	}
	if err := json.Unmarshal(listA.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode customer list: %v", err)
	}
	if len(listResponse.Customers) != 1 || listResponse.Customers[0].ID != customerA.ID {
		t.Fatalf("organization A list = %+v, want only its customer", listResponse.Customers)
	}

	getOwn := requestCustomer(t, server, http.MethodGet, "/api/v1/customers/"+customerA.ID, tokenA, nil)
	if getOwn.Code != http.StatusOK {
		t.Fatalf("get own customer status = %d, want %d", getOwn.Code, http.StatusOK)
	}
	getOther := requestCustomer(t, server, http.MethodGet, "/api/v1/customers/"+customerA.ID, tokenB, nil)
	if getOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization get status = %d, want %d", getOther.Code, http.StatusNotFound)
	}

	updateInput := map[string]any{
		"name": "Rahul Sharma Updated", "email": "rahul.updated@example.test", "phone": "+91 555 000 1111", "notes": "Updated notes",
	}
	updateOwn := requestCustomer(t, server, http.MethodPut, "/api/v1/customers/"+customerA.ID, tokenA, updateInput)
	if updateOwn.Code != http.StatusOK {
		t.Fatalf("update own customer status = %d: %s", updateOwn.Code, updateOwn.Body.String())
	}
	updated := decodeCreatedCustomer(t, updateOwn)
	if updated.Name != "Rahul Sharma Updated" || updated.Email != "rahul.updated@example.test" {
		t.Fatalf("updated customer = %+v", updated)
	}
	updateOther := requestCustomer(t, server, http.MethodPut, "/api/v1/customers/"+customerA.ID, tokenB, updateInput)
	if updateOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization update status = %d, want %d", updateOther.Code, http.StatusNotFound)
	}

	deleteOther := requestCustomer(t, server, http.MethodDelete, "/api/v1/customers/"+customerA.ID, tokenB, nil)
	if deleteOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization delete status = %d, want %d", deleteOther.Code, http.StatusNotFound)
	}
	deleteOwn := requestCustomer(t, server, http.MethodDelete, "/api/v1/customers/"+customerA.ID, tokenA, nil)
	if deleteOwn.Code != http.StatusNoContent {
		t.Fatalf("delete own customer status = %d, want %d", deleteOwn.Code, http.StatusNoContent)
	}
	getDeleted := requestCustomer(t, server, http.MethodGet, "/api/v1/customers/"+customerA.ID, tokenA, nil)
	if getDeleted.Code != http.StatusNotFound {
		t.Fatalf("get deleted customer status = %d, want %d", getDeleted.Code, http.StatusNotFound)
	}
	getOwnB := requestCustomer(t, server, http.MethodGet, "/api/v1/customers/"+customerB.ID, tokenB, nil)
	if getOwnB.Code != http.StatusOK {
		t.Fatalf("organization B cannot get its own customer: status = %d", getOwnB.Code)
	}
	if userA.OrganizationID == userB.OrganizationID {
		t.Fatal("test accounts unexpectedly share an organization")
	}
}

func TestCustomerRequestsRequireAuthentication(t *testing.T) {
	server := newAuthTestServer(t)
	requestBody := map[string]any{"name": "Sample Customer"}
	testCases := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "/api/v1/customers", requestBody},
		{http.MethodGet, "/api/v1/customers", nil},
		{http.MethodGet, "/api/v1/customers/00000000-0000-0000-0000-000000000001", nil},
		{http.MethodPut, "/api/v1/customers/00000000-0000-0000-0000-000000000001", requestBody},
		{http.MethodDelete, "/api/v1/customers/00000000-0000-0000-0000-000000000001", nil},
	}
	for _, testCase := range testCases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			recorder := requestCustomer(t, server, testCase.method, testCase.path, "", testCase.body)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestCustomerValidationAndOrganizationOverride(t *testing.T) {
	server := newAuthTestServer(t)
	emailA := "validation-a+" + testEmail()
	emailB := "validation-b+" + testEmail()
	t.Cleanup(func() {
		cleanupUser(t, server, emailA)
		cleanupUser(t, server, emailB)
	})
	_, tokenA := createServiceTestUser(t, server, emailA)
	userB, _ := createServiceTestUser(t, server, emailB)

	valid := map[string]any{"name": "Sample Customer", "email": "", "phone": "", "notes": ""}
	invalidCases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"blank name", func(input map[string]any) { input["name"] = "   " }},
		{"missing name", func(input map[string]any) { delete(input, "name") }},
		{"invalid email", func(input map[string]any) { input["email"] = "not-an-email" }},
		{"long email", func(input map[string]any) { input["email"] = strings.Repeat("a", 250) + "@example.test" }},
		{"long phone", func(input map[string]any) { input["phone"] = strings.Repeat("1", 51) }},
		{"long notes", func(input map[string]any) { input["notes"] = strings.Repeat("n", 2001) }},
	}
	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			input := cloneCustomerPayload(valid)
			testCase.change(input)
			recorder := requestCustomer(t, server, http.MethodPost, "/api/v1/customers", tokenA, input)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}

	spoofed := cloneCustomerPayload(valid)
	spoofed["organization_id"] = userB.OrganizationID
	spoofedResponse := requestCustomer(t, server, http.MethodPost, "/api/v1/customers", tokenA, spoofed)
	if spoofedResponse.Code != http.StatusBadRequest {
		t.Fatalf("request with organization_id status = %d, want %d", spoofedResponse.Code, http.StatusBadRequest)
	}
	listA := requestCustomer(t, server, http.MethodGet, "/api/v1/customers", tokenA, nil)
	if listA.Code != http.StatusOK {
		t.Fatalf("list customers status = %d, want %d", listA.Code, http.StatusOK)
	}
	var listResponse struct {
		Customers []customers.Customer `json:"customers"`
	}
	if err := json.Unmarshal(listA.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode customer list: %v", err)
	}
	if len(listResponse.Customers) != 0 {
		t.Fatalf("spoofed request created a customer: %+v", listResponse.Customers)
	}
}

func requestCustomer(t *testing.T, server *authTestServer, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if payload == nil {
		body = strings.NewReader("")
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("encode customer request: %v", err)
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

func decodeCreatedCustomer(t *testing.T, recorder *httptest.ResponseRecorder) customers.Customer {
	t.Helper()
	var body struct {
		Customer customers.Customer `json:"customer"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode customer response: %v", err)
	}
	return body.Customer
}

func cloneCustomerPayload(payload map[string]any) map[string]any {
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}
