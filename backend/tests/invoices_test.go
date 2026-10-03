package tests_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/invoices"
)

func TestInvoicesCRUDIsTenantScoped(t *testing.T) {
	server := newAuthTestServer(t)
	dataA := newAppointmentTestData(t, server, "invoice-a+")
	dataB := newAppointmentTestData(t, server, "invoice-b+")
	appointment := createInvoiceTestAppointment(t, server, dataA)

	createA := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices", dataA.token, invoiceRequest(dataA.customerID, appointment.ID))
	if createA.Code != http.StatusCreated {
		t.Fatalf("create invoice status = %d, want %d: %s", createA.Code, http.StatusCreated, createA.Body.String())
	}
	invoiceA := decodeInvoiceResponse(t, createA)
	if invoiceA.InvoiceNumber == "" || invoiceA.Status != invoices.StatusIssued || invoiceA.Subtotal.String() != "1300.00" || invoiceA.Tax.String() != "130.00" || invoiceA.Total.String() != "1430.00" {
		t.Fatalf("created invoice = %+v", invoiceA)
	}

	createB := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices", dataB.token, invoiceRequest(dataB.customerID, ""))
	if createB.Code != http.StatusCreated {
		t.Fatalf("organization B invoice create status = %d: %s", createB.Code, createB.Body.String())
	}
	invoiceB := decodeInvoiceResponse(t, createB)
	if invoiceA.InvoiceNumber != invoiceB.InvoiceNumber {
		t.Fatalf("invoice numbering should be organization-scoped: A=%q B=%q", invoiceA.InvoiceNumber, invoiceB.InvoiceNumber)
	}

	listA := requestInvoice(t, server, http.MethodGet, "/api/v1/invoices?organization_id="+dataB.organizationID, dataA.token, nil)
	var listResponse struct {
		Invoices []invoices.Invoice `json:"invoices"`
	}
	if listA.Code != http.StatusOK || json.Unmarshal(listA.Body.Bytes(), &listResponse) != nil {
		t.Fatalf("list invoices failed: %d %s", listA.Code, listA.Body.String())
	}
	if len(listResponse.Invoices) != 1 || listResponse.Invoices[0].ID != invoiceA.ID {
		t.Fatalf("organization A invoice list = %+v, want only its invoice", listResponse.Invoices)
	}
	listB := requestInvoice(t, server, http.MethodGet, "/api/v1/invoices", dataB.token, nil)
	var listBResponse struct {
		Invoices []invoices.Invoice `json:"invoices"`
	}
	if err := json.Unmarshal(listB.Body.Bytes(), &listBResponse); err != nil || len(listBResponse.Invoices) != 1 || listBResponse.Invoices[0].ID != invoiceB.ID {
		t.Fatalf("organization B invoice list = %+v, want only its invoice (decode error: %v)", listBResponse.Invoices, err)
	}
	getOther := requestInvoice(t, server, http.MethodGet, "/api/v1/invoices/"+invoiceA.ID, dataB.token, nil)
	if getOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization get = %d, want %d", getOther.Code, http.StatusNotFound)
	}
	updateOther := requestInvoice(t, server, http.MethodPut, "/api/v1/invoices/"+invoiceA.ID, dataB.token, invoiceRequest(dataA.customerID, appointment.ID))
	if updateOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization update = %d, want %d", updateOther.Code, http.StatusNotFound)
	}
	paymentOther := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices/"+invoiceA.ID+"/payments", dataB.token, validPaymentPayload("1.00"))
	if paymentOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization payment = %d, want %d", paymentOther.Code, http.StatusNotFound)
	}

	updateOwn := invoiceRequest(dataA.customerID, appointment.ID)
	updateOwn["notes"] = "Updated invoice note"
	updated := requestInvoice(t, server, http.MethodPut, "/api/v1/invoices/"+invoiceA.ID, dataA.token, updateOwn)
	if updated.Code != http.StatusOK || decodeInvoiceResponse(t, updated).Notes != "Updated invoice note" {
		t.Fatalf("update own invoice failed: %d %s", updated.Code, updated.Body.String())
	}
	detailsResponse := requestInvoice(t, server, http.MethodGet, "/api/v1/invoices/"+invoiceA.ID, dataA.token, nil)
	if detailsResponse.Code != http.StatusOK {
		t.Fatalf("get own invoice details status = %d: %s", detailsResponse.Code, detailsResponse.Body.String())
	}
	var details invoices.Details
	if err := json.Unmarshal(detailsResponse.Body.Bytes(), &details); err != nil {
		t.Fatalf("decode invoice details: %v", err)
	}
	if details.Customer.ID != dataA.customerID || len(details.Items) != 2 || details.Invoice.BalanceDue.String() != "1430.00" {
		t.Fatalf("invoice details = %+v", details)
	}

	cancelInput := invoiceRequest(dataA.customerID, "")
	cancelCreate := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices", dataA.token, cancelInput)
	if cancelCreate.Code != http.StatusCreated {
		t.Fatalf("create cancellation fixture = %d: %s", cancelCreate.Code, cancelCreate.Body.String())
	}
	cancelInvoice := decodeInvoiceResponse(t, cancelCreate)
	cancelOther := requestInvoice(t, server, http.MethodDelete, "/api/v1/invoices/"+cancelInvoice.ID, dataB.token, nil)
	if cancelOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization cancel = %d, want %d", cancelOther.Code, http.StatusNotFound)
	}
	cancelOwn := requestInvoice(t, server, http.MethodDelete, "/api/v1/invoices/"+cancelInvoice.ID, dataA.token, nil)
	if cancelOwn.Code != http.StatusNoContent {
		t.Fatalf("cancel own invoice = %d, want %d", cancelOwn.Code, http.StatusNoContent)
	}
	getCancelled := requestInvoice(t, server, http.MethodGet, "/api/v1/invoices/"+cancelInvoice.ID, dataA.token, nil)
	if getCancelled.Code != http.StatusOK || decodeInvoiceDetails(t, getCancelled).Invoice.Status != invoices.StatusCancelled {
		t.Fatalf("cancelled invoice was not retained: %s", getCancelled.Body.String())
	}
}

