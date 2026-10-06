package tests_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/pagination"
)

func listAppointments(t *testing.T, server *authTestServer, token, query string) (int, []bookings.Appointment, pagination.Metadata) {
	t.Helper()
	recorder := requestAppointment(t, server, http.MethodGet, "/api/v1/appointments"+query, token, nil)
	var body struct {
		Appointments []bookings.Appointment `json:"appointments"`
		Pagination   pagination.Metadata    `json:"pagination"`
	}
	if recorder.Code == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode appointment list: %v", err)
		}
	}
	return recorder.Code, body.Appointments, body.Pagination
}

func TestAppointmentOverdueDetectionAndFilters(t *testing.T) {
	server := newAuthTestServer(t)
	data := newAppointmentTestData(t, server, "overdue-a+")
	other := newAppointmentTestData(t, server, "overdue-b+")

	create := func(date string, technicianID string) string {
		payload := appointmentRequest(data, data.customerID, data.serviceID, technicianID, "10:00", "11:00")
		payload["appointment_date"] = date
		response := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", data.token, payload)
		if response.Code != http.StatusCreated {
			t.Fatalf("create appointment fixture: %d %s", response.Code, response.Body.String())
		}
		return decodeCreatedAppointment(t, response).ID
	}
	exec := func(query string, args ...any) {
		if _, err := server.pool.Exec(context.Background(), query, args...); err != nil {
			t.Fatalf("update appointment fixture: %v", err)
		}
	}
	future := create("2031-03-01", data.technicianID)
	current := create("2031-03-02", data.technicianID)
	pastOpen := create("2031-03-03", data.technicianID)
	pastCompleted := create("2031-03-04", data.technicianID)
	pastCancelled := create("2031-03-05", data.technicianID)
	unassigned := create("2031-03-06", "")
	exec(`UPDATE bookings SET booking_date = CURRENT_DATE, start_time = '00:00', end_time = '23:59' WHERE id = $1::uuid`, current)
	exec(`UPDATE bookings SET booking_date = '2020-01-01' WHERE id = ANY($1::uuid[])`, []string{pastOpen, pastCompleted, pastCancelled})
	exec(`UPDATE bookings SET status = 'COMPLETED' WHERE id = $1::uuid`, pastCompleted)
	exec(`UPDATE bookings SET status = 'CANCELLED' WHERE id = $1::uuid`, pastCancelled)

	ids := func(query string) map[string]bool {
		code, items, _ := listAppointments(t, server, data.token, query)
		if code != http.StatusOK {
			t.Fatalf("list %q status = %d", query, code)
		}
		result := map[string]bool{}
		for _, item := range items {
			result[item.ID] = true
		}
		return result
	}
	assertSet := func(name string, got map[string]bool, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: got %d appointments, want %d", name, len(got), len(want))
		}
		for _, id := range want {
			if !got[id] {
				t.Fatalf("%s: missing appointment %s", name, id)
			}
		}
	}
	assertSet("overdue", ids("?view=overdue"), pastOpen)
	assertSet("upcoming", ids("?view=upcoming"), future, current, unassigned)
	assertSet("today", ids("?view=today"), current)
	assertSet("unassigned", ids("?assignment=unassigned"), unassigned)
	assertSet("assigned overdue", ids("?assignment=assigned&view=overdue"), pastOpen)
	if got := ids("?assignment=assigned"); len(got) != 5 {
		t.Fatalf("assigned count = %d, want 5", len(got))
	}

	flags := map[string]bool{future: false, current: false, pastOpen: true, pastCompleted: false, pastCancelled: false}
	for id, want := range flags {
		response := requestAppointment(t, server, http.MethodGet, "/api/v1/appointments/"+id, data.token, nil)
		if got := decodeCreatedAppointment(t, response).IsOverdue; got != want {
			t.Fatalf("appointment %s is_overdue = %v, want %v", id, got, want)
		}
	}

	if _, items, page := listAppointments(t, server, data.token, "?limit=2&page=2&sort=booking_date&order=asc"); len(items) != 2 || page.Total != 6 {
		t.Fatalf("pagination returned %d items, total %d", len(items), page.Total)
	}
	if code, _, _ := listAppointments(t, server, data.token, "?view=bogus"); code != http.StatusBadRequest {
		t.Fatalf("invalid view status = %d, want 400", code)
	}
	if code, _, _ := listAppointments(t, server, data.token, "?assignment=bogus"); code != http.StatusBadRequest {
		t.Fatalf("invalid assignment status = %d, want 400", code)
	}
	if _, items, _ := listAppointments(t, server, other.token, "?view=overdue"); len(items) != 0 {
		t.Fatalf("other organization saw %d overdue appointments", len(items))
	}
	if code, _, _ := listAppointments(t, server, "", ""); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", code)
	}
	response := requestAppointment(t, server, http.MethodGet, "/api/v1/appointments/"+pastOpen, other.token, nil)
	if response.Code != http.StatusNotFound {
		t.Fatalf("cross-organization detail status = %d, want 404", response.Code)
	}
}

func TestAppointmentStatusTransitions(t *testing.T) {
	server := newAuthTestServer(t)
	data := newAppointmentTestData(t, server, "transition+")
	create := func(date string) bookings.Appointment {
		payload := appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "10:00", "11:00")
		payload["appointment_date"] = date
		response := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", data.token, payload)
		if response.Code != http.StatusCreated {
			t.Fatalf("create fixture: %d %s", response.Code, response.Body.String())
		}
		return decodeCreatedAppointment(t, response)
	}
	cancelled := create("2031-04-01")
	if response := requestAppointment(t, server, http.MethodDelete, "/api/v1/appointments/"+cancelled.ID, data.token, nil); response.Code != http.StatusNoContent {
		t.Fatalf("cancel status = %d", response.Code)
	}
	payload := appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "10:00", "11:00")
	payload["appointment_date"] = "2031-04-01"
	payload["status"] = "COMPLETED"
	if response := requestAppointment(t, server, http.MethodPut, "/api/v1/appointments/"+cancelled.ID, data.token, payload); response.Code == http.StatusOK {
		t.Fatal("a cancelled appointment was completed")
	}
	completed := create("2031-04-02")
	payload["appointment_date"] = "2031-04-02"
	if response := requestAppointment(t, server, http.MethodPut, "/api/v1/appointments/"+completed.ID, data.token, payload); response.Code != http.StatusOK {
		t.Fatalf("complete status = %d", response.Code)
	}
	payload["status"] = "BOOKED"
	if response := requestAppointment(t, server, http.MethodPut, "/api/v1/appointments/"+completed.ID, data.token, payload); response.Code == http.StatusOK {
		t.Fatal("a completed appointment was reopened")
	}
}
