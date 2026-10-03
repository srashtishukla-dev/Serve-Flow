package tests_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/technicians"
)

func TestAppointmentsCRUDIsTenantScoped(t *testing.T) {
	server := newAuthTestServer(t)
	dataA := newAppointmentTestData(t, server, "appointment-a+")
	dataB := newAppointmentTestData(t, server, "appointment-b+")

	create := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", dataA.token, appointmentRequest(dataA, dataA.customerID, dataA.serviceID, dataA.technicianID, "10:00", "11:00"))
	if create.Code != http.StatusCreated {
		t.Fatalf("create appointment status = %d, want %d: %s", create.Code, http.StatusCreated, create.Body.String())
	}
	appointment := decodeCreatedAppointment(t, create)
	if appointment.Status != bookings.StatusBooked || appointment.AppointmentDate != "2030-01-15" || appointment.TechnicianID != dataA.technicianID {
		t.Fatalf("created appointment = %+v", appointment)
	}

	listA := requestAppointment(t, server, http.MethodGet, "/api/v1/appointments?organization_id="+dataB.organizationID, dataA.token, nil)
	var listResponse struct {
		Appointments []bookings.Appointment `json:"appointments"`
	}
	if listA.Code != http.StatusOK || json.Unmarshal(listA.Body.Bytes(), &listResponse) != nil {
		t.Fatalf("list appointments failed: %d %s", listA.Code, listA.Body.String())
	}
	if len(listResponse.Appointments) != 1 || listResponse.Appointments[0].ID != appointment.ID {
		t.Fatalf("organization A list = %+v, want only its appointment", listResponse.Appointments)
	}
	listB := requestAppointment(t, server, http.MethodGet, "/api/v1/appointments", dataB.token, nil)
	var listBResponse struct {
		Appointments []bookings.Appointment `json:"appointments"`
	}
	if err := json.Unmarshal(listB.Body.Bytes(), &listBResponse); err != nil || len(listBResponse.Appointments) != 0 {
		t.Fatalf("organization B list = %+v, want no appointments (decode error: %v)", listBResponse.Appointments, err)
	}

	getOwn := requestAppointment(t, server, http.MethodGet, "/api/v1/appointments/"+appointment.ID, dataA.token, nil)
	if getOwn.Code != http.StatusOK || decodeCreatedAppointment(t, getOwn).ID != appointment.ID {
		t.Fatalf("get own appointment failed: %d %s", getOwn.Code, getOwn.Body.String())
	}
	getOther := requestAppointment(t, server, http.MethodGet, "/api/v1/appointments/"+appointment.ID, dataB.token, nil)
	if getOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization get = %d, want %d", getOther.Code, http.StatusNotFound)
	}

	updateInput := appointmentRequest(dataA, dataA.customerID, dataA.serviceID, dataA.technicianID, "10:00", "11:00")
	updateInput["notes"] = "Updated without conflicting with itself"
	updateOwn := requestAppointment(t, server, http.MethodPut, "/api/v1/appointments/"+appointment.ID, dataA.token, updateInput)
	if updateOwn.Code != http.StatusOK || decodeCreatedAppointment(t, updateOwn).Notes != "Updated without conflicting with itself" {
		t.Fatalf("update own appointment failed: %d %s", updateOwn.Code, updateOwn.Body.String())
	}
	updateOther := requestAppointment(t, server, http.MethodPut, "/api/v1/appointments/"+appointment.ID, dataB.token, updateInput)
	if updateOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization update = %d, want %d", updateOther.Code, http.StatusNotFound)
	}
	cancelOther := requestAppointment(t, server, http.MethodDelete, "/api/v1/appointments/"+appointment.ID, dataB.token, nil)
	if cancelOther.Code != http.StatusNotFound {
		t.Fatalf("cross-organization cancel = %d, want %d", cancelOther.Code, http.StatusNotFound)
	}
	cancelOwn := requestAppointment(t, server, http.MethodDelete, "/api/v1/appointments/"+appointment.ID, dataA.token, nil)
	if cancelOwn.Code != http.StatusNoContent {
		t.Fatalf("cancel own appointment = %d, want %d", cancelOwn.Code, http.StatusNoContent)
	}
	getCancelled := requestAppointment(t, server, http.MethodGet, "/api/v1/appointments/"+appointment.ID, dataA.token, nil)
	if getCancelled.Code != http.StatusOK || decodeCreatedAppointment(t, getCancelled).Status != bookings.StatusCancelled {
		t.Fatalf("cancelled appointment history was not retained: %s", getCancelled.Body.String())
	}
}

