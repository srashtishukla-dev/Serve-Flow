package tests_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/customers"
	"github.com/serveflow/serveflow/backend/internal/technicians"
	"github.com/serveflow/serveflow/backend/internal/users"
)

// roleUserToken links a new login to an existing customer/technician record and returns its JWT.
func roleUserToken(t *testing.T, server *authTestServer, data appointmentTestData, role, recordID string) string {
	t.Helper()
	email := fmt.Sprintf("%s-%d@example.test", role, time.Now().UnixNano())
	t.Cleanup(func() { cleanupUser(t, server, email) })
	user, err := users.NewRepository(server.pool).CreateLinked(context.Background(), data.organizationID, role, recordID, "Role User", email, "not-a-real-hash")
	if err != nil {
		t.Fatalf("create linked %s user: %v", role, err)
	}
	token, err := server.tokens.Issue(user.ID, user.OrganizationID)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return token
}

func createAppointmentAs(t *testing.T, server *authTestServer, token string, payload map[string]any) bookings.Appointment {
	t.Helper()
	recorder := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", token, payload)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create appointment = %d: %s", recorder.Code, recorder.Body.String())
	}
	return decodeCreatedAppointment(t, recorder)
}

func setStatus(t *testing.T, server *authTestServer, token, id, status string) int {
	t.Helper()
	return requestAppointment(t, server, http.MethodPost, "/api/v1/appointments/"+id+"/status", token, map[string]any{"status": status}).Code
}

func TestCustomerSeesOnlyOwnAppointmentsAndCannotSpoofCustomer(t *testing.T) {
	server := newAuthTestServer(t)
	data := newAppointmentTestData(t, server, "role-cust+")
	other, err := server.customers.Create(context.Background(), data.organizationID, customers.Input{Name: "Other Customer"})
	if err != nil {
		t.Fatalf("create other customer: %v", err)
	}
	customerToken := roleUserToken(t, server, data, users.RoleCustomer, data.customerID)

	mine := createAppointmentAs(t, server, data.token, appointmentRequest(data, data.customerID, data.serviceID, "", "09:00", "10:00"))
	theirs := createAppointmentAs(t, server, data.token, appointmentRequest(data, other.ID, data.serviceID, "", "11:00", "12:00"))

	_, list, _ := listAppointments(t, server, customerToken, "")
	if len(list) != 1 || list[0].ID != mine.ID {
		t.Fatalf("customer list = %+v, want only own appointment", list)
	}
	if code := requestAppointment(t, server, http.MethodGet, "/api/v1/appointments/"+theirs.ID, customerToken, nil).Code; code != http.StatusNotFound {
		t.Fatalf("customer get other's appointment = %d, want 404", code)
	}
	if code := requestAppointment(t, server, http.MethodDelete, "/api/v1/appointments/"+theirs.ID, customerToken, nil).Code; code != http.StatusNotFound {
		t.Fatalf("customer cancel other's appointment = %d, want 404", code)
	}

	// A client-supplied customer_id is ignored: the appointment is bound to the caller.
	created := createAppointmentAs(t, server, customerToken, appointmentRequest(data, other.ID, data.serviceID, "", "13:00", "14:00"))
	if created.CustomerID != data.customerID {
		t.Fatalf("customer_id = %s, want authenticated customer %s", created.CustomerID, data.customerID)
	}

	// Customers can reschedule their own appointment but cannot assign or change status.
	update := appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "15:00", "16:00")
	recorder := requestAppointment(t, server, http.MethodPut, "/api/v1/appointments/"+created.ID, customerToken, update)
	if recorder.Code != http.StatusOK {
		t.Fatalf("customer reschedule = %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := decodeCreatedAppointment(t, recorder); got.TechnicianID != "" || got.StartTime != "15:00" {
		t.Fatalf("customer reschedule result = %+v, technician must not be assignable by customer", got)
	}
	if code := setStatus(t, server, customerToken, created.ID, bookings.StatusCompleted); code != http.StatusForbidden {
		t.Fatalf("customer completing = %d, want 403", code)
	}
	if code := setStatus(t, server, customerToken, created.ID, bookings.StatusCancelled); code != http.StatusOK {
		t.Fatalf("customer cancelling = %d, want 200", code)
	}
	for _, path := range []string{"/api/v1/customers", "/api/v1/technicians", "/api/v1/bookings"} {
		if code := requestAppointment(t, server, http.MethodGet, path, customerToken, nil).Code; code != http.StatusForbidden {
			t.Fatalf("customer GET %s = %d, want 403", path, code)
		}
	}
	if code := requestInvoice(t, server, http.MethodGet, "/api/v1/invoices", customerToken, nil).Code; code != http.StatusOK {
		t.Fatalf("customer GET /api/v1/invoices = %d, want 200", code)
	}
	if code := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices", customerToken, invoiceRequest(data.customerID, "")).Code; code != http.StatusForbidden {
		t.Fatalf("customer POST /api/v1/invoices = %d, want 403", code)
	}
}

