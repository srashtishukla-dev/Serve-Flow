package handlers

import (
	"errors"
	"net/http"

	"github.com/serveflow/serveflow/backend/internal/invoices"
	"github.com/serveflow/serveflow/backend/internal/jobs"
	"github.com/serveflow/serveflow/backend/internal/pagination"
	"github.com/serveflow/serveflow/backend/internal/response"
	"github.com/serveflow/serveflow/backend/internal/users"
)

func CreateInvoice(repository *invoices.Repository, notificationWorker *jobs.Worker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		var input invoices.Input
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		totals, err := input.Validate()
		if err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_INVOICE", err.Error())
			return
		}
		item, err := repository.Create(r.Context(), organizationID, input, totals)
		if handleInvoiceError(w, err) {
			return
		}
		enqueueNotification(notificationWorker, r, organizationID, "invoice.created", "Invoice created", "A new invoice was created.")
		response.JSON(w, http.StatusCreated, struct {
			Invoice invoices.Invoice `json:"invoice"`
		}{Invoice: item})
	}
}

func ListInvoices(repository *invoices.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		query, ok := parseListQuery(w, r, map[string]bool{"search": true, "status": true}, []string{"issue_date", "due_date", "total", "status", "created_at"}, invoices.StatusDraft, invoices.StatusIssued, invoices.StatusPartiallyPaid, invoices.StatusPaid, invoices.StatusCancelled)
		if !ok {
			return
		}
		customerID, ok := invoiceCustomerScope(w, r)
		if !ok {
			return
		}
		var items []invoices.Invoice
		var page pagination.Metadata
		var err error
		if customerID == "" {
			items, page, err = repository.ListPage(r.Context(), organizationID, query)
		} else {
			items, page, err = repository.ListPageForCustomer(r.Context(), organizationID, customerID, query)
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load invoices")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Invoices   []invoices.Invoice  `json:"invoices"`
			Pagination pagination.Metadata `json:"pagination"`
		}{Invoices: items, Pagination: page})
	}
}

func GetInvoice(repository *invoices.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		invoiceID := r.PathValue("id")
		if !invoices.ValidID(invoiceID) {
			invoiceNotFound(w)
			return
		}
		customerID, ok := invoiceCustomerScope(w, r)
		if !ok {
			return
		}
		var details invoices.Details
		var err error
		if customerID == "" {
			details, err = repository.Get(r.Context(), organizationID, invoiceID)
		} else {
			details, err = repository.GetForCustomer(r.Context(), organizationID, invoiceID, customerID)
		}
		if errors.Is(err, invoices.ErrNotFound) {
			invoiceNotFound(w)
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load invoice")
			return
		}
		response.JSON(w, http.StatusOK, details)
	}
}

func UpdateInvoice(repository *invoices.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		invoiceID := r.PathValue("id")
		if !invoices.ValidID(invoiceID) {
			invoiceNotFound(w)
			return
		}
		var update invoices.UpdateInput
		if err := decodeJSON(w, r, &update); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		input := invoices.Input(update)
		totals, err := input.Validate()
		if err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_INVOICE", err.Error())
			return
		}
		item, err := repository.Update(r.Context(), organizationID, invoiceID, input, totals)
		if handleInvoiceError(w, err) {
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Invoice invoices.Invoice `json:"invoice"`
		}{Invoice: item})
	}
}

func CancelInvoice(repository *invoices.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		invoiceID := r.PathValue("id")
		if !invoices.ValidID(invoiceID) {
			invoiceNotFound(w)
			return
		}
		if err := repository.Cancel(r.Context(), organizationID, invoiceID); handleInvoiceError(w, err) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func RecordPayment(repository *invoices.Repository, notificationWorker *jobs.Worker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		invoiceID := r.PathValue("id")
		if !invoices.ValidID(invoiceID) {
			invoiceNotFound(w)
			return
		}
		var input invoices.PaymentInput
		if err := decodeJSON(w, r, &input); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
			return
		}
		amountCents, err := input.Validate()
		if err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_PAYMENT", err.Error())
			return
		}
		payment, invoice, err := repository.RecordPayment(r.Context(), organizationID, invoiceID, input, amountCents)
		if handleInvoiceError(w, err) {
			return
		}
		enqueueNotification(notificationWorker, r, organizationID, "payment.recorded", "Payment recorded", "A payment was recorded for an invoice.")
		response.JSON(w, http.StatusCreated, struct {
			Payment invoices.Payment `json:"payment"`
			Invoice invoices.Invoice `json:"invoice"`
		}{Payment: payment, Invoice: invoice})
	}
}

func handleInvoiceError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, invoices.ErrNotFound):
		invoiceNotFound(w)
	case errors.Is(err, invoices.ErrRelatedRecordNotFound):
		response.Error(w, http.StatusBadRequest, "INVALID_RELATED_RECORD", "Customer or appointment was not found in this organization")
	case errors.Is(err, invoices.ErrOverpayment):
		response.Error(w, http.StatusConflict, "PAYMENT_EXCEEDS_BALANCE", err.Error())
	case errors.Is(err, invoices.ErrConflict):
		response.Error(w, http.StatusConflict, "INVOICE_CONFLICT", err.Error())
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to save invoice")
	}
	return true
}

func invoiceNotFound(w http.ResponseWriter) {
	response.Error(w, http.StatusNotFound, "INVOICE_NOT_FOUND", "Invoice not found")
}

func invoiceCustomerScope(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, ok := users.FromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required")
		return "", false
	}
	if user.Role == users.RoleAdmin {
		return "", true
	}
	if user.Role == users.RoleCustomer && user.CustomerID != "" {
		return user.CustomerID, true
	}
	response.Error(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to view this invoice")
	return "", false
}
