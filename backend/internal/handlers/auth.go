package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/serveflow/serveflow/backend/internal/auth"
	"github.com/serveflow/serveflow/backend/internal/jobs"
	"github.com/serveflow/serveflow/backend/internal/organizations"
	"github.com/serveflow/serveflow/backend/internal/response"
	"github.com/serveflow/serveflow/backend/internal/users"
)

type credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func Register(service *auth.Service, notificationWorkers ...*jobs.Worker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request credentials
		if err := decodeJSON(w, r, &request); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}

		user, err := service.Register(r.Context(), request.Name, request.Email, request.Password)
		switch {
		case errors.Is(err, auth.ErrInvalidInput):
			response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		case errors.Is(err, auth.ErrEmailTaken):
			response.Error(w, http.StatusConflict, "EMAIL_TAKEN", err.Error())
		case err != nil:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to register this account")
		default:
			if len(notificationWorkers) > 0 && notificationWorkers[0] != nil {
				notificationWorkers[0].Enqueue(jobs.NotificationJob{
					OrganizationID: user.OrganizationID,
					UserID:         user.ID,
					Type:           "account.welcome",
					Title:          "Welcome to ServeFlow",
					Message:        "Your ServeFlow account and workspace are ready.",
				})
			}
			response.JSON(w, http.StatusCreated, struct {
				Message string     `json:"message"`
				User    users.User `json:"user"`
			}{Message: "registration successful", User: user})
		}
	}
}

func Login(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request credentials
		if err := decodeJSON(w, r, &request); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}

		user, token, err := service.Login(r.Context(), request.Email, request.Password)
		switch {
		case errors.Is(err, auth.ErrInvalidInput):
			response.Error(w, http.StatusBadRequest, "INVALID_INPUT", "Email and password are required")
		case errors.Is(err, auth.ErrInvalidLogin):
			response.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error())
		case err != nil:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to log in")
		default:
			response.JSON(w, http.StatusOK, struct {
				Message string     `json:"message"`
				Token   string     `json:"token"`
				User    users.User `json:"user"`
			}{Message: "login successful", Token: token, User: user})
		}
	}
}

func Me(userRepository *users.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required")
			return
		}

		user, err := userRepository.FindByID(r.Context(), userID)
		if errors.Is(err, users.ErrNotFound) {
			response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "The authenticated user no longer exists")
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load your account")
			return
		}
		response.JSON(w, http.StatusOK, user)
	}
}

func Organization(organizationRepository *organizations.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := auth.OrganizationIDFromContext(r.Context())
		if !ok {
			response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required")
			return
		}

		organization, err := organizationRepository.FindByID(r.Context(), organizationID)
		if errors.Is(err, organizations.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "ORGANIZATION_NOT_FOUND", "Your workspace could not be found")
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load your workspace")
			return
		}
		response.JSON(w, http.StatusOK, organization)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}