func TestTechnicianSeesOnlyAssignedAndFollowsWorkflow(t *testing.T) {
	server := newAuthTestServer(t)
	data := newAppointmentTestData(t, server, "role-tech+")
	otherTech, err := server.technicians.Create(context.Background(), data.organizationID, technicians.Input{Name: "Other Tech"})
	if err != nil {
		t.Fatalf("create technician: %v", err)
	}
	techToken := roleUserToken(t, server, data, users.RoleTechnician, data.technicianID)

	assigned := createAppointmentAs(t, server, data.token, appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "09:00", "10:00"))
	if assigned.Status != bookings.StatusAssigned {
		t.Fatalf("status with technician = %s, want ASSIGNED", assigned.Status)
	}
	unassigned := createAppointmentAs(t, server, data.token, appointmentRequest(data, data.customerID, data.serviceID, "", "11:00", "12:00"))
	others := createAppointmentAs(t, server, data.token, appointmentRequest(data, data.customerID, data.serviceID, otherTech.ID, "09:00", "10:00"))

	_, list, _ := listAppointments(t, server, techToken, "")
	if len(list) != 1 || list[0].ID != assigned.ID {
		t.Fatalf("technician list = %+v, want only assigned appointment", list)
	}
	for _, id := range []string{unassigned.ID, others.ID} {
		if code := requestAppointment(t, server, http.MethodGet, "/api/v1/appointments/"+id, techToken, nil).Code; code != http.StatusNotFound {
			t.Fatalf("technician get out-of-scope = %d, want 404", code)
		}
		if code := setStatus(t, server, techToken, id, bookings.StatusInProgress); code != http.StatusNotFound {
			t.Fatalf("technician status on out-of-scope = %d, want 404", code)
		}
	}
	if code := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", techToken, appointmentRequest(data, data.customerID, data.serviceID, "", "14:00", "15:00")).Code; code != http.StatusForbidden {
		t.Fatalf("technician create = %d, want 403", code)
	}
	if code := requestAppointment(t, server, http.MethodPut, "/api/v1/appointments/"+assigned.ID, techToken, appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "09:00", "10:00")).Code; code != http.StatusForbidden {
		t.Fatalf("technician update = %d, want 403", code)
	}
	if code := setStatus(t, server, techToken, assigned.ID, bookings.StatusNoShow); code != http.StatusForbidden {
		t.Fatalf("technician no-show = %d, want 403", code)
	}
	if code := setStatus(t, server, techToken, assigned.ID, bookings.StatusInProgress); code != http.StatusOK {
		t.Fatalf("technician start = %d, want 200", code)
	}
	if code := setStatus(t, server, techToken, assigned.ID, bookings.StatusCompleted); code != http.StatusOK {
		t.Fatalf("technician complete = %d, want 200", code)
	}
	if code := setStatus(t, server, techToken, assigned.ID, bookings.StatusInProgress); code != http.StatusConflict {
		t.Fatalf("technician reopen completed = %d, want 409", code)
	}
}

func TestAdminAssignmentTransitionsAndNoShow(t *testing.T) {
	server := newAuthTestServer(t)
	data := newAppointmentTestData(t, server, "role-admin+")

	appointment := createAppointmentAs(t, server, data.token, appointmentRequest(data, data.customerID, data.serviceID, "", "09:00", "10:00"))
	if appointment.Status != bookings.StatusBooked {
		t.Fatalf("status = %s, want BOOKED", appointment.Status)
	}
	if code := setStatus(t, server, data.token, appointment.ID, bookings.StatusAssigned); code != http.StatusConflict {
		t.Fatalf("ASSIGNED without technician = %d, want 409", code)
	}
	if code := setStatus(t, server, data.token, appointment.ID, "BOGUS"); code != http.StatusConflict && code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, want 4xx", code)
	}
	assign := appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "09:00", "10:00")
	recorder := requestAppointment(t, server, http.MethodPut, "/api/v1/appointments/"+appointment.ID, data.token, assign)
	if recorder.Code != http.StatusOK || decodeCreatedAppointment(t, recorder).Status != bookings.StatusAssigned {
		t.Fatalf("assignment = %d %s, want ASSIGNED", recorder.Code, recorder.Body.String())
	}
	// 2030 appointments have not started yet, so NO_SHOW is rejected.
	if code := setStatus(t, server, data.token, appointment.ID, bookings.StatusNoShow); code != http.StatusConflict {
		t.Fatalf("future no-show = %d, want 409", code)
	}

	past := map[string]any{
		"customer_id": data.customerID, "service_id": data.serviceID, "technician_id": "",
		"appointment_date": "2020-01-15", "start_time": "09:00", "end_time": "10:00",
	}
	pastAppointment := createAppointmentAs(t, server, data.token, past)
	if code := setStatus(t, server, data.token, pastAppointment.ID, bookings.StatusNoShow); code != http.StatusOK {
		t.Fatalf("past no-show = %d, want 200", code)
	}
	if code := setStatus(t, server, data.token, pastAppointment.ID, bookings.StatusInProgress); code != http.StatusConflict {
		t.Fatalf("no-show -> in progress = %d, want 409", code)
	}
	_, noShows, _ := listAppointments(t, server, data.token, "?status=NO_SHOW")
	if len(noShows) != 1 || noShows[0].IsOverdue {
		t.Fatalf("no-show filter = %+v, want one non-overdue appointment", noShows)
	}
}

