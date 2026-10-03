package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/serveflow/serveflow/backend/internal/response"
)

func Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	response.JSON(w, http.StatusOK, struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}{Status: "ok", Service: "serveflow-api"})
}

type Pinger interface {
	Ping(context.Context) error
}

func HealthWithDependencies(database, cache Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			response.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}

		databaseStatus := dependencyStatus(r.Context(), database)
		cacheStatus := dependencyStatus(r.Context(), cache)
		status := "ok"
		if databaseStatus == "unavailable" {
			status = "degraded"
		}
		response.JSON(w, http.StatusOK, struct {
			Status       string `json:"status"`
			Service      string `json:"service"`
			Dependencies struct {
				Database string `json:"database"`
				Cache    string `json:"cache"`
			} `json:"dependencies"`
		}{
			Status:  status,
			Service: "serveflow-api",
			Dependencies: struct {
				Database string `json:"database"`
				Cache    string `json:"cache"`
			}{Database: databaseStatus, Cache: cacheStatus},
		})
	}
}

func dependencyStatus(parent context.Context, pinger Pinger) string {
	if pinger == nil {
		return "disabled"
	}
	ctx, cancel := context.WithTimeout(parent, 300*time.Millisecond)
	defer cancel()
	if err := pinger.Ping(ctx); err != nil {
		return "unavailable"
	}
	return "available"
}
