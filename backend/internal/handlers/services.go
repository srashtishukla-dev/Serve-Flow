package handlers

import (
	"errors"
	"net/http"

	"github.com/serveflow/serveflow/backend/internal/auth"
	"github.com/serveflow/serveflow/backend/internal/response"
	"github.com/serveflow/serveflow/backend/internal/services"
)

func CreateService(repository *services.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}

		var input services.Input
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		price, err := input.Validate()
		if err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_SERVICE", err.Error())
			return
		}

		item, err := repository.Create(r.Context(), organizationID, input, price)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to create service")
			return
		}
		response.JSON(w, http.StatusCreated, struct {
			Service services.Service `json:"service"`
		}{Service: item})
	}
}

func ListServices(repository *services.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}

		items, err := repository.List(r.Context(), organizationID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load services")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Services []services.Service `json:"services"`
		}{Services: items})
	}
}

func GetService(repository *services.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		serviceID := r.PathValue("id")
		if !services.ValidID(serviceID) {
			serviceNotFound(w)
			return
		}

		item, err := repository.FindByID(r.Context(), organizationID, serviceID)
		if errors.Is(err, services.ErrNotFound) {
			serviceNotFound(w)
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load service")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Service services.Service `json:"service"`
		}{Service: item})
	}
}

func UpdateService(repository *services.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		serviceID := r.PathValue("id")
		if !services.ValidID(serviceID) {
			serviceNotFound(w)
			return
		}

		var input services.Input
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		price, err := input.Validate()
		if err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_SERVICE", err.Error())
			return
		}

		item, err := repository.Update(r.Context(), organizationID, serviceID, input, price)
		if errors.Is(err, services.ErrNotFound) {
			serviceNotFound(w)
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update service")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Service services.Service `json:"service"`
		}{Service: item})
	}
}

func DeleteService(repository *services.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		serviceID := r.PathValue("id")
		if !services.ValidID(serviceID) {
			serviceNotFound(w)
			return
		}

		if err := repository.Delete(r.Context(), organizationID, serviceID); errors.Is(err, services.ErrNotFound) {
			serviceNotFound(w)
			return
		} else if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to delete service")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func authenticatedOrganization(w http.ResponseWriter, r *http.Request) (string, bool) {
	organizationID, ok := auth.OrganizationIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required")
		return "", false
	}
	return organizationID, true
}

func serviceNotFound(w http.ResponseWriter) {
	response.Error(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "Service not found")
}