func TestAppointmentValidationAndRelatedTenantOwnership(t *testing.T) {
	server := newAuthTestServer(t)
	dataA := newAppointmentTestData(t, server, "appointment-validation-a+")
	dataB := newAppointmentTestData(t, server, "appointment-validation-b+")
	valid := appointmentRequest(dataA, dataA.customerID, dataA.serviceID, dataA.technicianID, "10:00", "11:00")
	invalidCases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing customer", func(input map[string]any) { delete(input, "customer_id") }},
		{"invalid customer ID", func(input map[string]any) { input["customer_id"] = "invalid" }},
		{"missing service", func(input map[string]any) { delete(input, "service_id") }},
		{"invalid service ID", func(input map[string]any) { input["service_id"] = "invalid" }},
		{"invalid technician ID", func(input map[string]any) { input["technician_id"] = "invalid" }},
		{"invalid date", func(input map[string]any) { input["appointment_date"] = "2030-2-01" }},
		{"invalid start time", func(input map[string]any) { input["start_time"] = "25:00" }},
		{"invalid end time", func(input map[string]any) { input["end_time"] = "not-time" }},
		{"end before start", func(input map[string]any) { input["end_time"] = "09:59" }},
		{"end equals start", func(input map[string]any) { input["end_time"] = "10:00" }},
	}
	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			payload := cloneAppointmentPayload(valid)
			testCase.change(payload)
			recorder := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", dataA.token, payload)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}
	invalidID := requestAppointment(t, server, http.MethodGet, "/api/v1/appointments/not-a-uuid", dataA.token, nil)
	if invalidID.Code != http.StatusNotFound {
		t.Fatalf("invalid appointment ID status = %d, want %d", invalidID.Code, http.StatusNotFound)
	}
	created := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", dataA.token, valid)
	if created.Code != http.StatusCreated {
		t.Fatalf("create validation fixture status = %d: %s", created.Code, created.Body.String())
	}
	invalidStatus := cloneAppointmentPayload(valid)
	invalidStatus["status"] = "SCHEDULED"
	invalidStatusResponse := requestAppointment(t, server, http.MethodPut, "/api/v1/appointments/"+decodeCreatedAppointment(t, created).ID, dataA.token, invalidStatus)
	if invalidStatusResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid status response = %d, want %d", invalidStatusResponse.Code, http.StatusBadRequest)
	}

	for _, testCase := range []struct {
		name  string
		field string
		value string
	}{
		{"other organization's customer", "customer_id", dataB.customerID},
		{"other organization's service", "service_id", dataB.serviceID},
		{"other organization's technician", "technician_id", dataB.technicianID},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			payload := cloneAppointmentPayload(valid)
			payload[testCase.field] = testCase.value
			recorder := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", dataA.token, payload)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}
	spoofed := cloneAppointmentPayload(valid)
	spoofed["organization_id"] = dataB.organizationID
	if recorder := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", dataA.token, spoofed); recorder.Code != http.StatusBadRequest {
		t.Fatalf("organization_id spoof status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestAppointmentTechnicianConflictsAndBoundaries(t *testing.T) {
	server := newAuthTestServer(t)
	data := newAppointmentTestData(t, server, "appointment-conflict+")
	first := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", data.token, appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "10:00", "11:00"))
	if first.Code != http.StatusCreated {
		t.Fatalf("create initial appointment = %d: %s", first.Code, first.Body.String())
	}
	firstAppointment := decodeCreatedAppointment(t, first)
	unassignedInput := appointmentRequest(data, data.customerID, data.serviceID, "", "10:00", "11:00")
	delete(unassignedInput, "technician_id")
	unassigned := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", data.token, unassignedInput)
	if unassigned.Code != http.StatusCreated || decodeCreatedAppointment(t, unassigned).TechnicianID != "" {
		t.Fatalf("appointment without technician was not accepted: %d %s", unassigned.Code, unassigned.Body.String())
	}

	for _, interval := range [][2]string{{"10:30", "11:30"}, {"09:30", "10:30"}} {
		recorder := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", data.token, appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, interval[0], interval[1]))
		if recorder.Code != http.StatusConflict {
			t.Fatalf("overlap %s-%s status = %d, want %d", interval[0], interval[1], recorder.Code, http.StatusConflict)
		}
	}
	adjacent := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", data.token, appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "11:00", "12:00"))
	if adjacent.Code != http.StatusCreated {
		t.Fatalf("adjacent appointment status = %d, want %d: %s", adjacent.Code, http.StatusCreated, adjacent.Body.String())
	}

	otherTechnician, err := server.technicians.Create(context.Background(), data.organizationID, technicians.Input{Name: "Second Technician"})
	if err != nil {
		t.Fatalf("create second technician: %v", err)
	}
	otherTechAppointment := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", data.token, appointmentRequest(data, data.customerID, data.serviceID, otherTechnician.ID, "10:00", "11:00"))
	if otherTechAppointment.Code != http.StatusCreated {
		t.Fatalf("different technician should be allowed to overlap: %d %s", otherTechAppointment.Code, otherTechAppointment.Body.String())
	}

	updateSelf := appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "10:00", "11:00")
	updateSelf["notes"] = "No self-conflict"
	updated := requestAppointment(t, server, http.MethodPut, "/api/v1/appointments/"+firstAppointment.ID, data.token, updateSelf)
	if updated.Code != http.StatusOK {
		t.Fatalf("updating appointment against itself returned %d: %s", updated.Code, updated.Body.String())
	}

	updateConflict := appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "10:30", "11:30")
	updateConflict["status"] = bookings.StatusBooked
	conflictedUpdate := requestAppointment(t, server, http.MethodPut, "/api/v1/appointments/"+firstAppointment.ID, data.token, updateConflict)
	if conflictedUpdate.Code != http.StatusConflict {
		t.Fatalf("conflicting update status = %d, want %d", conflictedUpdate.Code, http.StatusConflict)
	}

	if recorder := requestAppointment(t, server, http.MethodDelete, "/api/v1/appointments/"+firstAppointment.ID, data.token, nil); recorder.Code != http.StatusNoContent {
		t.Fatalf("cancel appointment status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	afterCancel := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", data.token, appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "10:00", "11:00"))
	if afterCancel.Code != http.StatusCreated {
		t.Fatalf("cancelled appointment blocked a new appointment: %d %s", afterCancel.Code, afterCancel.Body.String())
	}
}

