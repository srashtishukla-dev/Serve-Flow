package handlers

import (
	"errors"
	"net/http"

	"github.com/serveflow/serveflow/backend/internal/auth"
	"github.com/serveflow/serveflow/backend/internal/notifications"
	"github.com/serveflow/serveflow/backend/internal/pagination"
	"github.com/serveflow/serveflow/backend/internal/response"
)

func ListNotifications(repository *notifications.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required")
			return
		}

		query, ok := parseListQuery(w, r, map[string]bool{"search": true, "is_read": true}, []string{"created_at"})
		if !ok {
			return
		}
		items, page, err := repository.ListPage(r.Context(), organizationID, userID, query)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load notifications")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Notifications []notifications.Notification `json:"notifications"`
			Pagination    pagination.Metadata          `json:"pagination"`
		}{Notifications: items, Pagination: page})
	}
}

func MarkNotificationRead(repository *notifications.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := authenticatedOrganization(w, r)
		if !ok {
			return
		}
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required")
			return
		}
		notificationID := r.PathValue("id")
		if !notifications.ValidID(notificationID) {
			notificationNotFound(w)
			return
		}

		item, err := repository.MarkRead(r.Context(), organizationID, userID, notificationID)
		if errors.Is(err, notifications.ErrNotFound) {
			notificationNotFound(w)
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to update notification")
			return
		}
		response.JSON(w, http.StatusOK, struct {
			Notification notifications.Notification `json:"notification"`
		}{Notification: item})
	}
}

func notificationNotFound(w http.ResponseWriter) {
	response.Error(w, http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "Notification not found")
}