func TestInvoiceValidationAndRelatedTenantOwnership(t *testing.T) {
	server := newAuthTestServer(t)
	dataA := newAppointmentTestData(t, server, "invoice-validation-a+")
	dataB := newAppointmentTestData(t, server, "invoice-validation-b+")
	appointmentA := createInvoiceTestAppointment(t, server, dataA)
	valid := invoiceRequest(dataA.customerID, appointmentA.ID)
	invalidCases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing customer", func(input map[string]any) { delete(input, "customer_id") }},
		{"empty items", func(input map[string]any) { input["items"] = []any{} }},
		{"invalid quantity", func(input map[string]any) { input["items"].([]any)[0].(map[string]any)["quantity"] = 0 }},
		{"negative price", func(input map[string]any) { input["items"].([]any)[0].(map[string]any)["unit_price"] = "-2.00" }},
		{"invalid issue date", func(input map[string]any) { input["issue_date"] = "2030-1-01" }},
		{"due date before issue", func(input map[string]any) { input["due_date"] = "2029-12-31" }},
		{"negative tax", func(input map[string]any) { input["tax"] = "-1.00" }},
	}
	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			payload := cloneInvoicePayload(valid)
			testCase.change(payload)
			recorder := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices", dataA.token, payload)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}
	invalidID := requestInvoice(t, server, http.MethodGet, "/api/v1/invoices/not-a-uuid", dataA.token, nil)
	if invalidID.Code != http.StatusNotFound {
		t.Fatalf("invalid invoice ID status = %d, want %d", invalidID.Code, http.StatusNotFound)
	}

	otherCustomer := cloneInvoicePayload(valid)
	otherCustomer["customer_id"] = dataB.customerID
	if recorder := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices", dataA.token, otherCustomer); recorder.Code != http.StatusBadRequest {
		t.Fatalf("cross-organization customer status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	otherAppointment := cloneInvoicePayload(valid)
	appointmentB := createInvoiceTestAppointment(t, server, dataB)
	otherAppointment["appointment_id"] = appointmentB.ID
	if recorder := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices", dataA.token, otherAppointment); recorder.Code != http.StatusBadRequest {
		t.Fatalf("cross-organization appointment status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	spoofed := cloneInvoicePayload(valid)
	spoofed["organization_id"] = dataB.organizationID
	if recorder := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices", dataA.token, spoofed); recorder.Code != http.StatusBadRequest {
		t.Fatalf("organization_id spoof status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestInvoicePaymentsUpdateStatusAndRejectOverpayment(t *testing.T) {
	server := newAuthTestServer(t)
	data := newAppointmentTestData(t, server, "invoice-payment+")
	created := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices", data.token, invoiceRequest(data.customerID, ""))
	if created.Code != http.StatusCreated {
		t.Fatalf("create invoice = %d: %s", created.Code, created.Body.String())
	}
	item := decodeInvoiceResponse(t, created)

	invalidPayments := []struct {
		name string
		body map[string]any
	}{
		{"zero amount", validPaymentPayload("0")},
		{"negative amount", validPaymentPayload("-1.00")},
		{"invalid method", map[string]any{"amount": "1.00", "payment_date": "2030-01-10", "payment_method": "CHEQUE"}},
	}
	for _, testCase := range invalidPayments {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices/"+item.ID+"/payments", data.token, testCase.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}

	partial := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices/"+item.ID+"/payments", data.token, validPaymentPayload("500.00"))
	if partial.Code != http.StatusCreated {
		t.Fatalf("partial payment = %d: %s", partial.Code, partial.Body.String())
	}
	var partialResponse struct {
		Invoice invoices.Invoice `json:"invoice"`
	}
	if err := json.Unmarshal(partial.Body.Bytes(), &partialResponse); err != nil || partialResponse.Invoice.Status != invoices.StatusPartiallyPaid || partialResponse.Invoice.BalanceDue.String() != "930.00" {
		t.Fatalf("partial payment invoice = %+v (decode error: %v)", partialResponse.Invoice, err)
	}

	overpayment := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices/"+item.ID+"/payments", data.token, validPaymentPayload("930.01"))
	if overpayment.Code != http.StatusConflict {
		t.Fatalf("overpayment status = %d, want %d", overpayment.Code, http.StatusConflict)
	}
	full := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices/"+item.ID+"/payments", data.token, validPaymentPayload("930.00"))
	if full.Code != http.StatusCreated {
		t.Fatalf("final payment = %d: %s", full.Code, full.Body.String())
	}
	var fullResponse struct {
		Invoice invoices.Invoice `json:"invoice"`
	}
	if err := json.Unmarshal(full.Body.Bytes(), &fullResponse); err != nil || fullResponse.Invoice.Status != invoices.StatusPaid || fullResponse.Invoice.BalanceDue.String() != "0.00" {
		t.Fatalf("paid invoice = %+v (decode error: %v)", fullResponse.Invoice, err)
	}
}

func TestInvoiceEndpointsRequireAuthentication(t *testing.T) {
	server := newAuthTestServer(t)
	invoiceID := "00000000-0000-0000-0000-000000000001"
	testCases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/invoices"},
		{http.MethodGet, "/api/v1/invoices"},
		{http.MethodGet, "/api/v1/invoices/" + invoiceID},
		{http.MethodPut, "/api/v1/invoices/" + invoiceID},
		{http.MethodDelete, "/api/v1/invoices/" + invoiceID},
		{http.MethodPost, "/api/v1/invoices/" + invoiceID + "/payments"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			recorder := requestInvoice(t, server, testCase.method, testCase.path, "", nil)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
		})
	}
}

func createInvoiceTestAppointment(t *testing.T, server *authTestServer, data appointmentTestData) bookings.Appointment {
	t.Helper()
	recorder := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", data.token,
		appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "10:00", "11:00"))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create appointment fixture = %d: %s", recorder.Code, recorder.Body.String())
	}
	return decodeCreatedAppointment(t, recorder)
}

