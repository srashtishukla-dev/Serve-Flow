package handlers

import (
	"errors"
	"net/http"

	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/jobs"
	"github.com/serveflow/serveflow/backend/internal/pagination"
	"github.com/serveflow/serveflow/backend/internal/response"
)

func CreateAppointment(repository *bookings.Repository, notificationWorker *jobs.Worker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		var input bookings.AppointmentInput
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		if err := input.Validate(); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_APPOINTMENT", err.Error())
			return
		}
		appointment, err := repository.CreateAppointment(r.Context(), organizationID, input)
		if handleAppointmentError(w, err) {
			return
		}
		enqueueNotification(notificationWorker, r, organizationID, "appointment.created", "Appointment scheduled", "A new appointment was scheduled.")
		response.JSON(w, http.StatusCreated, struct {
			Appointment bookings.Appointment `json:"appointment"`
		}{Appointment: appointment})
	}
}

func ListAppointments(repository *bookings.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		query, ok := parseListQuery(w, r, map[string]bool{"search": true, "status": true}, []string{"booking_date", "created_at", "status"}, bookings.StatusBooked, bookings.StatusCancelled, bookings.StatusCompleted)
		if !ok {
			return
		}
		if r.URL.Query().Get("order") == "" {
			query.Order = "asc"
		}
		items, page, err := repository.ListAppointmentsPage(r.Context(), organizationID, query)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load appointments")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Appointments []bookings.Appointment `json:"appointments"`
			Pagination   pagination.Metadata    `json:"pagination"`
		}{Appointments: items, Pagination: page})
	}
}

func GetAppointment(repository *bookings.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		appointmentID := r.PathValue("id")
		if !bookings.ValidID(appointmentID) {
			appointmentNotFound(w)
			return
		}
		appointment, err := repository.FindAppointmentByID(r.Context(), organizationID, appointmentID)
		if errors.Is(err, bookings.ErrNotFound) {
			appointmentNotFound(w)
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load appointment")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Appointment bookings.Appointment `json:"appointment"`
		}{Appointment: appointment})
	}
}

func UpdateAppointment(repository *bookings.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		appointmentID := r.PathValue("id")
		if !bookings.ValidID(appointmentID) {
			appointmentNotFound(w)
			return
		}
		var input bookings.AppointmentUpdateInput
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		if err := input.Validate(); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_APPOINTMENT", err.Error())
			return
		}
		appointment, err := repository.UpdateAppointment(r.Context(), organizationID, appointmentID, input)
		if handleAppointmentError(w, err) {
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Appointment bookings.Appointment `json:"appointment"`
		}{Appointment: appointment})
	}
}

func CancelAppointment(repository *bookings.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		appointmentID := r.PathValue("id")
		if !bookings.ValidID(appointmentID) {
			appointmentNotFound(w)
			return
		}
		if err := repository.CancelAppointment(r.Context(), organizationID, appointmentID); errors.Is(err, bookings.ErrNotFound) {
			appointmentNotFound(w)
			return
		} else if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to cancel appointment")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleAppointmentError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, bookings.ErrRelatedNotFound):
		response.Error(w, http.StatusBadRequest, "INVALID_RELATED_RECORD", "Customer, service, or technician was not found in this organization")
	case errors.Is(err, bookings.ErrConflict):
		response.Error(w, http.StatusConflict, "APPOINTMENT_CONFLICT", "The technician already has an appointment during this time")
	case errors.Is(err, bookings.ErrNotFound):
		appointmentNotFound(w)
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to save appointment")
	}
	return true
}

func appointmentNotFound(w http.ResponseWriter) {
	response.Error(w, http.StatusNotFound, "APPOINTMENT_NOT_FOUND", "Appointment not found")
}
