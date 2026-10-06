package handlers

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/response"
	"github.com/serveflow/serveflow/backend/internal/users"
)

type loginInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CreateLinkedLogin lets an administrator create a login for an existing customer or
// technician record in their own organization.
func CreateLinkedLogin(repository *users.Repository, role string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		recordID := r.PathValue("id")
		if !bookings.ValidID(recordID) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Record not found")
			return
		}
		var input loginInput
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		parsed, err := mail.ParseAddress(strings.TrimSpace(input.Email))
		if err != nil {
			response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "A valid email is required")
			return
		}
		if len(input.Password) < 8 || len(input.Password) > 72 {
			response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Password must be between 8 and 72 characters")
			return
		}
		if strings.TrimSpace(input.Name) == "" {
			response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Name is required")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to create login")
			return
		}
		user, err := repository.CreateLinked(r.Context(), organizationID, role, recordID, strings.TrimSpace(input.Name), strings.ToLower(parsed.Address), string(hash))
		switch {
		case errors.Is(err, users.ErrLinkInvalid):
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Record not found")
		case errors.Is(err, users.ErrEmailExists):
			response.Error(w, http.StatusConflict, "EMAIL_TAKEN", "An account with this email already exists")
		case errors.Is(err, users.ErrLinkTaken):
			response.Error(w, http.StatusConflict, "LOGIN_EXISTS", "This record already has a login")
		case err != nil:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to create login")
		default:
			response.JSON(w, http.StatusCreated, struct {
				User users.User `json:"user"`
			}{User: user})
		}
	}
}
