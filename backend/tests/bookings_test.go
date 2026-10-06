package tests_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/customers"
	"github.com/serveflow/serveflow/backend/internal/services"
)

func TestBookingsCRUDIsTenantScoped(t *testing.T) {
	server := newAuthTestServer(t)
	dataA := newBookingTestData(t, server, "booking-a+")
	dataB := newBookingTestData(t, server, "booking-b+")

	createA := requestService(t, server, http.MethodPost, "/api/v1/bookings", dataA.token, bookingRequest(dataA, "10:00", "11:00"))
	if createA.Code != http.StatusCreated {
		t.Fatalf("create booking status = %d, want %d: %s", createA.Code, http.StatusCreated, createA.Body.String())
	}
	bookingA := decodeCreatedBooking(t, createA)
	if bookingA.Status != bookings.StatusBooked {
		t.Fatalf("new booking status = %q, want %q", bookingA.Status, bookings.StatusBooked)
	}
	createB := requestService(t, server, http.MethodPost, "/api/v1/bookings", dataB.token, bookingRequest(dataB, "10:00", "11:00"))
	if createB.Code != http.StatusCreated {
		t.Fatalf("organization B create status = %d: %s", createB.Code, createB.Body.String())
	}
	bookingB := decodeCreatedBooking(t, createB)

	listA := requestService(t, server, http.MethodGet, "/api/v1/bookings?organization_id="+dataB.organizationID, dataA.token, nil)
	if listA.Code != http.StatusOK {
		t.Fatalf("list bookings status = %d, want %d", listA.Code, http.StatusOK)
	}
	var listResponse struct {
		Bookings []bookings.Booking `json:"bookings"`
	}
	if err := json.Unmarshal(listA.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode booking list: %v", err)
	}
	if len(listResponse.Bookings) != 1 || listResponse.Bookings[0].ID != bookingA.ID {
		t.Fatalf("organization A list = %+v, want only its booking", listResponse.Bookings)
	}

	getOwn := requestService(t, server, http.MethodGet, "/api/v1/bookings/"+bookingA.ID, dataA.token, nil)
	if getOwn.Code != http.StatusOK {
		t.Fatalf("get own booking status = %d, want %d", getOwn.Code, http.StatusOK)
	}
	getOther := requestService(t, server, http.MethodGet, "/api/v1/bookings/"+bookingA.ID, dataB.token, nil)
	if getOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization get = %d, want %d", getOther.Code, http.StatusNotFound)
	}

	update := bookingRequest(dataA, "11:00", "12:00")
	update["notes"] = "Updated booking note"
	updateOwn := requestService(t, server, http.MethodPut, "/api/v1/bookings/"+bookingA.ID, dataA.token, update)
	if updateOwn.Code != http.StatusOK {
		t.Fatalf("update own booking status = %d: %s", updateOwn.Code, updateOwn.Body.String())
	}
	updatedBooking := decodeCreatedBooking(t, updateOwn)
	if updatedBooking.StartTime != "11:00" || updatedBooking.Notes != "Updated booking note" {
		t.Fatalf("updated booking = %+v", updatedBooking)
	}
	updateOther := requestService(t, server, http.MethodPut, "/api/v1/bookings/"+bookingA.ID, dataB.token, update)
	if updateOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization update = %d, want %d", updateOther.Code, http.StatusNotFound)
	}

	deleteOther := requestService(t, server, http.MethodDelete, "/api/v1/bookings/"+bookingA.ID, dataB.token, nil)
	if deleteOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization cancellation = %d, want %d", deleteOther.Code, http.StatusNotFound)
	}
	deleteOwn := requestService(t, server, http.MethodDelete, "/api/v1/bookings/"+bookingA.ID, dataA.token, nil)
	if deleteOwn.Code != http.StatusNoContent {
		t.Fatalf("cancel own booking = %d, want %d", deleteOwn.Code, http.StatusNoContent)
	}
	getCancelled := requestService(t, server, http.MethodGet, "/api/v1/bookings/"+bookingA.ID, dataA.token, nil)
	if getCancelled.Code != http.StatusOK || decodeCreatedBooking(t, getCancelled).Status != bookings.StatusCancelled {
		t.Fatalf("cancelled booking not retained with CANCELLED status: %s", getCancelled.Body.String())
	}
	getOwnB := requestService(t, server, http.MethodGet, "/api/v1/bookings/"+bookingB.ID, dataB.token, nil)
	if getOwnB.Code != http.StatusOK {
		t.Fatalf("organization B cannot access own booking: %d", getOwnB.Code)
	}
}

