package handlers

import (
	"errors"
	"net/http"

	"github.com/serveflow/serveflow/backend/internal/pagination"
	"github.com/serveflow/serveflow/backend/internal/response"
	"github.com/serveflow/serveflow/backend/internal/technicians"
)

func CreateTechnician(repository *technicians.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}

		var input technicians.Input
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		if err := input.Validate(); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_TECHNICIAN", err.Error())
			return
		}

		item, err := repository.Create(r.Context(), organizationID, input)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to create technician")
			return
		}
		response.JSON(w, http.StatusCreated, struct {
			Technician technicians.Technician `json:"technician"`
		}{Technician: item})
	}
}

func ListTechnicians(repository *technicians.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		query, ok := parseListQuery(w, r, map[string]bool{"search": true, "status": true}, []string{"created_at", "updated_at", "name", "status"}, technicians.StatusActive, technicians.StatusInactive)
		if !ok {
			return
		}
		items, page, err := repository.ListPage(r.Context(), organizationID, query)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load technicians")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Technicians []technicians.Technician `json:"technicians"`
			Pagination  pagination.Metadata      `json:"pagination"`
		}{Technicians: items, Pagination: page})
	}
}

func GetTechnician(repository *technicians.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		technicianID := r.PathValue("id")
		if !technicians.ValidID(technicianID) {
			technicianNotFound(w)
			return
		}
		item, err := repository.FindByID(r.Context(), organizationID, technicianID)
		if errors.Is(err, technicians.ErrNotFound) {
			technicianNotFound(w)
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load technician")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Technician technicians.Technician `json:"technician"`
		}{Technician: item})
	}
}

func UpdateTechnician(repository *technicians.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		technicianID := r.PathValue("id")
		if !technicians.ValidID(technicianID) {
			technicianNotFound(w)
			return
		}
		var input technicians.Input
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		if err := input.Validate(); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_TECHNICIAN", err.Error())
			return
		}

		item, err := repository.Update(r.Context(), organizationID, technicianID, input)
		if errors.Is(err, technicians.ErrNotFound) {
			technicianNotFound(w)
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update technician")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Technician technicians.Technician `json:"technician"`
		}{Technician: item})
	}
}

func DeactivateTechnician(repository *technicians.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		technicianID := r.PathValue("id")
		if !technicians.ValidID(technicianID) {
			technicianNotFound(w)
			return
		}
		if err := repository.Deactivate(r.Context(), organizationID, technicianID); errors.Is(err, technicians.ErrNotFound) {
			technicianNotFound(w)
			return
		} else if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to deactivate technician")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func technicianNotFound(w http.ResponseWriter) {
	response.Error(w, http.StatusNotFound, "TECHNICIAN_NOT_FOUND", "Technician not found")
}
