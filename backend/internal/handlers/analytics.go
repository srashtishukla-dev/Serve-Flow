package handlers

import (
	"errors"
	"net/http"
	"strings"

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

func AnalyticsActivity(repository *analytics.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		query := r.URL.Query()
		for key, values := range query {
			if (key != "range" && key != "organization_id") || len(values) != 1 {
				response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Only one range value is supported")
				return
			}
		}
		dateRange := strings.TrimSpace(query.Get("range"))
		if dateRange == "" {
			dateRange = analytics.RangeLast30Days
		}
		activity, err := repository.Activity(r.Context(), organizationID, dateRange)
		if errors.Is(err, analytics.ErrInvalidRange) {
			response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load analytics activity")
			return
		}
		response.JSON(w, http.StatusOK, activity)
	}
}

func AdminDashboardSummary(repository *analytics.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		summary, err := repository.DashboardSummary(r.Context(), organizationID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load dashboard summary")
			return
		}
		response.JSON(w, http.StatusOK, map[string]any{"summary": summary})
	}
}
