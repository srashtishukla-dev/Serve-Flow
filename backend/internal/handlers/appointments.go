package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/jobs"
	"github.com/serveflow/serveflow/backend/internal/pagination"
	"github.com/serveflow/serveflow/backend/internal/response"
	"github.com/serveflow/serveflow/backend/internal/users"
)

const (
	appointmentIdentityMissing = "Your account is not linked to a customer or technician profile"
)

// appointmentScope derives the data scope from the authenticated user, never from the request.
func appointmentScope(w http.ResponseWriter, r *http.Request) (users.User, bookings.Scope, bool) {
	user, ok := users.FromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required")
		return users.User{}, bookings.Scope{}, false
	}
	switch user.Role {
	case users.RoleAdmin:
		return user, bookings.Scope{}, true
	case users.RoleCustomer:
		if user.CustomerID != "" {
			return user, bookings.Scope{CustomerID: user.CustomerID}, true
		}
	case users.RoleTechnician:
		if user.TechnicianID != "" {
			return user, bookings.Scope{TechnicianID: user.TechnicianID}, true
		}
	}
	response.Error(w, http.StatusForbidden, "FORBIDDEN", appointmentIdentityMissing)
	return users.User{}, bookings.Scope{}, false
}

func forbidden(w http.ResponseWriter) {
	response.Error(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to perform this action")
}

func CreateAppointment(repository *bookings.Repository, notificationWorker *jobs.Worker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		user, scope, ok := appointmentScope(w, r)
		if !ok {
			return
		}
		if user.Role == users.RoleTechnician {
			forbidden(w)
			return
		}
		var input bookings.AppointmentInput
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		if user.Role == users.RoleCustomer {
			input.CustomerID = scope.CustomerID
			input.TechnicianID = ""
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
		_, scope, ok := appointmentScope(w, r)
		if !ok {
			return
		}
		values := r.URL.Query()
		filter := bookings.AppointmentFilter{
			View:       strings.ToLower(strings.TrimSpace(values.Get("view"))),
			Assignment: strings.ToLower(strings.TrimSpace(values.Get("assignment"))),
		}
		values.Del("view")
		values.Del("assignment")
		listRequest := r.Clone(r.Context())
		listRequest.URL.RawQuery = values.Encode()
		query, ok := parseListQuery(w, listRequest, map[string]bool{"search": true, "status": true}, []string{"booking_date", "created_at", "status"}, allBookingStatuses...)
		if !ok {
			return
		}
		if r.URL.Query().Get("order") == "" {
			query.Order = "asc"
		}
		items, page, err := repository.ListAppointmentsPage(r.Context(), organizationID, query, filter, scope)
		if errors.Is(err, bookings.ErrAppointmentFilterInvalid) {
			response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid list query parameters")
			return
		}
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
		_, scope, ok := appointmentScope(w, r)
		if !ok {
			return
		}
		appointmentID := r.PathValue("id")
		if !bookings.ValidID(appointmentID) {
			appointmentNotFound(w)
			return
		}
		appointment, err := repository.FindAppointmentByID(r.Context(), organizationID, appointmentID, scope)
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

// UpdateAppointment lets administrators edit everything and customers reschedule their own
// appointments (customer and technician cannot be changed). Technicians use the status endpoint.
func UpdateAppointment(repository *bookings.Repository, notificationWorker *jobs.Worker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		user, scope, ok := appointmentScope(w, r)
		if !ok {
			return
		}
		if user.Role == users.RoleTechnician {
			forbidden(w)
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
		previous, err := repository.FindAppointmentByID(r.Context(), organizationID, appointmentID, scope)
		if errors.Is(err, bookings.ErrNotFound) {
			appointmentNotFound(w)
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load appointment")
			return
		}
		if user.Role == users.RoleCustomer {
			if input.Status != "" || (previous.Status != bookings.StatusBooked && previous.Status != bookings.StatusAssigned) {
				forbidden(w)
				return
			}
			input.CustomerID = previous.CustomerID
			input.TechnicianID = previous.TechnicianID
		}
		if err := input.Validate(); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_APPOINTMENT", err.Error())
			return
		}
		appointment, err := repository.UpdateAppointment(r.Context(), organizationID, appointmentID, input, scope)
		if handleAppointmentError(w, err) {
			return
		}
		notifyAppointmentChange(notificationWorker, r, organizationID, previous, appointment)
		response.JSON(w, http.StatusOK, struct {
			Appointment bookings.Appointment `json:"appointment"`
		}{Appointment: appointment})
	}
}

// ChangeAppointmentStatus applies a status transition. Technicians may only start or complete
// their own appointments, customers may only cancel theirs, and NO_SHOW is administrator-only.
func ChangeAppointmentStatus(repository *bookings.Repository, notificationWorker *jobs.Worker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		user, scope, ok := appointmentScope(w, r)
		if !ok {
			return
		}
		appointmentID := r.PathValue("id")
		if !bookings.ValidID(appointmentID) {
			appointmentNotFound(w)
			return
		}
		var input bookings.StatusInput
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		status := strings.ToUpper(strings.TrimSpace(input.Status))
		if status == "" || status == bookings.StatusBooked {
			response.Error(w, http.StatusBadRequest, "INVALID_APPOINTMENT", bookings.ErrBookingStatusInvalid.Error())
			return
		}
		if !roleMaySetStatus(user.Role, status) {
			forbidden(w)
			return
		}
		previous, err := repository.FindAppointmentByID(r.Context(), organizationID, appointmentID, scope)
		if errors.Is(err, bookings.ErrNotFound) {
			appointmentNotFound(w)
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load appointment")
			return
		}
		appointment, err := repository.ChangeAppointmentStatus(r.Context(), organizationID, appointmentID, status, scope)
		if handleAppointmentError(w, err) {
			return
		}
		notifyAppointmentChange(notificationWorker, r, organizationID, previous, appointment)
		response.JSON(w, http.StatusOK, struct {
			Appointment bookings.Appointment `json:"appointment"`
		}{Appointment: appointment})
	}
}

func roleMaySetStatus(role, status string) bool {
	switch role {
	case users.RoleAdmin:
		return true
	case users.RoleTechnician:
		return status == bookings.StatusInProgress || status == bookings.StatusCompleted
	case users.RoleCustomer:
		return status == bookings.StatusCancelled
	}
	return false
}

func CancelAppointment(repository *bookings.Repository, notificationWorker *jobs.Worker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		user, scope, ok := appointmentScope(w, r)
		if !ok {
			return
		}
		if user.Role == users.RoleTechnician {
			forbidden(w)
			return
		}
		appointmentID := r.PathValue("id")
		if !bookings.ValidID(appointmentID) {
			appointmentNotFound(w)
			return
		}
		if err := repository.CancelAppointment(r.Context(), organizationID, appointmentID, scope); handleAppointmentError(w, err) {
			return
		}
		enqueueNotification(notificationWorker, r, organizationID, "appointment.cancelled", "Appointment cancelled", "An appointment was cancelled.")
		w.WriteHeader(http.StatusNoContent)
	}
}

