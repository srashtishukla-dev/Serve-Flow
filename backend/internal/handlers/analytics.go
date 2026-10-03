package handlers

import (
	"net/http"

	"github.com/serveflow/serveflow/backend/internal/analytics"
	"github.com/serveflow/serveflow/backend/internal/response"
)

func AnalyticsSummary(repository *analytics.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		summary, err := repository.Summary(r.Context(), organizationID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load analytics")
			return
		}
		response.JSON(w, http.StatusOK, summary)
	}
}