func TestBookingEndpointsRequireAuthentication(t *testing.T) {
	server := newAuthTestServer(t)
	payload := bookingRequest(bookingTestData{}, "10:00", "11:00")
	testCases := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "/api/v1/bookings", payload},
		{http.MethodGet, "/api/v1/bookings", nil},
		{http.MethodGet, "/api/v1/bookings/00000000-0000-0000-0000-000000000001", nil},
		{http.MethodPut, "/api/v1/bookings/00000000-0000-0000-0000-000000000001", payload},
		{http.MethodDelete, "/api/v1/bookings/00000000-0000-0000-0000-000000000001", nil},
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

func TestBookingValidationAndRelatedOrganization(t *testing.T) {
	server := newAuthTestServer(t)
	dataA := newBookingTestData(t, server, "booking-validation-a+")
	dataB := newBookingTestData(t, server, "booking-validation-b+")

	valid := bookingRequest(dataA, "10:00", "11:00")
	invalidCases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing customer", func(input map[string]any) { delete(input, "customer_id") }},
		{"invalid customer id", func(input map[string]any) { input["customer_id"] = "not-a-uuid" }},
		{"missing service", func(input map[string]any) { delete(input, "service_id") }},
		{"invalid service id", func(input map[string]any) { input["service_id"] = "not-a-uuid" }},
		{"invalid date", func(input map[string]any) { input["booking_date"] = "2030-2-01" }},
		{"invalid start time", func(input map[string]any) { input["start_time"] = "25:00" }},
		{"invalid end time", func(input map[string]any) { input["end_time"] = "not-time" }},
		{"end before start", func(input map[string]any) { input["end_time"] = "09:59" }},
		{"end equals start", func(input map[string]any) { input["end_time"] = "10:00" }},
		{"notes too long", func(input map[string]any) { input["notes"] = strings.Repeat("n", 2001) }},
	}
	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			input := cloneBookingPayload(valid)
			testCase.change(input)
			recorder := requestService(t, server, http.MethodPost, "/api/v1/bookings", dataA.token, input)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}

	otherCustomer := cloneBookingPayload(valid)
	otherCustomer["customer_id"] = dataB.customerID
	recorder := requestService(t, server, http.MethodPost, "/api/v1/bookings", dataA.token, otherCustomer)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("another organization's customer status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	otherService := cloneBookingPayload(valid)
	otherService["service_id"] = dataB.serviceID
	recorder = requestService(t, server, http.MethodPost, "/api/v1/bookings", dataA.token, otherService)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("another organization's service status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	spoofed := cloneBookingPayload(valid)
	spoofed["organization_id"] = dataB.organizationID
	recorder = requestService(t, server, http.MethodPost, "/api/v1/bookings", dataA.token, spoofed)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("client organization_id status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	list := requestService(t, server, http.MethodGet, "/api/v1/bookings", dataA.token, nil)
	var body struct {
		Bookings []bookings.Booking `json:"bookings"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode booking list: %v", err)
	}
	if len(body.Bookings) != 0 {
		t.Fatalf("invalid booking requests created rows: %+v", body.Bookings)
	}
}

func TestBookingDatabaseRejectsCrossOrganizationReferences(t *testing.T) {
	server := newAuthTestServer(t)
	dataA := newBookingTestData(t, server, "booking-fk-a+")
	dataB := newBookingTestData(t, server, "booking-fk-b+")
	insert := `
		INSERT INTO bookings (organization_id, customer_id, service_id, booking_date, start_time, end_time)
		VALUES ($1::uuid, $2::uuid, $3::uuid, DATE '2030-01-15', TIME '10:00', TIME '11:00')
	`
	for _, testCase := range []struct {
		name       string
		customerID string
		serviceID  string
	}{
		{name: "customer", customerID: dataB.customerID, serviceID: dataA.serviceID},
		{name: "service", customerID: dataA.customerID, serviceID: dataB.serviceID},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := server.pool.Exec(context.Background(), insert, dataA.organizationID, testCase.customerID, testCase.serviceID); err == nil {
				t.Fatalf("database accepted a booking with a cross-organization %s reference", testCase.name)
			}
		})
	}
}

func TestBookingConflictRules(t *testing.T) {
	server := newAuthTestServer(t)
	data := newBookingTestData(t, server, "booking-conflict+")

	first := requestService(t, server, http.MethodPost, "/api/v1/bookings", data.token, bookingRequest(data, "10:00", "11:00"))
	if first.Code != http.StatusCreated {
		t.Fatalf("create initial booking = %d: %s", first.Code, first.Body.String())
	}
	firstBooking := decodeCreatedBooking(t, first)

	overlap := requestService(t, server, http.MethodPost, "/api/v1/bookings", data.token, bookingRequest(data, "10:30", "11:30"))
	if overlap.Code != http.StatusConflict {
		t.Fatalf("overlap status = %d, want %d", overlap.Code, http.StatusConflict)
	}
	contained := requestService(t, server, http.MethodPost, "/api/v1/bookings", data.token, bookingRequest(data, "10:15", "10:45"))
	if contained.Code != http.StatusConflict {
		t.Fatalf("contained overlap status = %d, want %d", contained.Code, http.StatusConflict)
	}
	adjacent := requestService(t, server, http.MethodPost, "/api/v1/bookings", data.token, bookingRequest(data, "11:00", "12:00"))
	if adjacent.Code != http.StatusCreated {
		t.Fatalf("adjacent booking status = %d, want %d: %s", adjacent.Code, http.StatusCreated, adjacent.Body.String())
	}

	update := bookingRequest(data, "10:30", "11:30")
	update["status"] = bookings.StatusBooked
	updateResponse := requestService(t, server, http.MethodPut, "/api/v1/bookings/"+decodeCreatedBooking(t, adjacent).ID, data.token, update)
	if updateResponse.Code != http.StatusConflict {
		t.Fatalf("overlapping update status = %d, want %d", updateResponse.Code, http.StatusConflict)
	}

	if status := requestService(t, server, http.MethodDelete, "/api/v1/bookings/"+firstBooking.ID, data.token, nil).Code; status != http.StatusNoContent {
		t.Fatalf("cancel booking status = %d, want %d", status, http.StatusNoContent)
	}
	afterCancel := requestService(t, server, http.MethodPost, "/api/v1/bookings", data.token, bookingRequest(data, "10:00", "11:00"))
	if afterCancel.Code != http.StatusCreated {
		t.Fatalf("cancelled booking blocked a new booking: %d: %s", afterCancel.Code, afterCancel.Body.String())
	}
}

type bookingTestData struct {
	organizationID string
	customerID     string
	serviceID      string
	token          string
}

func newBookingTestData(t *testing.T, server *authTestServer, emailPrefix string) bookingTestData {
	t.Helper()
	email := emailPrefix + testEmail()
	t.Cleanup(func() { cleanupUser(t, server, email) })
	user, token := createServiceTestUser(t, server, email)

	customerInput := customers.Input{Name: "Booking Test Customer"}
	if err := customerInput.Validate(); err != nil {
		t.Fatalf("validate test customer: %v", err)
	}
	customer, err := server.customers.Create(context.Background(), user.OrganizationID, customerInput)
	if err != nil {
		t.Fatalf("create test customer: %v", err)
	}

	serviceInput := services.Input{Name: "Booking Test Service", DurationMinutes: 60, Price: services.Decimal("10.00")}
	price, err := serviceInput.Validate()
	if err != nil {
		t.Fatalf("validate test service: %v", err)
	}
	service, err := server.services.Create(context.Background(), user.OrganizationID, serviceInput, price)
	if err != nil {
		t.Fatalf("create test service: %v", err)
	}
	return bookingTestData{organizationID: user.OrganizationID, customerID: customer.ID, serviceID: service.ID, token: token}
}

func bookingRequest(data bookingTestData, startTime, endTime string) map[string]any {
	return map[string]any{
		"customer_id": data.customerID, "service_id": data.serviceID,
		"booking_date": "2030-01-15", "start_time": startTime, "end_time": endTime,
		"notes": "test booking",
	}
}

func cloneBookingPayload(payload map[string]any) map[string]any {
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}

func decodeCreatedBooking(t *testing.T, recorder *httptest.ResponseRecorder) bookings.Booking {
	t.Helper()
	var body struct {
		Booking bookings.Booking `json:"booking"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode booking response: %v", err)
	}
	return body.Booking
}