func notifyAppointmentChange(worker *jobs.Worker, r *http.Request, organizationID string, before, after bookings.Appointment) {
	switch {
	case after.Status == bookings.StatusCompleted && before.Status != bookings.StatusCompleted:
		enqueueNotification(worker, r, organizationID, "appointment.completed", "Appointment completed", "An appointment was completed.")
	case after.Status == bookings.StatusCancelled && before.Status != bookings.StatusCancelled:
		enqueueNotification(worker, r, organizationID, "appointment.cancelled", "Appointment cancelled", "An appointment was cancelled.")
	case after.TechnicianID != "" && after.TechnicianID != before.TechnicianID:
		enqueueNotification(worker, r, organizationID, "appointment.assigned", "Technician assigned", "A technician was assigned to an appointment.")
	case after.AppointmentDate != before.AppointmentDate || after.StartTime != before.StartTime || after.EndTime != before.EndTime:
		enqueueNotification(worker, r, organizationID, "appointment.rescheduled", "Appointment rescheduled", "An appointment was rescheduled.")
	}
}

var allBookingStatuses = []string{
	bookings.StatusBooked, bookings.StatusAssigned, bookings.StatusInProgress,
	bookings.StatusCompleted, bookings.StatusCancelled, bookings.StatusNoShow,
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
	case errors.Is(err, bookings.ErrInvalidStatusTransition), errors.Is(err, bookings.ErrTechnicianRequired), errors.Is(err, bookings.ErrNoShowTooEarly), errors.Is(err, bookings.ErrBookingStatusInvalid):
		response.Error(w, http.StatusConflict, "INVALID_STATUS_TRANSITION", err.Error())
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to save appointment")
	}
	return true
}

func appointmentNotFound(w http.ResponseWriter) {
	response.Error(w, http.StatusNotFound, "APPOINTMENT_NOT_FOUND", "Appointment not found")
}