type appointmentTestData struct {
	bookingTestData
	technicianID string
}

func newAppointmentTestData(t *testing.T, server *authTestServer, emailPrefix string) appointmentTestData {
	t.Helper()
	data := newBookingTestData(t, server, emailPrefix)
	technician, err := server.technicians.Create(context.Background(), data.organizationID, technicians.Input{Name: "Appointment Technician"})
	if err != nil {
		t.Fatalf("create appointment technician: %v", err)
	}
	return appointmentTestData{bookingTestData: data, technicianID: technician.ID}
}

func appointmentRequest(data appointmentTestData, customerID, serviceID, technicianID, startTime, endTime string) map[string]any {
	return map[string]any{
		"customer_id": customerID, "service_id": serviceID, "technician_id": technicianID,
		"appointment_date": "2030-01-15", "start_time": startTime, "end_time": endTime, "notes": "test appointment",
	}
}

func requestAppointment(t *testing.T, server *authTestServer, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if payload == nil {
		body = strings.NewReader("")
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("encode appointment request: %v", err)
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

func decodeCreatedAppointment(t *testing.T, recorder *httptest.ResponseRecorder) bookings.Appointment {
	t.Helper()
	var body struct {
		Appointment bookings.Appointment `json:"appointment"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode appointment response: %v", err)
	}
	return body.Appointment
}

func cloneAppointmentPayload(payload map[string]any) map[string]any {
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}
