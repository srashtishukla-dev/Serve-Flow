package handlers

import (
	"net/http"

	"github.com/serveflow/serveflow/backend/internal/auth"
	"github.com/serveflow/serveflow/backend/internal/jobs"
)

func enqueueNotification(worker *jobs.Worker, request *http.Request, organizationID, notificationType, title, message string) {
	if worker == nil {
		return
	}
	userID, ok := auth.UserIDFromContext(request.Context())
	if !ok {
		return
	}
	worker.Enqueue(jobs.NotificationJob{
		OrganizationID: organizationID,
		UserID:         userID,
		Type:           notificationType,
		Title:          title,
		Message:        message,
	})
}