func invoiceRequest(customerID, appointmentID string) map[string]any {
	payload := map[string]any{
		"customer_id": customerID,
		"issue_date":  "2030-01-01",
		"due_date":    "2030-01-31",
		"items": []any{
			map[string]any{"description": "Service A", "quantity": 2, "unit_price": "500.00"},
			map[string]any{"description": "Service B", "quantity": 1, "unit_price": "300.00"},
		},
		"tax": "130.00", "notes": "Test invoice",
	}
	if appointmentID != "" {
		payload["appointment_id"] = appointmentID
	}
	return payload
}

func validPaymentPayload(amount string) map[string]any {
	return map[string]any{"amount": amount, "payment_date": "2030-01-10", "payment_method": "UPI", "reference": "test-ref"}
}

func requestInvoice(t *testing.T, server *authTestServer, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if payload == nil {
		body = strings.NewReader("")
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("encode invoice request: %v", err)
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

func decodeInvoiceResponse(t *testing.T, recorder *httptest.ResponseRecorder) invoices.Invoice {
	t.Helper()
	var body struct {
		Invoice invoices.Invoice `json:"invoice"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode invoice response: %v", err)
	}
	return body.Invoice
}

func decodeInvoiceDetails(t *testing.T, recorder *httptest.ResponseRecorder) invoices.Details {
	t.Helper()
	var details invoices.Details
	if err := json.Unmarshal(recorder.Body.Bytes(), &details); err != nil {
		t.Fatalf("decode invoice details: %v", err)
	}
	return details
}

func cloneInvoicePayload(payload map[string]any) map[string]any {
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	cloned["items"] = append([]any(nil), payload["items"].([]any)...)
	for index, item := range cloned["items"].([]any) {
		original := item.(map[string]any)
		itemCopy := make(map[string]any, len(original))
		for key, value := range original {
			itemCopy[key] = value
		}
		cloned["items"].([]any)[index] = itemCopy
	}
	return cloned
}