func TestRoleUsersCannotCrossOrganizations(t *testing.T) {
	server := newAuthTestServer(t)
	dataA := newAppointmentTestData(t, server, "role-iso-a+")
	dataB := newAppointmentTestData(t, server, "role-iso-b+")
	customerA := roleUserToken(t, server, dataA, users.RoleCustomer, dataA.customerID)
	techA := roleUserToken(t, server, dataA, users.RoleTechnician, dataA.technicianID)
	appointmentB := createAppointmentAs(t, server, dataB.token, appointmentRequest(dataB, dataB.customerID, dataB.serviceID, dataB.technicianID, "09:00", "10:00"))

	for _, token := range []string{customerA, techA, dataA.token} {
		if code := requestAppointment(t, server, http.MethodGet, "/api/v1/appointments/"+appointmentB.ID, token, nil).Code; code != http.StatusNotFound {
			t.Fatalf("cross-org get = %d, want 404", code)
		}
		if code := setStatus(t, server, token, appointmentB.ID, bookings.StatusCancelled); code != http.StatusNotFound && code != http.StatusForbidden {
			t.Fatalf("cross-org status = %d, want 404/403", code)
		}
	}
	// A linked identity must not point at another organization's record.
	_, err := users.NewRepository(server.pool).CreateLinked(context.Background(), dataA.organizationID, users.RoleCustomer, dataB.customerID, "x", fmt.Sprintf("x-%d@example.test", time.Now().UnixNano()), "h")
	if err == nil {
		t.Fatal("cross-organization identity link was accepted")
	}
}

func TestAdminCreatesLinkedLogin(t *testing.T) {
	server := newAuthTestServer(t)
	data := newAppointmentTestData(t, server, "role-login+")
	email := fmt.Sprintf("login-%d@example.test", time.Now().UnixNano())
	t.Cleanup(func() { cleanupUser(t, server, email) })
	body := map[string]any{"name": "Cust Login", "email": email, "password": "correct horse battery"}

	recorder := requestAppointment(t, server, http.MethodPost, "/api/v1/customers/"+data.customerID+"/login", data.token, body)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create login = %d: %s", recorder.Code, recorder.Body.String())
	}
	var created struct {
		User map[string]any `json:"user"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil || created.User["role"] != "CUSTOMER" || created.User["customer_id"] != data.customerID {
		t.Fatalf("login response = %s", recorder.Body.String())
	}
	if _, leaked := created.User["password_hash"]; leaked {
		t.Fatal("password hash leaked")
	}
	if code := requestAppointment(t, server, http.MethodPost, "/api/v1/customers/"+data.customerID+"/login", data.token, map[string]any{"name": "Again", "email": "again-" + email, "password": "correct horse battery"}).Code; code != http.StatusConflict {
		t.Fatalf("duplicate login for record = %d, want 409", code)
	}
	if code := requestAppointment(t, server, http.MethodPost, "/api/v1/technicians/"+data.technicianID+"/login", data.token, map[string]any{"email": "bad", "password": "short"}).Code; code != http.StatusBadRequest {
		t.Fatalf("invalid login input = %d, want 400", code)
	}
	login := postJSON(t, server.handler, "/api/v1/auth/login", map[string]any{"email": email, "password": "correct horse battery"})
	if login.Code != http.StatusOK {
		t.Fatalf("linked user login = %d: %s", login.Code, login.Body.String())
	}
	customerToken := roleUserToken(t, server, data, users.RoleTechnician, data.technicianID)
	if code := requestAppointment(t, server, http.MethodPost, "/api/v1/customers/"+data.customerID+"/login", customerToken, body).Code; code != http.StatusForbidden {
		t.Fatalf("non-admin creating login = %d, want 403", code)
	}
}

func TestAppointmentPaginationAndSearch(t *testing.T) {
	server := newAuthTestServer(t)
	data := newAppointmentTestData(t, server, "role-page+")
	for i := 0; i < 3; i++ {
		createAppointmentAs(t, server, data.token, appointmentRequest(data, data.customerID, data.serviceID, "", fmt.Sprintf("%02d:00", 8+i), fmt.Sprintf("%02d:00", 9+i)))
	}
	_, items, page := listAppointments(t, server, data.token, "?limit=2&page=1")
	if len(items) != 2 || page.Total != 3 {
		t.Fatalf("page 1 = %d items total %d, want 2 of 3", len(items), page.Total)
	}
	_, items, _ = listAppointments(t, server, data.token, "?search=Booking+Test+Customer")
	if len(items) != 3 || items[0].CustomerName != "Booking Test Customer" || items[0].ServiceName == "" {
		t.Fatalf("search by customer name = %+v", items)
	}
	_, items, _ = listAppointments(t, server, data.token, "?search=no-such-thing")
	if len(items) != 0 {
		t.Fatalf("search returned %d unexpected items", len(items))
	}
}
