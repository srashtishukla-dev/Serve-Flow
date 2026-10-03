package handlers

import (
	"errors"
	"net/http"

	"github.com/serveflow/serveflow/backend/internal/customers"
	"github.com/serveflow/serveflow/backend/internal/pagination"
	"github.com/serveflow/serveflow/backend/internal/response"
)

func CreateCustomer(repository *customers.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}

		var input customers.Input
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		if err := input.Validate(); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_CUSTOMER", err.Error())
			return
		}

		customer, err := repository.Create(r.Context(), organizationID, input)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to create customer")
			return
		}
		response.JSON(w, http.StatusCreated, struct {
			Customer customers.Customer `json:"customer"`
		}{Customer: customer})
	}
}

func ListCustomers(repository *customers.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}

		query, ok := parseListQuery(w, r, map[string]bool{"search": true}, []string{"created_at", "updated_at", "name"})
		if !ok {
			return
		}
		items, page, err := repository.ListPage(r.Context(), organizationID, query)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load customers")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Customers  []customers.Customer `json:"customers"`
			Pagination pagination.Metadata  `json:"pagination"`
		}{Customers: items, Pagination: page})
	}
}

func GetCustomer(repository *customers.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		customerID := r.PathValue("id")
		if !customers.ValidID(customerID) {
			customerNotFound(w)
			return
		}

		customer, err := repository.FindByID(r.Context(), organizationID, customerID)
		if errors.Is(err, customers.ErrNotFound) {
			customerNotFound(w)
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load customer")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Customer customers.Customer `json:"customer"`
		}{Customer: customer})
	}
}

func UpdateCustomer(repository *customers.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		customerID := r.PathValue("id")
		if !customers.ValidID(customerID) {
			customerNotFound(w)
			return
		}

		var input customers.Input
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		if err := input.Validate(); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_CUSTOMER", err.Error())
			return
		}

		customer, err := repository.Update(r.Context(), organizationID, customerID, input)
		if errors.Is(err, customers.ErrNotFound) {
			customerNotFound(w)
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update customer")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Customer customers.Customer `json:"customer"`
		}{Customer: customer})
	}
}

func DeleteCustomer(repository *customers.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		customerID := r.PathValue("id")
		if !customers.ValidID(customerID) {
			customerNotFound(w)
			return
		}

		if err := repository.Delete(r.Context(), organizationID, customerID); errors.Is(err, customers.ErrNotFound) {
			customerNotFound(w)
			return
		} else if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to delete customer")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func customerNotFound(w http.ResponseWriter) {
	response.Error(w, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "Customer not found")
}
