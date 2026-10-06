package handlers

import (
	"net/http"

	"github.com/serveflow/serveflow/backend/internal/pagination"
	"github.com/serveflow/serveflow/backend/internal/response"
)

func parseListQuery(w http.ResponseWriter, r *http.Request, filters map[string]bool, sortable []string, statuses ...string) (pagination.Query, bool) {
	query, err := pagination.Parse(r.URL.Query(), filters, sortable)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid list query parameters")
		return pagination.Query{}, false
	}
	if query.Status != "" {
		for _, status := range statuses {
			if query.Status == status {
				return query, true
			}
		}
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid status filter")
		return pagination.Query{}, false
	}
	return query, true
}
