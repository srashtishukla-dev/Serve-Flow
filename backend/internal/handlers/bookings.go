package handlers

import (
	"errors"
	"net/http"

	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/jobs"
	"github.com/serveflow/serveflow/backend/internal/pagination"
	"github.com/serveflow/serveflow/backend/internal/response"
)

func CreateBooking(repository *bookings.Repository, notificationWorker *jobs.Worker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		var input bookings.Input
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		if err := input.Validate(); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_BOOKING", err.Error())
			return
		}

		booking, err := repository.Create(r.Context(), organizationID, input)
		if handleBookingError(w, err) {
			return
		}
		enqueueNotification(notificationWorker, r, organizationID, "appointment.created", "Appointment scheduled", "A new appointment was scheduled.")
		response.JSON(w, http.StatusCreated, struct {
			Booking bookings.Booking `json:"booking"`
		}{Booking: booking})
	}
}

func ListBookings(repository *bookings.Repository) http.HandlerFunc {
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
		items, page, err := repository.ListPage(r.Context(), organizationID, query)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load bookings")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Bookings   []bookings.Booking  `json:"bookings"`
			Pagination pagination.Metadata `json:"pagination"`
		}{Bookings: items, Pagination: page})
	}
}

func GetBooking(repository *bookings.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		bookingID := r.PathValue("id")
		if !bookings.ValidID(bookingID) {
			bookingNotFound(w)
			return
		}
		booking, err := repository.FindByID(r.Context(), organizationID, bookingID)
		if errors.Is(err, bookings.ErrNotFound) {
			bookingNotFound(w)
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load booking")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Booking bookings.Booking `json:"booking"`
		}{Booking: booking})
	}
}

func UpdateBooking(repository *bookings.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		bookingID := r.PathValue("id")
		if !bookings.ValidID(bookingID) {
			bookingNotFound(w)
			return
		}
		var input bookings.UpdateInput
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		if err := input.Validate(); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_BOOKING", err.Error())
			return
		}
		booking, err := repository.Update(r.Context(), organizationID, bookingID, input)
		if handleBookingError(w, err) {
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Booking bookings.Booking `json:"booking"`
		}{Booking: booking})
	}
}

func DeleteBooking(repository *bookings.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		bookingID := r.PathValue("id")
		if !bookings.ValidID(bookingID) {
			bookingNotFound(w)
			return
		}
		if err := repository.Cancel(r.Context(), organizationID, bookingID); errors.Is(err, bookings.ErrNotFound) {
			bookingNotFound(w)
			return
		} else if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to cancel booking")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleBookingError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, bookings.ErrRelatedNotFound):
		response.Error(w, http.StatusBadRequest, "INVALID_CUSTOMER_OR_SERVICE", "Customer or service was not found in this organization")
	case errors.Is(err, bookings.ErrConflict):
		response.Error(w, http.StatusConflict, "BOOKING_CONFLICT", "The selected time overlaps another active booking")
	case errors.Is(err, bookings.ErrNotFound):
		bookingNotFound(w)
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to save booking")
	}
	return true
}

func bookingNotFound(w http.ResponseWriter) {
	response.Error(w, http.StatusNotFound, "BOOKING_NOT_FOUND", "Booking not found")
}
